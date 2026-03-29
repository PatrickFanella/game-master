package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
)

type stubCreateNPCStore struct {
	playerCharacters map[uuid.UUID]*domain.PlayerCharacter
	validLocations   map[uuid.UUID]uuid.UUID
	npcsByCampaign   map[uuid.UUID][]domain.NPC

	lastCreateParams CreateNPCParams
	createdNPC       *domain.NPC

	playerErr         error
	locationExistsErr error
	createErr         error
	listErr           error
}

func (s *stubCreateNPCStore) GetPlayerCharacterByID(_ context.Context, playerCharacterID uuid.UUID) (*domain.PlayerCharacter, error) {
	if s.playerErr != nil {
		return nil, s.playerErr
	}
	pc, ok := s.playerCharacters[playerCharacterID]
	if !ok {
		return nil, nil
	}
	copy := *pc
	return &copy, nil
}

func (s *stubCreateNPCStore) LocationExistsInCampaign(_ context.Context, locationID, campaignID uuid.UUID) (bool, error) {
	if s.locationExistsErr != nil {
		return false, s.locationExistsErr
	}
	locationCampaignID, ok := s.validLocations[locationID]
	if !ok {
		return false, nil
	}
	return locationCampaignID == campaignID, nil
}

func (s *stubCreateNPCStore) CreateNPC(_ context.Context, params CreateNPCParams) (*domain.NPC, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	s.lastCreateParams = params
	if s.createdNPC != nil {
		copy := *s.createdNPC
		return &copy, nil
	}

	locationID := uuid.Nil
	if params.LocationID != nil {
		locationID = *params.LocationID
	}
	var locationPtr *uuid.UUID
	if locationID != uuid.Nil {
		locationCopy := locationID
		locationPtr = &locationCopy
	}

	npc := &domain.NPC{
		ID:          uuid.New(),
		CampaignID:  params.CampaignID,
		Name:        params.Name,
		Description: params.Description,
		Personality: params.Personality,
		Disposition: params.Disposition,
		LocationID:  locationPtr,
		FactionID:   params.FactionID,
		Alive:       true,
		Stats:       params.Stats,
		Properties:  params.Properties,
	}
	return npc, nil
}

func (s *stubCreateNPCStore) ListNPCsByCampaign(_ context.Context, campaignID uuid.UUID) ([]domain.NPC, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	npcs := s.npcsByCampaign[campaignID]
	out := make([]domain.NPC, len(npcs))
	copy(out, npcs)
	return out, nil
}

func TestRegisterCreateNPC(t *testing.T) {
	reg := NewRegistry()
	store := &stubCreateNPCStore{}
	memStore := &stubMemoryStore{}
	embedder := &stubEmbedder{vector: []float32{0.1}}

	if err := RegisterCreateNPC(reg, store, memStore, embedder); err != nil {
		t.Fatalf("register create_npc: %v", err)
	}

	registered := reg.List()
	if len(registered) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(registered))
	}
	if registered[0].Name != createNPCToolName {
		t.Fatalf("tool name = %q, want %q", registered[0].Name, createNPCToolName)
	}

	required, ok := registered[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", registered[0].Parameters["required"])
	}
	requiredSet := make(map[string]struct{}, len(required))
	for _, field := range required {
		requiredSet[field] = struct{}{}
	}
	for _, field := range []string{"name", "description", "personality"} {
		if _, exists := requiredSet[field]; !exists {
			t.Fatalf("schema missing required field %q", field)
		}
	}
}

