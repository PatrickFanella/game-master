package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/llm"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

const createCultureToolName = "create_culture"

// CultureStore persists cultures and validates related world entities.
type CultureStore interface {
	CreateCulture(ctx context.Context, arg statedb.CreateCultureParams) (statedb.Culture, error)
	GetLanguageByID(ctx context.Context, id pgtype.UUID) (statedb.Language, error)
	GetBeliefSystemByID(ctx context.Context, id pgtype.UUID) (statedb.BeliefSystem, error)
	GetFactionByID(ctx context.Context, id pgtype.UUID) (statedb.Faction, error)
}

// CreateCultureTool returns the create_culture tool definition and JSON schema.
func CreateCultureTool() llm.Tool {
	return llm.Tool{
		Name:        createCultureToolName,
		Description: "Create a world culture with values, customs, social norms, artistic forms, taboos, greetings, and linked institutions.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Culture name.",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Culture description.",
				},
				"values": map[string]any{
					"type":        "array",
					"description": "Core values upheld by the culture.",
					"items": map[string]any{
						"type": "string",
					},
				},
				"customs": map[string]any{
					"type":        "array",
					"description": "Common customs and rituals.",
					"items": map[string]any{
						"type": "string",
					},
				},
				"social_norms": map[string]any{
					"type":        "array",
					"description": "Expected social behaviors and etiquette.",
					"items": map[string]any{
						"type": "string",
					},
				},
				"art_forms": map[string]any{
					"type":        "array",
					"description": "Representative arts tied to this culture.",
					"items": map[string]any{
						"type": "string",
					},
				},
				"taboos": map[string]any{
					"type":        "array",
					"description": "Socially prohibited actions or beliefs.",
					"items": map[string]any{
						"type": "string",
					},
				},
				"greeting_customs": map[string]any{
					"type":        "array",
					"description": "How members of this culture commonly greet others.",
					"items": map[string]any{
						"type": "string",
					},
				},
				"language_id": map[string]any{
					"type":        "string",
					"description": "Language UUID linked to this culture.",
				},
				"belief_system_id": map[string]any{
					"type":        "string",
					"description": "Belief system UUID linked to this culture.",
				},
				"associated_factions": map[string]any{
					"type":        "array",
					"description": "Faction UUIDs associated with this culture.",
					"items": map[string]any{
						"type": "string",
					},
				},
			},
			"required": []string{
				"name",
				"description",
				"values",
				"customs",
				"social_norms",
				"art_forms",
				"taboos",
				"greeting_customs",
				"language_id",
				"belief_system_id",
				"associated_factions",
			},
			"additionalProperties": false,
		},
	}
}

// RegisterCreateCulture registers the create_culture tool and handler.
func RegisterCreateCulture(reg *Registry, cultureStore CultureStore, memoryStore MemoryStore, embedder Embedder) error {
	if cultureStore == nil {
		return errors.New("create_culture culture store is required")
	}
	return reg.Register(CreateCultureTool(), NewCreateCultureHandler(cultureStore, memoryStore, embedder).Handle)
}

// CreateCultureHandler executes create_culture tool calls.
type CreateCultureHandler struct {
	cultureStore CultureStore
	memoryStore  MemoryStore
	embedder     Embedder
}

// NewCreateCultureHandler creates a new create_culture handler.
func NewCreateCultureHandler(cultureStore CultureStore, memoryStore MemoryStore, embedder Embedder) *CreateCultureHandler {
	return &CreateCultureHandler{
		cultureStore: cultureStore,
		memoryStore:  memoryStore,
		embedder:     embedder,
	}
}

