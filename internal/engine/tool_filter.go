package engine

import (
	"github.com/PatrickFanella/game-master/internal/game"
	"github.com/PatrickFanella/game-master/internal/llm"
)

// GamePhase represents the current phase of gameplay, used to select which
// tools the LLM should see on a given turn.
type GamePhase int

const (
	// PhaseExploration is the default: exploring, investigating, interacting.
	PhaseExploration GamePhase = iota
	// PhaseCombat is active when a combat encounter is in progress.
	PhaseCombat
)

// String returns the human-readable name for a GamePhase.
func (p GamePhase) String() string {
	switch p {
	case PhaseExploration:
		return "exploration"
	case PhaseCombat:
		return "combat"
	default:
		return "unknown"
	}
}

// ToolFilter selects which tools to expose to the LLM on a given turn.
type ToolFilter interface {
	Filter(state *game.GameState, allTools []llm.Tool) []llm.Tool
}

// PhaseToolFilter implements ToolFilter by selecting tools based on game phase,
// quest state, and progression proximity.
type PhaseToolFilter struct{}

// baseTools are always available regardless of game phase.
var baseTools = map[string]struct{}{
	"skill_check":     {},
	"roll_dice":       {},
	"move_player":     {},
	"describe_scene":  {},
	"present_choices": {},
	"npc_dialogue":    {},
	"establish_fact":  {},
	"revise_fact":     {},
	"update_npc":      {},
	"add_item":        {},
	"remove_item":     {},
	"modify_item":     {},
	"create_item":     {},
	"generate_name":   {},
	"search_memory":   {},
}

// combatTools are added when combat is active.
var combatTools = map[string]struct{}{
	"initiate_combat":      {},
	"combat_round":         {},
	"apply_damage":         {},
	"apply_condition":      {},
	"resolve_combat":       {},
	"add_ability":          {},
	"remove_ability":       {},
	"update_player_status": {},
}

// explorationTools are added during non-combat phases.
var explorationTools = map[string]struct{}{
	"create_npc":             {},
	"create_location":        {},
	"create_city":            {},
	"create_faction":         {},
	"create_language":        {},
	"create_culture":         {},
	"create_belief_system":   {},
	"create_economic_system": {},
	"create_lore":            {},
	"establish_relationship": {},
	"reveal_location":        {},
	"initiate_combat":        {},
}

// questTools are added when quests are relevant.
var questTools = map[string]struct{}{
	"create_quest":       {},
	"create_subquest":    {},
	"update_quest":       {},
	"complete_objective": {},
	"branch_quest":       {},
	"link_quest_entity":  {},
}

// progressionTools are added near level thresholds.
var progressionTools = map[string]struct{}{
	"add_experience":       {},
	"level_up":             {},
	"update_player_stats":  {},
	"update_player_status": {},
}

// DetectPhase examines game state and returns the current game phase.
func DetectPhase(state *game.GameState) GamePhase {
	if state == nil {
		return PhaseExploration
	}
	if state.Player.Status == "in_combat" {
		return PhaseCombat
	}
	return PhaseExploration
}

// xpThresholds maps level → total XP required.
var xpThresholds = []int{0, 100, 300, 600, 1000, 1500, 2100, 2800, 3600, 4500, 5500}

// nearLevelThreshold returns true if the player has earned at least 50% of
// the XP needed for their next level. xpThresholds[i] is the cumulative XP
// to reach level i+1. A level-1 player needs xpThresholds[1]=100 XP total
// to reach level 2.
func nearLevelThreshold(state *game.GameState) bool {
	level := state.Player.Level
	xp := state.Player.Experience
	if level <= 0 || level >= len(xpThresholds) {
		return true
	}
	nextThreshold := xpThresholds[level] // XP needed for next level
	return xp >= nextThreshold/2
}

// Filter returns tools appropriate for the current game state.
func (f *PhaseToolFilter) Filter(state *game.GameState, allTools []llm.Tool) []llm.Tool {
	if state == nil {
		return allTools
	}

	phase := DetectPhase(state)
	allowed := make(map[string]struct{}, 30)

	// Always include base tools.
	for name := range baseTools {
		allowed[name] = struct{}{}
	}

	switch phase {
	case PhaseCombat:
		for name := range combatTools {
			allowed[name] = struct{}{}
		}
	default:
		for name := range explorationTools {
			allowed[name] = struct{}{}
		}
	}

	// Quest tools when quests exist or NPCs are nearby (quest triggers).
	if len(state.ActiveQuests) > 0 || len(state.NearbyNPCs) > 0 {
		for name := range questTools {
			allowed[name] = struct{}{}
		}
	}

	// Progression: always allow add_experience; full set near level-up.
	allowed["add_experience"] = struct{}{}
	if nearLevelThreshold(state) {
		for name := range progressionTools {
			allowed[name] = struct{}{}
		}
	}

	filtered := make([]llm.Tool, 0, len(allowed))
	for _, tool := range allTools {
		if _, ok := allowed[tool.Name]; ok {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}
