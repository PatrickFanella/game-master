package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/combat"
	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/llm"
)

const (
	resolveCombatToolName = "resolve_combat"

	outcomeVictory   = "victory"
	outcomeDefeat    = "defeat"
	outcomeFlee      = "flee"
	outcomeSurrender = "surrender"

	playerStatusActive   = "active"
	playerStatusDefeated = "defeated"
)

// lootInput represents a single item to create as combat loot.
type lootInput struct {
	Name        string
	Description string
	ItemType    string
	Quantity    int
}

// ResolveCombatStore provides persistence needed for resolve_combat.
type ResolveCombatStore interface {
	GetPlayerCharacterByID(ctx context.Context, playerCharacterID uuid.UUID) (*domain.PlayerCharacter, error)
	UpdatePlayerHP(ctx context.Context, playerCharacterID uuid.UUID, hp, maxHP int) error
	UpdatePlayerStatus(ctx context.Context, playerCharacterID uuid.UUID, status string) error
	UpdatePlayerLocation(ctx context.Context, playerCharacterID uuid.UUID, locationID uuid.UUID) error
	AddPlayerExperience(ctx context.Context, playerCharacterID uuid.UUID, xpAmount int) error
	CreatePlayerItem(ctx context.Context, playerCharacterID uuid.UUID, name, description, itemType, rarity string, quantity int) (uuid.UUID, error)
	MarkNPCDead(ctx context.Context, npcID uuid.UUID) error
	GetNPCByID(ctx context.Context, npcID uuid.UUID) (*domain.NPC, error)
	UpdateNPCDisposition(ctx context.Context, npcID uuid.UUID, newDisposition int) error
}

// ResolveCombatTool returns the resolve_combat tool definition and JSON schema.
func ResolveCombatTool() llm.Tool {
	return llm.Tool{
		Name: resolveCombatToolName,
		Description: "End combat and resolve the outcome: distribute XP and loot on victory, " +
			"handle player defeat or capture, move the player on flee, or apply NPC disposition changes " +
			"on surrender. Persists updated player HP back to the character record and clears combat mode.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"combat_state": map[string]any{
					"type":        "object",
					"description": "Current combat state from the last combat round.",
				},
				"outcome_type": map[string]any{
					"type":        "string",
					"description": "How combat ended. One of: victory, defeat, flee, surrender.",
				},
				"xp_earned": map[string]any{
					"type":        "integer",
					"description": "Experience points to award the player. Used on victory.",
				},
				"loot": map[string]any{
					"type":        "array",
					"description": "Items to add to the player's inventory. Used on victory.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name":        map[string]any{"type": "string", "description": "Item name."},
							"description": map[string]any{"type": "string", "description": "Item description."},
							"item_type":   map[string]any{"type": "string", "description": "Item type: weapon, armor, consumable, quest, or misc."},
							"quantity":    map[string]any{"type": "integer", "description": "Quantity. Defaults to 1."},
						},
						"required":             []string{"name", "description", "item_type"},
						"additionalProperties": false,
					},
				},
				"dead_npc_ids": map[string]any{
					"type":        "array",
					"description": "UUIDs of NPCs killed in this combat. Used on victory.",
					"items":       map[string]any{"type": "string"},
				},
				"flee_location_id": map[string]any{
					"type":        "string",
					"description": "Destination location UUID the player flees to. Used on flee.",
				},
				"surrender_npc_ids": map[string]any{
					"type":        "array",
					"description": "UUIDs of NPCs whose disposition changes on surrender.",
					"items":       map[string]any{"type": "string"},
				},
				"disposition_change": map[string]any{
					"type":        "integer",
					"description": "Disposition delta applied to each surrendering NPC (-100 to 100). Used on surrender.",
				},
				"consequences": map[string]any{
					"type":        "string",
					"description": "Narrative description of the lasting combat consequences.",
				},
			},
			"required":             []string{"combat_state", "outcome_type"},
			"additionalProperties": false,
		},
	}
}

// RegisterResolveCombat registers the resolve_combat tool and handler.
func RegisterResolveCombat(reg *Registry, store ResolveCombatStore) error {
	if store == nil {
		return errors.New("resolve_combat store is required")
	}
	return reg.Register(ResolveCombatTool(), NewResolveCombatHandler(store).Handle)
}

