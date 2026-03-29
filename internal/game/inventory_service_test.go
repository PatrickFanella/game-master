package game

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/internal/tools"
)

var _ tools.AddItemStore = (*inventoryService)(nil)
var _ tools.RemoveItemStore = (*inventoryService)(nil)

func TestToInt32Quantity(t *testing.T) {
	t.Run("within range", func(t *testing.T) {
		got, err := toInt32Quantity(42)
		if err != nil {
			t.Fatalf("toInt32Quantity(42) error = %v", err)
		}
		if got != 42 {
			t.Fatalf("toInt32Quantity(42) = %d, want 42", got)
		}
	})

	t.Run("too large", func(t *testing.T) {
		if _, err := toInt32Quantity(maxInt32 + 1); err == nil {
			t.Fatal("expected out-of-range error for maxInt32+1")
		}
	})

	t.Run("too small", func(t *testing.T) {
		if _, err := toInt32Quantity(minInt32 - 1); err == nil {
			t.Fatal("expected out-of-range error for minInt32-1")
		}
	})
}

func TestInventoryServiceCreatePlayerItem(t *testing.T) {
	q := newMockQuerier()
	playerID := uuid.New()
	campaignID := uuid.New()
	itemID := uuid.New()

	q.playerCharacter = statedb.PlayerCharacter{
		ID:         dbutil.ToPgtype(playerID),
		CampaignID: dbutil.ToPgtype(campaignID),
	}
	q.createItemResult = statedb.Item{ID: dbutil.ToPgtype(itemID)}
	svc := NewInventoryService(q)

	got, err := svc.CreatePlayerItem(context.Background(), playerID, "Iron Sword", "A sturdy blade.", "weapon", "common", 1)
	if err != nil {
		t.Fatalf("CreatePlayerItem() error = %v", err)
	}
	if got != itemID {
		t.Fatalf("returned ID = %s, want %s", got, itemID)
	}
}

func TestInventoryServiceCreatePlayerItemCharacterNotFound(t *testing.T) {
	q := newMockQuerier()
	svc := NewInventoryService(q)

	_, err := svc.CreatePlayerItem(context.Background(), uuid.New(), "Sword", "", "weapon", "common", 1)
	if err == nil {
		t.Fatal("expected error for missing player character")
	}
	if !strings.Contains(err.Error(), "get player character") {
		t.Fatalf("error = %v, want 'get player character' context", err)
	}
}

func TestInventoryServiceGetPlayerItemByIDFound(t *testing.T) {
	q := newMockQuerier()
	itemID := uuid.New()
	playerID := uuid.New()
	pgItemID := dbutil.ToPgtype(itemID)

	q.itemByID[pgItemID] = statedb.Item{
		ID:                pgItemID,
		PlayerCharacterID: dbutil.ToPgtype(playerID),
		Name:              "Health Potion",
		Quantity:          3,
	}
	svc := NewInventoryService(q)

	got, err := svc.GetPlayerItemByID(context.Background(), itemID)
	if err != nil {
		t.Fatalf("GetPlayerItemByID() error = %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil item")
	}
	if got.ID != itemID {
		t.Fatalf("item ID = %s, want %s", got.ID, itemID)
	}
	if got.Name != "Health Potion" {
		t.Fatalf("name = %q, want Health Potion", got.Name)
	}
	if got.Quantity != 3 {
		t.Fatalf("quantity = %d, want 3", got.Quantity)
	}
}

func TestInventoryServiceGetPlayerItemByIDNotFound(t *testing.T) {
	q := newMockQuerier()
	svc := NewInventoryService(q)

	got, err := svc.GetPlayerItemByID(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("GetPlayerItemByID() error = %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestInventoryServiceGetPlayerItemByIDNoPlayer(t *testing.T) {
	q := newMockQuerier()
	itemID := uuid.New()
	pgItemID := dbutil.ToPgtype(itemID)

	q.itemByID[pgItemID] = statedb.Item{
		ID:                pgItemID,
		PlayerCharacterID: pgtype.UUID{}, // not valid
		Name:              "Orphaned Item",
		Quantity:          1,
	}
	svc := NewInventoryService(q)

	got, err := svc.GetPlayerItemByID(context.Background(), itemID)
	if err != nil {
		t.Fatalf("GetPlayerItemByID() error = %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for item without player, got %+v", got)
	}
}

func TestInventoryServiceDeleteItem(t *testing.T) {
	q := newMockQuerier()
	itemID := uuid.New()
	svc := NewInventoryService(q)

	err := svc.DeleteItem(context.Background(), itemID)
	if err != nil {
		t.Fatalf("DeleteItem() error = %v", err)
	}
	if q.lastDeletedItemID == nil {
		t.Fatal("expected DeleteItem to be called")
	}
	if dbutil.FromPgtype(*q.lastDeletedItemID) != itemID {
		t.Fatalf("deleted ID = %v, want %s", q.lastDeletedItemID, itemID)
	}
}

func TestInventoryServiceUpdatePlayerItemProperties(t *testing.T) {
	q := newMockQuerier()
	itemID := uuid.New()
	svc := NewInventoryService(q)

	err := svc.UpdatePlayerItemProperties(context.Background(), itemID, map[string]any{
		"charges": 2,
		"damage":  "1d8",
	})
	if err != nil {
		t.Fatalf("UpdatePlayerItemProperties() error = %v", err)
	}
	if q.lastUpdateItemPropParams == nil {
		t.Fatal("expected UpdateItemProperties to be called")
	}
	if dbutil.FromPgtype(q.lastUpdateItemPropParams.ID) != itemID {
		t.Fatalf("updated ID = %v, want %s", q.lastUpdateItemPropParams.ID, itemID)
	}
	if len(q.lastUpdateItemPropParams.Properties) == 0 {
		t.Fatal("expected marshaled properties to be non-empty")
	}
}
