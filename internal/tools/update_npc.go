package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/llm"
)

const updateNPCToolName = "update_npc"

// UpdateNPCStore provides NPC lookup and persistence for update_npc.
type UpdateNPCStore interface {
	GetNPCByID(ctx context.Context, npcID uuid.UUID) (*domain.NPC, error)
	LocationExistsInCampaign(ctx context.Context, locationID, campaignID uuid.UUID) (bool, error)
	UpdateNPC(ctx context.Context, npc domain.NPC) (*domain.NPC, error)
}

// UpdateNPCTool returns the update_npc tool definition and JSON schema.
func UpdateNPCTool() llm.Tool {
	return llm.Tool{
		Name:        updateNPCToolName,
		Description: "Update mutable NPC state fields.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"npc_id": map[string]any{
					"type":        "string",
					"description": "NPC UUID to update.",
				},
				"disposition": map[string]any{
					"type":        "integer",
					"description": "Optional NPC disposition (-100 to 100).",
				},
				"location_id": map[string]any{
					"type":        "string",
					"description": "Optional destination location UUID.",
				},
				"alive": map[string]any{
					"type":        "boolean",
					"description": "Optional alive/dead state.",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Optional NPC description.",
				},
			},
			"required":             []string{"npc_id"},
			"additionalProperties": false,
		},
	}
}

// RegisterUpdateNPC registers the update_npc tool and handler.
func RegisterUpdateNPC(reg *Registry, store UpdateNPCStore) error {
	if store == nil {
		return errors.New("update_npc store is required")
	}
	return reg.Register(UpdateNPCTool(), NewUpdateNPCHandler(store).Handle)
}

// UpdateNPCHandler executes update_npc tool calls.
type UpdateNPCHandler struct {
	store UpdateNPCStore
}

// NewUpdateNPCHandler creates a new update_npc handler.
func NewUpdateNPCHandler(store UpdateNPCStore) *UpdateNPCHandler {
	return &UpdateNPCHandler{store: store}
}

// Handle executes the update_npc tool.
func (h *UpdateNPCHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("update_npc handler is required")
	}
	if h.store == nil {
		return nil, errors.New("update_npc store is required")
	}

	npcID, err := parseUUIDArg(args, "npc_id")
	if err != nil {
		return nil, err
	}

	npc, err := h.store.GetNPCByID(ctx, npcID)
	if err != nil {
		return nil, fmt.Errorf("get npc: %w", err)
	}
	if npc == nil {
		return nil, errors.New("npc_id does not reference an existing npc")
	}

	updated := *npc
	changed := false

	disposition, dispositionSet, err := parseOptionalIntArg(args, "disposition")
	if err != nil {
		return nil, err
	}
	if dispositionSet {
		updated.Disposition = clampDispositionValue(disposition)
		changed = true
	}

	locationID, locationSet, err := parseOptionalUUIDArg(args, "location_id")
	if err != nil {
		return nil, err
	}
	if locationSet {
		exists, err := h.store.LocationExistsInCampaign(ctx, locationID, npc.CampaignID)
		if err != nil {
			return nil, fmt.Errorf("check location exists in campaign: %w", err)
		}
		if !exists {
			return nil, errors.New("location_id does not reference an existing location in npc campaign")
		}
		updated.LocationID = &locationID
		changed = true
	}

	alive, aliveSet, err := parseOptionalBoolArg(args, "alive")
	if err != nil {
		return nil, err
	}
	if aliveSet {
		updated.Alive = alive
		changed = true
	}

	description, err := parseOptionalNonEmptyStringArg(args, "description")
	if err != nil {
		return nil, err
	}
	if description != nil {
		updated.Description = *description
		changed = true
	}
	if !changed {
		return nil, errors.New("at least one field must be provided to update_npc")
	}

	npc, err = h.store.UpdateNPC(ctx, updated)
	if err != nil {
		return nil, fmt.Errorf("update npc: %w", err)
	}

	data := map[string]any{
		"npc_id":      npc.ID.String(),
		"disposition": npc.Disposition,
		"alive":       npc.Alive,
		"description": npc.Description,
	}
	if npc.LocationID != nil {
		data["location_id"] = npc.LocationID.String()
	}

	return &ToolResult{
		Success:   true,
		Data:      data,
		Narrative: "NPC updated successfully.",
	}, nil
}

func parseOptionalIntArg(args map[string]any, key string) (int, bool, error) {
	if _, ok := args[key]; !ok {
		return 0, false, nil
	}
	value, err := parseIntArg(args, key)
	if err != nil {
		return 0, false, err
	}
	return value, true, nil
}

func parseOptionalUUIDArg(args map[string]any, key string) (uuid.UUID, bool, error) {
	if _, ok := args[key]; !ok {
		return uuid.Nil, false, nil
	}
	value, err := parseUUIDArg(args, key)
	if err != nil {
		return uuid.Nil, false, err
	}
	return value, true, nil
}

func parseOptionalBoolArg(args map[string]any, key string) (bool, bool, error) {
	raw, ok := args[key]
	if !ok {
		return false, false, nil
	}
	value, ok := raw.(bool)
	if !ok {
		return false, false, fmt.Errorf("%s must be a boolean", key)
	}
	return value, true, nil
}

func clampDispositionValue(disposition int) int {
	if disposition < -100 {
		return -100
	}
	if disposition > 100 {
		return 100
	}
	return disposition
}
