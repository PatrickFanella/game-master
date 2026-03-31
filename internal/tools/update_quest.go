package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/llm"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

const updateQuestToolName = "update_quest"

// UpdateQuestStore provides quest lookup and persistence for update_quest.
type UpdateQuestStore interface {
	GetQuestByID(ctx context.Context, id pgtype.UUID) (statedb.Quest, error)
	UpdateQuest(ctx context.Context, arg statedb.UpdateQuestParams) (statedb.Quest, error)
	UpdateQuestStatus(ctx context.Context, arg statedb.UpdateQuestStatusParams) (statedb.Quest, error)
	ListObjectivesByQuest(ctx context.Context, questID pgtype.UUID) ([]statedb.QuestObjective, error)
	CreateObjective(ctx context.Context, arg statedb.CreateObjectiveParams) (statedb.QuestObjective, error)
	ListQuestsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.Quest, error)
}

// UpdateQuestTool returns the update_quest tool definition and JSON schema.
func UpdateQuestTool() llm.Tool {
	return llm.Tool{
		Name:        updateQuestToolName,
		Description: "Update a quest's status, add objectives, and/or update the description.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"quest_id": map[string]any{
					"type":        "string",
					"description": "Quest UUID.",
				},
				"status": map[string]any{
					"type":        "string",
					"description": "Updated quest status.",
					"enum": []string{
						string(domain.QuestStatusActive),
						string(domain.QuestStatusCompleted),
						string(domain.QuestStatusFailed),
						string(domain.QuestStatusAbandoned),
					},
				},
				"new_objectives": map[string]any{
					"type":        "array",
					"description": "Objective descriptions to append to this quest.",
					"items": map[string]any{
						"type": "string",
					},
				},
				"description_update": map[string]any{
					"type":        "string",
					"description": "Replacement quest description.",
				},
			},
			"required":             []string{"quest_id"},
			"additionalProperties": false,
		},
	}
}

// RegisterUpdateQuest registers the update_quest tool and handler.
func RegisterUpdateQuest(reg *Registry, store UpdateQuestStore) error {
	if store == nil {
		return errors.New("update_quest store is required")
	}
	return reg.Register(UpdateQuestTool(), NewUpdateQuestHandler(store).Handle)
}

// UpdateQuestHandler executes update_quest tool calls.
type UpdateQuestHandler struct {
	store UpdateQuestStore
}

// NewUpdateQuestHandler creates a new update_quest handler.
func NewUpdateQuestHandler(store UpdateQuestStore) *UpdateQuestHandler {
	return &UpdateQuestHandler{store: store}
}