func TestCreateNPCHandleSuccessDefaultsAndEmbedding(t *testing.T) {
	campaignID := uuid.New()
	playerID := uuid.New()
	locationID := uuid.New()
	factionID := uuid.New()
	npcID := uuid.New()

	playerLocationCopy := locationID
	store := &stubCreateNPCStore{
		playerCharacters: map[uuid.UUID]*domain.PlayerCharacter{
			playerID: {
				ID:                playerID,
				CampaignID:        campaignID,
				CurrentLocationID: &playerLocationCopy,
			},
		},
		validLocations: map[uuid.UUID]uuid.UUID{
			locationID: campaignID,
		},
		createdNPC: &domain.NPC{
			ID:          npcID,
			CampaignID:  campaignID,
			Name:        "Captain Maris",
			Description: "Harbor master of the eastern docks.",
			Personality: "Disciplined and pragmatic",
			Disposition: 0,
			LocationID:  &playerLocationCopy,
			FactionID:   &factionID,
			Alive:       true,
			Stats:       json.RawMessage(`{"strength":8}`),
			Properties:  json.RawMessage(`{"role":"harbor master"}`),
		},
		npcsByCampaign: map[uuid.UUID][]domain.NPC{
			campaignID: {
				{
					ID:         npcID,
					CampaignID: campaignID,
					Name:       "Captain Maris",
					LocationID: &playerLocationCopy,
				},
			},
		},
	}
	memStore := &stubMemoryStore{}
	embedder := &stubEmbedder{vector: []float32{0.2, 0.4}}
	h := NewCreateNPCHandler(store, memStore, embedder)

	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)
	got, err := h.Handle(ctx, map[string]any{
		"name":        "Captain Maris",
		"description": "Harbor master of the eastern docks.",
		"personality": "Disciplined and pragmatic",
		"faction_id":  factionID.String(),
		"stats": map[string]any{
			"strength": 8,
		},
		"properties": map[string]any{
			"role": "harbor master",
		},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if store.lastCreateParams.Disposition != 0 {
		t.Fatalf("disposition = %d, want default 0", store.lastCreateParams.Disposition)
	}
	if store.lastCreateParams.LocationID == nil || *store.lastCreateParams.LocationID != locationID {
		t.Fatalf("location_id = %v, want player current location %s", store.lastCreateParams.LocationID, locationID)
	}

	var stats map[string]any
	if err := json.Unmarshal(store.lastCreateParams.Stats, &stats); err != nil {
		t.Fatalf("unmarshal stats: %v", err)
	}
	if stats["strength"] != float64(8) {
		t.Fatalf("stats.strength = %#v, want 8", stats["strength"])
	}

	if got.Data["id"] != npcID.String() {
		t.Fatalf("result id = %v, want %s", got.Data["id"], npcID.String())
	}
	if got.Data["disposition"] != 0 {
		t.Fatalf("result disposition = %v, want 0", got.Data["disposition"])
	}
	if got.Data["location_id"] != locationID.String() {
		t.Fatalf("result location_id = %v, want %s", got.Data["location_id"], locationID.String())
	}
	if memStore.lastParams.MemoryType != string(domain.MemoryTypeWorldFact) {
		t.Fatalf("memory type = %q, want %q", memStore.lastParams.MemoryType, domain.MemoryTypeWorldFact)
	}
	if embedder.lastInput == "" || !strings.Contains(embedder.lastInput, "Role: harbor master") {
		t.Fatalf("embedder input = %q, want role summary", embedder.lastInput)
	}
}

func TestCreateNPCHandleDuplicateNameAtLocationRejected(t *testing.T) {
	campaignID := uuid.New()
	playerID := uuid.New()
	locationID := uuid.New()
	createdID := uuid.New()

	playerLocationCopy := locationID
	store := &stubCreateNPCStore{
		playerCharacters: map[uuid.UUID]*domain.PlayerCharacter{
			playerID: {
				ID:                playerID,
				CampaignID:        campaignID,
				CurrentLocationID: &playerLocationCopy,
			},
		},
		validLocations: map[uuid.UUID]uuid.UUID{
			locationID: campaignID,
		},
		createdNPC: &domain.NPC{
			ID:          createdID,
			CampaignID:  campaignID,
			Name:        "Rook",
			Description: "A scout",
			Personality: "Wary",
			Disposition: 5,
			LocationID:  &playerLocationCopy,
			Alive:       true,
			Stats:       json.RawMessage(`{}`),
			Properties:  json.RawMessage(`{}`),
		},
		npcsByCampaign: map[uuid.UUID][]domain.NPC{
			campaignID: {
				{
					ID:         uuid.New(),
					CampaignID: campaignID,
					Name:       " rook ",
					LocationID: &playerLocationCopy,
				},
				{
					ID:         createdID,
					CampaignID: campaignID,
					Name:       "Rook",
					LocationID: &playerLocationCopy,
				},
			},
		},
	}

	h := NewCreateNPCHandler(store, nil, nil)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)
	_, err := h.Handle(ctx, map[string]any{
		"name":        "Rook",
		"description": "A scout",
		"personality": "Wary",
	})
	if err == nil {
		t.Fatal("expected duplicate detection error")
	}
	if !strings.Contains(err.Error(), "already exists at location") {
		t.Fatalf("error = %v, want duplicate-name message", err)
	}
}

