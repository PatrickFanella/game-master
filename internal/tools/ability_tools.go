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
	addAbilityToolName    = "add_ability"
	removeAbilityToolName = "remove_ability"
)

var allowedAbilityTypes = map[string]struct{}{
	"passive": {},
	"active":  {},
}

type playerAbility struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Cooldown    *int   `json:"cooldown"`
}

// AbilityStore provides player lookup and ability persistence for ability tools.
type AbilityStore interface {
	GetPlayerCharacterByID(ctx context.Context, playerCharacterID uuid.UUID) (*domain.PlayerCharacter, error)
	UpdatePlayerAbilities(ctx context.Context, playerCharacterID uuid.UUID, abilities json.RawMessage) error
}

// AddAbilityStore aliases AbilityStore for add_ability handlers.
type AddAbilityStore = AbilityStore

// RemoveAbilityStore aliases AbilityStore for remove_ability handlers.
type RemoveAbilityStore = AbilityStore

// AddAbilityTool returns the add_ability tool definition and JSON schema.
func AddAbilityTool() llm.Tool {
	return llm.Tool{
		Name:        addAbilityToolName,
		Description: "Add an ability to the current player character.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Ability name.",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Ability description.",
				},
				"type": map[string]any{
					"type":        "string",
					"description": "Ability type. One of: passive, active.",
					"enum":        []string{"passive", "active"},
				},
				"cooldown": map[string]any{
					"type":        "integer",
					"description": "Ability cooldown in turns. Optional.",
				},
			},
			"required":             []string{"name", "description", "type"},
			"additionalProperties": false,
		},
	}
}

// RegisterAddAbility registers the add_ability tool and handler.
func RegisterAddAbility(reg *Registry, store AddAbilityStore) error {
	if store == nil {
		return errors.New("add_ability store is required")
	}
	return reg.Register(AddAbilityTool(), NewAddAbilityHandler(store).Handle)
}

// AddAbilityHandler executes add_ability tool calls.
type AddAbilityHandler struct {
	store AddAbilityStore
}

// NewAddAbilityHandler creates a new add_ability handler.
func NewAddAbilityHandler(store AddAbilityStore) *AddAbilityHandler {
	return &AddAbilityHandler{store: store}
}

// Handle executes the add_ability tool.
func (h *AddAbilityHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("add_ability handler is nil")
	}
	if h.store == nil {
		return nil, errors.New("add_ability store is required")
	}

	playerCharacterID, ok := CurrentPlayerCharacterIDFromContext(ctx)
	if !ok {
		return nil, errors.New("add_ability requires current player character id in context")
	}

	name, err := parseStringArg(args, "name")
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("name cannot be empty or whitespace")
	}
	description, err := parseStringArg(args, "description")
	if err != nil {
		return nil, err
	}
	description = strings.TrimSpace(description)
	if description == "" {
		return nil, errors.New("description cannot be empty or whitespace")
	}
	abilityType, err := parseStringArg(args, "type")
	if err != nil {
		return nil, err
	}
	abilityType = strings.ToLower(strings.TrimSpace(abilityType))
	if _, allowed := allowedAbilityTypes[abilityType]; !allowed {
		return nil, errors.New("type must be one of: passive, active")
	}
	cooldown, hasCooldown, err := parseOptionalCooldownArg(args, "cooldown")
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

	abilities, err := parsePlayerAbilities(playerCharacter.Abilities)
	if err != nil {
		return nil, err
	}

	for _, existing := range abilities {
		if strings.EqualFold(strings.TrimSpace(existing.Name), strings.TrimSpace(name)) {
			return nil, fmt.Errorf("ability %q already exists", name)
		}
	}

	newAbility := playerAbility{
		Name:        name,
		Description: description,
		Type:        abilityType,
	}
	if hasCooldown {
		newAbility.Cooldown = &cooldown
	}

	abilities = append(abilities, newAbility)
	updatedAbilities, err := json.Marshal(abilities)
	if err != nil {
		return nil, fmt.Errorf("marshal updated abilities: %w", err)
	}
	if err := h.store.UpdatePlayerAbilities(ctx, playerCharacterID, updatedAbilities); err != nil {
		return nil, fmt.Errorf("update player abilities: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"player_character_id": playerCharacterID.String(),
			"name":                name,
			"description":         description,
			"type":                abilityType,
			"cooldown":            newAbility.Cooldown,
		},
		Narrative: fmt.Sprintf("Added ability %s to player character.", name),
	}, nil
}

