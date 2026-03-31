package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/llm"
)

const levelUpToolName = "level_up"

// LevelUpStore provides persistence required for level_up.
type LevelUpStore interface {
	GetPlayerCharacterByID(ctx context.Context, playerCharacterID uuid.UUID) (*domain.PlayerCharacter, error)
	UpdatePlayerLevel(ctx context.Context, playerCharacterID uuid.UUID, level int) error
	UpdatePlayerStats(ctx context.Context, playerCharacterID uuid.UUID, stats json.RawMessage) error
	UpdatePlayerAbilities(ctx context.Context, playerCharacterID uuid.UUID, abilities json.RawMessage) error
}

// LevelUpTool returns the level_up tool definition and JSON schema.
func LevelUpTool() llm.Tool {
	return llm.Tool{
		Name:        levelUpToolName,
		Description: "Increase the current player character level and optionally apply stat boosts or new abilities.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"stat_boosts": map[string]any{
					"type":        "object",
					"description": "Optional stat boosts to add to existing player stats. Keys are stat names and values are integer boosts.",
					"additionalProperties": map[string]any{
						"type": "integer",
					},
				},
				"new_abilities": map[string]any{
					"type":        "array",
					"description": "Optional abilities gained on level-up.",
					"items": map[string]any{
						"type": "string",
					},
				},
			},
			"additionalProperties": false,
		},
	}
}

// RegisterLevelUp registers the level_up tool and handler.
func RegisterLevelUp(reg *Registry, store LevelUpStore) error {
	if store == nil {
		return errors.New("level_up store is required")
	}
	return reg.Register(LevelUpTool(), NewLevelUpHandler(store).Handle)
}

// LevelUpHandler executes level_up tool calls.
type LevelUpHandler struct {
	store            LevelUpStore
	levelThresholdFn experienceThresholdFunc
}

// NewLevelUpHandler creates a new level_up handler.
func NewLevelUpHandler(store LevelUpStore) *LevelUpHandler {
	return NewLevelUpHandlerWithThreshold(store, defaultExperienceThreshold)
}

// NewLevelUpHandlerWithThreshold creates a new level_up handler with a custom level threshold function.
func NewLevelUpHandlerWithThreshold(store LevelUpStore, levelThresholdFn experienceThresholdFunc) *LevelUpHandler {
	if levelThresholdFn == nil {
		levelThresholdFn = defaultExperienceThreshold
	}
	return &LevelUpHandler{
		store:            store,
		levelThresholdFn: levelThresholdFn,
	}
}

