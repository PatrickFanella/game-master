package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	pgvector "github.com/pgvector/pgvector-go"

	"github.com/PatrickFanella/game-master/internal/domain"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

type stubLanguageStore struct {
	lastArgs  statedb.CreateLanguageParams
	created   statedb.Language
	factions  map[[16]byte]statedb.Faction
	cultures  map[[16]byte]statedb.Culture
	getFacErr error
	getCulErr error
	err       error
}

func (s *stubLanguageStore) CreateLanguage(_ context.Context, arg statedb.CreateLanguageParams) (statedb.Language, error) {
	if s.err != nil {
		return statedb.Language{}, s.err
	}
	s.lastArgs = arg
	return s.created, nil
}

func (s *stubLanguageStore) GetFactionByID(_ context.Context, id pgtype.UUID) (statedb.Faction, error) {
	if s.getFacErr != nil {
		return statedb.Faction{}, s.getFacErr
	}
	if !id.Valid {
		return statedb.Faction{}, errors.New("faction id is invalid")
	}
	faction, ok := s.factions[id.Bytes]
	if !ok {
		return statedb.Faction{}, errors.New("faction not found")
	}
	return faction, nil
}

func (s *stubLanguageStore) GetCultureByID(_ context.Context, id pgtype.UUID) (statedb.Culture, error) {
	if s.getCulErr != nil {
		return statedb.Culture{}, s.getCulErr
	}
	if !id.Valid {
		return statedb.Culture{}, errors.New("culture id is invalid")
	}
	culture, ok := s.cultures[id.Bytes]
	if !ok {
		return statedb.Culture{}, errors.New("culture not found")
	}
	return culture, nil
}

type stubMemoryStore struct {
	lastArgs statedb.CreateMemoryParams
	err      error
}

func (s *stubMemoryStore) CreateMemory(_ context.Context, arg statedb.CreateMemoryParams) (statedb.Memory, error) {
	if s.err != nil {
		return statedb.Memory{}, s.err
	}
	s.lastArgs = arg
	return statedb.Memory{}, nil
}

type stubEmbedder struct {
	lastInput string
	vector    []float32
	err       error
}

func (s *stubEmbedder) Embed(_ context.Context, input string) ([]float32, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.lastInput = input
	return s.vector, nil
}

func TestRegisterCreateLanguage(t *testing.T) {
	reg := NewRegistry()
	langStore := &stubLanguageStore{}
	memStore := &stubMemoryStore{}
	embedder := &stubEmbedder{vector: []float32{0.1, 0.2}}

	if err := RegisterCreateLanguage(reg, langStore, memStore, embedder); err != nil {
		t.Fatalf("register create_language: %v", err)
	}

	tools := reg.Tools()
	if len(tools) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(tools))
	}
	if tools[0].Name != createLanguageToolName {
		t.Fatalf("tool name = %q, want %q", tools[0].Name, createLanguageToolName)
	}

	required, ok := tools[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", tools[0].Parameters["required"])
	}
	for _, field := range []string{
		"campaign_id",
		"name",
		"description",
		"phonological_rules",
		"naming_conventions",
		"sample_vocabulary",
	} {
		found := false
		for _, got := range required {
			if got == field {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("required schema = %#v, missing field %q", required, field)
		}
	}
}

