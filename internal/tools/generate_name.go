package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/llm"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

const generateNameToolName = "generate_name"

// NameType identifies the kind of name to generate.
type NameType string

const (
	NameTypePersonFirst NameType = "person_first"
	NameTypePersonLast  NameType = "person_last"
	NameTypePlace       NameType = "place"
	NameTypeObject      NameType = "object"
)

var validNameTypes = map[NameType]bool{
	NameTypePersonFirst: true,
	NameTypePersonLast:  true,
	NameTypePlace:       true,
	NameTypeObject:      true,
}

// NameGeneratorStore retrieves language data used to produce phonologically
// consistent names.
type NameGeneratorStore interface {
	GetLanguageByID(ctx context.Context, id pgtype.UUID) (statedb.Language, error)
}

// default phonemes and syllable patterns used when no language is provided.
var (
	defaultVowels     = []string{"a", "e", "i", "o", "u"}
	defaultConsonants = []string{"b", "d", "f", "g", "k", "l", "m", "n", "p", "r", "s", "t", "v"}

	defaultPatterns = map[NameType][]string{
		NameTypePersonFirst: {"CV-CV", "CV-CVC", "CVC"},
		NameTypePersonLast:  {"CVC-CV", "CVC-CVC"},
		NameTypePlace:       {"CVC", "CV-CVC", "CVC-CV"},
		NameTypeObject:      {"VC", "CV", "CVC"},
	}
)

// GenerateNameTool returns the generate_name tool definition and JSON schema.
func GenerateNameTool() llm.Tool {
	return llm.Tool{
		Name:        generateNameToolName,
		Description: "Generate a phonologically consistent name using a language's stored phonological rules and naming conventions. Falls back to generic names when no language is specified.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name_type": map[string]any{
					"type":        "string",
					"description": "Kind of name to generate.",
					"enum":        []string{"person_first", "person_last", "place", "object"},
				},
				"language_id": map[string]any{
					"type":        "string",
					"description": "Optional language UUID. When provided, naming conventions and phonological rules from that language are applied. When omitted, generic names are generated.",
				},
			},
			"required":             []string{"name_type"},
			"additionalProperties": false,
		},
	}
}

// RegisterGenerateName registers the generate_name tool and handler.
// nameStore is optional; when nil, all names fall back to generic generation.
func RegisterGenerateName(reg *Registry, nameStore NameGeneratorStore) error {
	return reg.Register(GenerateNameTool(), NewGenerateNameHandler(nameStore).Handle)
}

// GenerateNameHandler executes generate_name tool calls.
type GenerateNameHandler struct {
	nameStore NameGeneratorStore
}

// NewGenerateNameHandler creates a new generate_name handler.
func NewGenerateNameHandler(nameStore NameGeneratorStore) *GenerateNameHandler {
	return &GenerateNameHandler{nameStore: nameStore}
}

// GenerateName generates a name for the given languageID and nameType.
// If languageID is nil, generic fallback phonemes are used.
func (h *GenerateNameHandler) GenerateName(ctx context.Context, languageID *pgtype.UUID, nameType NameType) (string, error) {
	if !validNameTypes[nameType] {
		return "", fmt.Errorf("name_type must be one of: person_first, person_last, place, object")
	}

	if languageID == nil || !languageID.Valid || h.nameStore == nil {
		return applyPattern(defaultVowels, defaultConsonants, pickPattern(nil, nameType)), nil
	}

	lang, err := h.nameStore.GetLanguageByID(ctx, *languageID)
	if err != nil {
		return "", fmt.Errorf("get language: %w", err)
	}

	vowels, consonants := parsePhonologicalRules(lang.Phonology)
	pattern := pickPatternFromNaming(lang.Naming, nameType)
	return applyPattern(vowels, consonants, pattern), nil
}

// Handle executes the generate_name tool call from the LLM.
func (h *GenerateNameHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("generate_name handler is nil")
	}

	nameTypeStr, err := parseStringArg(args, "name_type")
	if err != nil {
		return nil, err
	}
	nameType := NameType(nameTypeStr)
	if !validNameTypes[nameType] {
		return nil, fmt.Errorf("name_type must be one of: person_first, person_last, place, object")
	}

	var langIDPtr *pgtype.UUID
	if raw, ok := args["language_id"]; ok && raw != nil {
		s, ok := raw.(string)
		if !ok {
			return nil, errors.New("language_id must be a string when provided")
		}
		if strings.TrimSpace(s) != "" {
			id, err := parseUUIDArg(args, "language_id")
			if err != nil {
				return nil, err
			}
			pgID := dbutil.ToPgtype(id)
			langIDPtr = &pgID
		}
		// Empty/whitespace language_id is treated as not provided.
	}

	name, err := h.GenerateName(ctx, langIDPtr, nameType)
	if err != nil {
		return nil, err
	}

	data := map[string]any{
		"name":      name,
		"name_type": string(nameType),
	}
	if langIDPtr != nil {
		data["language_id"] = dbutil.FromPgtype(*langIDPtr).String()
	}

	return &ToolResult{
		Success:   true,
		Data:      data,
		Narrative: fmt.Sprintf("Generated %s name: %q.", nameType, name),
	}, nil
}

