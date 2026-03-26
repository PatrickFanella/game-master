package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/llm"
)

const (
	addItemToolName    = "add_item"
	removeItemToolName = "remove_item"
	defaultAddQuantity = 1
	defaultItemRarity  = "common"
)

var allowedItemTypes = map[string]struct{}{
	string(domain.ItemTypeWeapon):     {},
	string(domain.ItemTypeArmor):      {},
	string(domain.ItemTypeConsumable): {},
	string(domain.ItemTypeQuest):      {},
	string(domain.ItemTypeMisc):       {},
}

// PlayerItem represents a player's item stack.
type PlayerItem struct {
	ID                uuid.UUID
	PlayerCharacterID uuid.UUID
	Name              string
	Quantity          int
}

// AddItemStore persists item creation for player characters.
type AddItemStore interface {
	CreatePlayerItem(ctx context.Context, playerCharacterID uuid.UUID, name, description, itemType, rarity string, quantity int) (uuid.UUID, error)
}

// RemoveItemStore loads and mutates item stacks.
type RemoveItemStore interface {
	GetPlayerItemByID(ctx context.Context, itemID uuid.UUID) (*PlayerItem, error)
	UpdateItemQuantity(ctx context.Context, itemID uuid.UUID, quantity int) error
	DeleteItem(ctx context.Context, itemID uuid.UUID) error
}

// AddItemTool returns the add_item tool definition and JSON schema.
func AddItemTool() llm.Tool {
	return llm.Tool{
		Name:        addItemToolName,
		Description: "Add an item to the current player character inventory.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Item name.",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Item description.",
				},
				"item_type": map[string]any{
					"type":        "string",
					"description": "Item type. One of: weapon, armor, consumable, quest, misc.",
				},
				"quantity": map[string]any{
					"type":        "integer",
					"description": "Quantity to add. Defaults to 1.",
				},
			},
			"required":             []string{"name", "description", "item_type"},
			"additionalProperties": false,
		},
	}
}

// RegisterAddItem registers the add_item tool and handler.
func RegisterAddItem(reg *Registry, store AddItemStore) error {
	if store == nil {
		return errors.New("add_item store is required")
	}
	return reg.Register(AddItemTool(), NewAddItemHandler(store).Handle)
}

// AddItemHandler executes add_item tool calls.
type AddItemHandler struct {
	store AddItemStore
}

// NewAddItemHandler creates a new add_item handler.
func NewAddItemHandler(store AddItemStore) *AddItemHandler {
	return &AddItemHandler{store: store}
}

// Handle executes the add_item tool.
func (h *AddItemHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("add_item handler is nil")
	}
	if h.store == nil {
		return nil, errors.New("add_item store is required")
	}

	playerCharacterID, ok := CurrentPlayerCharacterIDFromContext(ctx)
	if !ok {
		return nil, errors.New("add_item requires current player character id in context")
	}

	name, err := parseStringArg(args, "name")
	if err != nil {
		return nil, err
	}
	description, err := parseStringArg(args, "description")
	if err != nil {
		return nil, err
	}
	itemType, err := parseStringArg(args, "item_type")
	if err != nil {
		return nil, err
	}
	if _, allowed := allowedItemTypes[itemType]; !allowed {
		return nil, errors.New("item_type must be one of: weapon, armor, consumable, quest, misc")
	}

	quantity, err := parsePositiveIntArgWithDefault(args, "quantity", defaultAddQuantity)
	if err != nil {
		return nil, err
	}

	itemID, err := h.store.CreatePlayerItem(ctx, playerCharacterID, name, description, itemType, defaultItemRarity, quantity)
	if err != nil {
		return nil, fmt.Errorf("create item: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"item_id":              itemID.String(),
			"player_character_id":  playerCharacterID.String(),
			"name":                 name,
			"description":          description,
			"item_type":            itemType,
			"quantity":             quantity,
		},
		Narrative: fmt.Sprintf("Added %d %s to player inventory.", quantity, name),
	}, nil
}