// Handle executes the level_up tool.
func (h *LevelUpHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("level_up handler is nil")
	}
	if h.store == nil {
		return nil, errors.New("level_up store is required")
	}

	playerCharacterID, ok := CurrentPlayerCharacterIDFromContext(ctx)
	if !ok {
		return nil, errors.New("level_up requires current player character id in context")
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
	requiredExperience := h.levelThresholdFn(currentLevel)
	if playerCharacter.Experience < requiredExperience {
		return nil, fmt.Errorf("insufficient experience to level up: need %d XP for level %d", requiredExperience, currentLevel+1)
	}

	statBoosts, err := parseOptionalStatBoosts(args, "stat_boosts")
	if err != nil {
		return nil, err
	}
	newAbilities, err := parseOptionalStringArrayArg(args, "new_abilities")
	if err != nil {
		return nil, err
	}

	var updatedStatsJSON json.RawMessage
	newLevel := currentLevel + 1
	updatedStats := make(map[string]int)
	if len(statBoosts) > 0 {
		stats, parseErr := parsePlayerStats(playerCharacter.Stats)
		if parseErr != nil {
			return nil, parseErr
		}
		for statName, boost := range statBoosts {
			statKey, found := findStatKey(stats, statName)
			if !found {
				return nil, fmt.Errorf("player stat %q does not exist", statName)
			}
			currentValue, parseValueErr := parseStatValue(stats[statKey], statName)
			if parseValueErr != nil {
				return nil, parseValueErr
			}
			stats[statKey] = currentValue + boost
			updatedStats[statKey] = currentValue + boost
		}
		statsJSON, marshalErr := json.Marshal(stats)
		if marshalErr != nil {
			return nil, fmt.Errorf("marshal updated stats: %w", marshalErr)
		}
		updatedStatsJSON = statsJSON
	}

	var updatedAbilitiesJSON json.RawMessage
	addedAbilities := []string{}
	if len(newAbilities) > 0 {
		abilities, parseErr := parseLevelUpPlayerAbilities(playerCharacter.Abilities)
		if parseErr != nil {
			return nil, parseErr
		}
		existing := make(map[string]struct{}, len(abilities))
		for _, ability := range abilities {
			existing[strings.ToLower(strings.TrimSpace(ability))] = struct{}{}
		}
		for _, ability := range newAbilities {
			normalizedAbility := strings.ToLower(strings.TrimSpace(ability))
			if _, found := existing[normalizedAbility]; found {
				continue
			}
			abilities = append(abilities, ability)
			existing[normalizedAbility] = struct{}{}
			addedAbilities = append(addedAbilities, ability)
		}
		abilitiesJSON, marshalErr := json.Marshal(abilities)
		if marshalErr != nil {
			return nil, fmt.Errorf("marshal updated abilities: %w", marshalErr)
		}
		updatedAbilitiesJSON = abilitiesJSON
	}

	if err := h.store.UpdatePlayerLevel(ctx, playerCharacterID, newLevel); err != nil {
		return nil, fmt.Errorf("update player level: %w", err)
	}
	if len(updatedStatsJSON) > 0 {
		if err := h.store.UpdatePlayerStats(ctx, playerCharacterID, updatedStatsJSON); err != nil {
			return nil, fmt.Errorf("update player stats: %w", err)
		}
	}
	if len(updatedAbilitiesJSON) > 0 {
		if err := h.store.UpdatePlayerAbilities(ctx, playerCharacterID, updatedAbilitiesJSON); err != nil {
			return nil, fmt.Errorf("update player abilities: %w", err)
		}
	}

	sort.Strings(addedAbilities)

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"player_character_id": playerCharacterID.String(),
			"old_level":           currentLevel,
			"new_level":           newLevel,
			"updated_stats":       updatedStats,
			"new_abilities_added": addedAbilities,
		},
		Narrative: fmt.Sprintf("You reached level %d.", newLevel),
	}, nil
}

func parseOptionalStatBoosts(args map[string]any, key string) (map[string]int, error) {
	raw, ok := args[key]
	if !ok {
		return nil, nil
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", key)
	}
	boosts := make(map[string]int, len(obj))
	for statName, rawValue := range obj {
		normalizedName := strings.ToLower(strings.TrimSpace(statName))
		if normalizedName == "" {
			return nil, fmt.Errorf("%s contains an empty stat name", key)
		}
		value, err := parseIntArg(map[string]any{"value": rawValue}, "value")
		if err != nil {
			return nil, fmt.Errorf("%s.%s must be an integer", key, normalizedName)
		}
		if value == 0 {
			continue
		}
		boosts[normalizedName] += value
	}
	return boosts, nil
}

func parseOptionalStringArrayArg(args map[string]any, key string) ([]string, error) {
	raw, ok := args[key]
	if !ok {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}
	out := make([]string, 0, len(items))
	for i, item := range items {
		s, ok := item.(string)
		if !ok || s == "" {
			return nil, fmt.Errorf("%s[%d] must be a non-empty string", key, i)
		}
		out = append(out, s)
	}
	return out, nil
}

func parseLevelUpPlayerAbilities(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	var abilities []string
	if err := json.Unmarshal(raw, &abilities); err != nil {
		return nil, fmt.Errorf("unmarshal player abilities: %w", err)
	}
	return abilities, nil
}
