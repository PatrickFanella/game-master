package game

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/internal/tools"
)

const (
	maxInt32 = int(^uint32(0) >> 1)
	minInt32 = -maxInt32 - 1
)

// inventoryService consolidates item-related persistence for both the add_item
// and remove_item tools.
type inventoryService struct {
	queries statedb.Querier
}

// NewInventoryService creates a service that satisfies both
// tools.AddItemStore and tools.RemoveItemStore.
func NewInventoryService(q statedb.Querier) *inventoryService {
	return &inventoryService{queries: q}
}

// --- tools.AddItemStore methods ---

func (s *inventoryService) CreatePlayerItem(ctx context.Context, playerCharacterID uuid.UUID, name, description, itemType, rarity string, quantity int) (uuid.UUID, error) {
	quantityInt32, err := toInt32Quantity(quantity)
	if err != nil {
		return uuid.Nil, err
	}

	playerCharacter, err := s.queries.GetPlayerCharacterByID(ctx, dbutil.ToPgtype(playerCharacterID))
	if err != nil {
		return uuid.Nil, fmt.Errorf("get player character: %w", err)
	}

	item, err := s.queries.CreateItem(ctx, statedb.CreateItemParams{
		CampaignID:        playerCharacter.CampaignID,
		PlayerCharacterID: dbutil.ToPgtype(playerCharacterID),
		Name:              name,
		Description:       pgtype.Text{String: description, Valid: true},
		ItemType:          itemType,
		Rarity:            rarity,
		Quantity:          quantityInt32,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return dbutil.FromPgtype(item.ID), nil
}

func (s *inventoryService) CreateGeneratedItem(ctx context.Context, playerCharacterID uuid.UUID, name, description, itemType, rarity string, properties map[string]any) (uuid.UUID, error) {
	propertiesBytes, err := json.Marshal(properties)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal properties: %w", err)
	}

	playerCharacter, err := s.queries.GetPlayerCharacterByID(ctx, dbutil.ToPgtype(playerCharacterID))
	if err != nil {
		return uuid.Nil, fmt.Errorf("get player character: %w", err)
	}

	item, err := s.queries.CreateItem(ctx, statedb.CreateItemParams{
		CampaignID:        playerCharacter.CampaignID,
		PlayerCharacterID: dbutil.ToPgtype(playerCharacterID),
		Name:              name,
		Description:       pgtype.Text{String: description, Valid: true},
		ItemType:          itemType,
		Rarity:            rarity,
		Properties:        propertiesBytes,
		Equipped:          false,
		Quantity:          1,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return dbutil.FromPgtype(item.ID), nil
}

// --- tools.RemoveItemStore methods ---

func (s *inventoryService) GetPlayerItemByID(ctx context.Context, itemID uuid.UUID) (*tools.PlayerItem, error) {
	item, err := s.queries.GetItemByID(ctx, dbutil.ToPgtype(itemID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if !item.PlayerCharacterID.Valid {
		return nil, nil
	}

	properties := map[string]any{}
	if len(item.Properties) > 0 {
		if err := json.Unmarshal(item.Properties, &properties); err != nil {
			return nil, fmt.Errorf("decode item properties: %w", err)
		}
	}

	return &tools.PlayerItem{
		ID:                dbutil.FromPgtype(item.ID),
		PlayerCharacterID: dbutil.FromPgtype(item.PlayerCharacterID),
		Name:              item.Name,
		Description:       item.Description.String,
		ItemType:          item.ItemType,
		Rarity:            item.Rarity,
		Properties:        properties,
		Equipped:          item.Equipped,
		Quantity:          int(item.Quantity),
	}, nil
}

func (s *inventoryService) UpdateItemQuantity(ctx context.Context, itemID uuid.UUID, quantity int) error {
	quantityInt32, err := toInt32Quantity(quantity)
	if err != nil {
		return err
	}

	_, err = s.queries.UpdateItemQuantity(ctx, statedb.UpdateItemQuantityParams{
		ID:       dbutil.ToPgtype(itemID),
		Quantity: quantityInt32,
	})
	return err
}

func (s *inventoryService) DeleteItem(ctx context.Context, itemID uuid.UUID) error {
	return s.queries.DeleteItem(ctx, dbutil.ToPgtype(itemID))
}

func (s *inventoryService) UpdatePlayerItemProperties(ctx context.Context, itemID uuid.UUID, properties map[string]any) error {
	propertiesBytes, err := json.Marshal(properties)
	if err != nil {
		return fmt.Errorf("marshal properties: %w", err)
	}

	_, err = s.queries.UpdateItemProperties(ctx, statedb.UpdateItemPropertiesParams{
		Properties: propertiesBytes,
		ID:         dbutil.ToPgtype(itemID),
	})
	return err
}

// --- helpers ---

func toInt32Quantity(quantity int) (int32, error) {
	if quantity < minInt32 || quantity > maxInt32 {
		return 0, fmt.Errorf("quantity %d is out of range for int32", quantity)
	}
	return int32(quantity), nil
}