// ResolveCombatHandler executes resolve_combat tool calls.
type ResolveCombatHandler struct {
	store ResolveCombatStore
}

// NewResolveCombatHandler creates a new resolve_combat handler.
func NewResolveCombatHandler(store ResolveCombatStore) *ResolveCombatHandler {
	return &ResolveCombatHandler{store: store}
}

// Handle executes the resolve_combat tool.
func (h *ResolveCombatHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("resolve_combat handler is nil")
	}
	if h.store == nil {
		return nil, errors.New("resolve_combat store is required")
	}

	playerCharacterID, ok := CurrentPlayerCharacterIDFromContext(ctx)
	if !ok {
		return nil, errors.New("resolve_combat requires current player character id in context")
	}

	state, err := parseCombatStateArg(args, "combat_state")
	if err != nil {
		return nil, err
	}

	outcomeType, err := parseStringArg(args, "outcome_type")
	if err != nil {
		return nil, err
	}
	outcomeType = strings.ToLower(strings.TrimSpace(outcomeType))
	switch outcomeType {
	case outcomeVictory, outcomeDefeat, outcomeFlee, outcomeSurrender:
	default:
		return nil, fmt.Errorf("outcome_type must be one of: victory, defeat, flee, surrender")
	}

	player, err := h.store.GetPlayerCharacterByID(ctx, playerCharacterID)
	if err != nil {
		return nil, fmt.Errorf("get player character: %w", err)
	}
	if player == nil {
		return nil, errors.New("player character not found")
	}

	// Persist the player's current HP and max HP from the combat state.
	playerCombatant := firstCombatantByType(state, combat.CombatantTypePlayer)
	if playerCombatant != nil {
		if err := h.store.UpdatePlayerHP(ctx, playerCharacterID, playerCombatant.HP, playerCombatant.MaxHP); err != nil {
			return nil, fmt.Errorf("persist player hp: %w", err)
		}
	}

	summary, err := h.resolveOutcome(ctx, outcomeType, args, state, playerCharacterID)
	if err != nil {
		return nil, err
	}

	// Update combat state status to reflect the resolved outcome.
	state.Status = combatStatusFromOutcome(outcomeType)
	summary["combat_state"] = combatStateToMap(state)
	summary["outcome_type"] = outcomeType

	consequences, _ := parseOptionalNonEmptyStringArg(args, "consequences")
	if consequences != nil {
		summary["consequences"] = *consequences
	}

	return &ToolResult{
		Success:   true,
		Data:      summary,
		Narrative: buildResolveCombatNarrative(outcomeType, player.Name, summary),
	}, nil
}

// resolveOutcome handles the outcome-specific logic and returns a summary map.
func (h *ResolveCombatHandler) resolveOutcome(
	ctx context.Context,
	outcomeType string,
	args map[string]any,
	state *combat.CombatState,
	playerCharacterID uuid.UUID,
) (map[string]any, error) {
	switch outcomeType {
	case outcomeVictory:
		return h.handleVictory(ctx, args, state, playerCharacterID)
	case outcomeDefeat:
		return h.handleDefeat(ctx, playerCharacterID)
	case outcomeFlee:
		return h.handleFlee(ctx, args, playerCharacterID)
	case outcomeSurrender:
		return h.handleSurrender(ctx, args, state, playerCharacterID)
	}
	return nil, fmt.Errorf("unknown outcome_type: %s", outcomeType)
}

