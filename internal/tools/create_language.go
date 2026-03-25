package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	pgvector "github.com/pgvector/pgvector-go"

	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/llm"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

const createLanguageToolName = "create_language"

// LanguageStore persists language records.
type LanguageStore interface {
	CreateLanguage(ctx context.Context, arg statedb.CreateLanguageParams) (statedb.Language, error)
	GetFactionByID(ctx context.Context, id pgtype.UUID) (statedb.Faction, error)
	GetCultureByID(ctx context.Context, id pgtype.UUID) (statedb.Culture, error)
}

// MemoryStore persists semantic memories.
type MemoryStore interface {
	CreateMemory(ctx context.Context, arg statedb.CreateMemoryParams) (statedb.Memory, error)
}

// Embedder generates vector embeddings for memory content.
type Embedder interface {
	Embed(ctx context.Context, input string) ([]float32, error)
}

// CreateLanguageTool returns the create_language tool definition and JSON schema.
func CreateLanguageTool() llm.Tool {
	return llm.Tool{
		Name:        createLanguageToolName,
		Description: "Create a world language including phonological rules, naming conventions, sample vocabulary, and who speaks it.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"campaign_id": map[string]any{
					"type":        "string",
					"description": "Campaign UUID that owns this language.",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Language name.",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Brief language description.",
				},
				"phonological_rules": map[string]any{
					"type":        "object",
					"description": "JSON object describing phonological rules.",
				},
				"naming_conventions": map[string]any{
					"type":        "object",
					"description": "JSON object describing person/place naming conventions.",
				},
				"sample_vocabulary": map[string]any{
					"type":        "object",
					"description": "JSON object containing sample vocabulary terms.",
				},
				"spoken_by_faction_ids": map[string]any{
					"type":        "array",
					"description": "Faction UUIDs that speak this language.",
					"items": map[string]any{
						"type": "string",
					},
				},
				"spoken_by_culture_ids": map[string]any{
					"type":        "array",
					"description": "Culture UUIDs that speak this language.",
					"items": map[string]any{
						"type": "string",
					},
				},
			},
			"required":             []string{"campaign_id", "name", "description", "phonological_rules", "naming_conventions", "sample_vocabulary"},
			"additionalProperties": false,
		},
	}
}

// RegisterCreateLanguage registers the create_language tool and handler.
func RegisterCreateLanguage(reg *Registry, languageStore LanguageStore, memoryStore MemoryStore, embedder Embedder) error {
	if languageStore == nil {
		return errors.New("create_language language store is required")
	}
	if memoryStore == nil {
		return errors.New("create_language memory store is required")
	}
	if embedder == nil {
		return errors.New("create_language embedder is required")
	}
	return reg.Register(CreateLanguageTool(), NewCreateLanguageHandler(languageStore, memoryStore, embedder).Handle)
}

// CreateLanguageHandler executes create_language tool calls.
type CreateLanguageHandler struct {
	languageStore LanguageStore
	memoryStore   MemoryStore
	embedder      Embedder
}

// NewCreateLanguageHandler creates a new create_language handler.
func NewCreateLanguageHandler(languageStore LanguageStore, memoryStore MemoryStore, embedder Embedder) *CreateLanguageHandler {
	return &CreateLanguageHandler{
		languageStore: languageStore,
		memoryStore:   memoryStore,
		embedder:      embedder,
	}
}