// RemoveItemTool returns the remove_item tool definition and JSON schema.
func RemoveItemTool() llm.Tool {
	return llm.Tool{
		Name:        removeItemToolName,
		Description: "Remove or decrement an item stack from the current player character inventory.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"item_id": map[string]any{
					"type":        "string",
					"description": "Item UUID to remove.",
				},
				"quantity": map[string]any{
					"type":        "integer",
					"description": "Quantity to remove. Defaults to all quantity.",
				},
			},
			"required":             []string{"item_id"},
			"additionalProperties": false,
		},
	}
}

// RegisterRemoveItem registers the remove_item tool and handler.
func RegisterRemoveItem(reg *Registry, store RemoveItemStore) error {
	if store == nil {
		return errors.New("remove_item store is required")
	}
	return reg.Register(RemoveItemTool(), NewRemoveItemHandler(store).Handle)
}

// RemoveItemHandler executes remove_item tool calls.
type RemoveItemHandler struct {
	store RemoveItemStore
}

// NewRemoveItemHandler creates a new remove_item handler.
func NewRemoveItemHandler(store RemoveItemStore) *RemoveItemHandler {
	return &RemoveItemHandler{store: store}
}

// Handle executes the remove_item tool.
func (h *RemoveItemHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("remove_item handler is nil")
	}
	if h.store == nil {
		return nil, errors.New("remove_item store is required")
	}

	playerCharacterID, ok := CurrentPlayerCharacterIDFromContext(ctx)
	if !ok {
		return nil, errors.New("remove_item requires current player character id in context")
	}

	itemID, err := parseUUIDArg(args, "item_id")
	if err != nil {
		return nil, err
	}

	item, err := h.store.GetPlayerItemByID(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("get item: %w", err)
	}
	if item == nil {
		return nil, errors.New("item_id does not reference an existing item")
	}
	if item.PlayerCharacterID != playerCharacterID {
		return nil, errors.New("item does not belong to current player")
	}

	removeQuantity, hasQuantity, err := parseOptionalPositiveIntArg(args, "quantity")
	if err != nil {
		return nil, err
	}
	if !hasQuantity {
		removeQuantity = item.Quantity
	}
	if removeQuantity > item.Quantity {
		return nil, errors.New("quantity exceeds item quantity")
	}

	remaining := item.Quantity - removeQuantity
	if remaining == 0 {
		if err := h.store.DeleteItem(ctx, item.ID); err != nil {
			return nil, fmt.Errorf("delete item: %w", err)
		}
		return &ToolResult{
			Success: true,
			Data: map[string]any{
				"item_id":             item.ID.String(),
				"player_character_id": playerCharacterID.String(),
				"name":                item.Name,
				"removed_quantity":    removeQuantity,
				"remaining_quantity":  0,
				"deleted":             true,
			},
			Narrative: fmt.Sprintf("Removed %d %s from player inventory. Item removed completely.", removeQuantity, item.Name),
		}, nil
	}

	if err := h.store.UpdateItemQuantity(ctx, item.ID, remaining); err != nil {
		return nil, fmt.Errorf("update item quantity: %w", err)
	}
	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"item_id":             item.ID.String(),
			"player_character_id": playerCharacterID.String(),
			"name":                item.Name,
			"removed_quantity":    removeQuantity,
			"remaining_quantity":  remaining,
			"deleted":             false,
		},
		Narrative: fmt.Sprintf("Removed %d %s from player inventory. %d remaining.", removeQuantity, item.Name, remaining),
	}, nil
}

func parsePositiveIntArgWithDefault(args map[string]any, key string, defaultValue int) (int, error) {
	raw, ok := args[key]
	if !ok {
		return defaultValue, nil
	}
	value, err := parseIntArg(map[string]any{key: raw}, key)
	if err != nil {
		return 0, err
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be greater than 0", key)
	}
	return value, nil
}

func parseOptionalPositiveIntArg(args map[string]any, key string) (int, bool, error) {
	raw, ok := args[key]
	if !ok {
		return 0, false, nil
	}
	value, err := parseIntArg(map[string]any{key: raw}, key)
	if err != nil {
		return 0, false, err
	}
	if value <= 0 {
		return 0, false, fmt.Errorf("%s must be greater than 0", key)
	}
	return value, true, nil
}
