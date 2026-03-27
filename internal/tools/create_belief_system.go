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

const createBeliefSystemToolName = "create_belief_system"
const beliefSystemFactCount = 5

// BeliefSystemStore persists belief systems and related world facts.
type BeliefSystemStore interface {
	CreateBeliefSystem(ctx context.Context, arg statedb.CreateBeliefSystemParams) (statedb.BeliefSystem, error)
	CreateFact(ctx context.Context, arg statedb.CreateFactParams) (statedb.WorldFact, error)
	GetFactionByID(ctx context.Context, id pgtype.UUID) (statedb.Faction, error)
	GetCultureByID(ctx context.Context, id pgtype.UUID) (statedb.Culture, error)
}

// CreateBeliefSystemTool returns the create_belief_system tool definition and JSON schema.
func CreateBeliefSystemTool() llm.Tool {
	return llm.Tool{
		Name:        createBeliefSystemToolName,
		Description: "Create a world belief system with core tenets, institutions, and follower groups.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"campaign_id": map[string]any{
					"type":        "string",
					"description": "Campaign UUID that owns this belief system.",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Belief system name.",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Belief system description.",
				},
				"deities_or_principles": map[string]any{
					"type":        "array",
					"description": "Core deities or philosophical principles.",
					"items": map[string]any{
						"type": "string",
					},
				},
				"practices": map[string]any{
					"type":        "array",
					"description": "Common practices and rituals.",
					"items": map[string]any{
						"type": "string",
					},
				},
				"institutions": map[string]any{
					"type":        "array",
					"description": "Organizations tied to the belief system.",
					"items": map[string]any{
						"type": "string",
					},
				},
				"moral_framework": map[string]any{
					"type":        "object",
					"description": "JSON object describing moral framework and ethics.",
				},
				"taboos": map[string]any{
					"type":        "array",
					"description": "Forbidden acts or taboos.",
					"items": map[string]any{
						"type": "string",
					},
				},
				"followers": map[string]any{
					"type":        "object",
					"description": "Follower linkage to factions and cultures.",
					"properties": map[string]any{
						"faction_ids": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
						},
						"culture_ids": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
						},
					},
					"additionalProperties": false,
				},
			},
			"required": []string{
				"campaign_id",
				"name",
				"description",
				"deities_or_principles",
				"practices",
				"institutions",
				"moral_framework",
				"taboos",
				"followers",
			},
			"additionalProperties": false,
		},
	}
}

// RegisterCreateBeliefSystem registers the create_belief_system tool and handler.
func RegisterCreateBeliefSystem(reg *Registry, beliefStore BeliefSystemStore, memoryStore MemoryStore, embedder Embedder) error {
	if beliefStore == nil {
		return errors.New("create_belief_system belief store is required")
	}
	return reg.Register(CreateBeliefSystemTool(), NewCreateBeliefSystemHandler(beliefStore, memoryStore, embedder).Handle)
}

// CreateBeliefSystemHandler executes create_belief_system tool calls.
type CreateBeliefSystemHandler struct {
	beliefStore BeliefSystemStore
	memoryStore MemoryStore
	embedder    Embedder
}

// NewCreateBeliefSystemHandler creates a new create_belief_system handler.
func NewCreateBeliefSystemHandler(beliefStore BeliefSystemStore, memoryStore MemoryStore, embedder Embedder) *CreateBeliefSystemHandler {
	return &CreateBeliefSystemHandler{
		beliefStore: beliefStore,
		memoryStore: memoryStore,
		embedder:    embedder,
	}
}

