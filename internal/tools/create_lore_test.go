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

type stubLoreStore struct {
	currentLocation         statedb.Location
	createdFact             statedb.WorldFact
	lastCreateFact          statedb.CreateFactParams
	createFactErr           error
	getLocationErr          error
	createRelationshipErr   error
	createRelationshipCalls []statedb.CreateRelationshipParams
}

func (s *stubLoreStore) CreateFact(_ context.Context, arg statedb.CreateFactParams) (statedb.WorldFact, error) {
	if s.createFactErr != nil {
		return statedb.WorldFact{}, s.createFactErr
	}
	s.lastCreateFact = arg
	if s.createdFact.ID.Valid {
		return s.createdFact, nil
	}
	return statedb.WorldFact{
		ID:         dbutil.ToPgtype(uuid.New()),
		CampaignID: arg.CampaignID,
		Fact:       arg.Fact,
		Category:   arg.Category,
		Source:     arg.Source,
	}, nil
}

func (s *stubLoreStore) CreateRelationship(_ context.Context, arg statedb.CreateRelationshipParams) (statedb.EntityRelationship, error) {
	if s.createRelationshipErr != nil {
		return statedb.EntityRelationship{}, s.createRelationshipErr
	}
	s.createRelationshipCalls = append(s.createRelationshipCalls, arg)
	return statedb.EntityRelationship{
		ID:               dbutil.ToPgtype(uuid.New()),
		CampaignID:       arg.CampaignID,
		SourceEntityType: arg.SourceEntityType,
		SourceEntityID:   arg.SourceEntityID,
		TargetEntityType: arg.TargetEntityType,
		TargetEntityID:   arg.TargetEntityID,
		RelationshipType: arg.RelationshipType,
		Description:      arg.Description,
	}, nil
}

func (s *stubLoreStore) GetLocationByID(_ context.Context, _ pgtype.UUID) (statedb.Location, error) {
	if s.getLocationErr != nil {
		return statedb.Location{}, s.getLocationErr
	}
	return s.currentLocation, nil
}

func (s *stubLoreStore) GetFactionByID(_ context.Context, id pgtype.UUID) (statedb.Faction, error) {
	return statedb.Faction{ID: id, CampaignID: s.currentLocation.CampaignID}, nil
}

func (s *stubLoreStore) GetNPCByID(_ context.Context, id pgtype.UUID) (statedb.Npc, error) {
	return statedb.Npc{ID: id, CampaignID: s.currentLocation.CampaignID}, nil
}

func (s *stubLoreStore) GetPlayerCharacterByID(_ context.Context, id pgtype.UUID) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{ID: id, CampaignID: s.currentLocation.CampaignID}, nil
}

func (s *stubLoreStore) GetItemByID(_ context.Context, id pgtype.UUID) (statedb.Item, error) {
	return statedb.Item{ID: id, CampaignID: s.currentLocation.CampaignID}, nil
}

func TestRegisterCreateLore(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterCreateLore(reg, &stubLoreStore{}, &stubMemoryStore{}, &stubEmbedder{vector: []float32{0.1}}); err != nil {
		t.Fatalf("register create_lore: %v", err)
	}

	tools := reg.List()
	if len(tools) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(tools))
	}
	if tools[0].Name != createLoreToolName {
		t.Fatalf("tool name = %q, want %q", tools[0].Name, createLoreToolName)
	}

	required, ok := tools[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", tools[0].Parameters["required"])
	}
	requiredSet := make(map[string]struct{}, len(required))
	for _, field := range required {
		requiredSet[field] = struct{}{}
	}
	for _, field := range []string{"content", "category"} {
		if _, exists := requiredSet[field]; !exists {
			t.Fatalf("schema missing required field %q", field)
		}
	}

	properties, ok := tools[0].Parameters["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties schema has unexpected type %T", tools[0].Parameters["properties"])
	}
	category, ok := properties["category"].(map[string]any)
	if !ok {
		t.Fatalf("category schema has unexpected type %T", properties["category"])
	}
	enumValues, ok := category["enum"].([]string)
	if !ok {
		t.Fatalf("category enum has unexpected type %T", category["enum"])
	}
	want := []string{"history", "legend", "cultural", "political", "magical", "religious"}
	for _, value := range want {
		found := false
		for _, got := range enumValues {
			if got == value {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("category enum missing %q", value)
		}
	}
}

