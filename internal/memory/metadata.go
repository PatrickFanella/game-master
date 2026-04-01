package memory

import (
	"github.com/google/uuid"
)

// TurnMetadata holds structured metadata extracted from a turn's tool calls
// and game state for enriching memory entries.
type TurnMetadata struct {
	LocationID   *uuid.UUID
	NPCsInvolved []uuid.UUID
	InGameTime   string
	MemoryType   string
}

// ToolCallInfo is a simplified representation of a tool call for metadata extraction.
type ToolCallInfo struct {
	ToolName  string
	Arguments map[string]any
}

// GameStateSnapshot provides the minimal state needed for metadata extraction.
type GameStateSnapshot struct {
	CurrentLocationID *uuid.UUID
}

// toolCategory maps tool names to their memory-type category.
var toolCategory = map[string]string{
	// Combat
	"initiate_combat":  "combat",
	"combat_round":     "combat",
	"resolve_combat":   "combat",
	"apply_damage":     "combat",
	"apply_condition":  "combat",
	// Quest
	"create_quest":       "quest_update",
	"update_quest":       "quest_update",
	"complete_objective": "quest_update",
	"create_subquest":    "quest_update",
	// NPC / dialogue
	"create_npc":    "dialogue",
	"update_npc":    "dialogue",
	"npc_dialogue":  "dialogue",
	// Location / exploration
	"create_location": "exploration",
	"move_player":     "exploration",
	// Lore / discovery
	"establish_fact": "discovery",
	"create_lore":    "discovery",
	"revise_fact":    "discovery",
	// Item / trade
	"add_item":    "trade",
	"remove_item": "trade",
	"modify_item": "trade",
	"create_item": "trade",
}

// categoryPriority defines tie-break order (lower index wins).
var categoryPriority = []string{
	"combat",
	"quest_update",
	"dialogue",
	"exploration",
	"discovery",
	"trade",
}

// npcTools lists tool names whose arguments may reference an NPC UUID.
var npcTools = map[string]bool{
	"create_npc":   true,
	"update_npc":   true,
	"npc_dialogue": true,
}

// ExtractMetadata derives structured metadata from tool calls and current game state.
// This is a pure function with no side effects.
func ExtractMetadata(toolCalls []ToolCallInfo, state *GameStateSnapshot) TurnMetadata {
	md := TurnMetadata{
		InGameTime: "",
	}

	// Location from state.
	if state != nil && state.CurrentLocationID != nil {
		loc := *state.CurrentLocationID
		md.LocationID = &loc
	}

	// Count categories and collect NPC IDs.
	counts := make(map[string]int, len(categoryPriority))
	seen := make(map[uuid.UUID]bool)

	for _, tc := range toolCalls {
		if cat, ok := toolCategory[tc.ToolName]; ok {
			counts[cat]++
		}

		if npcTools[tc.ToolName] {
			extractNPCIDs(tc.Arguments, seen)
		}
	}

	for id := range seen {
		md.NPCsInvolved = append(md.NPCsInvolved, id)
	}

	// Determine dominant category by count, then priority order for ties.
	md.MemoryType = "turn_summary"
	best := 0
	for _, cat := range categoryPriority {
		c := counts[cat]
		if c > best {
			best = c
			md.MemoryType = cat
		}
	}

	return md
}

// extractNPCIDs looks for "id" and "npc_id" keys in args, parses as UUID,
// and adds to the seen set. Invalid strings are silently skipped.
func extractNPCIDs(args map[string]any, seen map[uuid.UUID]bool) {
	for _, key := range []string{"id", "npc_id"} {
		v, ok := args[key]
		if !ok {
			continue
		}
		s, ok := v.(string)
		if !ok {
			continue
		}
		id, err := uuid.Parse(s)
		if err != nil {
			continue
		}
		seen[id] = true
	}
}
