package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/llm"
)

const addExperienceToolName = "add_experience"

// AddExperienceStore provides persistence required for add_experience.
type AddExperienceStore interface {
	GetPlayerCharacterByID(ctx context.Context, playerCharacterID uuid.UUID) (*domain.PlayerCharacter, error)
	UpdatePlayerExperience(ctx context.Context, playerCharacterID uuid.UUID, experience, level int) error
}

// AddExperienceTool returns the add_experience tool definition and JSON schema.
func AddExperienceTool() llm.Tool {
	return llm.Tool{
		Name:        addExperienceToolName,
		Description: "Add experience points to the current player character and report whether level-up is available.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"amount": map[string]any{
					"type":        "integer",
					"description": "Experience points to add (must be greater than 0).",
				},
				"reason": map[string]any{
					"type":        "string",
					"description": "Why the player gained experience.",
				},
			},
			"required":             []string{"amount", "reason"},
			"additionalProperties": false,
		},
	}
}

// RegisterAddExperience registers the add_experience tool and handler.
func RegisterAddExperience(reg *Registry, store AddExperienceStore) error {
	if store == nil {
		return errors.New("add_experience store is required")
	}
	return reg.Register(AddExperienceTool(), NewAddExperienceHandler(store).Handle)
}

// AddExperienceHandler executes add_experience tool calls.
type AddExperienceHandler struct {
	store            AddExperienceStore
	levelThresholdFn experienceThresholdFunc
}

// NewAddExperienceHandler creates a new add_experience handler.
func NewAddExperienceHandler(store AddExperienceStore) *AddExperienceHandler {
	return NewAddExperienceHandlerWithThreshold(store, defaultExperienceThreshold)
}

// NewAddExperienceHandlerWithThreshold creates a new add_experience handler with a custom level threshold function.
func NewAddExperienceHandlerWithThreshold(store AddExperienceStore, levelThresholdFn experienceThresholdFunc) *AddExperienceHandler {
	if levelThresholdFn == nil {
		levelThresholdFn = defaultExperienceThreshold
	}
	return &AddExperienceHandler{
		store:            store,
		levelThresholdFn: levelThresholdFn,
	}
}

// Handle executes the add_experience tool.
func (h *AddExperienceHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("add_experience handler is nil")
	}
	if h.store == nil {
		return nil, errors.New("add_experience store is required")
	}

	playerCharacterID, ok := CurrentPlayerCharacterIDFromContext(ctx)
	if !ok {
		return nil, errors.New("add_experience requires current player character id in context")
	}

	amount, err := parseIntArg(args, "amount")
	if err != nil {
		return nil, err
	}
	if amount <= 0 {
		return nil, errors.New("amount must be greater than 0")
	}

	reason, err := parseStringArg(args, "reason")
	if err != nil {
		return nil, err
	}

	playerCharacter, err := h.store.GetPlayerCharacterByID(ctx, playerCharacterID)
	if err != nil {
		return nil, fmt.Errorf("get player character: %w", err)
	}
	if playerCharacter == nil {
		return nil, errors.New("current player character does not exist")
	}

	currentLevel := playerCharacter.Level
	if currentLevel < 1 {
		currentLevel = 1
	}
	oldExperience := playerCharacter.Experience
	newExperience := oldExperience + amount
	levelUpThreshold := h.levelThresholdFn(currentLevel)
	levelUpAvailable := newExperience >= levelUpThreshold

	if err := h.store.UpdatePlayerExperience(ctx, playerCharacterID, newExperience, currentLevel); err != nil {
		return nil, fmt.Errorf("update player experience: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"player_character_id":  playerCharacterID.String(),
			"amount":               amount,
			"reason":               reason,
			"old_experience":       oldExperience,
			"new_experience":       newExperience,
			"current_level":        currentLevel,
			"level_up_threshold":   levelUpThreshold,
			"level_up_available":   levelUpAvailable,
			"experience_to_next":   maxInt(levelUpThreshold-newExperience, 0),
		},
		Narrative: fmt.Sprintf("You gained %d XP for %s.", amount, reason),
	}, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