// RemoveAbilityTool returns the remove_ability tool definition and JSON schema.
func RemoveAbilityTool() llm.Tool {
	return llm.Tool{
		Name:        removeAbilityToolName,
		Description: "Remove an ability from the current player character.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"ability_name": map[string]any{
					"type":        "string",
					"description": "Ability name to remove.",
				},
			},
			"required":             []string{"ability_name"},
			"additionalProperties": false,
		},
	}
}

// RegisterRemoveAbility registers the remove_ability tool and handler.
func RegisterRemoveAbility(reg *Registry, store RemoveAbilityStore) error {
	if store == nil {
		return errors.New("remove_ability store is required")
	}
	return reg.Register(RemoveAbilityTool(), NewRemoveAbilityHandler(store).Handle)
}

// RemoveAbilityHandler executes remove_ability tool calls.
type RemoveAbilityHandler struct {
	store RemoveAbilityStore
}

// NewRemoveAbilityHandler creates a new remove_ability handler.
func NewRemoveAbilityHandler(store RemoveAbilityStore) *RemoveAbilityHandler {
	return &RemoveAbilityHandler{store: store}
}

// Handle executes the remove_ability tool.
func (h *RemoveAbilityHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("remove_ability handler is nil")
	}
	if h.store == nil {
		return nil, errors.New("remove_ability store is required")
	}

	playerCharacterID, ok := CurrentPlayerCharacterIDFromContext(ctx)
	if !ok {
		return nil, errors.New("remove_ability requires current player character id in context")
	}
	abilityName, err := parseStringArg(args, "ability_name")
	if err != nil {
		return nil, err
	}
	abilityName = strings.TrimSpace(abilityName)
	if abilityName == "" {
		return nil, errors.New("ability_name cannot be empty or whitespace")
	}

	playerCharacter, err := h.store.GetPlayerCharacterByID(ctx, playerCharacterID)
	if err != nil {
		return nil, fmt.Errorf("get player character: %w", err)
	}
	if playerCharacter == nil {
		return nil, errors.New("current player character does not exist")
	}

	abilities, err := parsePlayerAbilities(playerCharacter.Abilities)
	if err != nil {
		return nil, err
	}

	remaining := make([]playerAbility, 0, len(abilities))
	removed := false
	for _, ability := range abilities {
		if strings.EqualFold(strings.TrimSpace(ability.Name), strings.TrimSpace(abilityName)) {
			removed = true
			continue
		}
		remaining = append(remaining, ability)
	}
	if !removed {
		return nil, fmt.Errorf("ability %q was not found", abilityName)
	}

	updatedAbilities, err := json.Marshal(remaining)
	if err != nil {
		return nil, fmt.Errorf("marshal updated abilities: %w", err)
	}
	if err := h.store.UpdatePlayerAbilities(ctx, playerCharacterID, updatedAbilities); err != nil {
		return nil, fmt.Errorf("update player abilities: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"player_character_id": playerCharacterID.String(),
			"ability_name":        abilityName,
		},
		Narrative: fmt.Sprintf("Removed ability %s from player character.", abilityName),
	}, nil
}

func parsePlayerAbilities(abilitiesJSON json.RawMessage) ([]playerAbility, error) {
	if len(abilitiesJSON) == 0 {
		return []playerAbility{}, nil
	}

	var abilities []playerAbility
	if err := json.Unmarshal(abilitiesJSON, &abilities); err != nil {
		return nil, fmt.Errorf("unmarshal player abilities: %w", err)
	}
	return abilities, nil
}

func parseOptionalCooldownArg(args map[string]any, key string) (int, bool, error) {
	raw, ok := args[key]
	if !ok {
		return 0, false, nil
	}
	value, err := parseIntArg(map[string]any{key: raw}, key)
	if err != nil {
		return 0, false, err
	}
	if value < 0 {
		return 0, false, fmt.Errorf("%s must be greater than or equal to 0", key)
	}
	return value, true, nil
}