// Handle executes the create_belief_system tool.
func (h *CreateBeliefSystemHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("create_belief_system handler is nil")
	}
	if h.beliefStore == nil {
		return nil, errors.New("create_belief_system belief store is required")
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
	deitiesOrPrinciples, err := parseStringArrayArg(args, "deities_or_principles")
	if err != nil {
		return nil, err
	}
	practices, err := parseStringArrayArg(args, "practices")
	if err != nil {
		return nil, err
	}
	institutions, err := parseStringArrayArg(args, "institutions")
	if err != nil {
		return nil, err
	}
	moralFramework, err := parseJSONObjectArg(args, "moral_framework")
	if err != nil {
		return nil, err
	}
	taboos, err := parseStringArrayArg(args, "taboos")
	if err != nil {
		return nil, err
	}
	followers, err := parseJSONObjectArg(args, "followers")
	if err != nil {
		return nil, err
	}
	followerFactionIDs, err := parseUUIDArrayFromObject(followers, "faction_ids")
	if err != nil {
		return nil, err
	}
	followerCultureIDs, err := parseUUIDArrayFromObject(followers, "culture_ids")
	if err != nil {
		return nil, err
	}

	dbCampaignID := dbutil.ToPgtype(campaignID)
	if err := h.validateFollowerIDs(ctx, dbCampaignID, followerFactionIDs, followerCultureIDs); err != nil {
		return nil, err
	}

	details := map[string]any{
		"description":            description,
		"deities_or_principles":  deitiesOrPrinciples,
		"practices":              practices,
		"institutions":           institutions,
		"moral_framework":        moralFramework,
		"taboos":                 taboos,
		"follower_faction_ids":   dbutil.PgUUIDsToStrings(followerFactionIDs),
		"follower_culture_ids":   dbutil.PgUUIDsToStrings(followerCultureIDs),
	}
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return nil, fmt.Errorf("marshal belief system details: %w", err)
	}

	beliefSystem, err := h.beliefStore.CreateBeliefSystem(ctx, statedb.CreateBeliefSystemParams{
		CampaignID: dbCampaignID,
		Name:       name,
		Details:    detailsJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("create belief system: %w", err)
	}

	for i, fact := range buildBeliefSystemFacts(name, deitiesOrPrinciples, practices, institutions, moralFramework, taboos) {
		if _, err := h.beliefStore.CreateFact(ctx, statedb.CreateFactParams{
			CampaignID: dbCampaignID,
			Fact:       fact,
			Category:   "belief_system",
			Source:     fmt.Sprintf("create_belief_system:%s", dbutil.FromPgtype(beliefSystem.ID).String()),
		}); err != nil {
			return nil, fmt.Errorf("create belief system world_fact[%d]: %w", i, err)
		}
	}

	if h.embedder != nil && h.memoryStore != nil {
		memoryContent, err := buildBeliefSystemMemoryContent(name, description, deitiesOrPrinciples, practices, institutions, moralFramework, taboos)
		if err != nil {
			return nil, fmt.Errorf("build belief system memory content: %w", err)
		}
		embedding, err := h.embedder.Embed(ctx, memoryContent)
		if err != nil {
			return nil, fmt.Errorf("embed belief system memory: %w", err)
		}
		metadata, err := json.Marshal(map[string]any{
			"belief_system_id":     dbutil.FromPgtype(beliefSystem.ID).String(),
			"follower_faction_ids": dbutil.PgUUIDsToStrings(followerFactionIDs),
			"follower_culture_ids": dbutil.PgUUIDsToStrings(followerCultureIDs),
		})
		if err != nil {
			return nil, fmt.Errorf("marshal belief system memory metadata: %w", err)
		}
		if err := h.memoryStore.CreateMemory(ctx, CreateMemoryParams{
			CampaignID: campaignID,
			Content:    memoryContent,
			Embedding:  embedding,
			MemoryType: string(domain.MemoryTypeWorldFact),
			Metadata:   metadata,
		}); err != nil {
			return nil, fmt.Errorf("create belief system memory: %w", err)
		}
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"id":                    dbutil.FromPgtype(beliefSystem.ID).String(),
			"campaign_id":           dbutil.FromPgtype(beliefSystem.CampaignID).String(),
		"name":                  beliefSystem.Name,
		"description":           description,
		"deities_or_principles": deitiesOrPrinciples,
		"practices":             practices,
		"institutions":          institutions,
		"moral_framework":       moralFramework,
		"taboos":                taboos,
		"followers": map[string]any{
				"faction_ids": dbutil.PgUUIDsToStrings(followerFactionIDs),
				"culture_ids": dbutil.PgUUIDsToStrings(followerCultureIDs),
		},
		},
		Narrative: fmt.Sprintf("Belief system %q created successfully.", beliefSystem.Name),
	}, nil
}

