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

type stubCultureStore struct {
	lastCreateCultureArgs statedb.CreateCultureParams
	createdCulture        statedb.Culture
	languages             map[[16]byte]statedb.Language
	beliefSystems         map[[16]byte]statedb.BeliefSystem
	factions              map[[16]byte]statedb.Faction
	createCultureErr      error
	getLanguageErr        error
	getBeliefSystemErr    error
	getFactionErr         error
}

func (s *stubCultureStore) CreateCulture(_ context.Context, arg statedb.CreateCultureParams) (statedb.Culture, error) {
	if s.createCultureErr != nil {
		return statedb.Culture{}, s.createCultureErr
	}
	s.lastCreateCultureArgs = arg
	return s.createdCulture, nil
}

func (s *stubCultureStore) GetLanguageByID(_ context.Context, id pgtype.UUID) (statedb.Language, error) {
	if s.getLanguageErr != nil {
		return statedb.Language{}, s.getLanguageErr
	}
	language, ok := s.languages[id.Bytes]
	if !ok {
		return statedb.Language{}, errors.New("language not found")
	}
	return language, nil
}

func (s *stubCultureStore) GetBeliefSystemByID(_ context.Context, id pgtype.UUID) (statedb.BeliefSystem, error) {
	if s.getBeliefSystemErr != nil {
		return statedb.BeliefSystem{}, s.getBeliefSystemErr
	}
	beliefSystem, ok := s.beliefSystems[id.Bytes]
	if !ok {
		return statedb.BeliefSystem{}, errors.New("belief system not found")
	}
	return beliefSystem, nil
}

func (s *stubCultureStore) GetFactionByID(_ context.Context, id pgtype.UUID) (statedb.Faction, error) {
	if s.getFactionErr != nil {
		return statedb.Faction{}, s.getFactionErr
	}
	faction, ok := s.factions[id.Bytes]
	if !ok {
		return statedb.Faction{}, errors.New("faction not found")
	}
	return faction, nil
}

func TestRegisterCreateCulture(t *testing.T) {
	reg := NewRegistry()
	cultureStore := &stubCultureStore{}
	memStore := &stubMemoryStore{}
	embedder := &stubEmbedder{vector: []float32{0.1}}

	if err := RegisterCreateCulture(reg, cultureStore, memStore, embedder); err != nil {
		t.Fatalf("register create_culture: %v", err)
	}

	tools := reg.List()
	if len(tools) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(tools))
	}
	if tools[0].Name != createCultureToolName {
		t.Fatalf("tool name = %q, want %q", tools[0].Name, createCultureToolName)
	}

	required, ok := tools[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", tools[0].Parameters["required"])
	}
	requiredSet := make(map[string]struct{}, len(required))
	for _, field := range required {
		requiredSet[field] = struct{}{}
	}
	for _, field := range []string{"name", "description", "values", "customs", "social_norms", "art_forms", "taboos", "greeting_customs", "language_id", "belief_system_id", "associated_factions"} {
		if _, ok := requiredSet[field]; !ok {
			t.Fatalf("schema missing required field %q", field)
		}
	}
}

