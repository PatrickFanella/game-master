package game

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/internal/tools"
)

// movePlayerStore adapts statedb.Querier to the tools.MovePlayerStore interface.
type movePlayerStore struct {
	queries statedb.Querier
}

// NewMovePlayerStore creates a tools.MovePlayerStore backed by the given Querier.
func NewMovePlayerStore(q statedb.Querier) tools.MovePlayerStore {
	return &movePlayerStore{queries: q}
}

func (s *movePlayerStore) GetLocation(ctx context.Context, locationID uuid.UUID) (name, description string, err error) {
	location, err := s.queries.GetLocationByID(ctx, dbutil.ToPgtype(locationID))
	if err != nil {
		return "", "", err
	}
	return location.Name, location.Description.String, nil
}

func (s *movePlayerStore) IsLocationConnected(ctx context.Context, fromLocationID, toLocationID uuid.UUID) (bool, error) {
	fromLocation, err := s.queries.GetLocationByID(ctx, dbutil.ToPgtype(fromLocationID))
	if err != nil {
		return false, fmt.Errorf("get current location: %w", err)
	}

	connections, err := s.queries.GetConnectionsFromLocation(ctx, statedb.GetConnectionsFromLocationParams{
		CampaignID: fromLocation.CampaignID,
		LocationID: dbutil.ToPgtype(fromLocationID),
	})
	if err != nil {
		return false, fmt.Errorf("get connections from location: %w", err)
	}

	for _, connection := range connections {
		if dbutil.FromPgtype(connection.ConnectedLocationID) == toLocationID {
			return true, nil
		}
	}
	return false, nil
}

func (s *movePlayerStore) UpdatePlayerLocation(ctx context.Context, playerCharacterID, locationID uuid.UUID) error {
	_, err := s.queries.UpdatePlayerLocation(ctx, statedb.UpdatePlayerLocationParams{
		CurrentLocationID: dbutil.ToPgtype(locationID),
		ID:                dbutil.ToPgtype(playerCharacterID),
	})
	return err
}