// Handle executes the create_language tool.
func (h *CreateLanguageHandler) Handle(ctx context.Context, args map[string]any) (map[string]any, error) {
	if h == nil {
		return nil, errors.New("create_language handler is nil")
	}
	if h.languageStore == nil {
		return nil, errors.New("create_language language store is required")
	}
	if h.memoryStore == nil {
		return nil, errors.New("create_language memory store is required")
	}
	if h.embedder == nil {
		return nil, errors.New("create_language embedder is required")
	}

	campaignID, err := parseUUIDArg(args, "campaign_id")
	if err != nil {
		return nil, err
	}
	name, err := parseStringArg(args, "name")
	if err != nil {
		return nil, err
	}
	description, err := parseStringArg(args, "description")
	if err != nil {
		return nil, err
	}
	phonologicalRules, err := parseJSONObjectArg(args, "phonological_rules")
	if err != nil {
		return nil, err
	}
	namingConventions, err := parseJSONObjectArg(args, "naming_conventions")
	if err != nil {
		return nil, err
	}
	sampleVocabulary, err := parseJSONObjectArg(args, "sample_vocabulary")
	if err != nil {
		return nil, err
	}
	spokenByFactionIDs, err := parseUUIDArrayArg(args, "spoken_by_faction_ids")
	if err != nil {
		return nil, err
	}
	spokenByCultureIDs, err := parseUUIDArrayArg(args, "spoken_by_culture_ids")
	if err != nil {
		return nil, err
	}

	if err := h.validateSpeakerIDs(ctx, uuidToPgtype(campaignID), spokenByFactionIDs, spokenByCultureIDs); err != nil {
		return nil, err
	}

	phonologyJSON, err := json.Marshal(phonologicalRules)
	if err != nil {
		return nil, fmt.Errorf("marshal phonological_rules: %w", err)
	}
	namingJSON, err := json.Marshal(namingConventions)
	if err != nil {
		return nil, fmt.Errorf("marshal naming_conventions: %w", err)
	}
	vocabularyJSON, err := json.Marshal(sampleVocabulary)
	if err != nil {
		return nil, fmt.Errorf("marshal sample_vocabulary: %w", err)
	}

	dbCampaignID := uuidToPgtype(campaignID)
	language, err := h.languageStore.CreateLanguage(ctx, statedb.CreateLanguageParams{
		CampaignID:         dbCampaignID,
		Name:               name,
		Description:        pgtype.Text{String: description, Valid: true},
		Phonology:          phonologyJSON,
		Naming:             namingJSON,
		Vocabulary:         vocabularyJSON,
		SpokenByFactionIds: spokenByFactionIDs,
		SpokenByCultureIds: spokenByCultureIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("create language: %w", err)
	}

	memoryContent, err := buildLanguageMemoryContent(name, description, phonologicalRules, namingConventions, sampleVocabulary)
	if err != nil {
		return nil, fmt.Errorf("build language memory content: %w", err)
	}
	embedding, err := h.embedder.Embed(ctx, memoryContent)
	if err != nil {
		return nil, fmt.Errorf("embed language memory: %w", err)
	}
	metadata, err := json.Marshal(map[string]any{
		"language_id":           uuidFromPgtype(language.ID).String(),
		"spoken_by_faction_ids": pgUUIDsToStrings(spokenByFactionIDs),
		"spoken_by_culture_ids": pgUUIDsToStrings(spokenByCultureIDs),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal language memory metadata: %w", err)
	}

	if _, err := h.memoryStore.CreateMemory(ctx, statedb.CreateMemoryParams{
		CampaignID: dbCampaignID,
		Content:    memoryContent,
		Embedding:  pgvector.NewVector(embedding),
		MemoryType: string(domain.MemoryTypeWorldFact),
		Metadata:   metadata,
	}); err != nil {
		return nil, fmt.Errorf("create language memory: %w", err)
	}

	return map[string]any{
		"id":                    uuidFromPgtype(language.ID).String(),
		"campaign_id":           uuidFromPgtype(language.CampaignID).String(),
		"name":                  language.Name,
		"description":           language.Description,
		"phonological_rules":    phonologicalRules,
		"naming_conventions":    namingConventions,
		"sample_vocabulary":     sampleVocabulary,
		"spoken_by_faction_ids": pgUUIDsToStrings(language.SpokenByFactionIds),
		"spoken_by_culture_ids": pgUUIDsToStrings(language.SpokenByCultureIds),
	}, nil
}

func parseJSONObjectArg(args map[string]any, key string) (map[string]any, error) {
	raw, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", key)
	}
	return obj, nil
}

func parseUUIDArrayArg(args map[string]any, key string) ([]pgtype.UUID, error) {
	raw, ok := args[key]
	if !ok {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}

	out := make([]pgtype.UUID, 0, len(items))
	for i, item := range items {
		s, ok := item.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, fmt.Errorf("%s[%d] must be a non-empty string UUID", key, i)
		}
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("%s[%d] must be a valid UUID", key, i)
		}
		out = append(out, uuidToPgtype(id))
	}
	return out, nil
}

func (h *CreateLanguageHandler) validateSpeakerIDs(ctx context.Context, campaignID pgtype.UUID, factionIDs, cultureIDs []pgtype.UUID) error {
	for i, factionID := range factionIDs {
		faction, err := h.languageStore.GetFactionByID(ctx, factionID)
		if err != nil {
			return fmt.Errorf("validate spoken_by_faction_ids[%d]: %w", i, err)
		}
		if faction.CampaignID != campaignID {
			return fmt.Errorf("spoken_by_faction_ids[%d] must belong to campaign_id", i)
		}
	}

	for i, cultureID := range cultureIDs {
		culture, err := h.languageStore.GetCultureByID(ctx, cultureID)
		if err != nil {
			return fmt.Errorf("validate spoken_by_culture_ids[%d]: %w", i, err)
		}
		if culture.CampaignID != campaignID {
			return fmt.Errorf("spoken_by_culture_ids[%d] must belong to campaign_id", i)
		}
	}

	return nil
}

func uuidToPgtype(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: u != uuid.Nil}
}

func uuidFromPgtype(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

func pgUUIDsToStrings(ids []pgtype.UUID) []string {
	if len(ids) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !id.Valid {
			continue
		}
		out = append(out, uuidFromPgtype(id).String())
	}
	return out
}

func buildLanguageMemoryContent(name, description string, phonologicalRules, namingConventions, sampleVocabulary map[string]any) (string, error) {
	phonologyJSON, err := json.Marshal(phonologicalRules)
	if err != nil {
		return "", err
	}
	namingJSON, err := json.Marshal(namingConventions)
	if err != nil {
		return "", err
	}
	vocabularyJSON, err := json.Marshal(sampleVocabulary)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"Language created: %s. Description: %s. Phonological rules: %s. Naming conventions: %s. Sample vocabulary: %s.",
		name,
		description,
		string(phonologyJSON),
		string(namingJSON),
		string(vocabularyJSON),
	), nil
}
