package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/domain"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

type stubBeliefSystemStore struct {
	lastCreateBeliefArgs statedb.CreateBeliefSystemParams
	createdBelief        statedb.BeliefSystem
	createdFacts         []statedb.CreateFactParams
	factions             map[[16]byte]statedb.Faction
	cultures             map[[16]byte]statedb.Culture
	createBeliefErr      error
	createFactErr        error
	getFactionErr        error
	getCultureErr        error
}

func (s *stubBeliefSystemStore) CreateBeliefSystem(_ context.Context, arg statedb.CreateBeliefSystemParams) (statedb.BeliefSystem, error) {
	if s.createBeliefErr != nil {
		return statedb.BeliefSystem{}, s.createBeliefErr
	}
	s.lastCreateBeliefArgs = arg
	return s.createdBelief, nil
}

func (s *stubBeliefSystemStore) CreateFact(_ context.Context, arg statedb.CreateFactParams) (statedb.WorldFact, error) {
	if s.createFactErr != nil {
		return statedb.WorldFact{}, s.createFactErr
	}
	s.createdFacts = append(s.createdFacts, arg)
	return statedb.WorldFact{}, nil
}

func (s *stubBeliefSystemStore) GetFactionByID(_ context.Context, id pgtype.UUID) (statedb.Faction, error) {
	if s.getFactionErr != nil {
		return statedb.Faction{}, s.getFactionErr
	}
	faction, ok := s.factions[id.Bytes]
	if !ok {
		return statedb.Faction{}, errors.New("faction not found")
	}
	return faction, nil
}

func (s *stubBeliefSystemStore) GetCultureByID(_ context.Context, id pgtype.UUID) (statedb.Culture, error) {
	if s.getCultureErr != nil {
		return statedb.Culture{}, s.getCultureErr
	}
	culture, ok := s.cultures[id.Bytes]
	if !ok {
		return statedb.Culture{}, errors.New("culture not found")
	}
	return culture, nil
}

func TestRegisterCreateBeliefSystem(t *testing.T) {
	reg := NewRegistry()
	beliefStore := &stubBeliefSystemStore{}
	memStore := &stubMemoryStore{}
	embedder := &stubEmbedder{vector: []float32{0.1}}

	if err := RegisterCreateBeliefSystem(reg, beliefStore, memStore, embedder); err != nil {
		t.Fatalf("register create_belief_system: %v", err)
	}

	tools := reg.Tools()
	if len(tools) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(tools))
	}
	if tools[0].Name != createBeliefSystemToolName {
		t.Fatalf("tool name = %q, want %q", tools[0].Name, createBeliefSystemToolName)
	}
}

