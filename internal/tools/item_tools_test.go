package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type stubAddItemStore struct {
	lastPlayerID    uuid.UUID
	lastName        string
	lastDescription string
	lastType        string
	lastRarity      string
	lastQuantity    int

	itemID uuid.UUID
	err    error
}

func (s *stubAddItemStore) CreatePlayerItem(_ context.Context, playerCharacterID uuid.UUID, name, description, itemType, rarity string, quantity int) (uuid.UUID, error) {
	if s.err != nil {
		return uuid.Nil, s.err
	}
	s.lastPlayerID = playerCharacterID
	s.lastName = name
	s.lastDescription = description
	s.lastType = itemType
	s.lastRarity = rarity
	s.lastQuantity = quantity
	if s.itemID == uuid.Nil {
		s.itemID = uuid.New()
	}
	return s.itemID, nil
}

type stubRemoveItemStore struct {
	items map[uuid.UUID]*PlayerItem

	lastUpdatedID       uuid.UUID
	lastUpdatedQuantity int
	lastDeletedID       uuid.UUID

	getErr    error
	updateErr error
	deleteErr error
}

func (s *stubRemoveItemStore) GetPlayerItemByID(_ context.Context, itemID uuid.UUID) (*PlayerItem, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	item := s.items[itemID]
	if item == nil {
		return nil, nil
	}
	copied := *item
	return &copied, nil
}

func (s *stubRemoveItemStore) UpdateItemQuantity(_ context.Context, itemID uuid.UUID, quantity int) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.lastUpdatedID = itemID
	s.lastUpdatedQuantity = quantity
	return nil
}

func (s *stubRemoveItemStore) DeleteItem(_ context.Context, itemID uuid.UUID) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.lastDeletedID = itemID
	return nil
}

func TestRegisterAddItem(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterAddItem(reg, &stubAddItemStore{}); err != nil {
		t.Fatalf("register add_item: %v", err)
	}
	registered := reg.List()
	if len(registered) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(registered))
	}
	if registered[0].Name != addItemToolName {
		t.Fatalf("tool name = %q, want %q", registered[0].Name, addItemToolName)
	}
	required, ok := registered[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", registered[0].Parameters["required"])
	}
	if len(required) != 3 {
		t.Fatalf("required schema length = %d, want 3", len(required))
	}
}

func TestRegisterRemoveItem(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterRemoveItem(reg, &stubRemoveItemStore{}); err != nil {
		t.Fatalf("register remove_item: %v", err)
	}
	registered := reg.List()
	if len(registered) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(registered))
	}
	if registered[0].Name != removeItemToolName {
		t.Fatalf("tool name = %q, want %q", registered[0].Name, removeItemToolName)
	}
	required, ok := registered[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", registered[0].Parameters["required"])
	}
	if len(required) != 1 || required[0] != "item_id" {
		t.Fatalf("required schema = %#v, want [item_id]", required)
	}
}

