package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/llm"
)

const (
	updatePlayerStatsToolName = "update_player_stats"
	defaultMinStatValue       = 1
	defaultMaxStatValue       = 30
)

var knownPlayerStats = map[string]struct{}{
	"strength":     {},
	"dexterity":    {},
	"constitution": {},
	"intelligence": {},
	"wisdom":       {},
	"charisma":     {},
}

// UpdatePlayerStatsStore provides player stats lookup and persistence for update_player_stats.
type UpdatePlayerStatsStore interface {
	GetPlayerCharacterByID(ctx context.Context, playerCharacterID uuid.UUID) (*domain.PlayerCharacter, error)
	UpdatePlayerStats(ctx context.Context, playerCharacterID uuid.UUID, stats json.RawMessage) error
}

// UpdatePlayerStatsTool returns the update_player_stats tool definition and JSON schema.
func UpdatePlayerStatsTool() llm.Tool {
	return llm.Tool{
		Name:        updatePlayerStatsToolName,
		Description: "Update a player character stat value.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"stat_name": map[string]any{
					"type":        "string",
					"description": "Player stat name to update (strength, dexterity, constitution, intelligence, wisdom, charisma).",
					"enum":        []string{"strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"},
				},
				"value": map[string]any{
					"type":        "integer",
					"description": "Stat value to apply with the selected operation.",
				},
				"operation": map[string]any{
					"type":        "string",
					"description": "Operation to apply: set, add, or subtract.",
					"enum":        []string{"set", "add", "subtract"},
				},
			},
			"required":             []string{"stat_name", "value", "operation"},
			"additionalProperties": false,
		},
	}
}

// RegisterUpdatePlayerStats registers the update_player_stats tool and handler.
func RegisterUpdatePlayerStats(reg *Registry, store UpdatePlayerStatsStore) error {
	if store == nil {
		return errors.New("update_player_stats store is required")
	}
	return reg.Register(UpdatePlayerStatsTool(), NewUpdatePlayerStatsHandler(store).Handle)
}

// UpdatePlayerStatsHandler executes update_player_stats tool calls.
type UpdatePlayerStatsHandler struct {
	store        UpdatePlayerStatsStore
	minStatValue int
	maxStatValue int
}

// NewUpdatePlayerStatsHandler creates a new update_player_stats handler using default stat bounds.
func NewUpdatePlayerStatsHandler(store UpdatePlayerStatsStore) *UpdatePlayerStatsHandler {
	return NewUpdatePlayerStatsHandlerWithBounds(store, defaultMinStatValue, defaultMaxStatValue)
}

// NewUpdatePlayerStatsHandlerWithBounds creates a new update_player_stats handler using custom stat bounds.
func NewUpdatePlayerStatsHandlerWithBounds(store UpdatePlayerStatsStore, minStatValue, maxStatValue int) *UpdatePlayerStatsHandler {
	if minStatValue > maxStatValue {
		minStatValue, maxStatValue = defaultMinStatValue, defaultMaxStatValue
	}
	return &UpdatePlayerStatsHandler{
		store:        store,
		minStatValue: minStatValue,
		maxStatValue: maxStatValue,
	}
}

// Handle executes the update_player_stats tool.
func (h *UpdatePlayerStatsHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("update_player_stats handler is nil")
	}
	if h.store == nil {
		return nil, errors.New("update_player_stats store is required")
	}

	playerCharacterID, ok := CurrentPlayerCharacterIDFromContext(ctx)
	if !ok {
		return nil, errors.New("update_player_stats requires current player character id in context")
	}

	statName, err := parseStringArg(args, "stat_name")
	if err != nil {
		return nil, err
	}
	statName = strings.ToLower(strings.TrimSpace(statName))
	if _, known := knownPlayerStats[statName]; !known {
		return nil, errors.New("stat_name must be one of: strength, dexterity, constitution, intelligence, wisdom, charisma")
	}

	operation, err := parseStringArg(args, "operation")
	if err != nil {
		return nil, err
	}
	operation = strings.ToLower(strings.TrimSpace(operation))
	if operation != "set" && operation != "add" && operation != "subtract" {
		return nil, errors.New("operation must be one of: set, add, subtract")
	}

	value, err := parseIntArg(args, "value")
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

	stats, err := parsePlayerStats(playerCharacter.Stats)
	if err != nil {
		return nil, err
	}

	statKey, found := findStatKey(stats, statName)
	if !found {
		return nil, fmt.Errorf("player stat %q does not exist", statName)
	}

	oldValue, err := parseStatValue(stats[statKey], statName)
	if err != nil {
		return nil, err
	}

	newValue := oldValue
	switch operation {
	case "set":
		newValue = value
	case "add":
		newValue += value
	case "subtract":
		newValue -= value
	}
	newValue = clampStatValue(newValue, h.minStatValue, h.maxStatValue)

	stats[statKey] = newValue
	updatedStats, err := json.Marshal(stats)
	if err != nil {
		return nil, fmt.Errorf("marshal updated stats: %w", err)
	}

	if err := h.store.UpdatePlayerStats(ctx, playerCharacterID, updatedStats); err != nil {
		return nil, fmt.Errorf("update player stats: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"player_character_id": playerCharacterID.String(),
			"stat_name":           statName,
			"operation":           operation,
			"old_value":           oldValue,
			"new_value":           newValue,
		},
		Narrative: fmt.Sprintf("Updated %s from %d to %d.", statName, oldValue, newValue),
	}, nil
}

func parsePlayerStats(statsJSON json.RawMessage) (map[string]any, error) {
	stats := map[string]any{}
	if len(statsJSON) == 0 {
		return stats, nil
	}
	if err := json.Unmarshal(statsJSON, &stats); err != nil {
		return nil, fmt.Errorf("unmarshal player stats: %w", err)
	}
	return stats, nil
}

func findStatKey(stats map[string]any, statName string) (string, bool) {
	for key := range stats {
		if strings.EqualFold(key, statName) {
			return key, true
		}
	}
	return "", false
}

func parseStatValue(value any, statName string) (int, error) {
	switch typed := value.(type) {
	case int:
		return typed, nil
	case int8:
		return int(typed), nil
	case int16:
		return int(typed), nil
	case int32:
		return int(typed), nil
	case int64:
		return int(typed), nil
	case float64:
		return int(typed), nil
	default:
		return 0, fmt.Errorf("player stat %q has non-numeric value", statName)
	}
}

func clampStatValue(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
