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
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

// stubNameGeneratorStore implements NameGeneratorStore for tests.
type stubNameGeneratorStore struct {
	languages map[pgtype.UUID]statedb.Language
	err       error
}

func (s *stubNameGeneratorStore) GetLanguageByID(_ context.Context, id pgtype.UUID) (statedb.Language, error) {
	if s.err != nil {
		return statedb.Language{}, s.err
	}
	lang, ok := s.languages[id]
	if !ok {
		return statedb.Language{}, pgtype.ErrScanTargetTypeChanged
	}
	return lang, nil
}

var _ NameGeneratorStore = (*stubNameGeneratorStore)(nil)

// makeLanguage builds a statedb.Language for use in tests.
func makeLanguage(id, campaignID uuid.UUID, phonology, naming map[string]any) statedb.Language {
	phonologyJSON, _ := json.Marshal(phonology)
	namingJSON, _ := json.Marshal(naming)
	return statedb.Language{
		ID:         dbutil.ToPgtype(id),
		CampaignID: dbutil.ToPgtype(campaignID),
		Name:       "Eldertongue",
		Phonology:  phonologyJSON,
		Naming:     namingJSON,
	}
}

// --- Registration tests ---

func TestRegisterGenerateName(t *testing.T) {
	reg := NewRegistry()
	nameStore := &stubNameGeneratorStore{}

	if err := RegisterGenerateName(reg, nameStore); err != nil {
		t.Fatalf("RegisterGenerateName: %v", err)
	}

	tools := reg.List()
	if len(tools) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(tools))
	}
	if tools[0].Name != generateNameToolName {
		t.Fatalf("tool name = %q, want %q", tools[0].Name, generateNameToolName)
	}

	required, ok := tools[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", tools[0].Parameters["required"])
	}
	for _, field := range []string{"name_type"} {
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

func TestRegisterGenerateName_NilStore(t *testing.T) {
	reg := NewRegistry()
	// nil store is allowed — names fall back to generic generation
	if err := RegisterGenerateName(reg, nil); err != nil {
		t.Fatalf("RegisterGenerateName with nil store: %v", err)
	}
	tools := reg.List()
	if len(tools) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(tools))
	}
}

// --- Handle tests: no language_id (generic fallback) ---

func TestGenerateNameHandle_GenericFallback(t *testing.T) {
	h := NewGenerateNameHandler(nil)

	for _, nt := range []string{"person_first", "person_last", "place", "object"} {
		t.Run(nt, func(t *testing.T) {
			result, err := h.Handle(context.Background(), map[string]any{
				"name_type": nt,
			})
			if err != nil {
				t.Fatalf("Handle(%q): %v", nt, err)
			}
			if !result.Success {
				t.Fatalf("expected Success=true for %q", nt)
			}
			name, ok := result.Data["name"].(string)
			if !ok || name == "" {
				t.Fatalf("expected non-empty name for %q, got %v", nt, result.Data["name"])
			}
			if result.Data["name_type"] != nt {
				t.Fatalf("name_type = %v, want %q", result.Data["name_type"], nt)
			}
			// generic fallback should not include language_id in data
			if _, hasLang := result.Data["language_id"]; hasLang {
				t.Fatalf("expected no language_id in data for generic fallback")
			}
		})
	}
}

// --- Handle tests: with language_id ---