func TestAddItemHandleDefaultQuantity(t *testing.T) {
	playerID := uuid.New()
	store := &stubAddItemStore{itemID: uuid.New()}
	h := NewAddItemHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	got, err := h.Handle(ctx, map[string]any{
		"name":        "Potion",
		"description": "Restores health",
		"item_type":   "consumable",
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if store.lastPlayerID != playerID {
		t.Fatalf("stored player id = %s, want %s", store.lastPlayerID, playerID)
	}
	if store.lastQuantity != 1 {
		t.Fatalf("stored quantity = %d, want 1", store.lastQuantity)
	}
	if got.Data["quantity"] != 1 {
		t.Fatalf("result quantity = %v, want 1", got.Data["quantity"])
	}
}

func TestAddItemHandleInvalidInputs(t *testing.T) {
	playerID := uuid.New()
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	t.Run("missing context player id", func(t *testing.T) {
		h := NewAddItemHandler(&stubAddItemStore{})
		_, err := h.Handle(context.Background(), map[string]any{
			"name":        "Potion",
			"description": "Restores health",
			"item_type":   "consumable",
		})
		if err == nil || !strings.Contains(err.Error(), "requires current player character id in context") {
			t.Fatalf("error = %v, want context player id error", err)
		}
	})

	t.Run("invalid item type", func(t *testing.T) {
		h := NewAddItemHandler(&stubAddItemStore{})
		_, err := h.Handle(ctx, map[string]any{
			"name":        "Potion",
			"description": "Restores health",
			"item_type":   "invalid",
		})
		if err == nil || !strings.Contains(err.Error(), "item_type must be one of") {
			t.Fatalf("error = %v, want item_type validation error", err)
		}
	})

	t.Run("invalid quantity", func(t *testing.T) {
		h := NewAddItemHandler(&stubAddItemStore{})
		_, err := h.Handle(ctx, map[string]any{
			"name":        "Potion",
			"description": "Restores health",
			"item_type":   "consumable",
			"quantity":    0,
		})
		if err == nil || !strings.Contains(err.Error(), "quantity must be greater than 0") {
			t.Fatalf("error = %v, want quantity validation error", err)
		}
	})

	t.Run("store error wrapped", func(t *testing.T) {
		h := NewAddItemHandler(&stubAddItemStore{err: errors.New("db down")})
		_, err := h.Handle(ctx, map[string]any{
			"name":        "Potion",
			"description": "Restores health",
			"item_type":   "consumable",
		})
		if err == nil || !strings.Contains(err.Error(), "create item: db down") {
			t.Fatalf("error = %v, want wrapped store error", err)
		}
	})
}

func TestRemoveItemHandleDecrementAndDelete(t *testing.T) {
	playerID := uuid.New()
	itemID := uuid.New()
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	t.Run("decrement quantity", func(t *testing.T) {
		store := &stubRemoveItemStore{
			items: map[uuid.UUID]*PlayerItem{
				itemID: {ID: itemID, PlayerCharacterID: playerID, Name: "Arrow", Quantity: 5},
			},
		}
		h := NewRemoveItemHandler(store)
		got, err := h.Handle(ctx, map[string]any{
			"item_id":  itemID.String(),
			"quantity": 2,
		})
		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if store.lastUpdatedID != itemID || store.lastUpdatedQuantity != 3 {
			t.Fatalf("updated id/qty = %s/%d, want %s/3", store.lastUpdatedID, store.lastUpdatedQuantity, itemID)
		}
		if store.lastDeletedID != uuid.Nil {
			t.Fatalf("unexpected delete id = %s", store.lastDeletedID)
		}
		if got.Data["remaining_quantity"] != 3 {
			t.Fatalf("remaining_quantity = %v, want 3", got.Data["remaining_quantity"])
		}
	})

	t.Run("default quantity removes all and deletes", func(t *testing.T) {
		store := &stubRemoveItemStore{
			items: map[uuid.UUID]*PlayerItem{
				itemID: {ID: itemID, PlayerCharacterID: playerID, Name: "Arrow", Quantity: 5},
			},
		}
		h := NewRemoveItemHandler(store)
		got, err := h.Handle(ctx, map[string]any{
			"item_id": itemID.String(),
		})
		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if store.lastDeletedID != itemID {
			t.Fatalf("deleted id = %s, want %s", store.lastDeletedID, itemID)
		}
		if store.lastUpdatedID != uuid.Nil {
			t.Fatalf("unexpected update id = %s", store.lastUpdatedID)
		}
		if got.Data["deleted"] != true {
			t.Fatalf("deleted flag = %v, want true", got.Data["deleted"])
		}
	})
}

func TestRemoveItemHandleInvalidInputs(t *testing.T) {
	playerID := uuid.New()
	otherPlayerID := uuid.New()
	itemID := uuid.New()
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	t.Run("missing context player id", func(t *testing.T) {
		h := NewRemoveItemHandler(&stubRemoveItemStore{})
		_, err := h.Handle(context.Background(), map[string]any{
			"item_id": itemID.String(),
		})
		if err == nil || !strings.Contains(err.Error(), "requires current player character id in context") {
			t.Fatalf("error = %v, want context player id error", err)
		}
	})

	t.Run("item not found", func(t *testing.T) {
		h := NewRemoveItemHandler(&stubRemoveItemStore{items: map[uuid.UUID]*PlayerItem{}})
		_, err := h.Handle(ctx, map[string]any{"item_id": itemID.String()})
		if err == nil || !strings.Contains(err.Error(), "does not reference an existing item") {
			t.Fatalf("error = %v, want missing item error", err)
		}
	})

	t.Run("item belongs to other player", func(t *testing.T) {
		h := NewRemoveItemHandler(&stubRemoveItemStore{
			items: map[uuid.UUID]*PlayerItem{
				itemID: {ID: itemID, PlayerCharacterID: otherPlayerID, Name: "Ring", Quantity: 1},
			},
		})
		_, err := h.Handle(ctx, map[string]any{"item_id": itemID.String()})
		if err == nil || !strings.Contains(err.Error(), "does not belong to current player") {
			t.Fatalf("error = %v, want ownership error", err)
		}
	})

	t.Run("quantity exceeds", func(t *testing.T) {
		h := NewRemoveItemHandler(&stubRemoveItemStore{
			items: map[uuid.UUID]*PlayerItem{
				itemID: {ID: itemID, PlayerCharacterID: playerID, Name: "Ring", Quantity: 1},
			},
		})
		_, err := h.Handle(ctx, map[string]any{
			"item_id":  itemID.String(),
			"quantity": 2,
		})
		if err == nil || !strings.Contains(err.Error(), "quantity exceeds item quantity") {
			t.Fatalf("error = %v, want quantity exceeds error", err)
		}
	})

	t.Run("invalid quantity", func(t *testing.T) {
		h := NewRemoveItemHandler(&stubRemoveItemStore{
			items: map[uuid.UUID]*PlayerItem{
				itemID: {ID: itemID, PlayerCharacterID: playerID, Name: "Ring", Quantity: 1},
			},
		})
		_, err := h.Handle(ctx, map[string]any{
			"item_id":  itemID.String(),
			"quantity": 0,
		})
		if err == nil || !strings.Contains(err.Error(), "quantity must be greater than 0") {
			t.Fatalf("error = %v, want invalid quantity error", err)
		}
	})

	t.Run("wrapped store errors", func(t *testing.T) {
		hGet := NewRemoveItemHandler(&stubRemoveItemStore{getErr: errors.New("read fail")})
		_, err := hGet.Handle(ctx, map[string]any{"item_id": itemID.String()})
		if err == nil || !strings.Contains(err.Error(), "get item: read fail") {
			t.Fatalf("error = %v, want wrapped get error", err)
		}

		hUpd := NewRemoveItemHandler(&stubRemoveItemStore{
			items: map[uuid.UUID]*PlayerItem{
				itemID: {ID: itemID, PlayerCharacterID: playerID, Name: "Ring", Quantity: 2},
			},
			updateErr: errors.New("write fail"),
		})
		_, err = hUpd.Handle(ctx, map[string]any{"item_id": itemID.String(), "quantity": 1})
		if err == nil || !strings.Contains(err.Error(), "update item quantity: write fail") {
			t.Fatalf("error = %v, want wrapped update error", err)
		}

		hDel := NewRemoveItemHandler(&stubRemoveItemStore{
			items: map[uuid.UUID]*PlayerItem{
				itemID: {ID: itemID, PlayerCharacterID: playerID, Name: "Ring", Quantity: 1},
			},
			deleteErr: errors.New("delete fail"),
		})
		_, err = hDel.Handle(ctx, map[string]any{"item_id": itemID.String()})
		if err == nil || !strings.Contains(err.Error(), "delete item: delete fail") {
			t.Fatalf("error = %v, want wrapped delete error", err)
		}
	})
}

var _ AddItemStore = (*stubAddItemStore)(nil)
var _ RemoveItemStore = (*stubRemoveItemStore)(nil)
