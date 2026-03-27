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

type stubEconomicSystemStore struct {
	lastCreateEconomicArgs statedb.CreateEconomicSystemParams
	createdEconomic        statedb.EconomicSystem
	createdFacts           []statedb.CreateFactParams
	factions               map[[16]byte]statedb.Faction
	locations              map[[16]byte]statedb.Location
	createEconomicErr      error
	createFactErr          error
	getFactionErr          error
	getLocationErr         error
}

func (s *stubEconomicSystemStore) CreateEconomicSystem(_ context.Context, arg statedb.CreateEconomicSystemParams) (statedb.EconomicSystem, error) {
	if s.createEconomicErr != nil {
		return statedb.EconomicSystem{}, s.createEconomicErr
	}
	s.lastCreateEconomicArgs = arg
	return s.createdEconomic, nil
}

func (s *stubEconomicSystemStore) CreateFact(_ context.Context, arg statedb.CreateFactParams) (statedb.WorldFact, error) {
	if s.createFactErr != nil {
		return statedb.WorldFact{}, s.createFactErr
	}
	s.createdFacts = append(s.createdFacts, arg)
	return statedb.WorldFact{}, nil
}

func (s *stubEconomicSystemStore) GetFactionByID(_ context.Context, id pgtype.UUID) (statedb.Faction, error) {
	if s.getFactionErr != nil {
		return statedb.Faction{}, s.getFactionErr
	}
	faction, ok := s.factions[id.Bytes]
	if !ok {
		return statedb.Faction{}, errors.New("faction not found")
	}
	return faction, nil
}

func (s *stubEconomicSystemStore) GetLocationByID(_ context.Context, id pgtype.UUID) (statedb.Location, error) {
	if s.getLocationErr != nil {
		return statedb.Location{}, s.getLocationErr
	}
	location, ok := s.locations[id.Bytes]
	if !ok {
		return statedb.Location{}, errors.New("location not found")
	}
	return location, nil
}

func TestRegisterCreateEconomicSystem(t *testing.T) {
	reg := NewRegistry()
	economicStore := &stubEconomicSystemStore{}
	memStore := &stubMemoryStore{}
	embedder := &stubEmbedder{vector: []float32{0.1}}

	if err := RegisterCreateEconomicSystem(reg, economicStore, memStore, embedder); err != nil {
		t.Fatalf("register create_economic_system: %v", err)
	}

	tools := reg.List()
	if len(tools) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(tools))
	}
	if tools[0].Name != createEconomicSystemToolName {
		t.Fatalf("tool name = %q, want %q", tools[0].Name, createEconomicSystemToolName)
	}

	required, ok := tools[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", tools[0].Parameters["required"])
	}
	if len(required) == 0 {
		t.Fatalf("schema required fields is empty")
	}

	requiredSet := make(map[string]struct{}, len(required))
	for _, field := range required {
		requiredSet[field] = struct{}{}
	}
	for _, field := range []string{"campaign_id", "name", "currency", "primary_resources", "trade_routes", "class_structure", "economic_type", "scope"} {
		if _, ok := requiredSet[field]; !ok {
			t.Fatalf("schema missing required field %q", field)
		}
	}
}

