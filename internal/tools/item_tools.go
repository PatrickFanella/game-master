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
	addItemToolName    = "add_item"
	removeItemToolName = "remove_item"
	modifyItemToolName = "modify_item"
	createItemToolName = "create_item"
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

var allowedItemRarities = map[string]struct{}{
	"common":    {},
	"uncommon":  {},
	"rare":      {},
	"epic":      {},
	"legendary": {},
}

var supportedItemPropertyKeys = map[string]struct{}{
	"effects": {},
	"damage":  {},
	"armor":   {},
	"charges": {},
	"weight":  {},
}

// PlayerItem represents a player's item stack.
type PlayerItem struct {
	ID                uuid.UUID
	PlayerCharacterID uuid.UUID
	Name              string
	Description       string
	ItemType          string
	Rarity            string
	Properties        map[string]any
	Equipped          bool
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

// ModifyItemStore loads and mutates item properties.
type ModifyItemStore interface {
	GetPlayerItemByID(ctx context.Context, itemID uuid.UUID) (*PlayerItem, error)
	UpdatePlayerItemProperties(ctx context.Context, itemID uuid.UUID, properties map[string]any) error
}

// CreateItemStore creates player items with full generated properties.
type CreateItemStore interface {
	CreateGeneratedItem(ctx context.Context, playerCharacterID uuid.UUID, name, description, itemType, rarity string, properties map[string]any) (uuid.UUID, error)
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
			"item_id":             itemID.String(),
			"player_character_id": playerCharacterID.String(),
			"name":                name,
			"description":         description,
			"item_type":           itemType,
			"quantity":            quantity,
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

// ModifyItemTool returns the modify_item tool definition and JSON schema.
func ModifyItemTool() llm.Tool {
	return llm.Tool{
		Name:        modifyItemToolName,
		Description: "Modify properties on an existing item owned by the current player character.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"item_id": map[string]any{
					"type":        "string",
					"description": "Item UUID to modify.",
				},
				"properties": map[string]any{
					"type":        "object",
					"description": "Item property updates. Supported keys: effects, damage, armor, charges, weight.",
				},
			},
			"required":             []string{"item_id", "properties"},
			"additionalProperties": false,
		},
	}
}

// CreateItemTool returns the create_item tool definition and JSON schema.
func CreateItemTool() llm.Tool {
	return llm.Tool{
		Name:        createItemToolName,
		Description: "Create a new generated item with full properties for the current player character.",
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
				"rarity": map[string]any{
					"type":        "string",
					"description": "Item rarity. One of: common, uncommon, rare, epic, legendary.",
				},
				"properties": map[string]any{
					"type":        "object",
					"description": "Item properties. Supported keys: effects, damage, armor, charges, weight.",
				},
			},
			"required":             []string{"name", "description", "item_type", "rarity", "properties"},
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

// RegisterModifyItem registers the modify_item tool and handler.
func RegisterModifyItem(reg *Registry, store ModifyItemStore) error {
	if store == nil {
		return errors.New("modify_item store is required")
	}
	return reg.Register(ModifyItemTool(), NewModifyItemHandler(store).Handle)
}

// RegisterCreateItem registers the create_item tool and handler.
func RegisterCreateItem(reg *Registry, store CreateItemStore) error {
	if store == nil {
		return errors.New("create_item store is required")
	}
	return reg.Register(CreateItemTool(), NewCreateItemHandler(store).Handle)
}

// RemoveItemHandler executes remove_item tool calls.
type RemoveItemHandler struct {
	store RemoveItemStore
}

// NewRemoveItemHandler creates a new remove_item handler.
func NewRemoveItemHandler(store RemoveItemStore) *RemoveItemHandler {
	return &RemoveItemHandler{store: store}
}

// ModifyItemHandler executes modify_item tool calls.
type ModifyItemHandler struct {
	store ModifyItemStore
}

// NewModifyItemHandler creates a new modify_item handler.
func NewModifyItemHandler(store ModifyItemStore) *ModifyItemHandler {
	return &ModifyItemHandler{store: store}
}

// CreateItemHandler executes create_item tool calls.
type CreateItemHandler struct {
	store CreateItemStore
}

// NewCreateItemHandler creates a new create_item handler.
func NewCreateItemHandler(store CreateItemStore) *CreateItemHandler {
	return &CreateItemHandler{store: store}
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

// Handle executes the modify_item tool.
func (h *ModifyItemHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("modify_item handler is nil")
	}
	if h.store == nil {
		return nil, errors.New("modify_item store is required")
	}

	playerCharacterID, ok := CurrentPlayerCharacterIDFromContext(ctx)
	if !ok {
		return nil, errors.New("modify_item requires current player character id in context")
	}

	itemID, err := parseUUIDArg(args, "item_id")
	if err != nil {
		return nil, err
	}
	updates, err := parseItemPropertiesArg(args, "properties")
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

	mergedProperties := mergeItemProperties(item.Properties, updates)
	if err := h.store.UpdatePlayerItemProperties(ctx, item.ID, mergedProperties); err != nil {
		return nil, fmt.Errorf("update item properties: %w", err)
	}

	formatted := formatItemDescription(item.Name, item.Description, item.ItemType, item.Rarity, mergedProperties)
	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"item_id":               item.ID.String(),
			"player_character_id":   playerCharacterID.String(),
			"name":                  item.Name,
			"description":           item.Description,
			"item_type":             item.ItemType,
			"rarity":                item.Rarity,
			"properties":            mergedProperties,
			"formatted_description": formatted,
		},
		Narrative: fmt.Sprintf("Modified item: %s", formatted),
	}, nil
}

