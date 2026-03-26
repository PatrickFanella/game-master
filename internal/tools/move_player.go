package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/llm"
)

const movePlayerToolName = "move_player"

// MovePlayerStore provides persistence and relationship checks for player movement.
type MovePlayerStore interface {
	GetLocation(ctx context.Context, locationID uuid.UUID) (name, description string, err error)
	IsLocationConnected(ctx context.Context, fromLocationID, toLocationID uuid.UUID) (bool, error)
	UpdatePlayerLocation(ctx context.Context, playerCharacterID, locationID uuid.UUID) error
}

// MovePlayerTool returns the move_player tool definition and JSON schema.
func MovePlayerTool() llm.Tool {
	return llm.Tool{
		Name:        movePlayerToolName,
		Description: "Move the player character to a connected location.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location_id": map[string]any{
					"type":        "string",
					"description": "Destination location UUID.",
				},
			},
			"required":             []string{"location_id"},
			"additionalProperties": false,
		},
	}
}

// RegisterMovePlayer registers the move_player tool and handler.
func RegisterMovePlayer(reg *Registry, store MovePlayerStore) error {
	if store == nil {
		return errors.New("move_player store is required")
	}
	return reg.Register(MovePlayerTool(), NewMovePlayerHandler(store).Handle)
}

// MovePlayerHandler executes move_player tool calls.
type MovePlayerHandler struct {
	store MovePlayerStore
}

// NewMovePlayerHandler creates a new move_player handler.
func NewMovePlayerHandler(store MovePlayerStore) *MovePlayerHandler {
	return &MovePlayerHandler{store: store}
}

// Handle executes the move_player tool.
func (h *MovePlayerHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("move_player handler is nil")
	}
	if h.store == nil {
		return nil, errors.New("move_player store is required")
	}

	targetLocationID, err := parseUUIDArg(args, "location_id")
	if err != nil {
		return nil, err
	}

	locationName, locationDescription, err := h.store.GetLocation(ctx, targetLocationID)
	if err != nil {
		return nil, fmt.Errorf("get target location: %w", err)
	}

	currentLocationID, ok := CurrentLocationIDFromContext(ctx)
	if !ok {
		return nil, errors.New("move_player requires current location id in context")
	}

	isConnected, err := h.store.IsLocationConnected(ctx, currentLocationID, targetLocationID)
	if err != nil {
		return nil, fmt.Errorf("check location connection: %w", err)
	}
	if !isConnected {
		return nil, errors.New("target location is not connected to current location")
	}

	playerCharacterID, ok := CurrentPlayerCharacterIDFromContext(ctx)
	if !ok {
		return nil, errors.New("move_player requires current player character id in context")
	}

	if err := h.store.UpdatePlayerLocation(ctx, playerCharacterID, targetLocationID); err != nil {
		return nil, fmt.Errorf("update player location: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"location_id": targetLocationID.String(),
			"name":        locationName,
			"description": locationDescription,
		},
		Narrative: fmt.Sprintf("Player moved to %s.", locationName),
	}, nil
}
