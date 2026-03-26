package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/llm"
)

const npcDialogueToolName = "npc_dialogue"

// NPCDialogueLogEntry captures the dialogue event to persist in the session log.
type NPCDialogueLogEntry struct {
	CampaignID        uuid.UUID
	LocationID        uuid.UUID
	NPCID             uuid.UUID
	Dialogue          string
	Emotion           *string
	FormattedDialogue string
}

// NPCDialogueStore loads NPCs and logs NPC dialogue events.
type NPCDialogueStore interface {
	GetNPCByID(ctx context.Context, npcID uuid.UUID) (*domain.NPC, error)
	LogNPCDialogue(ctx context.Context, entry NPCDialogueLogEntry) error
}

// NPCDialogueTool returns the npc_dialogue tool definition and JSON schema.
func NPCDialogueTool() llm.Tool {
	return llm.Tool{
		Name:        npcDialogueToolName,
		Description: "Record dialogue spoken by an NPC at the current location.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"npc_id": map[string]any{
					"type":        "string",
					"description": "NPC UUID that is speaking.",
				},
				"dialogue": map[string]any{
					"type":        "string",
					"description": "Dialogue line spoken by the NPC.",
				},
				"emotion": map[string]any{
					"type":        "string",
					"description": "Optional emotion to tag the dialogue line.",
				},
			},
			"required":             []string{"npc_id", "dialogue"},
			"additionalProperties": false,
		},
	}
}

// RegisterNPCDialogue registers the npc_dialogue tool and handler.
func RegisterNPCDialogue(reg *Registry, store NPCDialogueStore) error {
	if store == nil {
		return errors.New("npc_dialogue store is required")
	}
	return reg.Register(NPCDialogueTool(), NewNPCDialogueHandler(store).Handle)
}

// NPCDialogueHandler executes npc_dialogue tool calls.
type NPCDialogueHandler struct {
	store NPCDialogueStore
}

// NewNPCDialogueHandler creates a new npc_dialogue handler.
func NewNPCDialogueHandler(store NPCDialogueStore) *NPCDialogueHandler {
	return &NPCDialogueHandler{store: store}
}

// Handle executes the npc_dialogue tool.
func (h *NPCDialogueHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("npc_dialogue handler is nil")
	}
	if h.store == nil {
		return nil, errors.New("npc_dialogue store is required")
	}

	npcID, err := parseUUIDArg(args, "npc_id")
	if err != nil {
		return nil, err
	}
	dialogue, err := parseStringArg(args, "dialogue")
	if err != nil {
		return nil, err
	}
	emotion, err := parseOptionalNonEmptyStringArg(args, "emotion")
	if err != nil {
		return nil, err
	}

	locationID, ok := CurrentLocationIDFromContext(ctx)
	if !ok {
		return nil, errors.New("npc_dialogue requires current location id in context")
	}

	npc, err := h.store.GetNPCByID(ctx, npcID)
	if err != nil {
		return nil, fmt.Errorf("get npc: %w", err)
	}
	if npc == nil {
		return nil, errors.New("npc_id does not reference an existing npc")
	}
	if !npc.Alive {
		return nil, errors.New("npc must be alive")
	}
	if npc.LocationID == nil || *npc.LocationID != locationID {
		return nil, errors.New("npc is not at current location")
	}

	formatted := formatNPCDialogue(npc.Name, dialogue, emotion)
	if err := h.store.LogNPCDialogue(ctx, NPCDialogueLogEntry{
		CampaignID:        npc.CampaignID,
		LocationID:        locationID,
		NPCID:             npc.ID,
		Dialogue:          dialogue,
		Emotion:           emotion,
		FormattedDialogue: formatted,
	}); err != nil {
		return nil, fmt.Errorf("log npc dialogue: %w", err)
	}

	data := map[string]any{
		"npc_id":             npc.ID.String(),
		"npc_name":           npc.Name,
		"dialogue":           dialogue,
		"location_id":        locationID.String(),
		"formatted_dialogue": formatted,
	}
	if emotion != nil {
		data["emotion"] = *emotion
	}

	return &ToolResult{
		Success:   true,
		Data:      data,
		Narrative: formatted,
	}, nil
}

func formatNPCDialogue(npcName, dialogue string, emotion *string) string {
	if emotion == nil {
		return fmt.Sprintf("%s: %s", npcName, dialogue)
	}
	return fmt.Sprintf("%s (%s): %s", npcName, *emotion, dialogue)
}
