package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/llm"
)

const revealLocationToolName = "reveal_location"

// RevealLocationStore provides persistence for revealing locations.
type RevealLocationStore interface {
	GetLocation(ctx context.Context, locationID uuid.UUID) (name, description string, err error)
	SetLocationPlayerKnown(ctx context.Context, locationID uuid.UUID) error
}

// RevealLocationTool returns the reveal_location tool definition.
func RevealLocationTool() llm.Tool {
	return llm.Tool{
		Name:        revealLocationToolName,
		Description: "Mark a location as known to the player character, making it visible on their map.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location_id": map[string]any{
					"type":        "string",
					"description": "UUID of the location to reveal to the player.",
				},
			},
			"required":             []string{"location_id"},
			"additionalProperties": false,
		},
	}
}

// RegisterRevealLocation registers the reveal_location tool.
func RegisterRevealLocation(reg *Registry, store RevealLocationStore) error {
	if store == nil {
		return errors.New("reveal_location store is required")
	}
	return reg.Register(RevealLocationTool(), NewRevealLocationHandler(store).Handle)
}

// RevealLocationHandler executes reveal_location tool calls.
type RevealLocationHandler struct {
	store RevealLocationStore
}

// NewRevealLocationHandler creates a new reveal_location handler.
func NewRevealLocationHandler(store RevealLocationStore) *RevealLocationHandler {
	return &RevealLocationHandler{store: store}
}

// Handle executes the reveal_location tool.
func (h *RevealLocationHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	locationID, err := parseUUIDArg(args, "location_id")
	if err != nil {
		return nil, err
	}

	locationName, _, err := h.store.GetLocation(ctx, locationID)
	if err != nil {
		return nil, fmt.Errorf("get location: %w", err)
	}

	if err := h.store.SetLocationPlayerKnown(ctx, locationID); err != nil {
		return nil, fmt.Errorf("reveal location: %w", err)
	}

	return &ToolResult{
		Success: true,
		Data: map[string]any{
			"location_id": locationID.String(),
			"name":        locationName,
		},
		Narrative: fmt.Sprintf("Location revealed: %s.", locationName),
	}, nil
}
