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

const (
	createLoreToolName               = "create_lore"
	loreSource                       = "lore"
	loreRelationshipSourceEntityType = "world_fact"
	loreRelationshipType             = "lore_related"
)

var allowedLoreCategories = map[string]struct{}{
	"history":   {},
	"legend":    {},
	"cultural":  {},
	"political": {},
	"magical":   {},
	"religious": {},
}

type loreRelatedEntityInput struct {
	EntityType string
	EntityID   uuid.UUID
}

// LoreStore persists lore facts and their related entity links.
type LoreStore interface {
	CreateFact(ctx context.Context, arg statedb.CreateFactParams) (statedb.WorldFact, error)
	CreateRelationship(ctx context.Context, arg statedb.CreateRelationshipParams) (statedb.EntityRelationship, error)
	GetLocationByID(ctx context.Context, id pgtype.UUID) (statedb.Location, error)
}

// CreateLoreTool returns the create_lore tool definition and JSON schema.
func CreateLoreTool() llm.Tool {
	return llm.Tool{
		Name:        createLoreToolName,
		Description: "Record lore that may be incomplete, biased, or unreliable while still making it retrievable for narrative use.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"content": map[string]any{
					"type":        "string",
					"description": "The lore text to record for later retrieval and narrative inclusion.",
				},
				"category": map[string]any{
					"type":        "string",
					"description": "Lore category.",
					"enum":        []string{"history", "legend", "cultural", "political", "magical", "religious"},
				},
				"related_entities": map[string]any{
					"type":        "array",
					"description": "Optional entities that this lore relates to.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"entity_type": map[string]any{
								"type":        "string",
								"description": "Entity type (npc, location, faction, player_character, item, or another supported entity type).",
							},
							"entity_id": map[string]any{
								"type":        "string",
								"description": "Entity UUID.",
							},
						},
						"required":             []string{"entity_type", "entity_id"},
						"additionalProperties": false,
					},
				},
			},
			"required":             []string{"content", "category"},
			"additionalProperties": false,
		},
	}
}

// RegisterCreateLore registers the create_lore tool and handler.
func RegisterCreateLore(reg *Registry, loreStore LoreStore, memoryStore MemoryStore, embedder Embedder) error {
	if loreStore == nil {
		return errors.New("create_lore lore store is required")
	}
	return reg.Register(CreateLoreTool(), NewCreateLoreHandler(loreStore, memoryStore, embedder).Handle)
}

// CreateLoreHandler executes create_lore tool calls.
type CreateLoreHandler struct {
	loreStore   LoreStore
	memoryStore MemoryStore
	embedder    Embedder
}

// NewCreateLoreHandler creates a new create_lore handler.
func NewCreateLoreHandler(loreStore LoreStore, memoryStore MemoryStore, embedder Embedder) *CreateLoreHandler {
	return &CreateLoreHandler{
		loreStore:   loreStore,
		memoryStore: memoryStore,
		embedder:    embedder,
	}
}

// Handle executes the create_lore tool.
func (h *CreateLoreHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("create_lore handler is nil")
	}
	if h.loreStore == nil {
		return nil, errors.New("create_lore lore store is required")
	}

	content, err := parseStringArg(args, "content")
	if err != nil {
		return nil, err
	}
	category, err := parseLoreCategoryArg(args, "category")
	if err != nil {
		return nil, err
	}
	relatedEntities, err := parseLoreRelatedEntitiesArg(args, "related_entities")
	if err != nil {
		return nil, err
	}

	currentLocationID, ok := CurrentLocationIDFromContext(ctx)
	if !ok {
		return nil, errors.New("create_lore requires current location id in context")
	}
	currentLocation, err := h.loreStore.GetLocationByID(ctx, dbutil.ToPgtype(currentLocationID))
	if err != nil {
		return nil, fmt.Errorf("resolve campaign from current location: %w", err)
	}

	worldFact, err := h.loreStore.CreateFact(ctx, statedb.CreateFactParams{
		CampaignID: currentLocation.CampaignID,
		Fact:       strings.TrimSpace(content),
		Category:   category,
		Source:     loreSource,
	})
	if err != nil {
		return nil, fmt.Errorf("create lore world fact: %w", err)
	}

	factID := dbutil.FromPgtype(worldFact.ID)
	campaignID := dbutil.FromPgtype(worldFact.CampaignID)

	createdRelationships, err := h.createLoreRelationships(ctx, currentLocation.CampaignID, worldFact.ID, relatedEntities)
	if err != nil {
		return nil, err
	}

	if h.embedder != nil && h.memoryStore != nil {
		if err := h.embedLoreMemory(ctx, campaignID, factID, content, category, createdRelationships); err != nil {
			return nil, err
		}
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"id":               factID.String(),
			"campaign_id":      campaignID.String(),
			"content":          strings.TrimSpace(content),
			"category":         category,
			"source":           loreSource,
			"related_entities": createdRelationships,
		},
		Narrative: strings.TrimSpace(content),
	}, nil
}

