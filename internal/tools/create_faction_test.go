package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

type stubFactionStore struct {
	currentLocation        statedb.Location
	factionsByID           map[[16]byte]statedb.Faction
	lastCreateFaction      statedb.CreateFactionParams
	createFactionCalls     int
	createFactionResult    statedb.Faction
	lastRelationships      []statedb.CreateFactionRelationshipParams
	relationshipResults    []statedb.FactionRelationship
	getLocationErr         error
	getFactionErr          error
	createFactionErr       error
	createRelationshipErr  error
}

func (s *stubFactionStore) CreateFaction(_ context.Context, arg statedb.CreateFactionParams) (statedb.Faction, error) {
	if s.createFactionErr != nil {
		return statedb.Faction{}, s.createFactionErr
	}
	s.createFactionCalls++
	s.lastCreateFaction = arg
	if s.createFactionResult.ID.Valid {
		return s.createFactionResult, nil
	}
	return statedb.Faction{
		ID:         dbutil.ToPgtype(uuid.New()),
		CampaignID: arg.CampaignID,
		Name:       arg.Name,
	}, nil
}

func (s *stubFactionStore) CreateFactionRelationship(_ context.Context, arg statedb.CreateFactionRelationshipParams) (statedb.FactionRelationship, error) {
	if s.createRelationshipErr != nil {
		return statedb.FactionRelationship{}, s.createRelationshipErr
	}
	s.lastRelationships = append(s.lastRelationships, arg)
	if len(s.relationshipResults) > 0 {
		out := s.relationshipResults[0]
		s.relationshipResults = s.relationshipResults[1:]
		return out, nil
	}
	return statedb.FactionRelationship{
		ID:               dbutil.ToPgtype(uuid.New()),
		FactionID:        arg.FactionID,
		RelatedFactionID: arg.RelatedFactionID,
		RelationshipType: arg.RelationshipType,
		Description:      arg.Description,
	}, nil
}

func (s *stubFactionStore) GetFactionByID(_ context.Context, id pgtype.UUID) (statedb.Faction, error) {
	if s.getFactionErr != nil {
		return statedb.Faction{}, s.getFactionErr
	}
	faction, ok := s.factionsByID[id.Bytes]
	if !ok {
		return statedb.Faction{}, errors.New("faction not found")
	}
	return faction, nil
}

func (s *stubFactionStore) GetLocationByID(_ context.Context, _ pgtype.UUID) (statedb.Location, error) {
	if s.getLocationErr != nil {
		return statedb.Location{}, s.getLocationErr
	}
	return s.currentLocation, nil
}

func TestRegisterCreateFaction(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterCreateFaction(reg, &stubFactionStore{}, &stubMemoryStore{}, &stubEmbedder{vector: []float32{0.1}}); err != nil {
		t.Fatalf("register create_faction: %v", err)
	}

	tools := reg.List()
	if len(tools) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(tools))
	}
	if tools[0].Name != createFactionToolName {
		t.Fatalf("tool name = %q, want %q", tools[0].Name, createFactionToolName)
	}

	required, ok := tools[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", tools[0].Parameters["required"])
	}
	requiredSet := make(map[string]struct{}, len(required))
	for _, field := range required {
		requiredSet[field] = struct{}{}
	}
	for _, field := range []string{"name", "description", "agenda", "territory", "properties", "relationships"} {
		if _, ok := requiredSet[field]; !ok {
			t.Fatalf("schema missing required field %q", field)
		}
	}
}