func TestGenerateNameHandle_WithLanguage_NestedConventions(t *testing.T) {
	langID := uuid.New()
	campaignID := uuid.New()

	phonology := map[string]any{
		"vowels":     []any{"a", "e"},
		"consonants": []any{"k", "l", "r"},
	}
	naming := map[string]any{
		"person_first": map[string]any{
			"patterns": []any{"CV-CV"},
		},
		"place": map[string]any{
			"patterns": []any{"CVC"},
		},
	}
	lang := makeLanguage(langID, campaignID, phonology, naming)
	store := &stubNameGeneratorStore{
		languages: map[pgtype.UUID]statedb.Language{
			dbutil.ToPgtype(langID): lang,
		},
	}

	h := NewGenerateNameHandler(store)
	result, err := h.Handle(context.Background(), map[string]any{
		"language_id": langID.String(),
		"name_type":   "person_first",
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !result.Success {
		t.Fatal("expected Success=true")
	}
	name, ok := result.Data["name"].(string)
	if !ok || name == "" {
		t.Fatalf("expected non-empty name, got %v", result.Data["name"])
	}
	// With phonemes [a,e] and [k,l,r] and pattern "CV-CV", the name must use
	// only those phonemes.
	lower := strings.ToLower(name)
	for _, ch := range lower {
		if !strings.ContainsRune("aeklr", ch) {
			t.Fatalf("generated name %q contains unexpected phoneme %q", name, string(ch))
		}
	}
	if result.Data["language_id"] != langID.String() {
		t.Fatalf("result language_id = %v, want %s", result.Data["language_id"], langID)
	}
}

func TestGenerateNameHandle_WithLanguage_FlatConventions(t *testing.T) {
	langID := uuid.New()
	campaignID := uuid.New()

	phonology := map[string]any{
		"vowels":     []any{"i", "u"},
		"consonants": []any{"t", "s"},
	}
	naming := map[string]any{
		"place_patterns": []any{"CVC"},
	}
	lang := makeLanguage(langID, campaignID, phonology, naming)
	store := &stubNameGeneratorStore{
		languages: map[pgtype.UUID]statedb.Language{
			dbutil.ToPgtype(langID): lang,
		},
	}

	h := NewGenerateNameHandler(store)
	result, err := h.Handle(context.Background(), map[string]any{
		"language_id": langID.String(),
		"name_type":   "place",
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	name, ok := result.Data["name"].(string)
	if !ok || name == "" {
		t.Fatalf("expected non-empty name")
	}
	lower := strings.ToLower(name)
	for _, ch := range lower {
		if !strings.ContainsRune("iuts", ch) {
			t.Fatalf("generated name %q contains unexpected phoneme %q", name, string(ch))
		}
	}
}

func TestGenerateNameHandle_WithLanguage_TopLevelPatterns(t *testing.T) {
	langID := uuid.New()
	campaignID := uuid.New()

	phonology := map[string]any{
		"vowels":     []any{"o"},
		"consonants": []any{"m"},
	}
	naming := map[string]any{
		"patterns": []any{"CV"},
	}
	lang := makeLanguage(langID, campaignID, phonology, naming)
	store := &stubNameGeneratorStore{
		languages: map[pgtype.UUID]statedb.Language{
			dbutil.ToPgtype(langID): lang,
		},
	}

	h := NewGenerateNameHandler(store)
	result, err := h.Handle(context.Background(), map[string]any{
		"language_id": langID.String(),
		"name_type":   "object",
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	name, ok := result.Data["name"].(string)
	if !ok || name == "" {
		t.Fatalf("expected non-empty name")
	}
	// "CV" with vowels=[o] consonants=[m] → always "Mo"
	if strings.ToLower(name) != "mo" {
		t.Fatalf("expected name Mo (CV from m,o), got %q", name)
	}
}

// --- Validation tests ---

func TestGenerateNameHandle_Validation(t *testing.T) {
	h := NewGenerateNameHandler(nil)

	t.Run("missing name_type", func(t *testing.T) {
		_, err := h.Handle(context.Background(), map[string]any{})
		if err == nil {
			t.Fatal("expected error for missing name_type")
		}
		if !strings.Contains(err.Error(), "name_type is required") {
			t.Fatalf("error = %v, want name_type-required message", err)
		}
	})

	t.Run("invalid name_type", func(t *testing.T) {
		_, err := h.Handle(context.Background(), map[string]any{
			"name_type": "weapon",
		})
		if err == nil {
			t.Fatal("expected error for invalid name_type")
		}
		if !strings.Contains(err.Error(), "name_type must be one of") {
			t.Fatalf("error = %v, want name_type-enum message", err)
		}
	})

	t.Run("invalid language_id format", func(t *testing.T) {
		_, err := h.Handle(context.Background(), map[string]any{
			"name_type":   "place",
			"language_id": "not-a-uuid",
		})
		if err == nil {
			t.Fatal("expected error for invalid language_id")
		}
		if !strings.Contains(err.Error(), "language_id must be a valid UUID") {
			t.Fatalf("error = %v, want UUID-format message", err)
		}
	})

	t.Run("empty language_id treated as not provided", func(t *testing.T) {
		result, err := h.Handle(context.Background(), map[string]any{
			"name_type":   "place",
			"language_id": "   ",
		})
		if err != nil {
			t.Fatalf("expected no error for empty language_id, got: %v", err)
		}
		if !result.Success {
			t.Fatal("expected Success=true")
		}
		name, ok := result.Data["name"].(string)
		if !ok || name == "" {
			t.Fatalf("expected non-empty fallback name, got %v", result.Data["name"])
		}
		if _, hasLang := result.Data["language_id"]; hasLang {
			t.Fatal("expected no language_id in data when empty string provided")
		}
	})
}

func TestGenerateNameHandle_StoreError(t *testing.T) {
	langID := uuid.New()
	store := &stubNameGeneratorStore{
		err: errors.New("database error"),
	}

	h := NewGenerateNameHandler(store)
	_, err := h.Handle(context.Background(), map[string]any{
		"language_id": langID.String(),
		"name_type":   "person_first",
	})
	if err == nil {
		t.Fatal("expected error from store")
	}
	if !strings.Contains(err.Error(), "get language") {
		t.Fatalf("error = %v, want get-language context", err)
	}
}

// --- GenerateName method tests ---

func TestGenerateName_FallbackWhenNoLanguageID(t *testing.T) {
	h := NewGenerateNameHandler(nil)
	name, err := h.GenerateName(context.Background(), nil, NameTypePersonFirst)
	if err != nil {
		t.Fatalf("GenerateName: %v", err)
	}
	if name == "" {
		t.Fatal("expected non-empty name")
	}
}

func TestGenerateName_FallbackWhenNilStore(t *testing.T) {
	langID := uuid.New()
	pgID := dbutil.ToPgtype(langID)
	h := NewGenerateNameHandler(nil)
	name, err := h.GenerateName(context.Background(), &pgID, NameTypePlace)
	if err != nil {
		t.Fatalf("GenerateName with nil store: %v", err)
	}
	if name == "" {
		t.Fatal("expected non-empty name")
	}
}

func TestGenerateName_AllNameTypes(t *testing.T) {
	nameTypes := []NameType{
		NameTypePersonFirst,
		NameTypePersonLast,
		NameTypePlace,
		NameTypeObject,
	}

	h := NewGenerateNameHandler(nil)
	for _, nt := range nameTypes {
		name, err := h.GenerateName(context.Background(), nil, nt)
		if err != nil {
			t.Fatalf("GenerateName(%q): %v", nt, err)
		}
		if name == "" {
			t.Fatalf("GenerateName(%q) returned empty name", nt)
		}
		// Name must start with uppercase letter
		if name[0] < 'A' || name[0] > 'Z' {
			t.Fatalf("GenerateName(%q) = %q, first char not uppercase", nt, name)
		}
	}
}

func TestGenerateName_InvalidNameType(t *testing.T) {
	h := NewGenerateNameHandler(nil)
	_, err := h.GenerateName(context.Background(), nil, NameType("monster"))
	if err == nil {
		t.Fatal("expected error for invalid name type")
	}
}
