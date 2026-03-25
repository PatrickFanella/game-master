// Package combat defines the interface and types for abstracting combat
// resolution. Implementations may provide purely narrative (LLM-driven),
// mechanical (stat-based), or hybrid resolution strategies.
package combat

import "context"

// CombatResolver abstracts combat resolution so that different strategies
// (narrative, mechanical, or hybrid) can be plugged in without changing
// the rest of the game engine.
type CombatResolver interface {
	// InitiateCombat sets up a new combat encounter with the given
	// combatants and environment, returning the initial combat state.
	InitiateCombat(ctx context.Context, combatants []Combatant, environment Environment) (*CombatState, error)

	// ProcessRound advances combat by one round, applying the player's
	// chosen action and resolving NPC actions against the current state.
	ProcessRound(ctx context.Context, playerAction PlayerAction, combatState *CombatState) (*RoundResult, error)

	// ResolveCombat determines the final outcome of the combat encounter
	// based on the current combat state.
	ResolveCombat(ctx context.Context, combatState *CombatState) (*CombatOutcome, error)
}
