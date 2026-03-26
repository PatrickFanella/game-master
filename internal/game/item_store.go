package game

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/internal/tools"
)

// itemStore adapts statedb.Querier to add_item/remove_item tool store interfaces.
type itemStore struct {
	queries statedb.Querier
}

// NewAddItemStore creates a tools.AddItemStore backed by the given Querier.
func NewAddItemStore(q statedb.Querier) tools.AddItemStore {
	return &itemStore{queries: q}
}

// NewRemoveItemStore creates a tools.RemoveItemStore backed by the given Querier.
func NewRemoveItemStore(q statedb.Querier) tools.RemoveItemStore {
	return &itemStore{queries: q}
}

func (s *itemStore) CreatePlayerItem(ctx context.Context, playerCharacterID uuid.UUID, name, description, itemType, rarity string, quantity int) (uuid.UUID, error) {
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
		Quantity:          int32(quantity),
	})
	if err != nil {
		return uuid.Nil, err
	}
	return dbutil.FromPgtype(item.ID), nil
}

func (s *itemStore) GetPlayerItemByID(ctx context.Context, itemID uuid.UUID) (*tools.PlayerItem, error) {
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

	return &tools.PlayerItem{
		ID:                dbutil.FromPgtype(item.ID),
		PlayerCharacterID: dbutil.FromPgtype(item.PlayerCharacterID),
		Name:              item.Name,
		Quantity:          int(item.Quantity),
	}, nil
}

func (s *itemStore) UpdateItemQuantity(ctx context.Context, itemID uuid.UUID, quantity int) error {
	_, err := s.queries.UpdateItemQuantity(ctx, statedb.UpdateItemQuantityParams{
		ID:       dbutil.ToPgtype(itemID),
		Quantity: int32(quantity),
	})
	return err
}

func (s *itemStore) DeleteItem(ctx context.Context, itemID uuid.UUID) error {
	return s.queries.DeleteItem(ctx, dbutil.ToPgtype(itemID))
}