func TestCreateLoreHandleSuccess(t *testing.T) {
	campaignID := uuid.New()
	locationID := uuid.New()
	factID := uuid.New()
	relatedNPCID := uuid.New()
	relatedLocationID := uuid.New()

	store := &stubLoreStore{
		currentLocation: statedb.Location{
			ID:         dbutil.ToPgtype(locationID),
			CampaignID: dbutil.ToPgtype(campaignID),
		},
		createdFact: statedb.WorldFact{
			ID:         dbutil.ToPgtype(factID),
			CampaignID: dbutil.ToPgtype(campaignID),
			Fact:       "The moon temple was raised by exiled stargazers.",
			Category:   "legend",
			Source:     loreSource,
		},
	}
	memStore := &stubMemoryStore{}
	embedder := &stubEmbedder{vector: []float32{0.1, 0.2}}

	h := NewCreateLoreHandler(store, memStore, embedder)
	result, err := h.Handle(WithCurrentLocationID(context.Background(), locationID), map[string]any{
		"content":  "The moon temple was raised by exiled stargazers.",
		"category": "legend",
		"related_entities": []any{
			map[string]any{"entity_type": "npc", "entity_id": relatedNPCID.String()},
			map[string]any{"entity_type": "location", "entity_id": relatedLocationID.String()},
		},
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}

	if store.lastCreateFact.Fact != "The moon temple was raised by exiled stargazers." {
		t.Fatalf("CreateFact.Fact = %q, want content", store.lastCreateFact.Fact)
	}
	if store.lastCreateFact.Category != "legend" {
		t.Fatalf("CreateFact.Category = %q, want legend", store.lastCreateFact.Category)
	}
	if store.lastCreateFact.Source != loreSource {
		t.Fatalf("CreateFact.Source = %q, want %q", store.lastCreateFact.Source, loreSource)
	}

	if len(store.createRelationshipCalls) != 2 {
		t.Fatalf("CreateRelationship call count = %d, want 2", len(store.createRelationshipCalls))
	}
	firstRelationship := store.createRelationshipCalls[0]
	if firstRelationship.SourceEntityType != loreRelationshipSourceEntityType {
		t.Fatalf("SourceEntityType = %q, want %q", firstRelationship.SourceEntityType, loreRelationshipSourceEntityType)
	}
	if dbutil.FromPgtype(firstRelationship.SourceEntityID) != factID {
		t.Fatalf("SourceEntityID = %s, want %s", dbutil.FromPgtype(firstRelationship.SourceEntityID), factID)
	}
	if firstRelationship.RelationshipType != loreRelationshipType {
		t.Fatalf("RelationshipType = %q, want %q", firstRelationship.RelationshipType, loreRelationshipType)
	}

	if result.Data["content"] != "The moon temple was raised by exiled stargazers." {
		t.Fatalf("result.Data[content] = %v, want lore content", result.Data["content"])
	}
	if result.Data["source"] != loreSource {
		t.Fatalf("result.Data[source] = %v, want %q", result.Data["source"], loreSource)
	}
	if result.Narrative != "The moon temple was raised by exiled stargazers." {
		t.Fatalf("Narrative = %q, want lore content", result.Narrative)
	}

	if !memStore.called {
		t.Fatal("expected CreateMemory to be called")
	}
	if memStore.lastParams.MemoryType != string(domain.MemoryTypeLore) {
		t.Fatalf("CreateMemory memory_type = %q, want %q", memStore.lastParams.MemoryType, domain.MemoryTypeLore)
	}
	if memStore.lastParams.CampaignID != campaignID {
		t.Fatal("CreateMemory CampaignID mismatch")
	}

	var metadata map[string]any
	if err := json.Unmarshal(memStore.lastParams.Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if metadata["category"] != "legend" {
		t.Fatalf("metadata[category] = %v, want legend", metadata["category"])
	}
	if metadata["source"] != loreSource {
		t.Fatalf("metadata[source] = %v, want %q", metadata["source"], loreSource)
	}
	if metadata["related_entity_count"] != float64(2) {
		t.Fatalf("metadata[related_entity_count] = %v, want 2", metadata["related_entity_count"])
	}
}

func TestCreateLoreHandleAcceptsAllCategories(t *testing.T) {
	for _, category := range []string{"history", "legend", "cultural", "political", "magical", "religious"} {
		t.Run(category, func(t *testing.T) {
			campaignID := uuid.New()
			locationID := uuid.New()
			store := &stubLoreStore{
				currentLocation: statedb.Location{
					ID:         dbutil.ToPgtype(locationID),
					CampaignID: dbutil.ToPgtype(campaignID),
				},
			}

			h := NewCreateLoreHandler(store, nil, nil)
			result, err := h.Handle(WithCurrentLocationID(context.Background(), locationID), map[string]any{
				"content":  "Lore entry for " + category,
				"category": category,
			})
			if err != nil {
				t.Fatalf("Handle returned error: %v", err)
			}
			if result.Data["category"] != category {
				t.Fatalf("result.Data[category] = %v, want %q", result.Data["category"], category)
			}
		})
	}
}

func TestCreateLoreHandleNoEmbedder(t *testing.T) {
	campaignID := uuid.New()
	locationID := uuid.New()
	store := &stubLoreStore{
		currentLocation: statedb.Location{
			ID:         dbutil.ToPgtype(locationID),
			CampaignID: dbutil.ToPgtype(campaignID),
		},
	}

	h := NewCreateLoreHandler(store, nil, nil)
	result, err := h.Handle(WithCurrentLocationID(context.Background(), locationID), map[string]any{
		"content":  "The river sings when storms approach.",
		"category": "magical",
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}

func TestCreateLoreValidationAndErrors(t *testing.T) {
	campaignID := uuid.New()
	locationID := uuid.New()
	baseStore := &stubLoreStore{
		currentLocation: statedb.Location{
			ID:         dbutil.ToPgtype(locationID),
			CampaignID: dbutil.ToPgtype(campaignID),
		},
	}

	t.Run("missing location context", func(t *testing.T) {
		h := NewCreateLoreHandler(baseStore, nil, nil)
		_, err := h.Handle(context.Background(), map[string]any{
			"content":  "Lore",
			"category": "history",
		})
		if err == nil {
			t.Fatal("expected error for missing location context")
		}
	})

	t.Run("missing content", func(t *testing.T) {
		h := NewCreateLoreHandler(baseStore, nil, nil)
		_, err := h.Handle(WithCurrentLocationID(context.Background(), locationID), map[string]any{"category": "history"})
		if err == nil {
			t.Fatal("expected error for missing content")
		}
	})

	t.Run("missing category", func(t *testing.T) {
		h := NewCreateLoreHandler(baseStore, nil, nil)
		_, err := h.Handle(WithCurrentLocationID(context.Background(), locationID), map[string]any{"content": "Lore"})
		if err == nil {
			t.Fatal("expected error for missing category")
		}
	})

	t.Run("invalid category", func(t *testing.T) {
		h := NewCreateLoreHandler(baseStore, nil, nil)
		_, err := h.Handle(WithCurrentLocationID(context.Background(), locationID), map[string]any{
			"content":  "Lore",
			"category": "science",
		})
		if err == nil {
			t.Fatal("expected error for invalid category")
		}
	})

	t.Run("get location error", func(t *testing.T) {
		store := &stubLoreStore{getLocationErr: errors.New("db error")}
		h := NewCreateLoreHandler(store, nil, nil)
		_, err := h.Handle(WithCurrentLocationID(context.Background(), locationID), map[string]any{
			"content":  "Lore",
			"category": "history",
		})
		if err == nil {
			t.Fatal("expected error when GetLocationByID fails")
		}
	})

	t.Run("create fact error", func(t *testing.T) {
		store := &stubLoreStore{
			currentLocation: statedb.Location{
				ID:         dbutil.ToPgtype(locationID),
				CampaignID: dbutil.ToPgtype(campaignID),
			},
			createFactErr: errors.New("db error"),
		}
		h := NewCreateLoreHandler(store, nil, nil)
		_, err := h.Handle(WithCurrentLocationID(context.Background(), locationID), map[string]any{
			"content":  "Lore",
			"category": "history",
		})
		if err == nil {
			t.Fatal("expected error when CreateFact fails")
		}
	})

	t.Run("create relationship error", func(t *testing.T) {
		store := &stubLoreStore{
			currentLocation: statedb.Location{
				ID:         dbutil.ToPgtype(locationID),
				CampaignID: dbutil.ToPgtype(campaignID),
			},
			createdFact: statedb.WorldFact{
				ID:         dbutil.ToPgtype(uuid.New()),
				CampaignID: dbutil.ToPgtype(campaignID),
				Source:     loreSource,
			},
			createRelationshipErr: errors.New("db error"),
		}
		h := NewCreateLoreHandler(store, nil, nil)
		_, err := h.Handle(WithCurrentLocationID(context.Background(), locationID), map[string]any{
			"content":  "Lore",
			"category": "history",
			"related_entities": []any{
				map[string]any{"entity_type": "npc", "entity_id": uuid.New().String()},
			},
		})
		if err == nil {
			t.Fatal("expected error when CreateRelationship fails")
		}
	})

	t.Run("embed error", func(t *testing.T) {
		store := &stubLoreStore{
			currentLocation: statedb.Location{
				ID:         dbutil.ToPgtype(locationID),
				CampaignID: dbutil.ToPgtype(campaignID),
			},
			createdFact: statedb.WorldFact{
				ID:         dbutil.ToPgtype(uuid.New()),
				CampaignID: dbutil.ToPgtype(campaignID),
				Source:     loreSource,
			},
		}
		h := NewCreateLoreHandler(store, &stubMemoryStore{}, &stubEmbedder{err: errors.New("embed error")})
		_, err := h.Handle(WithCurrentLocationID(context.Background(), locationID), map[string]any{
			"content":  "Lore",
			"category": "history",
		})
		if err == nil {
			t.Fatal("expected error when Embed fails")
		}
	})
}

func TestRegisterCreateLoreNilStore(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterCreateLore(reg, nil, nil, nil); err == nil {
		t.Fatal("expected error when registering with nil loreStore")
	}
}

func TestCreateLoreHandleNilHandler(t *testing.T) {
	var h *CreateLoreHandler
	_, err := h.Handle(context.Background(), map[string]any{
		"content":  "Lore",
		"category": "history",
	})
	if err == nil {
		t.Fatal("expected error for nil handler")
	}
}

func TestCreateLoreHandleNilStore(t *testing.T) {
	h := &CreateLoreHandler{}
	_, err := h.Handle(WithCurrentLocationID(context.Background(), uuid.New()), map[string]any{
		"content":  "Lore",
		"category": "history",
	})
	if err == nil {
		t.Fatal("expected error for nil loreStore")
	}
}


func TestCreateLoreHandleEmptyContent(t *testing.T) {
	campaignID := uuid.New()
	locationID := uuid.New()
	store := &stubLoreStore{
		currentLocation: statedb.Location{
			ID:         dbutil.ToPgtype(locationID),
			CampaignID: dbutil.ToPgtype(campaignID),
		},
	}
	h := NewCreateLoreHandler(store, nil, nil)
	_, err := h.Handle(WithCurrentLocationID(context.Background(), locationID), map[string]any{
		"content":  "",
		"category": "history",
	})
	if err == nil {
		t.Fatal("expected error for empty content")
	}
	if !strings.Contains(err.Error(), "content must be a non-empty string") {
		t.Fatalf("error = %q, want to contain %q", err.Error(), "content must be a non-empty string")
	}
}

func TestCreateLoreHandleEmptyRelatedEntities(t *testing.T) {
	campaignID := uuid.New()
	locationID := uuid.New()
	store := &stubLoreStore{
		currentLocation: statedb.Location{
			ID:         dbutil.ToPgtype(locationID),
			CampaignID: dbutil.ToPgtype(campaignID),
		},
	}
	h := NewCreateLoreHandler(store, nil, nil)
	result, err := h.Handle(WithCurrentLocationID(context.Background(), locationID), map[string]any{
		"content":          "The old forest remembers the war.",
		"category":         "history",
		"related_entities": []any{},
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if len(store.createRelationshipCalls) != 0 {
		t.Fatalf("CreateRelationship call count = %d, want 0", len(store.createRelationshipCalls))
	}
}

func TestCreateLoreHandleMemoryStoreError(t *testing.T) {
	campaignID := uuid.New()
	locationID := uuid.New()
	factID := uuid.New()
	store := &stubLoreStore{
		currentLocation: statedb.Location{
			ID:         dbutil.ToPgtype(locationID),
			CampaignID: dbutil.ToPgtype(campaignID),
		},
		createdFact: statedb.WorldFact{
			ID:         dbutil.ToPgtype(factID),
			CampaignID: dbutil.ToPgtype(campaignID),
			Source:     loreSource,
		},
	}
	memStore := &stubMemoryStore{err: errors.New("mem error")}
	embedder := &stubEmbedder{vector: []float32{0.1, 0.2}}
	h := NewCreateLoreHandler(store, memStore, embedder)
	_, err := h.Handle(WithCurrentLocationID(context.Background(), locationID), map[string]any{
		"content":  "Ancient pact sealed beneath the mountain.",
		"category": "history",
	})
	if err == nil {
		t.Fatal("expected error when memory store CreateMemory fails")
	}
}