func TestCreateNPCValidationAndContextErrors(t *testing.T) {
	playerID := uuid.New()
	campaignID := uuid.New()
	locationID := uuid.New()
	playerLocationCopy := locationID

	baseStore := &stubCreateNPCStore{
		playerCharacters: map[uuid.UUID]*domain.PlayerCharacter{
			playerID: {
				ID:                playerID,
				CampaignID:        campaignID,
				CurrentLocationID: &playerLocationCopy,
			},
		},
		validLocations: map[uuid.UUID]uuid.UUID{
			locationID: campaignID,
		},
		npcsByCampaign: map[uuid.UUID][]domain.NPC{
			campaignID: {},
		},
	}

	t.Run("missing player context", func(t *testing.T) {
		h := NewCreateNPCHandler(baseStore, nil, nil)
		_, err := h.Handle(context.Background(), map[string]any{
			"name":        "Ari",
			"description": "desc",
			"personality": "kind",
		})
		if err == nil || !strings.Contains(err.Error(), "requires current player character id in context") {
			t.Fatalf("error = %v, want context error", err)
		}
	})

	t.Run("invalid optional object type", func(t *testing.T) {
		h := NewCreateNPCHandler(baseStore, nil, nil)
		ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)
		_, err := h.Handle(ctx, map[string]any{
			"name":        "Ari",
			"description": "desc",
			"personality": "kind",
			"stats":       []any{"bad"},
		})
		if err == nil || !strings.Contains(err.Error(), "stats must be an object") {
			t.Fatalf("error = %v, want stats type error", err)
		}
	})

	t.Run("location not in campaign rejected", func(t *testing.T) {
		h := NewCreateNPCHandler(baseStore, nil, nil)
		ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)
		_, err := h.Handle(ctx, map[string]any{
			"name":        "Ari",
			"description": "desc",
			"personality": "kind",
			"location_id": uuid.New().String(),
		})
		if err == nil || !strings.Contains(err.Error(), "location_id does not reference an existing location in player campaign") {
			t.Fatalf("error = %v, want campaign location validation error", err)
		}
	})

	t.Run("store errors wrapped", func(t *testing.T) {
		store := &stubCreateNPCStore{
			playerCharacters: map[uuid.UUID]*domain.PlayerCharacter{
				playerID: {
					ID:                playerID,
					CampaignID:        campaignID,
					CurrentLocationID: &playerLocationCopy,
				},
			},
			validLocations: map[uuid.UUID]uuid.UUID{
				locationID: campaignID,
			},
			createErr: errors.New("db unavailable"),
		}
		h := NewCreateNPCHandler(store, nil, nil)
		ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)
		_, err := h.Handle(ctx, map[string]any{
			"name":        "Ari",
			"description": "desc",
			"personality": "kind",
		})
		if err == nil || !strings.Contains(err.Error(), "create npc: db unavailable") {
			t.Fatalf("error = %v, want wrapped create error", err)
		}
	})
}

var _ CreateNPCStore = (*stubCreateNPCStore)(nil)