// Handle executes the update_quest tool.
func (h *UpdateQuestHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("update_quest handler is nil")
	}
	if h.store == nil {
		return nil, errors.New("update_quest store is required")
	}

	questID, err := parseUUIDArg(args, "quest_id")
	if err != nil {
		return nil, err
	}

	quest, err := h.store.GetQuestByID(ctx, dbutil.ToPgtype(questID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("quest_id does not reference an existing quest")
		}
		return nil, fmt.Errorf("get quest: %w", err)
	}

	statusUpdate, hasStatus, err := parseQuestStatusUpdateArg(args, "status")
	if err != nil {
		return nil, err
	}
	descriptionUpdate, hasDescriptionUpdate, err := parseOptionalQuestDescriptionUpdateArg(args, "description_update")
	if err != nil {
		return nil, err
	}
	newObjectives, hasNewObjectives, err := parseOptionalObjectiveDescriptionsArg(args, "new_objectives")
	if err != nil {
		return nil, err
	}
	if !hasStatus && !hasDescriptionUpdate && !hasNewObjectives {
		return nil, errors.New("at least one of status, description_update, or new_objectives is required")
	}

	if hasDescriptionUpdate {
		quest, err = h.store.UpdateQuest(ctx, statedb.UpdateQuestParams{
			ParentQuestID: quest.ParentQuestID,
			Title:         quest.Title,
			Description:   pgtype.Text{String: descriptionUpdate, Valid: true},
			QuestType:     quest.QuestType,
			Status:        quest.Status,
			ID:            quest.ID,
		})
		if err != nil {
			return nil, fmt.Errorf("update quest description: %w", err)
		}
	}

	originalStatus := quest.Status
	statusChanged := hasStatus && statusUpdate != originalStatus
	if statusChanged {
		quest, err = h.store.UpdateQuestStatus(ctx, statedb.UpdateQuestStatusParams{
			Status: statusUpdate,
			ID:     quest.ID,
		})
		if err != nil {
			return nil, fmt.Errorf("update quest status: %w", err)
		}
	}

	cascadedSubquests := make([]map[string]any, 0)
	if statusChanged && (statusUpdate == string(domain.QuestStatusCompleted) || statusUpdate == string(domain.QuestStatusFailed)) {
		cascadedSubquests, err = h.cascadeSubquestStatus(ctx, quest, statusUpdate)
		if err != nil {
			return nil, err
		}
	}

	appendedObjectives := make([]map[string]any, 0, len(newObjectives))
	if hasNewObjectives {
		objectives, err := h.store.ListObjectivesByQuest(ctx, quest.ID)
		if err != nil {
			return nil, fmt.Errorf("list quest objectives: %w", err)
		}
		maxOrderIndex := int32(-1)
		for _, objective := range objectives {
			if objective.OrderIndex > maxOrderIndex {
				maxOrderIndex = objective.OrderIndex
			}
		}
		nextOrderIndex := maxOrderIndex + 1
		for i, objectiveDescription := range newObjectives {
			createdObjective, err := h.store.CreateObjective(ctx, statedb.CreateObjectiveParams{
				QuestID:     quest.ID,
				Description: objectiveDescription,
				Completed:   false,
				OrderIndex:  nextOrderIndex + int32(i),
			})
			if err != nil {
				return nil, fmt.Errorf("create new_objectives[%d]: %w", i, err)
			}
			appendedObjectives = append(appendedObjectives, map[string]any{
				"id":          dbutil.FromPgtype(createdObjective.ID).String(),
				"description": createdObjective.Description,
				"completed":   createdObjective.Completed,
				"order_index": createdObjective.OrderIndex,
			})
		}
	}

	updatedObjectives, err := h.store.ListObjectivesByQuest(ctx, quest.ID)
	if err != nil {
		return nil, fmt.Errorf("list updated quest objectives: %w", err)
	}

	objectiveData := make([]map[string]any, 0, len(updatedObjectives))
	for _, objective := range updatedObjectives {
		objectiveData = append(objectiveData, map[string]any{
			"id":          dbutil.FromPgtype(objective.ID).String(),
			"description": objective.Description,
			"completed":   objective.Completed,
			"order_index": objective.OrderIndex,
		})
	}

	narrativeParts := []string{fmt.Sprintf("Quest %q updated", quest.Title)}
	if hasStatus {
		narrativeParts = append(narrativeParts, fmt.Sprintf("status is now %q", quest.Status))
	}
	if hasDescriptionUpdate {
		narrativeParts = append(narrativeParts, "description updated")
	}
	if len(appendedObjectives) > 0 {
		narrativeParts = append(narrativeParts, fmt.Sprintf("%d new objective(s) added", len(appendedObjectives)))
	}
	if len(cascadedSubquests) > 0 {
		narrativeParts = append(narrativeParts, fmt.Sprintf("%d subquest(s) cascaded", len(cascadedSubquests)))
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"id":                dbutil.FromPgtype(quest.ID).String(),
			"campaign_id":       dbutil.FromPgtype(quest.CampaignID).String(),
			"parent_quest_id":   optionalQuestIDString(quest.ParentQuestID),
			"title":             quest.Title,
			"description":       quest.Description.String,
			"quest_type":        quest.QuestType,
			"status":            quest.Status,
			"added_objectives":  appendedObjectives,
			"objectives":        objectiveData,
			"cascaded_subquests": cascadedSubquests,
		},
		Narrative: strings.Join(narrativeParts, ". ") + ".",
	}, nil
}