func (h *ResolveCombatHandler) handleVictory(
	ctx context.Context,
	args map[string]any,
	state *combat.CombatState,
	playerCharacterID uuid.UUID,
) (map[string]any, error) {
	summary := map[string]any{}

	// Award XP.
	xpEarned, xpSet, err := parseOptionalIntArg(args, "xp_earned")
	if err != nil {
		return nil, err
	}
	if xpSet {
		if xpEarned < 0 {
			return nil, errors.New("xp_earned must be greater than or equal to 0")
		}
		if xpEarned > 0 {
			if err := h.store.AddPlayerExperience(ctx, playerCharacterID, xpEarned); err != nil {
				return nil, fmt.Errorf("add player experience: %w", err)
			}
		}
	}
	summary["xp_earned"] = xpEarned

	// Mark dead NPCs — validate each ID belongs to the combat state.
	deadNPCIDs, err := parseUUIDArrayArg(args, "dead_npc_ids")
	if err != nil {
		return nil, err
	}
	npcIDs := npcCombatantIDSet(state)
	killedNPCIDs := make([]string, 0, len(deadNPCIDs))
	for _, npcID := range deadNPCIDs {
		if !npcIDs[npcID] {
			return nil, fmt.Errorf("dead_npc_ids: %s is not an NPC combatant in this combat", npcID)
		}
		if err := h.store.MarkNPCDead(ctx, npcID); err != nil {
			return nil, fmt.Errorf("mark npc %s dead: %w", npcID, err)
		}
		killedNPCIDs = append(killedNPCIDs, npcID.String())
	}
	summary["dead_npc_ids"] = killedNPCIDs

	// Create loot items.
	lootItems, err := parseLootArg(args, "loot")
	if err != nil {
		return nil, err
	}
	createdLoot := make([]map[string]any, 0, len(lootItems))
	for _, item := range lootItems {
		itemID, createErr := h.store.CreatePlayerItem(ctx, playerCharacterID, item.Name, item.Description, item.ItemType, defaultItemRarity, item.Quantity)
		if createErr != nil {
			return nil, fmt.Errorf("create loot item %q: %w", item.Name, createErr)
		}
		createdLoot = append(createdLoot, map[string]any{
			"item_id":     itemID.String(),
			"name":        item.Name,
			"description": item.Description,
			"item_type":   item.ItemType,
			"quantity":    item.Quantity,
		})
	}
	summary["loot"] = createdLoot

	// Exit combat mode.
	if err := h.store.UpdatePlayerStatus(ctx, playerCharacterID, playerStatusActive); err != nil {
		return nil, fmt.Errorf("exit combat mode: %w", err)
	}
	summary["player_status"] = playerStatusActive
	return summary, nil
}

func (h *ResolveCombatHandler) handleDefeat(ctx context.Context, playerCharacterID uuid.UUID) (map[string]any, error) {
	if err := h.store.UpdatePlayerStatus(ctx, playerCharacterID, playerStatusDefeated); err != nil {
		return nil, fmt.Errorf("set defeated status: %w", err)
	}
	return map[string]any{
		"player_status": playerStatusDefeated,
	}, nil
}

func (h *ResolveCombatHandler) handleFlee(
	ctx context.Context,
	args map[string]any,
	playerCharacterID uuid.UUID,
) (map[string]any, error) {
	summary := map[string]any{}

	fleeLocationID, fleeLocationSet, err := parseOptionalUUIDArg(args, "flee_location_id")
	if err != nil {
		return nil, err
	}
	if fleeLocationSet {
		if err := h.store.UpdatePlayerLocation(ctx, playerCharacterID, fleeLocationID); err != nil {
			return nil, fmt.Errorf("move player on flee: %w", err)
		}
		summary["flee_location_id"] = fleeLocationID.String()
	}

	// Exit combat mode.
	if err := h.store.UpdatePlayerStatus(ctx, playerCharacterID, playerStatusActive); err != nil {
		return nil, fmt.Errorf("exit combat mode: %w", err)
	}
	summary["player_status"] = playerStatusActive
	return summary, nil
}

func (h *ResolveCombatHandler) handleSurrender(
	ctx context.Context,
	args map[string]any,
	state *combat.CombatState,
	playerCharacterID uuid.UUID,
) (map[string]any, error) {
	surrenderNPCIDs, err := parseUUIDArrayArg(args, "surrender_npc_ids")
	if err != nil {
		return nil, err
	}

	dispositionChange, _, err := parseOptionalIntArg(args, "disposition_change")
	if err != nil {
		return nil, err
	}

	// Validate all surrender_npc_ids are NPC combatants in this combat state.
	npcIDs := npcCombatantIDSet(state)
	for _, npcID := range surrenderNPCIDs {
		if !npcIDs[npcID] {
			return nil, fmt.Errorf("surrender_npc_ids: %s is not an NPC combatant in this combat", npcID)
		}
	}

	updatedNPCs := make([]map[string]any, 0, len(surrenderNPCIDs))
	for _, npcID := range surrenderNPCIDs {
		npc, err := h.store.GetNPCByID(ctx, npcID)
		if err != nil {
			return nil, fmt.Errorf("get npc %s: %w", npcID, err)
		}
		if npc == nil {
			continue
		}
		newDisposition := clampDispositionValue(npc.Disposition + dispositionChange)
		if err := h.store.UpdateNPCDisposition(ctx, npcID, newDisposition); err != nil {
			return nil, fmt.Errorf("update npc %s disposition: %w", npcID, err)
		}
		updatedNPCs = append(updatedNPCs, map[string]any{
			"npc_id":          npcID.String(),
			"old_disposition": npc.Disposition,
			"new_disposition": newDisposition,
		})
	}

	// Exit combat mode.
	if err := h.store.UpdatePlayerStatus(ctx, playerCharacterID, playerStatusActive); err != nil {
		return nil, fmt.Errorf("exit combat mode: %w", err)
	}
	return map[string]any{
		"updated_npcs":  updatedNPCs,
		"player_status": playerStatusActive,
	}, nil
}