// parsePhonologicalRules extracts vowels and consonants from the language's
// Phonology JSONB. Missing or malformed values fall back to defaults.
func parsePhonologicalRules(phonologyJSON []byte) (vowels, consonants []string) {
	vowels = defaultVowels
	consonants = defaultConsonants

	if len(phonologyJSON) == 0 {
		return
	}

	var rules struct {
		Vowels     []string `json:"vowels"`
		Consonants []string `json:"consonants"`
	}
	if err := json.Unmarshal(phonologyJSON, &rules); err != nil {
		return
	}
	if len(rules.Vowels) > 0 {
		vowels = rules.Vowels
	}
	if len(rules.Consonants) > 0 {
		consonants = rules.Consonants
	}
	return
}

// pickPatternFromNaming extracts a pattern for nameType from the language's
// Naming JSONB, falling back to the default pattern set.
func pickPatternFromNaming(namingJSON []byte, nameType NameType) string {
	if len(namingJSON) == 0 {
		return pickPattern(nil, nameType)
	}

	var raw map[string]any
	if err := json.Unmarshal(namingJSON, &raw); err != nil {
		return pickPattern(nil, nameType)
	}

	// Try nested structure: {person_first: {patterns: [...]}, ...}
	if nested, ok := raw[string(nameType)].(map[string]any); ok {
		if patterns := extractStringSlice(nested, "patterns"); len(patterns) > 0 {
			return patterns[rand.IntN(len(patterns))]
		}
	}

	// Try flat keys per type: person_first_patterns, person_last_patterns, place_patterns, object_patterns, etc.
	flatKey := flatPatternKey(nameType)
	if patterns := extractStringSlice(raw, flatKey); len(patterns) > 0 {
		return patterns[rand.IntN(len(patterns))]
	}

	// Try top-level "patterns" as a catch-all.
	if patterns := extractStringSlice(raw, "patterns"); len(patterns) > 0 {
		return patterns[rand.IntN(len(patterns))]
	}

	return pickPattern(nil, nameType)
}

// flatPatternKey maps a NameType to the flat naming-convention key used when
// the naming JSON is stored without nesting (e.g. "person_first_patterns").
func flatPatternKey(nt NameType) string {
	switch nt {
	case NameTypePersonFirst:
		return "person_first_patterns"
	case NameTypePersonLast:
		return "person_last_patterns"
	case NameTypePlace:
		return "place_patterns"
	case NameTypeObject:
		return "object_patterns"
	default:
		return "patterns"
	}
}

// extractStringSlice safely retrieves a []string from a nested map value.
func extractStringSlice(m map[string]any, key string) []string {
	raw, ok := m[key]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// pickPattern selects a random pattern from the defaults for the given nameType.
// When patterns is non-nil, it picks from that slice instead.
func pickPattern(patterns []string, nameType NameType) string {
	if len(patterns) > 0 {
		return patterns[rand.IntN(len(patterns))]
	}
	defs := defaultPatterns[nameType]
	if len(defs) == 0 {
		defs = []string{"CV-CV"}
	}
	return defs[rand.IntN(len(defs))]
}

// applyPattern resolves a pattern string into a name.
// In the pattern, 'C' is replaced by a random consonant from consonants,
// 'V' is replaced by a random vowel from vowels, and '-' (syllable separator)
// is discarded. All other characters are kept as-is.
func applyPattern(vowels, consonants []string, pattern string) string {
	if len(vowels) == 0 {
		vowels = defaultVowels
	}
	if len(consonants) == 0 {
		consonants = defaultConsonants
	}

	var sb strings.Builder
	for _, ch := range pattern {
		switch ch {
		case 'C':
			sb.WriteString(consonants[rand.IntN(len(consonants))])
		case 'V':
			sb.WriteString(vowels[rand.IntN(len(vowels))])
		case '-':
			// syllable separator — discard
		default:
			sb.WriteRune(ch)
		}
	}

	name := sb.String()
	if name == "" {
		// ultimate fallback: two-syllable generic name
		v := vowels[rand.IntN(len(vowels))]
		c := consonants[rand.IntN(len(consonants))]
		name = c + v + c + vowels[rand.IntN(len(vowels))]
	}

	return capitalize(name)
}

// capitalize upper-cases the first rune of s.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	if runes[0] >= 'a' && runes[0] <= 'z' {
		runes[0] = runes[0] - 'a' + 'A'
	}
	return string(runes)
}
