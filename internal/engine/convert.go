package engine

import "github.com/PatrickFanella/game-master/internal/game"

// GameStateFromFull projects a full game.GameState (used for LLM context)
// into the slimmer engine.GameState (used by the TUI and API).
func GameStateFromFull(gs *game.GameState) *GameState {
	if gs == nil {
		return nil
	}
	return &GameState{
		CurrentLocation:       gs.CurrentLocation,
		PlayerCharacter:       gs.Player,
		NPCsPresent:           gs.NearbyNPCs,
		ActiveQuests:          gs.ActiveQuests,
		ActiveQuestObjectives: gs.ActiveQuestObjectives,
		PlayerInventory:       gs.PlayerInventory,
	}
}