func (h *UpdateQuestHandler) cascadeSubquestStatus(ctx context.Context, parentQuest statedb.Quest, parentStatus string) ([]map[string]any, error) {
	quests, err := h.store.ListQuestsByCampaign(ctx, parentQuest.CampaignID)
	if err != nil {
		return nil, fmt.Errorf("list quests by campaign: %w", err)
	}

	cascadeStatus := string(domain.QuestStatusFailed)
	if parentStatus == string(domain.QuestStatusCompleted) {
		cascadeStatus = string(domain.QuestStatusAbandoned)
	}

	childrenByParent := make(map[[16]byte][]statedb.Quest)
	for _, quest := range quests {
		if !quest.ParentQuestID.Valid {
			continue
		}
		childrenByParent[quest.ParentQuestID.Bytes] = append(childrenByParent[quest.ParentQuestID.Bytes], quest)
	}

	queue := append([]statedb.Quest(nil), childrenByParent[parentQuest.ID.Bytes]...)
	cascaded := make([]map[string]any, 0)
	for len(queue) > 0 {
		subquest := queue[0]
		queue = queue[1:]
		if subquest.Status == string(domain.QuestStatusActive) {
			updatedSubquest, err := h.store.UpdateQuestStatus(ctx, statedb.UpdateQuestStatusParams{
				Status: cascadeStatus,
				ID:     subquest.ID,
			})
			if err != nil {
				return nil, fmt.Errorf("cascade subquest status for %s: %w", dbutil.FromPgtype(subquest.ID).String(), err)
			}
			cascaded = append(cascaded, map[string]any{
				"id":         dbutil.FromPgtype(updatedSubquest.ID).String(),
				"title":      updatedSubquest.Title,
				"old_status": subquest.Status,
				"new_status": updatedSubquest.Status,
			})
		}
		queue = append(queue, childrenByParent[subquest.ID.Bytes]...)
	}

	return cascaded, nil
}

func parseQuestStatusUpdateArg(args map[string]any, key string) (string, bool, error) {
	if _, ok := args[key]; !ok {
		return "", false, nil
	}
	status, err := parseStringArg(args, key)
	if err != nil {
		return "", false, err
	}
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case string(domain.QuestStatusActive), string(domain.QuestStatusCompleted), string(domain.QuestStatusFailed), string(domain.QuestStatusAbandoned):
		return status, true, nil
	default:
		return "", false, errors.New("status must be one of: active, completed, failed, abandoned")
	}
}

func parseOptionalQuestDescriptionUpdateArg(args map[string]any, key string) (string, bool, error) {
	if _, ok := args[key]; !ok {
		return "", false, nil
	}
	value, err := parseStringArg(args, key)
	if err != nil {
		return "", false, err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false, fmt.Errorf("%s must not be empty or whitespace", key)
	}
	return value, true, nil
}

func parseOptionalObjectiveDescriptionsArg(args map[string]any, key string) ([]string, bool, error) {
	raw, ok := args[key]
	if !ok {
		return nil, false, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, false, fmt.Errorf("%s must be an array", key)
	}
	if len(items) == 0 {
		return nil, false, fmt.Errorf("%s must contain at least one objective", key)
	}
	out := make([]string, 0, len(items))
	for i, item := range items {
		value, ok := item.(string)
		if !ok {
			return nil, false, fmt.Errorf("%s[%d] must be a string", key, i)
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, false, fmt.Errorf("%s[%d] must be a non-empty string", key, i)
		}
		out = append(out, value)
	}
	return out, true, nil
}

func optionalQuestIDString(id pgtype.UUID) any {
	if !id.Valid {
		return nil
	}
	return dbutil.FromPgtype(id).String()
}