func TestCreateLanguageHandleSuccess(t *testing.T) {
	campaignID := uuid.New()
	languageID := uuid.New()
	factionID := uuid.New()
	cultureID := uuid.New()

	langStore := &stubLanguageStore{
		created: statedb.Language{
			ID:                 uuidToPgtype(languageID),
			CampaignID:         uuidToPgtype(campaignID),
			Name:               "Eldertongue",
			Description:        "Ancient ritual language",
			Phonology:          []byte(`{"vowels":["a","e","i"]}`),
			Naming:             []byte(`{"person_name_patterns":["CV-CV"],"place_name_patterns":["CVC"]}`),
			Vocabulary:         []byte(`{"sun":"sol","moon":"luna"}`),
			SpokenByFactionIds: []pgtype.UUID{uuidToPgtype(factionID)},
			SpokenByCultureIds: []pgtype.UUID{uuidToPgtype(cultureID)},
		},
		factions: map[[16]byte]statedb.Faction{
			factionID: {ID: uuidToPgtype(factionID), CampaignID: uuidToPgtype(campaignID)},
		},
		cultures: map[[16]byte]statedb.Culture{
			cultureID: {ID: uuidToPgtype(cultureID), CampaignID: uuidToPgtype(campaignID)},
		},
	}
	memStore := &stubMemoryStore{}
	embedder := &stubEmbedder{vector: []float32{0.3, 0.4}}

	h := NewCreateLanguageHandler(langStore, memStore, embedder)
	got, err := h.Handle(context.Background(), map[string]any{
		"campaign_id": campaignID.String(),
		"name":        "Eldertongue",
		"description": "Ancient ritual language",
		"phonological_rules": map[string]any{
			"vowels": []any{"a", "e", "i"},
		},
		"naming_conventions": map[string]any{
			"person_name_patterns": []any{"CV-CV"},
			"place_name_patterns":  []any{"CVC"},
		},
		"sample_vocabulary": map[string]any{
			"sun":  "sol",
			"moon": "luna",
		},
		"spoken_by_faction_ids": []any{factionID.String()},
		"spoken_by_culture_ids": []any{cultureID.String()},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if langStore.lastArgs.Name != "Eldertongue" {
		t.Fatalf("CreateLanguage name = %q, want Eldertongue", langStore.lastArgs.Name)
	}
	if langStore.lastArgs.Description.String != "Ancient ritual language" || !langStore.lastArgs.Description.Valid {
		t.Fatalf("CreateLanguage description = %#v, want valid text", langStore.lastArgs.Description)
	}
	if len(langStore.lastArgs.SpokenByFactionIds) != 1 || uuidFromPgtype(langStore.lastArgs.SpokenByFactionIds[0]) != factionID {
		t.Fatalf("CreateLanguage spoken_by_faction_ids = %#v, want [%s]", langStore.lastArgs.SpokenByFactionIds, factionID)
	}
	if len(langStore.lastArgs.SpokenByCultureIds) != 1 || uuidFromPgtype(langStore.lastArgs.SpokenByCultureIds[0]) != cultureID {
		t.Fatalf("CreateLanguage spoken_by_culture_ids = %#v, want [%s]", langStore.lastArgs.SpokenByCultureIds, cultureID)
	}

	if memStore.lastArgs.MemoryType != string(domain.MemoryTypeWorldFact) {
		t.Fatalf("CreateMemory memory_type = %q, want %q", memStore.lastArgs.MemoryType, domain.MemoryTypeWorldFact)
	}
	if memStore.lastArgs.CampaignID != uuidToPgtype(campaignID) {
		t.Fatalf("CreateMemory campaign_id mismatch")
	}
	if embedder.lastInput == "" {
		t.Fatal("expected embedder input to be populated")
	}
	if got["id"] != languageID.String() {
		t.Fatalf("result id = %v, want %s", got["id"], languageID.String())
	}
	if got["name"] != "Eldertongue" {
		t.Fatalf("result name = %v, want Eldertongue", got["name"])
	}
	if got["description"] != "Ancient ritual language" {
		t.Fatalf("result description = %v, want Ancient ritual language", got["description"])
	}
}

func TestCreateLanguageValidationAndErrors(t *testing.T) {
	campaignID := uuid.New()
	baseArgs := map[string]any{
		"campaign_id": campaignID.String(),
		"name":        "Lang",
		"description": "desc",
		"phonological_rules": map[string]any{
			"rule": "value",
		},
		"naming_conventions": map[string]any{
			"person_name_patterns": []any{"CV"},
		},
		"sample_vocabulary": map[string]any{
			"hello": "hala",
		},
	}

	t.Run("missing required field", func(t *testing.T) {
		h := NewCreateLanguageHandler(&stubLanguageStore{}, &stubMemoryStore{}, &stubEmbedder{vector: []float32{0.1}})
		args := copyArgs(baseArgs)
		delete(args, "description")

		_, err := h.Handle(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for missing description")
		}
		if !strings.Contains(err.Error(), "description is required") {
			t.Fatalf("error = %v, want description-required message", err)
		}
	})

	t.Run("invalid spoken_by_faction_ids type", func(t *testing.T) {
		h := NewCreateLanguageHandler(&stubLanguageStore{}, &stubMemoryStore{}, &stubEmbedder{vector: []float32{0.1}})
		args := copyArgs(baseArgs)
		args["spoken_by_faction_ids"] = "not-an-array"

		_, err := h.Handle(context.Background(), args)
		if err == nil {
			t.Fatal("expected error for invalid spoken_by_faction_ids")
		}
		if !strings.Contains(err.Error(), "spoken_by_faction_ids must be an array") {
			t.Fatalf("error = %v, want array-type message", err)
		}
	})

	t.Run("embedder error", func(t *testing.T) {
		factionID := uuid.New()
		cultureID := uuid.New()
		h := NewCreateLanguageHandler(
			&stubLanguageStore{
				created: statedb.Language{
					ID:         uuidToPgtype(uuid.New()),
					CampaignID: uuidToPgtype(campaignID),
					Name:       "Lang",
				},
				factions: map[[16]byte]statedb.Faction{
					factionID: {ID: uuidToPgtype(factionID), CampaignID: uuidToPgtype(campaignID)},
				},
				cultures: map[[16]byte]statedb.Culture{
					cultureID: {ID: uuidToPgtype(cultureID), CampaignID: uuidToPgtype(campaignID)},
				},
			},
			&stubMemoryStore{},
			&stubEmbedder{err: errors.New("embed failed")},
		)

		args := copyArgs(baseArgs)
		args["spoken_by_faction_ids"] = []any{factionID.String()}
		args["spoken_by_culture_ids"] = []any{cultureID.String()}
		_, err := h.Handle(context.Background(), args)
		if err == nil {
			t.Fatal("expected embedder error")
		}
		if !strings.Contains(err.Error(), "embed language memory") {
			t.Fatalf("error = %v, want embed context", err)
		}
	})

	t.Run("memory store error", func(t *testing.T) {
		factionID := uuid.New()
		cultureID := uuid.New()
		h := NewCreateLanguageHandler(
			&stubLanguageStore{
				created: statedb.Language{
					ID:         uuidToPgtype(uuid.New()),
					CampaignID: uuidToPgtype(campaignID),
					Name:       "Lang",
				},
				factions: map[[16]byte]statedb.Faction{
					factionID: {ID: uuidToPgtype(factionID), CampaignID: uuidToPgtype(campaignID)},
				},
				cultures: map[[16]byte]statedb.Culture{
					cultureID: {ID: uuidToPgtype(cultureID), CampaignID: uuidToPgtype(campaignID)},
				},
			},
			&stubMemoryStore{err: errors.New("insert failed")},
			&stubEmbedder{vector: []float32{0.1}},
		)

		args := copyArgs(baseArgs)
		args["spoken_by_faction_ids"] = []any{factionID.String()}
		args["spoken_by_culture_ids"] = []any{cultureID.String()}

		_, err := h.Handle(context.Background(), args)
		if err == nil {
			t.Fatal("expected memory store error")
		}
		if !strings.Contains(err.Error(), "create language memory") {
			t.Fatalf("error = %v, want create-language-memory context", err)
		}
	})

	t.Run("speaker IDs must belong to campaign", func(t *testing.T) {
		factionID := uuid.New()
		cultureID := uuid.New()
		otherCampaignID := uuid.New()

		h := NewCreateLanguageHandler(
			&stubLanguageStore{
				factions: map[[16]byte]statedb.Faction{
					factionID: {ID: uuidToPgtype(factionID), CampaignID: uuidToPgtype(otherCampaignID)},
				},
				cultures: map[[16]byte]statedb.Culture{
					cultureID: {ID: uuidToPgtype(cultureID), CampaignID: uuidToPgtype(campaignID)},
				},
			},
			&stubMemoryStore{},
			&stubEmbedder{vector: []float32{0.1}},
		)

		args := copyArgs(baseArgs)
		args["spoken_by_faction_ids"] = []any{factionID.String()}
		args["spoken_by_culture_ids"] = []any{cultureID.String()}
		_, err := h.Handle(context.Background(), args)
		if err == nil {
			t.Fatal("expected campaign scope validation error")
		}
		if !strings.Contains(err.Error(), "must belong to campaign_id") {
			t.Fatalf("error = %v, want campaign-scope message", err)
		}
	})
}

func TestCreateLanguageStoresMemoryMetadata(t *testing.T) {
	campaignID := uuid.New()
	languageID := uuid.New()
	factionID := uuid.New()
	cultureID := uuid.New()

	langStore := &stubLanguageStore{
		created: statedb.Language{
			ID:                 uuidToPgtype(languageID),
			CampaignID:         uuidToPgtype(campaignID),
			Name:               "Lang",
			Description:        "desc",
			SpokenByFactionIds: []pgtype.UUID{uuidToPgtype(factionID)},
			SpokenByCultureIds: []pgtype.UUID{uuidToPgtype(cultureID)},
		},
		factions: map[[16]byte]statedb.Faction{
			factionID: {ID: uuidToPgtype(factionID), CampaignID: uuidToPgtype(campaignID)},
		},
		cultures: map[[16]byte]statedb.Culture{
			cultureID: {ID: uuidToPgtype(cultureID), CampaignID: uuidToPgtype(campaignID)},
		},
	}
	memStore := &stubMemoryStore{}
	h := NewCreateLanguageHandler(langStore, memStore, &stubEmbedder{vector: []float32{1.0}})

	_, err := h.Handle(context.Background(), map[string]any{
		"campaign_id": campaignID.String(),
		"name":        "Lang",
		"description": "desc",
		"phonological_rules": map[string]any{
			"consonants": []any{"k"},
		},
		"naming_conventions": map[string]any{
			"person_name_patterns": []any{"VC"},
		},
		"sample_vocabulary": map[string]any{
			"water": "aqua",
		},
		"spoken_by_faction_ids": []any{factionID.String()},
		"spoken_by_culture_ids": []any{cultureID.String()},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var metadata map[string]any
	if err := json.Unmarshal(memStore.lastArgs.Metadata, &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if metadata["language_id"] != languageID.String() {
		t.Fatalf("metadata.language_id = %v, want %s", metadata["language_id"], languageID)
	}
}

func copyArgs(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

var _ Embedder = (*stubEmbedder)(nil)
var _ LanguageStore = (*stubLanguageStore)(nil)
var _ MemoryStore = (*stubMemoryStore)(nil)
var _ = pgvector.NewVector
