package game

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

// locationService consolidates location-related persistence for both the
// move_player and describe_scene tools.
type locationService struct {
	queries statedb.Querier
}

// NewLocationService creates a service that satisfies both
// tools.MovePlayerStore and tools.DescribeSceneStore.
func NewLocationService(q statedb.Querier) *locationService {
	return &locationService{queries: q}
}

// --- tools.MovePlayerStore methods ---

func (s *locationService) GetLocation(ctx context.Context, locationID uuid.UUID) (name, description string, err error) {
	location, err := s.queries.GetLocationByID(ctx, statedb.GetLocationByIDParams{ID: dbutil.ToPgtype(locationID)})
	if err != nil {
		return "", "", err
	}
	return location.Name, location.Description.String, nil
}

func (s *locationService) IsLocationConnected(ctx context.Context, fromLocationID, toLocationID uuid.UUID) (bool, error) {
	fromLocation, err := s.queries.GetLocationByID(ctx, statedb.GetLocationByIDParams{ID: dbutil.ToPgtype(fromLocationID)})
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

func (s *locationService) UpdatePlayerLocation(ctx context.Context, playerCharacterID, locationID uuid.UUID) error {
	_, err := s.queries.UpdatePlayerLocation(ctx, statedb.UpdatePlayerLocationParams{
		CurrentLocationID: dbutil.ToPgtype(locationID),
		ID:                dbutil.ToPgtype(playerCharacterID),
	})
	return err
}

func (s *locationService) SetLocationPlayerVisited(ctx context.Context, locationID uuid.UUID) error {
	return s.queries.SetLocationPlayerVisited(ctx, dbutil.ToPgtype(locationID))
}

func (s *locationService) SetLocationPlayerKnown(ctx context.Context, locationID uuid.UUID) error {
	return s.queries.SetLocationPlayerKnown(ctx, dbutil.ToPgtype(locationID))
}

// --- tools.DescribeSceneStore methods ---

func (s *locationService) UpdateScene(ctx context.Context, locationID uuid.UUID, description string, mood, timeOfDay *string) error {
	location, err := s.queries.GetLocationByID(ctx, statedb.GetLocationByIDParams{ID: dbutil.ToPgtype(locationID)})
	if err != nil {
		return fmt.Errorf("get location: %w", err)
	}

	properties := map[string]any{}
	if len(location.Properties) > 0 {
		if err := json.Unmarshal(location.Properties, &properties); err != nil {
			return fmt.Errorf("unmarshal location properties: %w", err)
		}
	}
	if mood != nil {
		properties["mood"] = *mood
	}
	if timeOfDay != nil {
		properties["time_of_day"] = *timeOfDay
	}
	propertiesJSON, err := json.Marshal(properties)
	if err != nil {
		return fmt.Errorf("marshal location properties: %w", err)
	}

	_, err = s.queries.UpdateLocation(ctx, statedb.UpdateLocationParams{
		ID:           dbutil.ToPgtype(locationID),
		Name:         location.Name,
		Description:  pgtype.Text{String: description, Valid: true},
		Region:       location.Region,
		LocationType: location.LocationType,
		Properties:   propertiesJSON,
	})
	if err != nil {
		return fmt.Errorf("update location: %w", err)
	}

	return nil
}