func TestCreateFactionHandleSuccessWithRelationships(t *testing.T) {
	campaignID := uuid.New()
	currentLocationID := uuid.New()
	newFactionID := uuid.New()
	relatedFactionID := uuid.New()
	relationshipID := uuid.New()

	store := &stubFactionStore{
		currentLocation: statedb.Location{
			ID:         dbutil.ToPgtype(currentLocationID),
			CampaignID: dbutil.ToPgtype(campaignID),
			Name:       "Current Place",
		},
		factionsByID: map[[16]byte]statedb.Faction{
			dbutil.ToPgtype(relatedFactionID).Bytes: {
				ID:         dbutil.ToPgtype(relatedFactionID),
				CampaignID: dbutil.ToPgtype(campaignID),
				Name:       "Existing Faction",
			},
		},
		createFactionResult: statedb.Faction{
			ID:         dbutil.ToPgtype(newFactionID),
			CampaignID: dbutil.ToPgtype(campaignID),
			Name:       "Iron Accord",
		},
		relationshipResults: []statedb.FactionRelationship{
			{
				ID:               dbutil.ToPgtype(relationshipID),
				FactionID:        dbutil.ToPgtype(newFactionID),
				RelatedFactionID: dbutil.ToPgtype(relatedFactionID),
				RelationshipType: "allied",
				Description:      pgtype.Text{String: "Mutual defense pact", Valid: true},
			},
		},
	}
	memStore := &stubMemoryStore{}
	embedder := &stubEmbedder{vector: []float32{0.2, 0.3}}
	h := NewCreateFactionHandler(store, memStore, embedder)

	ctx := WithCurrentLocationID(context.Background(), currentLocationID)
	got, err := h.Handle(ctx, map[string]any{
		"name":        "Iron Accord",
		"description": "A federation of mountain holds.",
		"agenda":      "Secure the passes and expand trade.",
		"territory":   "Northern peaks",
		"properties": map[string]any{
			"motto": "Steel and honor",
		},
		"relationships": []any{
			map[string]any{
				"faction_id":  relatedFactionID.String(),
				"type":        "Allied",
				"description": "Mutual defense pact",
			},
		},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if store.lastCreateFaction.Name != "Iron Accord" {
		t.Fatalf("CreateFaction name = %q, want Iron Accord", store.lastCreateFaction.Name)
	}
	if len(store.lastRelationships) != 1 {
		t.Fatalf("CreateFactionRelationship call count = %d, want 1", len(store.lastRelationships))
	}
	if store.lastRelationships[0].RelationshipType != "allied" {
		t.Fatalf("relationship type = %q, want allied", store.lastRelationships[0].RelationshipType)
	}
	if got.Data["id"] != newFactionID.String() {
		t.Fatalf("result id = %v, want %s", got.Data["id"], newFactionID.String())
	}
	if memStore.lastParams.MemoryType != string(domain.MemoryTypeWorldFact) {
		t.Fatalf("CreateMemory memory_type = %q, want %q", memStore.lastParams.MemoryType, domain.MemoryTypeWorldFact)
	}
	if embedder.lastInput == "" {
		t.Fatal("expected embedder input to be populated")
	}
}

func TestCreateFactionValidationAndErrors(t *testing.T) {
	campaignID := uuid.New()
	currentLocationID := uuid.New()
	relatedFactionID := uuid.New()
	otherCampaignID := uuid.New()

	baseStore := &stubFactionStore{
		currentLocation: statedb.Location{
			ID:         dbutil.ToPgtype(currentLocationID),
			CampaignID: dbutil.ToPgtype(campaignID),
		},
		factionsByID: map[[16]byte]statedb.Faction{
			dbutil.ToPgtype(relatedFactionID).Bytes: {
				ID:         dbutil.ToPgtype(relatedFactionID),
				CampaignID: dbutil.ToPgtype(campaignID),
			},
		},
	}
	baseArgs := map[string]any{
		"name":        "Test Faction",
		"description": "desc",
		"agenda":      "agenda",
		"territory":   "territory",
		"properties":  map[string]any{},
		"relationships": []any{
			map[string]any{
				"faction_id":  relatedFactionID.String(),
				"type":        "neutral",
				"description": "Uneasy truce",
			},
		},
	}

	t.Run("requires current location context", func(t *testing.T) {
		h := NewCreateFactionHandler(baseStore, nil, nil)
		_, err := h.Handle(context.Background(), copyArgs(baseArgs))
		if err == nil || !strings.Contains(err.Error(), "requires current location id in context") {
			t.Fatalf("error = %v, want context-required message", err)
		}
	})

	t.Run("invalid relationship type", func(t *testing.T) {
		h := NewCreateFactionHandler(baseStore, nil, nil)
		args := copyArgs(baseArgs)
		args["relationships"] = []any{
			map[string]any{
				"faction_id":  relatedFactionID.String(),
				"type":        "friend",
				"description": "invalid",
			},
		}
		_, err := h.Handle(WithCurrentLocationID(context.Background(), currentLocationID), args)
		if err == nil || !strings.Contains(err.Error(), "must be one of allied, hostile, neutral, vassal, rival, trade_partner") {
			t.Fatalf("error = %v, want relationship type validation", err)
		}
	})

	t.Run("referenced faction must belong to active campaign", func(t *testing.T) {
		store := &stubFactionStore{
			currentLocation: baseStore.currentLocation,
			factionsByID: map[[16]byte]statedb.Faction{
				dbutil.ToPgtype(relatedFactionID).Bytes: {
					ID:         dbutil.ToPgtype(relatedFactionID),
					CampaignID: dbutil.ToPgtype(otherCampaignID),
				},
			},
		}
		h := NewCreateFactionHandler(store, nil, nil)
		_, err := h.Handle(WithCurrentLocationID(context.Background(), currentLocationID), copyArgs(baseArgs))
		if err == nil || !strings.Contains(err.Error(), "must belong to active campaign") {
			t.Fatalf("error = %v, want campaign validation", err)
		}
		if store.createFactionCalls != 0 {
			t.Fatalf("CreateFaction calls = %d, want 0", store.createFactionCalls)
		}
	})

	t.Run("properties is required", func(t *testing.T) {
		h := NewCreateFactionHandler(baseStore, nil, nil)
		args := copyArgs(baseArgs)
		delete(args, "properties")
		_, err := h.Handle(WithCurrentLocationID(context.Background(), currentLocationID), args)
		if err == nil || !strings.Contains(err.Error(), "properties is required") {
			t.Fatalf("error = %v, want properties required error", err)
		}
	})
}

func TestCreateFactionStoresMemoryMetadata(t *testing.T) {
	campaignID := uuid.New()
	currentLocationID := uuid.New()
	newFactionID := uuid.New()

	store := &stubFactionStore{
		currentLocation: statedb.Location{
			ID:         dbutil.ToPgtype(currentLocationID),
			CampaignID: dbutil.ToPgtype(campaignID),
		},
		factionsByID: map[[16]byte]statedb.Faction{},
		createFactionResult: statedb.Faction{
			ID:         dbutil.ToPgtype(newFactionID),
			CampaignID: dbutil.ToPgtype(campaignID),
		},
	}
	memStore := &stubMemoryStore{}
	h := NewCreateFactionHandler(store, memStore, &stubEmbedder{vector: []float32{1.0}})

	_, err := h.Handle(WithCurrentLocationID(context.Background(), currentLocationID), map[string]any{
		"name":          "No Relations",
		"description":   "desc",
		"agenda":        "agenda",
		"territory":     "territory",
		"properties":    map[string]any{"x": "y"},
		"relationships": []any{},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var metadata map[string]any
	if err := json.Unmarshal(memStore.lastParams.Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if metadata["faction_id"] != newFactionID.String() {
		t.Fatalf("metadata.faction_id = %v, want %s", metadata["faction_id"], newFactionID.String())
	}
}

var _ FactionStore = (*stubFactionStore)(nil)
