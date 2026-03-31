package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/llm"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

const completeObjectiveToolName = "complete_objective"

// CompleteObjectiveStore provides quest objective lookup and persistence for complete_objective.
type CompleteObjectiveStore interface {
	GetQuestByID(ctx context.Context, id pgtype.UUID) (statedb.Quest, error)
	ListObjectivesByQuest(ctx context.Context, questID pgtype.UUID) ([]statedb.QuestObjective, error)
	CompleteObjective(ctx context.Context, id pgtype.UUID) (statedb.QuestObjective, error)
	UpdateQuestStatus(ctx context.Context, arg statedb.UpdateQuestStatusParams) (statedb.Quest, error)
}

// CompleteObjectiveTool returns the complete_objective tool definition and JSON schema.
func CompleteObjectiveTool() llm.Tool {
	return llm.Tool{
		Name:        completeObjectiveToolName,
		Description: "Complete a single quest objective by objective ID or objective description match.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"quest_id": map[string]any{
					"type":        "string",
					"description": "Quest UUID.",
				},
				"objective_id": map[string]any{
					"type":        "string",
					"description": "Objective UUID to complete.",
				},
				"objective_description": map[string]any{
					"type":        "string",
					"description": "Objective description text to match when objective_id is unknown.",
				},
				"auto_complete_quest": map[string]any{
					"type":        "boolean",
					"description": "When true, automatically mark quest completed once all objectives are complete.",
				},
			},
			"required":             []string{"quest_id"},
			"additionalProperties": false,
		},
	}
}

// RegisterCompleteObjective registers the complete_objective tool and handler.
func RegisterCompleteObjective(reg *Registry, store CompleteObjectiveStore) error {
	if store == nil {
		return errors.New("complete_objective store is required")
	}
	return reg.Register(CompleteObjectiveTool(), NewCompleteObjectiveHandler(store).Handle)
}

// CompleteObjectiveHandler executes complete_objective tool calls.
type CompleteObjectiveHandler struct {
	store CompleteObjectiveStore
}

// NewCompleteObjectiveHandler creates a new complete_objective handler.
func NewCompleteObjectiveHandler(store CompleteObjectiveStore) *CompleteObjectiveHandler {
	return &CompleteObjectiveHandler{store: store}
}

