package tools

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/llm"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const createSubquestToolName = "create_subquest"

// CreateSubquestTool returns the create_subquest tool definition and JSON schema.
func CreateSubquestTool() llm.Tool {
	return llm.Tool{
		Name:        createSubquestToolName,
		Description: "Create a subquest under an active parent quest.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"parent_quest_id": map[string]any{
					"type":        "string",
					"description": "Parent quest UUID.",
				},
				"title": map[string]any{
					"type":        "string",
					"description": "Subquest title.",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Subquest description.",
				},
				"quest_type": map[string]any{
					"type":        "string",
					"description": "Subquest duration type.",
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
								"description": "Objective order within the subquest.",
							},
						},
						"required":             []string{"description", "order_index"},
						"additionalProperties": false,
					},
				},
			},
			"required":             []string{"parent_quest_id", "title", "description", "quest_type", "objectives"},
			"additionalProperties": false,
		},
	}
}

// RegisterCreateSubquest registers the create_subquest tool and handler.
func RegisterCreateSubquest(reg *Registry, questStore QuestStore) error {
	if questStore == nil {
		return errors.New("create_subquest quest store is required")
	}
	return reg.Register(CreateSubquestTool(), NewCreateSubquestHandler(questStore).Handle)
}

// CreateSubquestHandler executes create_subquest tool calls.
type CreateSubquestHandler struct {
	questStore QuestStore
}

// NewCreateSubquestHandler creates a new create_subquest handler.
func NewCreateSubquestHandler(questStore QuestStore) *CreateSubquestHandler {
	return &CreateSubquestHandler{questStore: questStore}
}

// Handle executes the create_subquest tool.
func (h *CreateSubquestHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("create_subquest handler is nil")
	}
	if h.questStore == nil {
		return nil, errors.New("create_subquest quest store is required")
	}

	parentQuestID, err := parseUUIDArg(args, "parent_quest_id")
	if err != nil {
		return nil, err
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

	parentQuest, err := h.questStore.GetQuestByID(ctx, dbutil.ToPgtype(parentQuestID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("parent quest not found: %w", err)
		}
		return nil, fmt.Errorf("parent quest lookup failed: %w", err)
	}
	if parentQuest.Status != string(domain.QuestStatusActive) {
		return nil, errors.New("parent quest must be active")
	}
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		return nil, errors.New("title must not be empty or whitespace")
	}
	trimmedDescription := strings.TrimSpace(description)
	if trimmedDescription == "" {
		return nil, errors.New("description must not be empty or whitespace")
	}

	quest, err := h.questStore.CreateQuest(ctx, statedb.CreateQuestParams{
		CampaignID:    parentQuest.CampaignID,
		ParentQuestID: pgtype.UUID{Bytes: dbutil.ToPgtype(parentQuestID).Bytes, Valid: true},
		Title:         trimmedTitle,
		Description:   pgtype.Text{String: trimmedDescription, Valid: true},
		QuestType:     questType,
		Status:        string(domain.QuestStatusActive),
	})
	if err != nil {
		return nil, fmt.Errorf("create subquest: %w", err)
	}

	slices.SortFunc(objectives, func(a, b createQuestObjectiveInput) int {
		return a.OrderIndex - b.OrderIndex
	})

	createdObjectives := make([]map[string]any, 0, len(objectives))
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

	summary := fmt.Sprintf("Subquest %q (%s) is now active under parent quest %s.", quest.Title, quest.QuestType, parentQuestID.String())
	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"id":              dbutil.FromPgtype(quest.ID).String(),
			"campaign_id":     dbutil.FromPgtype(quest.CampaignID).String(),
			"parent_quest_id": parentQuestID.String(),
			"title":           quest.Title,
			"description":     quest.Description.String,
			"quest_type":      quest.QuestType,
			"status":          quest.Status,
			"objectives":      createdObjectives,
			"summary":         summary,
		},
		Narrative: summary,
	}, nil
}