func TestCreateBeliefSystemHandleSuccess(t *testing.T) {
	campaignID := uuid.New()
	beliefID := uuid.New()
	factionID := uuid.New()
	cultureID := uuid.New()

	beliefStore := &stubBeliefSystemStore{
		createdBelief: statedb.BeliefSystem{
			ID:         uuidToPgtype(beliefID),
			CampaignID: uuidToPgtype(campaignID),
			Name:       "Way of the Dawn",
		},
		factions: map[[16]byte]statedb.Faction{
			uuidToPgtype(factionID).Bytes: {ID: uuidToPgtype(factionID), CampaignID: uuidToPgtype(campaignID)},
		},
		cultures: map[[16]byte]statedb.Culture{
			uuidToPgtype(cultureID).Bytes: {ID: uuidToPgtype(cultureID), CampaignID: uuidToPgtype(campaignID)},
		},
	}
	memStore := &stubMemoryStore{}
	embedder := &stubEmbedder{vector: []float32{0.2, 0.3}}
	h := NewCreateBeliefSystemHandler(beliefStore, memStore, embedder)

	got, err := h.Handle(context.Background(), map[string]any{
		"campaign_id":           campaignID.String(),
		"name":                  "Way of the Dawn",
		"description":           "A creed of renewal through sacrifice.",
		"deities_or_principles": []any{"Solar Trinity", "Cycle of Renewal"},
		"practices":             []any{"Dawn fast", "Equinox pilgrimage"},
		"institutions":          []any{"Temple of First Light"},
		"moral_framework": map[string]any{
			"virtues": []any{"discipline", "charity"},
		},
		"taboos": []any{"break an oath"},
		"followers": map[string]any{
			"faction_ids": []any{factionID.String()},
			"culture_ids": []any{cultureID.String()},
		},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if beliefStore.lastCreateBeliefArgs.Name != "Way of the Dawn" {
		t.Fatalf("CreateBeliefSystem name = %q, want Way of the Dawn", beliefStore.lastCreateBeliefArgs.Name)
	}
	if len(beliefStore.createdFacts) == 0 {
		t.Fatal("expected world facts to be created")
	}
	if memStore.lastArgs.MemoryType != string(domain.MemoryTypeWorldFact) {
		t.Fatalf("CreateMemory memory_type = %q, want %q", memStore.lastArgs.MemoryType, domain.MemoryTypeWorldFact)
	}
	if embedder.lastInput == "" {
		t.Fatal("expected embedder input to be populated")
	}
	if got["id"] != beliefID.String() {
		t.Fatalf("result id = %v, want %s", got["id"], beliefID.String())
	}
}

func TestCreateBeliefSystemValidationAndErrors(t *testing.T) {
	campaignID := uuid.New()
	baseArgs := map[string]any{
		"campaign_id":           campaignID.String(),
		"name":                  "Faith",
		"description":           "desc",
		"deities_or_principles": []any{"principle"},
		"practices":             []any{"practice"},
		"institutions":          []any{"institution"},
		"moral_framework": map[string]any{
			"virtues": []any{"mercy"},
		},
		"taboos": []any{"taboo"},
		"followers": map[string]any{
			"faction_ids": []any{},
			"culture_ids": []any{},
		},
	}

	t.Run("missing required field", func(t *testing.T) {
		h := NewCreateBeliefSystemHandler(&stubBeliefSystemStore{}, &stubMemoryStore{}, &stubEmbedder{vector: []float32{0.1}})
		args := copyArgs(baseArgs)
		delete(args, "description")
		_, err := h.Handle(context.Background(), args)
		if err == nil || !strings.Contains(err.Error(), "description is required") {
			t.Fatalf("error = %v, want description-required message", err)
		}
	})

	t.Run("invalid deities type", func(t *testing.T) {
		h := NewCreateBeliefSystemHandler(&stubBeliefSystemStore{}, &stubMemoryStore{}, &stubEmbedder{vector: []float32{0.1}})
		args := copyArgs(baseArgs)
		args["deities_or_principles"] = "bad"
		_, err := h.Handle(context.Background(), args)
		if err == nil || !strings.Contains(err.Error(), "deities_or_principles must be an array") {
			t.Fatalf("error = %v, want array-type message", err)
		}
	})

	t.Run("follower faction campaign mismatch", func(t *testing.T) {
		factionID := uuid.New()
		otherCampaignID := uuid.New()
		h := NewCreateBeliefSystemHandler(
			&stubBeliefSystemStore{
				factions: map[[16]byte]statedb.Faction{
					uuidToPgtype(factionID).Bytes: {ID: uuidToPgtype(factionID), CampaignID: uuidToPgtype(otherCampaignID)},
				},
				cultures: map[[16]byte]statedb.Culture{},
			},
			&stubMemoryStore{},
			&stubEmbedder{vector: []float32{0.1}},
		)
		args := copyArgs(baseArgs)
		args["followers"] = map[string]any{
			"faction_ids": []any{factionID.String()},
			"culture_ids": []any{},
		}
		_, err := h.Handle(context.Background(), args)
		if err == nil || !strings.Contains(err.Error(), "must belong to campaign_id") {
			t.Fatalf("error = %v, want campaign validation message", err)
		}
	})

	t.Run("create fact error", func(t *testing.T) {
		factionID := uuid.New()
		h := NewCreateBeliefSystemHandler(
			&stubBeliefSystemStore{
				createdBelief: statedb.BeliefSystem{
					ID:         uuidToPgtype(uuid.New()),
					CampaignID: uuidToPgtype(campaignID),
					Name:       "Faith",
				},
				factions: map[[16]byte]statedb.Faction{
					uuidToPgtype(factionID).Bytes: {ID: uuidToPgtype(factionID), CampaignID: uuidToPgtype(campaignID)},
				},
				cultures:      map[[16]byte]statedb.Culture{},
				createFactErr: errors.New("insert fact failed"),
			},
			&stubMemoryStore{},
			&stubEmbedder{vector: []float32{0.1}},
		)
		args := copyArgs(baseArgs)
		args["followers"] = map[string]any{
			"faction_ids": []any{factionID.String()},
			"culture_ids": []any{},
		}
		_, err := h.Handle(context.Background(), args)
		if err == nil || !strings.Contains(err.Error(), "create belief system world_fact") {
			t.Fatalf("error = %v, want world_fact context", err)
		}
	})
}

func TestCreateBeliefSystemStoresMemoryMetadata(t *testing.T) {
	campaignID := uuid.New()
	beliefID := uuid.New()

	beliefStore := &stubBeliefSystemStore{
		createdBelief: statedb.BeliefSystem{
			ID:         uuidToPgtype(beliefID),
			CampaignID: uuidToPgtype(campaignID),
			Name:       "Faith",
		},
		factions: map[[16]byte]statedb.Faction{},
		cultures: map[[16]byte]statedb.Culture{},
	}
	memStore := &stubMemoryStore{}
	h := NewCreateBeliefSystemHandler(beliefStore, memStore, &stubEmbedder{vector: []float32{0.1}})

	_, err := h.Handle(context.Background(), map[string]any{
		"campaign_id":           campaignID.String(),
		"name":                  "Faith",
		"description":           "desc",
		"deities_or_principles": []any{"principle"},
		"practices":             []any{"practice"},
		"institutions":          []any{"institution"},
		"moral_framework": map[string]any{
			"virtues": []any{"mercy"},
		},
		"taboos": []any{"taboo"},
		"followers": map[string]any{
			"faction_ids": []any{},
			"culture_ids": []any{},
		},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var metadata map[string]any
	if err := json.Unmarshal(memStore.lastArgs.Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if metadata["belief_system_id"] != beliefID.String() {
		t.Fatalf("metadata.belief_system_id = %v, want %s", metadata["belief_system_id"], beliefID.String())
	}
}

var _ BeliefSystemStore = (*stubBeliefSystemStore)(nil)
