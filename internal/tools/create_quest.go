package tools

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/llm"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

const createQuestToolName = "create_quest"

// QuestStore persists quests, objectives, and related entity links.
type QuestStore interface {
	GetLocationByID(ctx context.Context, id pgtype.UUID) (statedb.Location, error)
	GetQuestByID(ctx context.Context, id pgtype.UUID) (statedb.Quest, error)
	CreateQuest(ctx context.Context, arg statedb.CreateQuestParams) (statedb.Quest, error)
	CreateObjective(ctx context.Context, arg statedb.CreateObjectiveParams) (statedb.QuestObjective, error)
	CreateRelationship(ctx context.Context, arg statedb.CreateRelationshipParams) (statedb.EntityRelationship, error)
}

// CreateQuestTool returns the create_quest tool definition and JSON schema.
func CreateQuestTool() llm.Tool {
	return llm.Tool{
		Name:        createQuestToolName,
		Description: "Create a quest with ordered objectives and optional related entities.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{
					"type":        "string",
					"description": "Quest title.",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Quest description.",
				},
				"quest_type": map[string]any{
					"type":        "string",
					"description": "Quest duration type.",
					"enum":        []string{string(domain.QuestTypeShortTerm), string(domain.QuestTypeMediumTerm), string(domain.QuestTypeLongTerm)},
				},
				"objectives": map[string]any{
					"type":        "array",
					"description": "Ordered objective definitions.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"description": map[string]any{
								"type":        "string",
								"description": "Objective description.",
							},
							"order_index": map[string]any{
								"type":        "integer",
								"description": "Objective order within the quest.",
							},
						},
						"required":             []string{"description", "order_index"},
						"additionalProperties": false,
					},
				},
				"related_entities": map[string]any{
					"type":        "array",
					"description": "Optional entities connected to this quest.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"entity_type": map[string]any{
								"type":        "string",
								"description": "Entity type (npc, location, faction, player_character, player alias, item).",
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
			"required":             []string{"title", "description", "quest_type", "objectives"},
			"additionalProperties": false,
		},
	}
}

// RegisterCreateQuest registers the create_quest tool and handler.
func RegisterCreateQuest(reg *Registry, questStore QuestStore) error {
	if questStore == nil {
		return errors.New("create_quest quest store is required")
	}
	return reg.Register(CreateQuestTool(), NewCreateQuestHandler(questStore).Handle)
}

// CreateQuestHandler executes create_quest tool calls.
type CreateQuestHandler struct {
	questStore QuestStore
}

// NewCreateQuestHandler creates a new create_quest handler.
func NewCreateQuestHandler(questStore QuestStore) *CreateQuestHandler {
	return &CreateQuestHandler{questStore: questStore}
}

// Handle executes the create_quest tool.
func (h *CreateQuestHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("create_quest handler is nil")
	}
	if h.questStore == nil {
		return nil, errors.New("create_quest quest store is required")
	}

	title, err := parseStringArg(args, "title")
	if err != nil {
		return nil, err
	}
	description, err := parseStringArg(args, "description")
	if err != nil {
		return nil, err
	}
	questType, err := parseQuestTypeArg(args, "quest_type")
	if err != nil {
		return nil, err
	}
	objectives, err := parseQuestObjectivesArg(args, "objectives")
	if err != nil {
		return nil, err
	}
	relatedEntities, err := parseQuestRelatedEntitiesArg(args, "related_entities")
	if err != nil {
		return nil, err
	}

	currentLocationID, ok := CurrentLocationIDFromContext(ctx)
	if !ok {
		return nil, errors.New("create_quest requires current location id in context")
	}
	currentLocation, err := h.questStore.GetLocationByID(ctx, dbutil.ToPgtype(currentLocationID))
	if err != nil {
		return nil, fmt.Errorf("resolve campaign from current location: %w", err)
	}

	quest, err := h.questStore.CreateQuest(ctx, statedb.CreateQuestParams{
		CampaignID:  currentLocation.CampaignID,
		Title:       strings.TrimSpace(title),
		Description: pgtype.Text{String: strings.TrimSpace(description), Valid: true},
		QuestType:   questType,
		Status:      string(domain.QuestStatusActive),
	})
	if err != nil {
		return nil, fmt.Errorf("create quest: %w", err)
	}

	slices.SortFunc(objectives, func(a, b createQuestObjectiveInput) int {
		return a.OrderIndex - b.OrderIndex
	})

	createdObjectives := make([]map[string]any, 0, len(objectives))
	questID := dbutil.FromPgtype(quest.ID)
	for i, objective := range objectives {
		createdObjective, err := h.questStore.CreateObjective(ctx, statedb.CreateObjectiveParams{
			QuestID:     quest.ID,
			Description: objective.Description,
			Completed:   false,
			OrderIndex:  int32(objective.OrderIndex),
		})
		if err != nil {
			return nil, fmt.Errorf("create objectives[%d]: %w", i, err)
		}
		createdObjectives = append(createdObjectives, map[string]any{
			"id":          dbutil.FromPgtype(createdObjective.ID).String(),
			"description": createdObjective.Description,
			"completed":   createdObjective.Completed,
			"order_index": createdObjective.OrderIndex,
		})
	}

	createdRelationships := make([]map[string]any, 0, len(relatedEntities))
	sourceEntityType := string(domain.EntityTypeLocation)
	sourceEntityID := currentLocationID
	if currentPlayerID, ok := CurrentPlayerCharacterIDFromContext(ctx); ok {
		sourceEntityType = string(domain.EntityTypePlayerCharacter)
		sourceEntityID = currentPlayerID
	}
	for i, related := range relatedEntities {
		relationship, err := h.questStore.CreateRelationship(ctx, statedb.CreateRelationshipParams{
			CampaignID:       currentLocation.CampaignID,
			SourceEntityType: sourceEntityType,
			SourceEntityID:   dbutil.ToPgtype(sourceEntityID),
			TargetEntityType: related.EntityType,
			TargetEntityID:   dbutil.ToPgtype(related.EntityID),
			RelationshipType: "quest_related",
			Description:      pgtype.Text{String: fmt.Sprintf("Related to quest %s.", quest.Title), Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("create related_entities[%d]: %w", i, err)
		}
		createdRelationships = append(createdRelationships, map[string]any{
			"id":                 dbutil.FromPgtype(relationship.ID).String(),
			"source_entity_type": relationship.SourceEntityType,
			"source_entity_id":   dbutil.FromPgtype(relationship.SourceEntityID).String(),
			"target_entity_type": relationship.TargetEntityType,
			"target_entity_id":   dbutil.FromPgtype(relationship.TargetEntityID).String(),
			"relationship_type":  relationship.RelationshipType,
		})
	}

	summary := fmt.Sprintf("Quest %q (%s) is now active with %d objective(s).", quest.Title, quest.QuestType, len(createdObjectives))
	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"id":               questID.String(),
			"campaign_id":      dbutil.FromPgtype(quest.CampaignID).String(),
			"title":            quest.Title,
			"description":      quest.Description.String,
			"quest_type":       quest.QuestType,
			"status":           quest.Status,
			"objectives":       createdObjectives,
			"related_entities": createdRelationships,
			"summary":          summary,
		},
		Narrative: summary,
	}, nil
}
