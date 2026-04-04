package engine

import (
	"github.com/PatrickFanella/game-master/internal/game"
	"github.com/PatrickFanella/game-master/internal/llm"
)

// GamePhase represents the current phase of gameplay.
type GamePhase int

const (
	// PhaseExploration indicates the player is exploring the world.
	PhaseExploration GamePhase = iota
	// PhaseCombat indicates an active combat encounter.
	PhaseCombat
	// PhaseSocial indicates a social interaction or dialogue.
	PhaseSocial
	// PhaseRest indicates the player is resting.
	PhaseRest
)

// String returns the human-readable name for a GamePhase.
func (p GamePhase) String() string {
	switch p {
	case PhaseExploration:
		return "Exploration"
	case PhaseCombat:
		return "Combat"
	case PhaseSocial:
		return "Social"
	case PhaseRest:
		return "Rest"
	default:
		return "Unknown"
	}
}

// ToolFilter selects which tools to expose to the LLM on a given turn.
type ToolFilter interface {
	// Filter returns the subset of allTools that should be available to the
	// LLM for the current turn given the provided internal/game game state
	// snapshot used for LLM context and tool selection.
	Filter(gameState *game.GameState, allTools []llm.Tool) []llm.Tool
}