func parseUUIDArrayFromObject(obj map[string]any, key string) ([]pgtype.UUID, error) {
	raw, ok := obj[key]
	if !ok {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("followers.%s must be an array", key)
	}

	out := make([]pgtype.UUID, 0, len(items))
	for i, item := range items {
		s, ok := item.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, fmt.Errorf("followers.%s[%d] must be a non-empty string UUID", key, i)
		}
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("followers.%s[%d] must be a valid UUID", key, i)
		}
		out = append(out, dbutil.ToPgtype(id))
	}
	return out, nil
}

func (h *CreateBeliefSystemHandler) validateFollowerIDs(ctx context.Context, campaignID pgtype.UUID, factionIDs, cultureIDs []pgtype.UUID) error {
	for i, factionID := range factionIDs {
		faction, err := h.beliefStore.GetFactionByID(ctx, factionID)
		if err != nil {
			return fmt.Errorf("validate followers.faction_ids[%d]: %w", i, err)
		}
		if faction.CampaignID != campaignID {
			return fmt.Errorf("followers.faction_ids[%d] must belong to campaign_id", i)
		}
	}
	for i, cultureID := range cultureIDs {
		culture, err := h.beliefStore.GetCultureByID(ctx, cultureID)
		if err != nil {
			return fmt.Errorf("validate followers.culture_ids[%d]: %w", i, err)
		}
		if culture.CampaignID != campaignID {
			return fmt.Errorf("followers.culture_ids[%d] must belong to campaign_id", i)
		}
	}
	return nil
}

func buildBeliefSystemFacts(name string, deitiesOrPrinciples, practices, institutions []string, moralFramework map[string]any, taboos []string) []string {
	facts := make([]string, 0, beliefSystemFactCount)
	if len(deitiesOrPrinciples) > 0 {
		facts = append(facts, fmt.Sprintf("%s deities_or_principles: %s.", name, strings.Join(deitiesOrPrinciples, ", ")))
	}
	if len(practices) > 0 {
		facts = append(facts, fmt.Sprintf("%s practices: %s.", name, strings.Join(practices, ", ")))
	}
	if len(institutions) > 0 {
		facts = append(facts, fmt.Sprintf("%s institutions: %s.", name, strings.Join(institutions, ", ")))
	}
	if len(moralFramework) > 0 {
		moralJSON, err := json.Marshal(moralFramework)
		if err == nil {
			facts = append(facts, fmt.Sprintf("%s moral_framework: %s.", name, string(moralJSON)))
		}
	}
	if len(taboos) > 0 {
		facts = append(facts, fmt.Sprintf("%s taboos: %s.", name, strings.Join(taboos, ", ")))
	}
	return facts
}

func buildBeliefSystemMemoryContent(name, description string, deitiesOrPrinciples, practices, institutions []string, moralFramework map[string]any, taboos []string) (string, error) {
	moralJSON, err := json.Marshal(moralFramework)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"Belief system created: %s. Description: %s. Deities or principles: %s. Practices: %s. Institutions: %s. Moral framework: %s. Taboos: %s.",
		name,
		description,
		strings.Join(deitiesOrPrinciples, ", "),
		strings.Join(practices, ", "),
		strings.Join(institutions, ", "),
		string(moralJSON),
		strings.Join(taboos, ", "),
	), nil
}