// Handle executes the complete_objective tool.
func (h *CompleteObjectiveHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("complete_objective handler is nil")
	}
	if h.store == nil {
		return nil, errors.New("complete_objective store is required")
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

	var objectiveID uuid.UUID
	objectiveIDSet := false
	if _, ok := args["objective_id"]; ok {
		objectiveID, err = parseUUIDArg(args, "objective_id")
		if err != nil {
			return nil, err
		}
		objectiveIDSet = true
	}

	var objectiveDescription string
	descriptionSet := false
	if _, ok := args["objective_description"]; ok {
		objectiveDescription, err = parseStringArg(args, "objective_description")
		if err != nil {
			return nil, err
		}
		objectiveDescription = strings.TrimSpace(objectiveDescription)
		if objectiveDescription == "" {
			return nil, errors.New("objective_description must not be empty or whitespace")
		}
		descriptionSet = true
	}

	if objectiveIDSet == descriptionSet {
		return nil, errors.New("exactly one of objective_id or objective_description must be provided")
	}

	autoCompleteQuest := false
	if _, ok := args["auto_complete_quest"]; ok {
		autoCompleteQuest, err = parseBoolArg(args, "auto_complete_quest")
		if err != nil {
			return nil, err
		}
	}

	objectives, err := h.store.ListObjectivesByQuest(ctx, dbutil.ToPgtype(questID))
	if err != nil {
		return nil, fmt.Errorf("list quest objectives: %w", err)
	}
	if len(objectives) == 0 {
		return nil, errors.New("quest has no objectives")
	}

	targetObjective, err := selectObjectiveToComplete(objectives, objectiveIDSet, objectiveID, objectiveDescription)
	if err != nil {
		return nil, err
	}

	wasCompleted := targetObjective.Completed
	if !wasCompleted {
		targetObjective, err = h.store.CompleteObjective(ctx, targetObjective.ID)
		if err != nil {
			return nil, fmt.Errorf("complete objective: %w", err)
		}
	}

	completedCount := 0
	for _, objective := range objectives {
		completed := objective.Completed
		if objective.ID == targetObjective.ID {
			completed = true
		}
		if completed {
			completedCount++
		}
	}
	totalObjectives := len(objectives)
	allObjectivesComplete := completedCount == totalObjectives

	questStatus := quest.Status
	questAutoCompleted := false
	if allObjectivesComplete && autoCompleteQuest {
		if questStatus != string(domain.QuestStatusCompleted) {
			updatedQuest, err := h.store.UpdateQuestStatus(ctx, statedb.UpdateQuestStatusParams{
				Status: string(domain.QuestStatusCompleted),
				ID:     dbutil.ToPgtype(questID),
			})
			if err != nil {
				return nil, fmt.Errorf("auto-complete quest: %w", err)
			}
			questStatus = updatedQuest.Status
			questAutoCompleted = true
		}
	}
	questReadyForCompletion := allObjectivesComplete && questStatus != string(domain.QuestStatusCompleted)

	progress := fmt.Sprintf("%d/%d", completedCount, totalObjectives)
	narrativeParts := []string{
		fmt.Sprintf("Objective %q completed", targetObjective.Description),
		fmt.Sprintf("Progress: %s complete", progress),
	}
	if wasCompleted {
		narrativeParts[0] = fmt.Sprintf("Objective %q was already complete", targetObjective.Description)
	}
	if questReadyForCompletion {
		narrativeParts = append(narrativeParts, fmt.Sprintf("All objectives are complete; quest %q is ready for completion", quest.Title))
	}
	if questAutoCompleted {
		narrativeParts = append(narrativeParts, fmt.Sprintf("Quest %q has been auto-completed", quest.Title))
	}
	narrative := strings.Join(narrativeParts, ". ") + "."

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"quest_id":                questID.String(),
			"quest_title":             quest.Title,
			"quest_status":            questStatus,
			"objective_id":            dbutil.FromPgtype(targetObjective.ID).String(),
			"objective_description":   targetObjective.Description,
			"objective_completed":     true,
			"objectives_completed":    completedCount,
			"objectives_total":        totalObjectives,
			"progress":                progress,
			"all_objectives_complete": allObjectivesComplete,
			"quest_ready_for_completion": questReadyForCompletion,
			"quest_auto_completed":       questAutoCompleted,
		},
		Narrative: narrative,
	}, nil
}

func selectObjectiveToComplete(
	objectives []statedb.QuestObjective,
	objectiveIDSet bool,
	objectiveID uuid.UUID,
	objectiveDescription string,
) (statedb.QuestObjective, error) {
	if objectiveIDSet {
		for _, objective := range objectives {
			if dbutil.FromPgtype(objective.ID) == objectiveID {
				return objective, nil
			}
		}
		return statedb.QuestObjective{}, errors.New("objective_id does not belong to the specified quest")
	}

	descriptionNeedle := normalizeObjectiveText(objectiveDescription)
	exactMatches := make([]statedb.QuestObjective, 0, 1)
	containsMatches := make([]statedb.QuestObjective, 0, 1)

	for _, objective := range objectives {
		candidate := normalizeObjectiveText(objective.Description)
		if candidate == descriptionNeedle {
			exactMatches = append(exactMatches, objective)
			continue
		}
		if strings.Contains(candidate, descriptionNeedle) {
			containsMatches = append(containsMatches, objective)
		}
	}

	if len(exactMatches) == 1 {
		return exactMatches[0], nil
	}
	if len(exactMatches) > 1 {
		return statedb.QuestObjective{}, errors.New("objective_description matches multiple objectives; provide objective_id")
	}
	if len(containsMatches) == 1 {
		return containsMatches[0], nil
	}
	if len(containsMatches) > 1 {
		return statedb.QuestObjective{}, errors.New("objective_description matches multiple objectives; provide objective_id")
	}

	return statedb.QuestObjective{}, errors.New("objective_description did not match any objective in the quest")
}

func normalizeObjectiveText(text string) string {
	return strings.ToLower(strings.TrimSpace(text))
}
