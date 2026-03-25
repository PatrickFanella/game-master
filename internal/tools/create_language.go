package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/llm"
)

const createLanguageToolName = "create_language"

// CreateLanguageParams holds the parameters for creating a language.
type CreateLanguageParams struct {
	CampaignID         uuid.UUID
	Name               string
	Description        string
	PhonologicalRules  json.RawMessage
	NamingConventions  json.RawMessage
	SampleVocabulary   json.RawMessage
	SpokenByFactionIDs []uuid.UUID
	SpokenByCultureIDs []uuid.UUID
}

// LanguageStore persists language records using domain types.
type LanguageStore interface {
	CreateLanguage(ctx context.Context, params CreateLanguageParams) (uuid.UUID, error)
	FactionBelongsToCampaign(ctx context.Context, factionID, campaignID uuid.UUID) (bool, error)
	CultureBelongsToCampaign(ctx context.Context, cultureID, campaignID uuid.UUID) (bool, error)
}

// CreateMemoryParams holds the parameters for creating a semantic memory.
type CreateMemoryParams struct {
	CampaignID uuid.UUID
	Content    string
	Embedding  []float32
	MemoryType string
	Metadata   json.RawMessage
}

// MemoryStore persists semantic memories using domain types.
type MemoryStore interface {
	CreateMemory(ctx context.Context, params CreateMemoryParams) error
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
// The memoryStore and embedder parameters are optional; when nil, embedding
// is skipped (suitable for Phase 2 before embedding infrastructure exists).
func RegisterCreateLanguage(reg *Registry, languageStore LanguageStore, memoryStore MemoryStore, embedder Embedder) error {
	if languageStore == nil {
		return errors.New("create_language language store is required")
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
func (h *CreateLanguageHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("create_language handler is nil")
	}
	if h.languageStore == nil {
		return nil, errors.New("create_language language store is required")
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

	if err := h.validateSpeakerIDs(ctx, campaignID, spokenByFactionIDs, spokenByCultureIDs); err != nil {
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

	languageID, err := h.languageStore.CreateLanguage(ctx, CreateLanguageParams{
		CampaignID:         campaignID,
		Name:               name,
		Description:        description,
		PhonologicalRules:  phonologyJSON,
		NamingConventions:  namingJSON,
		SampleVocabulary:   vocabularyJSON,
		SpokenByFactionIDs: spokenByFactionIDs,
		SpokenByCultureIDs: spokenByCultureIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("create language: %w", err)
	}

	if h.embedder != nil && h.memoryStore != nil {
		if err := h.embedLanguageMemory(ctx, campaignID, languageID, name, description, phonologicalRules, namingConventions, sampleVocabulary, spokenByFactionIDs, spokenByCultureIDs); err != nil {
			return nil, err
		}
	}

	data := map[string]any{
		"id":                    languageID.String(),
		"campaign_id":           campaignID.String(),
		"name":                  name,
		"description":           description,
		"phonological_rules":    phonologicalRules,
		"naming_conventions":    namingConventions,
		"sample_vocabulary":     sampleVocabulary,
		"spoken_by_faction_ids": uuidsToStrings(spokenByFactionIDs),
		"spoken_by_culture_ids": uuidsToStrings(spokenByCultureIDs),
	}

	return &ToolResult{
		Success:   true,
		Data:      data,
		Narrative: fmt.Sprintf("Language %q created successfully.", name),
	}, nil
}

func (h *CreateLanguageHandler) embedLanguageMemory(ctx context.Context, campaignID, languageID uuid.UUID, name, description string, phonologicalRules, namingConventions, sampleVocabulary map[string]any, spokenByFactionIDs, spokenByCultureIDs []uuid.UUID) error {
	memoryContent, err := buildLanguageMemoryContent(name, description, phonologicalRules, namingConventions, sampleVocabulary)
	if err != nil {
		return fmt.Errorf("build language memory content: %w", err)
	}
	embedding, err := h.embedder.Embed(ctx, memoryContent)
	if err != nil {
		return fmt.Errorf("embed language memory: %w", err)
	}
	metadata, err := json.Marshal(map[string]any{
		"language_id":           languageID.String(),
		"spoken_by_faction_ids": uuidsToStrings(spokenByFactionIDs),
		"spoken_by_culture_ids": uuidsToStrings(spokenByCultureIDs),
	})
	if err != nil {
		return fmt.Errorf("marshal language memory metadata: %w", err)
	}

	if err := h.memoryStore.CreateMemory(ctx, CreateMemoryParams{
		CampaignID: campaignID,
		Content:    memoryContent,
		Embedding:  embedding,
		MemoryType: "world_fact",
		Metadata:   metadata,
	}); err != nil {
		return fmt.Errorf("create language memory: %w", err)
	}
	return nil
}

func (h *CreateLanguageHandler) validateSpeakerIDs(ctx context.Context, campaignID uuid.UUID, factionIDs, cultureIDs []uuid.UUID) error {
	for i, factionID := range factionIDs {
		belongs, err := h.languageStore.FactionBelongsToCampaign(ctx, factionID, campaignID)
		if err != nil {
			return fmt.Errorf("validate spoken_by_faction_ids[%d]: %w", i, err)
		}
		if !belongs {
			return fmt.Errorf("spoken_by_faction_ids[%d] must belong to campaign_id", i)
		}
	}

	for i, cultureID := range cultureIDs {
		belongs, err := h.languageStore.CultureBelongsToCampaign(ctx, cultureID, campaignID)
		if err != nil {
			return fmt.Errorf("validate spoken_by_culture_ids[%d]: %w", i, err)
		}
		if !belongs {
			return fmt.Errorf("spoken_by_culture_ids[%d] must belong to campaign_id", i)
		}
	}

	return nil
}

func uuidsToStrings(ids []uuid.UUID) []string {
	if len(ids) == 0 {
		return []string{}
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
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