// parseLootArg parses the "loot" argument into a slice of lootInput.
func parseLootArg(args map[string]any, key string) ([]lootInput, error) {
	raw, ok := args[key]
	if !ok {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}

	out := make([]lootInput, 0, len(items))
	for i, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an object", key, i)
		}
		name, err := parseStringArg(obj, "name")
		if err != nil {
			return nil, fmt.Errorf("%s[%d].%w", key, i, err)
		}
		description, err := parseStringArg(obj, "description")
		if err != nil {
			return nil, fmt.Errorf("%s[%d].%w", key, i, err)
		}
		itemType, err := parseStringArg(obj, "item_type")
		if err != nil {
			return nil, fmt.Errorf("%s[%d].%w", key, i, err)
		}
		if _, allowed := allowedItemTypes[itemType]; !allowed {
			return nil, fmt.Errorf("%s[%d].item_type must be one of: weapon, armor, consumable, quest, misc", key, i)
		}
		quantity, err := parsePositiveIntArgWithDefault(obj, "quantity", defaultAddQuantity)
		if err != nil {
			return nil, fmt.Errorf("%s[%d].%w", key, i, err)
		}
		out = append(out, lootInput{
			Name:        name,
			Description: description,
			ItemType:    itemType,
			Quantity:    quantity,
		})
	}
	return out, nil
}

// combatStatusFromOutcome maps an outcome type to the final CombatStatus.
func combatStatusFromOutcome(outcomeType string) combat.CombatStatus {
	if outcomeType == outcomeFlee {
		return combat.CombatStatusFled
	}
	return combat.CombatStatusCompleted
}

// npcCombatantIDSet returns the set of entity IDs for all NPC combatants in state.
func npcCombatantIDSet(state *combat.CombatState) map[uuid.UUID]bool {
	ids := make(map[uuid.UUID]bool, len(state.Combatants))
	for _, c := range state.Combatants {
		if c.EntityType == combat.CombatantTypeNPC {
			ids[c.EntityID] = true
		}
	}
	return ids
}

// buildResolveCombatNarrative constructs a narrative string for the combat outcome.
func buildResolveCombatNarrative(outcomeType, playerName string, summary map[string]any) string {
	switch outcomeType {
	case outcomeVictory:
		xpEarned, _ := summary["xp_earned"].(int)
		loot, _ := summary["loot"].([]map[string]any)
		if xpEarned > 0 && len(loot) > 0 {
			return fmt.Sprintf("Victory! %s has defeated all enemies, earning %d XP and %d item(s) of loot.", playerName, xpEarned, len(loot))
		}
		if xpEarned > 0 {
			return fmt.Sprintf("Victory! %s has defeated all enemies, earning %d XP.", playerName, xpEarned)
		}
		if len(loot) > 0 {
			return fmt.Sprintf("Victory! %s has defeated all enemies and claimed %d item(s) of loot.", playerName, len(loot))
		}
		return fmt.Sprintf("Victory! %s has defeated all enemies.", playerName)
	case outcomeDefeat:
		return fmt.Sprintf("%s has been defeated in combat.", playerName)
	case outcomeFlee:
		if loc, ok := summary["flee_location_id"].(string); ok && loc != "" {
			return fmt.Sprintf("%s fled from combat and moved to a new location.", playerName)
		}
		return fmt.Sprintf("%s fled from combat.", playerName)
	case outcomeSurrender:
		return "The combat has ended in surrender. Terms have been negotiated."
	}
	return fmt.Sprintf("Combat resolved with outcome: %s.", outcomeType)
}
