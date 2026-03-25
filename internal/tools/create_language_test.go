package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type stubLanguageStore struct {
	lastParams          CreateLanguageParams
	createdID           uuid.UUID
	factionCampaigns    map[uuid.UUID]uuid.UUID // factionID -> campaignID
	cultureCampaigns    map[uuid.UUID]uuid.UUID // cultureID -> campaignID
	getFacErr           error
	getCulErr           error
	err                 error
}

func (s *stubLanguageStore) CreateLanguage(_ context.Context, params CreateLanguageParams) (uuid.UUID, error) {
	if s.err != nil {
		return uuid.Nil, s.err
	}
	s.lastParams = params
	return s.createdID, nil
}

func (s *stubLanguageStore) FactionBelongsToCampaign(_ context.Context, factionID, campaignID uuid.UUID) (bool, error) {
	if s.getFacErr != nil {
		return false, s.getFacErr
	}
	camp, ok := s.factionCampaigns[factionID]
	if !ok {
		return false, errors.New("faction not found")
	}
	return camp == campaignID, nil
}

func (s *stubLanguageStore) CultureBelongsToCampaign(_ context.Context, cultureID, campaignID uuid.UUID) (bool, error) {
	if s.getCulErr != nil {
		return false, s.getCulErr
	}
	camp, ok := s.cultureCampaigns[cultureID]
	if !ok {
		return false, errors.New("culture not found")
	}
	return camp == campaignID, nil
}

type stubMemoryStore struct {
	lastParams CreateMemoryParams
	called     bool
	err        error
}

func (s *stubMemoryStore) CreateMemory(_ context.Context, params CreateMemoryParams) error {
	if s.err != nil {
		return s.err
	}
	s.called = true
	s.lastParams = params
	return nil
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
	langStore := &stubLanguageStore{createdID: uuid.New()}
	memStore := &stubMemoryStore{}
	embedder := &stubEmbedder{vector: []float32{0.1, 0.2}}

	if err := RegisterCreateLanguage(reg, langStore, memStore, embedder); err != nil {
		t.Fatalf("register create_language: %v", err)
	}

	tools := reg.List()
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

func TestRegisterCreateLanguage_NilEmbedding(t *testing.T) {
	reg := NewRegistry()
	langStore := &stubLanguageStore{createdID: uuid.New()}

	if err := RegisterCreateLanguage(reg, langStore, nil, nil); err != nil {
		t.Fatalf("register with nil embedding: %v", err)
	}

	tools := reg.List()
	if len(tools) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(tools))
	}
}

func TestCreateLanguageHandleSuccess(t *testing.T) {
	campaignID := uuid.New()
	languageID := uuid.New()
	factionID := uuid.New()
	cultureID := uuid.New()

	langStore := &stubLanguageStore{
		createdID: languageID,
		factionCampaigns: map[uuid.UUID]uuid.UUID{
			factionID: campaignID,
		},
		cultureCampaigns: map[uuid.UUID]uuid.UUID{
			cultureID: campaignID,
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

	if langStore.lastParams.Name != "Eldertongue" {
		t.Fatalf("CreateLanguage name = %q, want Eldertongue", langStore.lastParams.Name)
	}
	if langStore.lastParams.Description != "Ancient ritual language" {
		t.Fatalf("CreateLanguage description = %q, want 'Ancient ritual language'", langStore.lastParams.Description)
	}
	if len(langStore.lastParams.SpokenByFactionIDs) != 1 || langStore.lastParams.SpokenByFactionIDs[0] != factionID {
		t.Fatalf("CreateLanguage spoken_by_faction_ids = %v, want [%s]", langStore.lastParams.SpokenByFactionIDs, factionID)
	}
	if len(langStore.lastParams.SpokenByCultureIDs) != 1 || langStore.lastParams.SpokenByCultureIDs[0] != cultureID {
		t.Fatalf("CreateLanguage spoken_by_culture_ids = %v, want [%s]", langStore.lastParams.SpokenByCultureIDs, cultureID)
	}

	if memStore.lastParams.MemoryType != "world_fact" {
		t.Fatalf("CreateMemory memory_type = %q, want %q", memStore.lastParams.MemoryType, "world_fact")
	}
	if memStore.lastParams.CampaignID != campaignID {
		t.Fatalf("CreateMemory campaign_id mismatch")
	}
	if embedder.lastInput == "" {
		t.Fatal("expected embedder input to be populated")
	}
	if got.Data["id"] != languageID.String() {
		t.Fatalf("result id = %v, want %s", got.Data["id"], languageID.String())
	}
	if got.Data["name"] != "Eldertongue" {
		t.Fatalf("result name = %v, want Eldertongue", got.Data["name"])
	}
	if got.Data["description"] != "Ancient ritual language" {
		t.Fatalf("result description = %v, want Ancient ritual language", got.Data["description"])
	}
}

func TestCreateLanguageHandleSuccess_NilEmbedding(t *testing.T) {
	campaignID := uuid.New()
	languageID := uuid.New()

	langStore := &stubLanguageStore{createdID: languageID}

	h := NewCreateLanguageHandler(langStore, nil, nil)
	got, err := h.Handle(context.Background(), map[string]any{
		"campaign_id": campaignID.String(),
		"name":        "Elvish",
		"description": "Forest language",
		"phonological_rules": map[string]any{
			"vowels": []any{"a", "e"},
		},
		"naming_conventions": map[string]any{
			"patterns": []any{"VCV"},
		},
		"sample_vocabulary": map[string]any{
			"tree": "tala",
		},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if got.Data["id"] != languageID.String() {
		t.Fatalf("result id = %v, want %s", got.Data["id"], languageID.String())
	}
	if got.Data["name"] != "Elvish" {
		t.Fatalf("result name = %v, want Elvish", got.Data["name"])
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
		h := NewCreateLanguageHandler(
			&stubLanguageStore{createdID: uuid.New()},
			&stubMemoryStore{},
			&stubEmbedder{vector: []float32{0.1}},
		)
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
		h := NewCreateLanguageHandler(
			&stubLanguageStore{createdID: uuid.New()},
			&stubMemoryStore{},
			&stubEmbedder{vector: []float32{0.1}},
		)
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
				createdID: uuid.New(),
				factionCampaigns: map[uuid.UUID]uuid.UUID{
					factionID: campaignID,
				},
				cultureCampaigns: map[uuid.UUID]uuid.UUID{
					cultureID: campaignID,
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
				createdID: uuid.New(),
				factionCampaigns: map[uuid.UUID]uuid.UUID{
					factionID: campaignID,
				},
				cultureCampaigns: map[uuid.UUID]uuid.UUID{
					cultureID: campaignID,
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
				createdID: uuid.New(),
				factionCampaigns: map[uuid.UUID]uuid.UUID{
					factionID: otherCampaignID,
				},
				cultureCampaigns: map[uuid.UUID]uuid.UUID{
					cultureID: campaignID,
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
		createdID: languageID,
		factionCampaigns: map[uuid.UUID]uuid.UUID{
			factionID: campaignID,
		},
		cultureCampaigns: map[uuid.UUID]uuid.UUID{
			cultureID: campaignID,
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
	if err := json.Unmarshal(memStore.lastParams.Metadata, &metadata); err != nil {
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