func TestCreateCultureHandleSuccess(t *testing.T) {
	campaignID := uuid.New()
	cultureID := uuid.New()
	languageID := uuid.New()
	beliefSystemID := uuid.New()
	factionID := uuid.New()

	store := &stubCultureStore{
		createdCulture: statedb.Culture{
			ID:             dbutil.ToPgtype(cultureID),
			CampaignID:     dbutil.ToPgtype(campaignID),
			LanguageID:     dbutil.ToPgtype(languageID),
			BeliefSystemID: dbutil.ToPgtype(beliefSystemID),
			Name:           "Riverborn",
		},
		languages: map[[16]byte]statedb.Language{
			dbutil.ToPgtype(languageID).Bytes: {ID: dbutil.ToPgtype(languageID), CampaignID: dbutil.ToPgtype(campaignID)},
		},
		beliefSystems: map[[16]byte]statedb.BeliefSystem{
			dbutil.ToPgtype(beliefSystemID).Bytes: {ID: dbutil.ToPgtype(beliefSystemID), CampaignID: dbutil.ToPgtype(campaignID)},
		},
		factions: map[[16]byte]statedb.Faction{
			dbutil.ToPgtype(factionID).Bytes: {ID: dbutil.ToPgtype(factionID), CampaignID: dbutil.ToPgtype(campaignID)},
		},
	}
	memStore := &stubMemoryStore{}
	embedder := &stubEmbedder{vector: []float32{0.2, 0.3}}
	h := NewCreateCultureHandler(store, memStore, embedder)

	got, err := h.Handle(context.Background(), map[string]any{
		"name":             "Riverborn",
		"description":      "A culture shaped by river trade and flood seasons.",
		"values":           []any{"hospitality", "adaptability"},
		"customs":          []any{"flood festival"},
		"social_norms":     []any{"share water rights"},
		"art_forms":        []any{"boat carving"},
		"taboos":           []any{"polluting sacred springs"},
		"greeting_customs": []any{"touch wrist bands"},
		"language_id":      languageID.String(),
		"belief_system_id": beliefSystemID.String(),
		"associated_factions": []any{
			factionID.String(),
		},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if store.lastCreateCultureArgs.Name != "Riverborn" {
		t.Fatalf("CreateCulture name = %q, want Riverborn", store.lastCreateCultureArgs.Name)
	}
	if store.lastCreateCultureArgs.CampaignID != dbutil.ToPgtype(campaignID) {
		t.Fatalf("CreateCulture campaign_id mismatch")
	}
	if store.lastCreateCultureArgs.LanguageID != dbutil.ToPgtype(languageID) {
		t.Fatalf("CreateCulture language_id mismatch")
	}
	if store.lastCreateCultureArgs.BeliefSystemID != dbutil.ToPgtype(beliefSystemID) {
		t.Fatalf("CreateCulture belief_system_id mismatch")
	}
	if memStore.lastParams.MemoryType != string(domain.MemoryTypeWorldFact) {
		t.Fatalf("CreateMemory memory_type = %q, want %q", memStore.lastParams.MemoryType, domain.MemoryTypeWorldFact)
	}
	if embedder.lastInput == "" {
		t.Fatal("expected embedder input to be populated")
	}
	if got.Data["id"] != cultureID.String() {
		t.Fatalf("result id = %v, want %s", got.Data["id"], cultureID.String())
	}
	if got.Data["campaign_id"] != campaignID.String() {
		t.Fatalf("result campaign_id = %v, want %s", got.Data["campaign_id"], campaignID.String())
	}
}

func TestCreateCultureValidationAndErrors(t *testing.T) {
	campaignID := uuid.New()
	languageID := uuid.New()
	beliefSystemID := uuid.New()
	baseArgs := map[string]any{
		"name":                "Harborfolk",
		"description":         "desc",
		"values":              []any{"cooperation"},
		"customs":             []any{"night market"},
		"social_norms":        []any{"honor dock law"},
		"art_forms":           []any{"sea shanties"},
		"taboos":              []any{"stealing anchors"},
		"greeting_customs":    []any{"double nod"},
		"language_id":         languageID.String(),
		"belief_system_id":    beliefSystemID.String(),
		"associated_factions": []any{},
	}

	t.Run("missing required field", func(t *testing.T) {
		h := NewCreateCultureHandler(&stubCultureStore{}, &stubMemoryStore{}, &stubEmbedder{vector: []float32{0.1}})
		args := copyArgs(baseArgs)
		delete(args, "description")
		_, err := h.Handle(context.Background(), args)
		if err == nil || !strings.Contains(err.Error(), "description is required") {
			t.Fatalf("error = %v, want description-required message", err)
		}
	})

	t.Run("invalid associated_factions type", func(t *testing.T) {
		h := NewCreateCultureHandler(&stubCultureStore{}, &stubMemoryStore{}, &stubEmbedder{vector: []float32{0.1}})
		args := copyArgs(baseArgs)
		args["associated_factions"] = "bad"
		_, err := h.Handle(context.Background(), args)
		if err == nil || !strings.Contains(err.Error(), "associated_factions must be an array") {
			t.Fatalf("error = %v, want associated_factions array-type message", err)
		}
	})

	t.Run("language and belief campaign mismatch", func(t *testing.T) {
		otherCampaignID := uuid.New()
		h := NewCreateCultureHandler(
			&stubCultureStore{
				languages: map[[16]byte]statedb.Language{
					dbutil.ToPgtype(languageID).Bytes: {ID: dbutil.ToPgtype(languageID), CampaignID: dbutil.ToPgtype(campaignID)},
				},
				beliefSystems: map[[16]byte]statedb.BeliefSystem{
					dbutil.ToPgtype(beliefSystemID).Bytes: {ID: dbutil.ToPgtype(beliefSystemID), CampaignID: dbutil.ToPgtype(otherCampaignID)},
				},
				factions: map[[16]byte]statedb.Faction{},
			},
			&stubMemoryStore{},
			&stubEmbedder{vector: []float32{0.1}},
		)
		_, err := h.Handle(context.Background(), copyArgs(baseArgs))
		if err == nil || !strings.Contains(err.Error(), "must belong to the same campaign") {
			t.Fatalf("error = %v, want same campaign validation message", err)
		}
	})

	t.Run("associated faction campaign mismatch", func(t *testing.T) {
		factionID := uuid.New()
		otherCampaignID := uuid.New()
		h := NewCreateCultureHandler(
			&stubCultureStore{
				languages: map[[16]byte]statedb.Language{
					dbutil.ToPgtype(languageID).Bytes: {ID: dbutil.ToPgtype(languageID), CampaignID: dbutil.ToPgtype(campaignID)},
				},
				beliefSystems: map[[16]byte]statedb.BeliefSystem{
					dbutil.ToPgtype(beliefSystemID).Bytes: {ID: dbutil.ToPgtype(beliefSystemID), CampaignID: dbutil.ToPgtype(campaignID)},
				},
				factions: map[[16]byte]statedb.Faction{
					dbutil.ToPgtype(factionID).Bytes: {ID: dbutil.ToPgtype(factionID), CampaignID: dbutil.ToPgtype(otherCampaignID)},
				},
			},
			&stubMemoryStore{},
			&stubEmbedder{vector: []float32{0.1}},
		)
		args := copyArgs(baseArgs)
		args["associated_factions"] = []any{factionID.String()}
		_, err := h.Handle(context.Background(), args)
		if err == nil || !strings.Contains(err.Error(), "must belong to campaign_id") {
			t.Fatalf("error = %v, want campaign validation message", err)
		}
	})

	t.Run("embedder error", func(t *testing.T) {
		h := NewCreateCultureHandler(
			&stubCultureStore{
				createdCulture: statedb.Culture{
					ID:             dbutil.ToPgtype(uuid.New()),
					CampaignID:     dbutil.ToPgtype(campaignID),
					LanguageID:     dbutil.ToPgtype(languageID),
					BeliefSystemID: dbutil.ToPgtype(beliefSystemID),
				},
				languages: map[[16]byte]statedb.Language{
					dbutil.ToPgtype(languageID).Bytes: {ID: dbutil.ToPgtype(languageID), CampaignID: dbutil.ToPgtype(campaignID)},
				},
				beliefSystems: map[[16]byte]statedb.BeliefSystem{
					dbutil.ToPgtype(beliefSystemID).Bytes: {ID: dbutil.ToPgtype(beliefSystemID), CampaignID: dbutil.ToPgtype(campaignID)},
				},
				factions: map[[16]byte]statedb.Faction{},
			},
			&stubMemoryStore{},
			&stubEmbedder{err: errors.New("embed failed")},
		)
		_, err := h.Handle(context.Background(), copyArgs(baseArgs))
		if err == nil || !strings.Contains(err.Error(), "embed culture memory") {
			t.Fatalf("error = %v, want embed context", err)
		}
	})
}

func TestCreateCultureStoresMemoryMetadata(t *testing.T) {
	campaignID := uuid.New()
	cultureID := uuid.New()
	languageID := uuid.New()
	beliefSystemID := uuid.New()

	store := &stubCultureStore{
		createdCulture: statedb.Culture{
			ID:             dbutil.ToPgtype(cultureID),
			CampaignID:     dbutil.ToPgtype(campaignID),
			LanguageID:     dbutil.ToPgtype(languageID),
			BeliefSystemID: dbutil.ToPgtype(beliefSystemID),
		},
		languages: map[[16]byte]statedb.Language{
			dbutil.ToPgtype(languageID).Bytes: {ID: dbutil.ToPgtype(languageID), CampaignID: dbutil.ToPgtype(campaignID)},
		},
		beliefSystems: map[[16]byte]statedb.BeliefSystem{
			dbutil.ToPgtype(beliefSystemID).Bytes: {ID: dbutil.ToPgtype(beliefSystemID), CampaignID: dbutil.ToPgtype(campaignID)},
		},
		factions: map[[16]byte]statedb.Faction{},
	}
	memStore := &stubMemoryStore{}
	h := NewCreateCultureHandler(store, memStore, &stubEmbedder{vector: []float32{0.1}})

	_, err := h.Handle(context.Background(), map[string]any{
		"name":                "Culture",
		"description":         "desc",
		"values":              []any{"value"},
		"customs":             []any{"custom"},
		"social_norms":        []any{"norm"},
		"art_forms":           []any{"art"},
		"taboos":              []any{"taboo"},
		"greeting_customs":    []any{"greet"},
		"language_id":         languageID.String(),
		"belief_system_id":    beliefSystemID.String(),
		"associated_factions": []any{},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var metadata map[string]any
	if err := json.Unmarshal(memStore.lastParams.Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if metadata["culture_id"] != cultureID.String() {
		t.Fatalf("metadata.culture_id = %v, want %s", metadata["culture_id"], cultureID.String())
	}
	if metadata["language_id"] != languageID.String() {
		t.Fatalf("metadata.language_id = %v, want %s", metadata["language_id"], languageID.String())
	}
	if metadata["belief_system_id"] != beliefSystemID.String() {
		t.Fatalf("metadata.belief_system_id = %v, want %s", metadata["belief_system_id"], beliefSystemID.String())
	}
}

var _ CultureStore = (*stubCultureStore)(nil)