// Handle executes the create_culture tool.
func (h *CreateCultureHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("create_culture handler is nil")
	}
	if h.cultureStore == nil {
		return nil, errors.New("create_culture culture store is required")
	}

	name, err := parseStringArg(args, "name")
	if err != nil {
		return nil, err
	}
	description, err := parseStringArg(args, "description")
	if err != nil {
		return nil, err
	}
	values, err := parseStringArrayArg(args, "values")
	if err != nil {
		return nil, err
	}
	customs, err := parseStringArrayArg(args, "customs")
	if err != nil {
		return nil, err
	}
	socialNorms, err := parseStringArrayArg(args, "social_norms")
	if err != nil {
		return nil, err
	}
	artForms, err := parseStringArrayArg(args, "art_forms")
	if err != nil {
		return nil, err
	}
	taboos, err := parseStringArrayArg(args, "taboos")
	if err != nil {
		return nil, err
	}
	greetingCustoms, err := parseStringArrayArg(args, "greeting_customs")
	if err != nil {
		return nil, err
	}
	languageID, err := parseUUIDArg(args, "language_id")
	if err != nil {
		return nil, err
	}
	beliefSystemID, err := parseUUIDArg(args, "belief_system_id")
	if err != nil {
		return nil, err
	}
	associatedFactions, err := parseRequiredUUIDArrayArg(args, "associated_factions")
	if err != nil {
		return nil, err
	}

	language, err := h.cultureStore.GetLanguageByID(ctx, dbutil.ToPgtype(languageID))
	if err != nil {
		return nil, fmt.Errorf("validate language_id: %w", err)
	}
	beliefSystem, err := h.cultureStore.GetBeliefSystemByID(ctx, dbutil.ToPgtype(beliefSystemID))
	if err != nil {
		return nil, fmt.Errorf("validate belief_system_id: %w", err)
	}
	if language.CampaignID != beliefSystem.CampaignID {
		return nil, errors.New("language_id and belief_system_id must belong to the same campaign")
	}

	if err := h.validateAssociatedFactions(ctx, language.CampaignID, associatedFactions); err != nil {
		return nil, err
	}

	details := map[string]any{
		"description":            description,
		"values":                 values,
		"customs":                customs,
		"social_norms":           socialNorms,
		"art_forms":              artForms,
		"taboos":                 taboos,
		"greeting_customs":       greetingCustoms,
		"associated_faction_ids": uuidsToStrings(associatedFactions),
	}
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return nil, fmt.Errorf("marshal culture details: %w", err)
	}

	culture, err := h.cultureStore.CreateCulture(ctx, statedb.CreateCultureParams{
		CampaignID:     language.CampaignID,
		LanguageID:     dbutil.ToPgtype(languageID),
		BeliefSystemID: dbutil.ToPgtype(beliefSystemID),
		Name:           name,
		Details:        detailsJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("create culture: %w", err)
	}

	campaignID := dbutil.FromPgtype(language.CampaignID)
	cultureID := dbutil.FromPgtype(culture.ID)
	if h.embedder != nil && h.memoryStore != nil {
		if err := h.embedCultureMemory(
			ctx,
			campaignID,
			cultureID,
			name,
			description,
			values,
			customs,
			socialNorms,
			artForms,
			taboos,
			greetingCustoms,
			languageID,
			beliefSystemID,
			associatedFactions,
		); err != nil {
			return nil, err
		}
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"id":                  cultureID.String(),
			"campaign_id":         campaignID.String(),
			"language_id":         languageID.String(),
			"belief_system_id":    beliefSystemID.String(),
			"name":                name,
			"description":         description,
			"values":              values,
			"customs":             customs,
			"social_norms":        socialNorms,
			"art_forms":           artForms,
			"taboos":              taboos,
			"greeting_customs":    greetingCustoms,
			"associated_factions": uuidsToStrings(associatedFactions),
		},
		Narrative: fmt.Sprintf("Culture %q created successfully.", name),
	}, nil
}

func parseRequiredUUIDArrayArg(args map[string]any, key string) ([]uuid.UUID, error) {
	if _, ok := args[key]; !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	return parseUUIDArrayArg(args, key)
}

func (h *CreateCultureHandler) validateAssociatedFactions(ctx context.Context, campaignID pgtype.UUID, factionIDs []uuid.UUID) error {
	for i, factionID := range factionIDs {
		faction, err := h.cultureStore.GetFactionByID(ctx, dbutil.ToPgtype(factionID))
		if err != nil {
			return fmt.Errorf("validate associated_factions[%d]: %w", i, err)
		}
		if faction.CampaignID != campaignID {
			return fmt.Errorf("associated_factions[%d] must belong to campaign_id", i)
		}
	}
	return nil
}

func (h *CreateCultureHandler) embedCultureMemory(
	ctx context.Context,
	campaignID uuid.UUID,
	cultureID uuid.UUID,
	name string,
	description string,
	values []string,
	customs []string,
	socialNorms []string,
	artForms []string,
	taboos []string,
	greetingCustoms []string,
	languageID uuid.UUID,
	beliefSystemID uuid.UUID,
	associatedFactions []uuid.UUID,
) error {
	memoryContent := fmt.Sprintf(
		"Culture created: %s. Description: %s. Values: %s. Customs: %s. Social norms: %s. Art forms: %s. Taboos: %s. Greeting customs: %s.",
		name,
		description,
		strings.Join(values, ", "),
		strings.Join(customs, ", "),
		strings.Join(socialNorms, ", "),
		strings.Join(artForms, ", "),
		strings.Join(taboos, ", "),
		strings.Join(greetingCustoms, ", "),
	)
	embedding, err := h.embedder.Embed(ctx, memoryContent)
	if err != nil {
		return fmt.Errorf("embed culture memory: %w", err)
	}
	metadata, err := json.Marshal(map[string]any{
		"culture_id":             cultureID.String(),
		"language_id":            languageID.String(),
		"belief_system_id":       beliefSystemID.String(),
		"associated_faction_ids": uuidsToStrings(associatedFactions),
	})
	if err != nil {
		return fmt.Errorf("marshal culture memory metadata: %w", err)
	}
	if err := h.memoryStore.CreateMemory(ctx, CreateMemoryParams{
		CampaignID: campaignID,
		Content:    memoryContent,
		Embedding:  embedding,
		MemoryType: string(domain.MemoryTypeWorldFact),
		Metadata:   metadata,
	}); err != nil {
		return fmt.Errorf("create culture memory: %w", err)
	}
	return nil
}