func (h *CreateLoreHandler) createLoreRelationships(
	ctx context.Context,
	campaignID pgtype.UUID,
	factID pgtype.UUID,
	relatedEntities []loreRelatedEntityInput,
) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(relatedEntities))
	for i, related := range relatedEntities {
		relationship, err := h.loreStore.CreateRelationship(ctx, statedb.CreateRelationshipParams{
			CampaignID:       campaignID,
			SourceEntityType: loreRelationshipSourceEntityType,
			SourceEntityID:   factID,
			TargetEntityType: related.EntityType,
			TargetEntityID:   dbutil.ToPgtype(related.EntityID),
			RelationshipType: loreRelationshipType,
			Description:      pgtype.Text{String: "Lore relates to this entity.", Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("create related_entities[%d]: %w", i, err)
		}
		out = append(out, map[string]any{
			"id":                 dbutil.FromPgtype(relationship.ID).String(),
			"source_entity_type": relationship.SourceEntityType,
			"source_entity_id":   dbutil.FromPgtype(relationship.SourceEntityID).String(),
			"target_entity_type": relationship.TargetEntityType,
			"target_entity_id":   dbutil.FromPgtype(relationship.TargetEntityID).String(),
			"relationship_type":  relationship.RelationshipType,
		})
	}
	return out, nil
}

func (h *CreateLoreHandler) embedLoreMemory(
	ctx context.Context,
	campaignID uuid.UUID,
	factID uuid.UUID,
	content string,
	category string,
	relatedEntities []map[string]any,
) error {
	memoryContent := fmt.Sprintf("Lore (%s): %s", category, strings.TrimSpace(content))
	embedding, err := h.embedder.Embed(ctx, memoryContent)
	if err != nil {
		return fmt.Errorf("embed lore memory: %w", err)
	}
	metadata, err := json.Marshal(map[string]any{
		"fact_id":              factID.String(),
		"category":             category,
		"source":               loreSource,
		"related_entity_count": len(relatedEntities),
	})
	if err != nil {
		return fmt.Errorf("marshal lore memory metadata: %w", err)
	}
	if err := h.memoryStore.CreateMemory(ctx, CreateMemoryParams{
		CampaignID: campaignID,
		Content:    memoryContent,
		Embedding:  embedding,
		MemoryType: string(domain.MemoryTypeLore),
		Metadata:   metadata,
	}); err != nil {
		return fmt.Errorf("create lore memory: %w", err)
	}
	return nil
}

func parseLoreCategoryArg(args map[string]any, key string) (string, error) {
	value, err := parseStringArg(args, key)
	if err != nil {
		return "", err
	}
	value = strings.ToLower(strings.TrimSpace(value))
	if _, ok := allowedLoreCategories[value]; !ok {
		return "", fmt.Errorf("%s must be one of history, legend, cultural, political, magical, religious", key)
	}
	return value, nil
}

func parseLoreRelatedEntitiesArg(args map[string]any, key string) ([]loreRelatedEntityInput, error) {
	raw, ok := args[key]
	if !ok {
		return []loreRelatedEntityInput{}, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}

	out := make([]loreRelatedEntityInput, 0, len(items))
	for i, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an object", key, i)
		}
		prefix := fmt.Sprintf("%s[%d]", key, i)
		entityType, err := parseObjectStringArg(obj, "entity_type", prefix)
		if err != nil {
			return nil, err
		}
		entityID, err := parseUUIDFromNestedObject(obj, "entity_id", prefix)
		if err != nil {
			return nil, err
		}
		out = append(out, loreRelatedEntityInput{
			EntityType: strings.ToLower(strings.TrimSpace(entityType)),
			EntityID:   entityID,
		})
	}
	return out, nil
}
