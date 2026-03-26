package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type stubMovePlayerStore struct {
	locationNames        map[uuid.UUID]string
	locationDescriptions map[uuid.UUID]string
	connected            map[uuid.UUID]map[uuid.UUID]bool

	lastUpdatedPlayerID   uuid.UUID
	lastUpdatedLocationID uuid.UUID

	getLocationErr error
	connectionErr  error
	updateErr      error
}

func (s *stubMovePlayerStore) GetLocation(_ context.Context, locationID uuid.UUID) (string, string, error) {
	if s.getLocationErr != nil {
		return "", "", s.getLocationErr
	}
	name, ok := s.locationNames[locationID]
	if !ok {
		return "", "", errors.New("location not found")
	}
	return name, s.locationDescriptions[locationID], nil
}

func (s *stubMovePlayerStore) IsLocationConnected(_ context.Context, fromLocationID, toLocationID uuid.UUID) (bool, error) {
	if s.connectionErr != nil {
		return false, s.connectionErr
	}
	return s.connected[fromLocationID][toLocationID], nil
}

func (s *stubMovePlayerStore) UpdatePlayerLocation(_ context.Context, playerCharacterID, locationID uuid.UUID) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.lastUpdatedPlayerID = playerCharacterID
	s.lastUpdatedLocationID = locationID
	return nil
}

func TestRegisterMovePlayer(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterMovePlayer(reg, &stubMovePlayerStore{}); err != nil {
		t.Fatalf("register move_player: %v", err)
	}

	registered := reg.List()
	if len(registered) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(registered))
	}
	if registered[0].Name != movePlayerToolName {
		t.Fatalf("tool name = %q, want %q", registered[0].Name, movePlayerToolName)
	}
	required, ok := registered[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", registered[0].Parameters["required"])
	}
	if len(required) != 1 || required[0] != "location_id" {
		t.Fatalf("required schema = %#v, want [location_id]", required)
	}
}

func TestMovePlayerHandleConnectedLocation(t *testing.T) {
	currentLocationID := uuid.New()
	targetLocationID := uuid.New()
	playerID := uuid.New()
	store := &stubMovePlayerStore{
		locationNames: map[uuid.UUID]string{
			targetLocationID: "Ancient Gate",
		},
		locationDescriptions: map[uuid.UUID]string{
			targetLocationID: "A ruined gate covered in glowing runes.",
		},
		connected: map[uuid.UUID]map[uuid.UUID]bool{
			currentLocationID: {
				targetLocationID: true,
			},
		},
	}
	h := NewMovePlayerHandler(store)
	ctx := WithCurrentPlayerCharacterID(WithCurrentLocationID(context.Background(), currentLocationID), playerID)

	got, err := h.Handle(ctx, map[string]any{
		"location_id": targetLocationID.String(),
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if store.lastUpdatedPlayerID != playerID {
		t.Fatalf("updated player_id = %s, want %s", store.lastUpdatedPlayerID, playerID)
	}
	if store.lastUpdatedLocationID != targetLocationID {
		t.Fatalf("updated location_id = %s, want %s", store.lastUpdatedLocationID, targetLocationID)
	}
	if got.Data["name"] != "Ancient Gate" {
		t.Fatalf("result name = %v, want Ancient Gate", got.Data["name"])
	}
	if got.Data["description"] != "A ruined gate covered in glowing runes." {
		t.Fatalf("result description = %v, want expected description", got.Data["description"])
	}
}

func TestMovePlayerHandleUnconnectedLocation(t *testing.T) {
	currentLocationID := uuid.New()
	targetLocationID := uuid.New()
	playerID := uuid.New()
	store := &stubMovePlayerStore{
		locationNames: map[uuid.UUID]string{
			targetLocationID: "Frost Bridge",
		},
		locationDescriptions: map[uuid.UUID]string{
			targetLocationID: "A narrow bridge suspended above a frozen chasm.",
		},
		connected: map[uuid.UUID]map[uuid.UUID]bool{
			currentLocationID: {},
		},
	}
	h := NewMovePlayerHandler(store)
	ctx := WithCurrentPlayerCharacterID(WithCurrentLocationID(context.Background(), currentLocationID), playerID)

	_, err := h.Handle(ctx, map[string]any{
		"location_id": targetLocationID.String(),
	})
	if err == nil {
		t.Fatal("expected unconnected location error")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("error = %v, want unconnected-location message", err)
	}
	if store.lastUpdatedLocationID != uuid.Nil {
		t.Fatalf("unexpected location update to %s", store.lastUpdatedLocationID)
	}
}

func TestMovePlayerHandleNonexistentLocation(t *testing.T) {
	currentLocationID := uuid.New()
	targetLocationID := uuid.New()
	playerID := uuid.New()
	store := &stubMovePlayerStore{
		locationNames:        map[uuid.UUID]string{},
		locationDescriptions: map[uuid.UUID]string{},
		connected: map[uuid.UUID]map[uuid.UUID]bool{
			currentLocationID: {
				targetLocationID: true,
			},
		},
	}
	h := NewMovePlayerHandler(store)
	ctx := WithCurrentPlayerCharacterID(WithCurrentLocationID(context.Background(), currentLocationID), playerID)

	_, err := h.Handle(ctx, map[string]any{
		"location_id": targetLocationID.String(),
	})
	if err == nil {
		t.Fatal("expected nonexistent location error")
	}
	if !strings.Contains(err.Error(), "get target location") {
		t.Fatalf("error = %v, want get-target-location message", err)
	}
	if store.lastUpdatedLocationID != uuid.Nil {
		t.Fatalf("unexpected location update to %s", store.lastUpdatedLocationID)
	}
}

var _ MovePlayerStore = (*stubMovePlayerStore)(nil)
