package engine

import (
	"github.com/PatrickFanella/game-master/internal/game"
	"github.com/PatrickFanella/game-master/internal/llm"
	"github.com/PatrickFanella/game-master/internal/tools"
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
// quest state, and progression proximity. It reads tool categories from the
// Registry's metadata rather than hardcoded maps.
type PhaseToolFilter struct {
	registry *tools.Registry
}

// NewPhaseToolFilter creates a PhaseToolFilter backed by the given registry.
func NewPhaseToolFilter(registry *tools.Registry) *PhaseToolFilter {
	return &PhaseToolFilter{registry: registry}
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

// xpThresholds maps level -> total XP required.
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

// narrativeExcludeTools lists tool names removed in narrative mode.
var narrativeExcludeTools = map[string]struct{}{
	"initiate_combat": {},
	"combat_round":    {},
	"apply_damage":    {},
	"apply_condition": {},
	"resolve_combat":  {},
	"add_experience":  {},
	"level_up":        {},
}

// crunchOnlyTools lists tool names only available in crunch mode.
var crunchOnlyTools = map[string]struct{}{
	"grant_feat":     {},
	"allocate_skill": {},
}

// Filter returns tools appropriate for the current game state.
func (f *PhaseToolFilter) Filter(state *game.GameState, allTools []llm.Tool) []llm.Tool {
	if state == nil {
		return allTools
	}

	phase := DetectPhase(state)
	allowed := make(map[string]struct{}, 30)

	// Build allowed set from registry metadata.
	for _, tool := range allTools {
		meta, hasMeta := f.registry.GetMeta(tool.Name)
		if !hasMeta {
			// Tools without metadata are always allowed (backward compat).
			allowed[tool.Name] = struct{}{}
			continue
		}
		switch meta.Category {
		case tools.CategoryBase:
			// Always include base tools.
			allowed[tool.Name] = struct{}{}
		case tools.CategoryCombat:
			if phase == PhaseCombat {
				allowed[tool.Name] = struct{}{}
			}
		case tools.CategoryExploration:
			if phase == PhaseExploration {
				allowed[tool.Name] = struct{}{}
			}
		case tools.CategoryQuest:
			if len(state.ActiveQuests) > 0 || len(state.NearbyNPCs) > 0 {
				allowed[tool.Name] = struct{}{}
			}
		case tools.CategoryProgression:
			// add_experience is always available; full set near level-up.
			if tool.Name == "add_experience" || nearLevelThreshold(state) {
				allowed[tool.Name] = struct{}{}
			}
		}
	}

	// Handle tools with dual category membership: initiate_combat is in both
	// combat and exploration categories, and update_player_status is in both
	// combat and progression categories in the original design.
	if phase == PhaseCombat {
		allowed["initiate_combat"] = struct{}{}
	}
	if nearLevelThreshold(state) {
		allowed["update_player_status"] = struct{}{}
	}

	// Apply rules_mode filtering.
	rulesMode := state.RulesMode
	if rulesMode == "" {
		rulesMode = "narrative"
	}

	switch rulesMode {
	case "narrative":
		// Remove narrative-excluded tools.
		for name := range narrativeExcludeTools {
			delete(allowed, name)
		}
	case "crunch":
		// Add crunch-only tools.
		for name := range crunchOnlyTools {
			allowed[name] = struct{}{}
		}
	}
	// "light" mode keeps the default phase-based behavior.

	filtered := make([]llm.Tool, 0, len(allowed))
	for _, tool := range allTools {
		if _, ok := allowed[tool.Name]; ok {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}