// Handle executes the create_item tool.
func (h *CreateItemHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("create_item handler is nil")
	}
	if h.store == nil {
		return nil, errors.New("create_item store is required")
	}

	playerCharacterID, ok := CurrentPlayerCharacterIDFromContext(ctx)
	if !ok {
		return nil, errors.New("create_item requires current player character id in context")
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
	itemType = strings.ToLower(strings.TrimSpace(itemType))
	if _, allowed := allowedItemTypes[itemType]; !allowed {
		return nil, errors.New("item_type must be one of: weapon, armor, consumable, quest, misc")
	}
	rarity, err := parseStringArg(args, "rarity")
	if err != nil {
		return nil, err
	}
	rarity = strings.ToLower(strings.TrimSpace(rarity))
	if _, allowed := allowedItemRarities[rarity]; !allowed {
		return nil, errors.New("rarity must be one of: common, uncommon, rare, epic, legendary")
	}

	properties, err := parseItemPropertiesArg(args, "properties")
	if err != nil {
		return nil, err
	}

	itemID, err := h.store.CreateGeneratedItem(ctx, playerCharacterID, name, description, itemType, rarity, properties)
	if err != nil {
		return nil, fmt.Errorf("create item: %w", err)
	}

	formatted := formatItemDescription(name, description, itemType, rarity, properties)
	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"item_id":               itemID.String(),
			"player_character_id":   playerCharacterID.String(),
			"name":                  name,
			"description":           description,
			"item_type":             itemType,
			"rarity":                rarity,
			"properties":            properties,
			"formatted_description": formatted,
		},
		Narrative: fmt.Sprintf("Created item: %s", formatted),
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

func parseItemPropertiesArg(args map[string]any, key string) (map[string]any, error) {
	raw, err := parseJSONObjectArg(args, key)
	if err != nil {
		return nil, err
	}

	properties := make(map[string]any, len(raw))
	for rawKey, value := range raw {
		normalizedKey := strings.ToLower(strings.TrimSpace(rawKey))
		if normalizedKey == "" {
			return nil, fmt.Errorf("%s contains an empty property key", key)
		}
		if _, ok := supportedItemPropertyKeys[normalizedKey]; !ok {
			return nil, fmt.Errorf("%s supports only: effects, damage, armor, charges, weight", key)
		}
		properties[normalizedKey] = value
	}
	return properties, nil
}

func mergeItemProperties(existing, updates map[string]any) map[string]any {
	merged := make(map[string]any, len(existing)+len(updates))
	for key, value := range existing {
		merged[key] = value
	}
	for key, value := range updates {
		merged[key] = value
	}
	return merged
}

func formatItemDescription(name, description, itemType, rarity string, properties map[string]any) string {
	orderedKeys := []string{"effects", "damage", "armor", "charges", "weight"}
	propertyParts := make([]string, 0, len(orderedKeys))
	for _, key := range orderedKeys {
		value, ok := properties[key]
		if !ok {
			continue
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			propertyParts = append(propertyParts, fmt.Sprintf("%s=%v", key, value))
			continue
		}
		propertyParts = append(propertyParts, fmt.Sprintf("%s=%s", key, string(encoded)))
	}
	if len(propertyParts) == 0 {
		return fmt.Sprintf("%s (%s %s): %s", name, rarity, itemType, description)
	}
	return fmt.Sprintf("%s (%s %s): %s [properties: %s]", name, rarity, itemType, description, strings.Join(propertyParts, ", "))
}