func TestCreateEconomicSystemHandleSuccess(t *testing.T) {
	campaignID := uuid.New()
	economicID := uuid.New()
	factionID := uuid.New()
	fromLocationID := uuid.New()
	toLocationID := uuid.New()

	store := &stubEconomicSystemStore{
		createdEconomic: statedb.EconomicSystem{
			ID:         dbutil.ToPgtype(economicID),
			CampaignID: dbutil.ToPgtype(campaignID),
			Name:       "Guild Ledger",
		},
		factions: map[[16]byte]statedb.Faction{
			dbutil.ToPgtype(factionID).Bytes: {ID: dbutil.ToPgtype(factionID), CampaignID: dbutil.ToPgtype(campaignID)},
		},
		locations: map[[16]byte]statedb.Location{
			dbutil.ToPgtype(fromLocationID).Bytes: {ID: dbutil.ToPgtype(fromLocationID), CampaignID: dbutil.ToPgtype(campaignID)},
			dbutil.ToPgtype(toLocationID).Bytes:   {ID: dbutil.ToPgtype(toLocationID), CampaignID: dbutil.ToPgtype(campaignID)},
		},
	}
	memStore := &stubMemoryStore{}
	embedder := &stubEmbedder{vector: []float32{0.2, 0.3}}
	h := NewCreateEconomicSystemHandler(store, memStore, embedder)

	got, err := h.Handle(context.Background(), map[string]any{
		"campaign_id": campaignID.String(),
		"name":        "Guild Ledger",
		"currency": map[string]any{
			"name":          "Crown",
			"denominations": []any{"copper", "silver", "gold"},
		},
		"primary_resources": []any{"iron", "grain"},
		"trade_routes": []any{
			map[string]any{
				"from_location_id": fromLocationID.String(),
				"to_location_id":   toLocationID.String(),
				"goods":            []any{"iron"},
			},
		},
		"class_structure": map[string]any{
			"tiers": []any{"peasants", "merchants", "nobility"},
		},
		"economic_type": "mercantile",
		"scope": map[string]any{
			"faction_ids":  []any{factionID.String()},
			"region_names": []any{"Northern Reach"},
		},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if store.lastCreateEconomicArgs.Name != "Guild Ledger" {
		t.Fatalf("CreateEconomicSystem name = %q, want Guild Ledger", store.lastCreateEconomicArgs.Name)
	}
	if len(store.createdFacts) == 0 {
		t.Fatal("expected world facts to be created")
	}
	if memStore.lastParams.MemoryType != string(domain.MemoryTypeWorldFact) {
		t.Fatalf("CreateMemory memory_type = %q, want %q", memStore.lastParams.MemoryType, domain.MemoryTypeWorldFact)
	}
	if embedder.lastInput == "" {
		t.Fatal("expected embedder input to be populated")
	}
	if got.Data["id"] != economicID.String() {
		t.Fatalf("result id = %v, want %s", got.Data["id"], economicID.String())
	}
}

func TestCreateEconomicSystemValidationAndErrors(t *testing.T) {
	campaignID := uuid.New()
	baseArgs := map[string]any{
		"campaign_id": campaignID.String(),
		"name":        "Market",
		"currency": map[string]any{
			"name":          "Mark",
			"denominations": []any{"mark"},
		},
		"primary_resources": []any{"timber"},
		"trade_routes":      []any{},
		"class_structure": map[string]any{
			"tiers": []any{"workers"},
		},
		"economic_type": "agrarian",
		"scope": map[string]any{
			"faction_ids":  []any{},
			"region_names": []any{},
		},
	}

	t.Run("missing required field", func(t *testing.T) {
		h := NewCreateEconomicSystemHandler(&stubEconomicSystemStore{}, &stubMemoryStore{}, &stubEmbedder{vector: []float32{0.1}})
		args := copyArgs(baseArgs)
		delete(args, "currency")
		_, err := h.Handle(context.Background(), args)
		if err == nil || !strings.Contains(err.Error(), "currency is required") {
			t.Fatalf("error = %v, want currency-required message", err)
		}
	})

	t.Run("invalid trade_routes type", func(t *testing.T) {
		h := NewCreateEconomicSystemHandler(&stubEconomicSystemStore{}, &stubMemoryStore{}, &stubEmbedder{vector: []float32{0.1}})
		args := copyArgs(baseArgs)
		args["trade_routes"] = "bad"
		_, err := h.Handle(context.Background(), args)
		if err == nil || !strings.Contains(err.Error(), "trade_routes must be an array") {
			t.Fatalf("error = %v, want array-type message", err)
		}
	})

	t.Run("missing required currency denominations", func(t *testing.T) {
		h := NewCreateEconomicSystemHandler(&stubEconomicSystemStore{}, &stubMemoryStore{}, &stubEmbedder{vector: []float32{0.1}})
		args := copyArgs(baseArgs)
		args["currency"] = map[string]any{
			"name": "Mark",
		}
		_, err := h.Handle(context.Background(), args)
		if err == nil || !strings.Contains(err.Error(), "currency.denominations is required") {
			t.Fatalf("error = %v, want required denominations message", err)
		}
	})

	t.Run("scope faction campaign mismatch", func(t *testing.T) {
		factionID := uuid.New()
		otherCampaignID := uuid.New()
		h := NewCreateEconomicSystemHandler(
			&stubEconomicSystemStore{
				factions: map[[16]byte]statedb.Faction{
					dbutil.ToPgtype(factionID).Bytes: {ID: dbutil.ToPgtype(factionID), CampaignID: dbutil.ToPgtype(otherCampaignID)},
				},
				locations: map[[16]byte]statedb.Location{},
			},
			&stubMemoryStore{},
			&stubEmbedder{vector: []float32{0.1}},
		)
		args := copyArgs(baseArgs)
		args["scope"] = map[string]any{
			"faction_ids":  []any{factionID.String()},
			"region_names": []any{},
		}
		_, err := h.Handle(context.Background(), args)
		if err == nil || !strings.Contains(err.Error(), "must belong to campaign_id") {
			t.Fatalf("error = %v, want campaign validation message", err)
		}
	})

	t.Run("create fact error", func(t *testing.T) {
		fromLocationID := uuid.New()
		toLocationID := uuid.New()
		h := NewCreateEconomicSystemHandler(
			&stubEconomicSystemStore{
				createdEconomic: statedb.EconomicSystem{
					ID:         dbutil.ToPgtype(uuid.New()),
					CampaignID: dbutil.ToPgtype(campaignID),
					Name:       "Market",
				},
				locations: map[[16]byte]statedb.Location{
					dbutil.ToPgtype(fromLocationID).Bytes: {ID: dbutil.ToPgtype(fromLocationID), CampaignID: dbutil.ToPgtype(campaignID)},
					dbutil.ToPgtype(toLocationID).Bytes:   {ID: dbutil.ToPgtype(toLocationID), CampaignID: dbutil.ToPgtype(campaignID)},
				},
				factions:      map[[16]byte]statedb.Faction{},
				createFactErr: errors.New("insert fact failed"),
			},
			&stubMemoryStore{},
			&stubEmbedder{vector: []float32{0.1}},
		)
		args := copyArgs(baseArgs)
		args["trade_routes"] = []any{
			map[string]any{
				"from_location_id": fromLocationID.String(),
				"to_location_id":   toLocationID.String(),
			},
		}
		_, err := h.Handle(context.Background(), args)
		if err == nil || !strings.Contains(err.Error(), "create economic system world_fact") {
			t.Fatalf("error = %v, want world_fact context", err)
		}
	})
}

func TestCreateEconomicSystemStoresMemoryMetadata(t *testing.T) {
	campaignID := uuid.New()
	economicID := uuid.New()
	fromLocationID := uuid.New()
	toLocationID := uuid.New()

	store := &stubEconomicSystemStore{
		createdEconomic: statedb.EconomicSystem{
			ID:         dbutil.ToPgtype(economicID),
			CampaignID: dbutil.ToPgtype(campaignID),
			Name:       "Market",
		},
		factions: map[[16]byte]statedb.Faction{},
		locations: map[[16]byte]statedb.Location{
			dbutil.ToPgtype(fromLocationID).Bytes: {ID: dbutil.ToPgtype(fromLocationID), CampaignID: dbutil.ToPgtype(campaignID)},
			dbutil.ToPgtype(toLocationID).Bytes:   {ID: dbutil.ToPgtype(toLocationID), CampaignID: dbutil.ToPgtype(campaignID)},
		},
	}
	memStore := &stubMemoryStore{}
	h := NewCreateEconomicSystemHandler(store, memStore, &stubEmbedder{vector: []float32{0.1}})

	_, err := h.Handle(context.Background(), map[string]any{
		"campaign_id": campaignID.String(),
		"name":        "Market",
		"currency": map[string]any{
			"name":          "Mark",
			"denominations": []any{"mark"},
		},
		"primary_resources": []any{"timber"},
		"trade_routes": []any{
			map[string]any{
				"from_location_id": fromLocationID.String(),
				"to_location_id":   toLocationID.String(),
			},
		},
		"class_structure": map[string]any{
			"tiers": []any{"workers"},
		},
		"economic_type": "agrarian",
		"scope": map[string]any{
			"faction_ids":  []any{},
			"region_names": []any{"East Vale"},
		},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var metadata map[string]any
	if err := json.Unmarshal(memStore.lastParams.Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if metadata["economic_system_id"] != economicID.String() {
		t.Fatalf("metadata.economic_system_id = %v, want %s", metadata["economic_system_id"], economicID.String())
	}
}

var _ EconomicSystemStore = (*stubEconomicSystemStore)(nil)
