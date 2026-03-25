// Package assembly provides the LLM context assembler, which constructs the
// message array sent to an LLM provider for each player turn.
package assembly

import (
	"fmt"
	"strings"

	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/game"
	"github.com/PatrickFanella/game-master/internal/llm"
	"github.com/PatrickFanella/game-master/internal/prompt"
	"github.com/PatrickFanella/game-master/internal/tools"
)

// maxRecentTurns is the fixed sliding-window size for turn history included in
// the LLM context.
const maxRecentTurns = 10

// ContextAssembler builds the complete LLM message array for a player turn.
// It combines GM system instructions, serialized game state, recent turn
// history, and the current player input into an ordered []llm.Message slice
// ready for an llm.Provider call.
type ContextAssembler struct {
	registry *tools.Registry
}

// NewContextAssembler creates a ContextAssembler backed by the given tool
// registry. The registry may be nil if no tools are needed.
func NewContextAssembler(registry *tools.Registry) *ContextAssembler {
	return &ContextAssembler{registry: registry}
}

// AssembleContext constructs the ordered message array for an LLM call.
//
// The resulting slice contains:
//  1. A system message with GM behavioural guidelines and the current game
//     state serialized as structured text.
//  2. Up to [maxRecentTurns] prior turns from recentTurns (oldest first) as
//     alternating user / assistant messages.
//  3. The current playerInput as the final user message.
//
// recentTurns must be ordered oldest-first; only the last maxRecentTurns
// entries are included when the slice is longer.
func (a *ContextAssembler) AssembleContext(
	state *game.GameState,
	recentTurns []domain.SessionLog,
	playerInput string,
) []llm.Message {
	// Pre-allocate: 1 system + up to 2*maxRecentTurns history + 1 player.
	capacity := 1 + 2*maxRecentTurns + 1
	messages := make([]llm.Message, 0, capacity)

	// 1. System message: GM instructions + current state.
	messages = append(messages, llm.Message{
		Role:    llm.RoleSystem,
		Content: buildSystemContent(state),
	})

	// 2. Turn history – cap to the last maxRecentTurns entries.
	turns := recentTurns
	if len(turns) > maxRecentTurns {
		turns = turns[len(turns)-maxRecentTurns:]
	}
	for _, turn := range turns {
		messages = append(messages, llm.Message{
			Role:    llm.RoleUser,
			Content: turn.PlayerInput,
		})
		if turn.LLMResponse != "" {
			messages = append(messages, llm.Message{
				Role:    llm.RoleAssistant,
				Content: turn.LLMResponse,
			})
		}
	}

	// 3. Current player input as the final user message.
	messages = append(messages, llm.Message{
		Role:    llm.RoleUser,
		Content: playerInput,
	})

	return messages
}

// Tools returns the tool definitions registered in the assembler's registry.
// These should be passed alongside the messages when calling an llm.Provider.
// Returns nil if no registry was provided.
func (a *ContextAssembler) Tools() []llm.Tool {
	if a.registry == nil {
		return nil
	}
	return a.registry.List()
}

// ---------------------------------------------------------------------------
// State serialization helpers
// ---------------------------------------------------------------------------

// buildSystemContent produces the full text of the system message by
// concatenating the embedded GM prompt with a structured rendering of the
// current game state.
func buildSystemContent(state *game.GameState) string {
	var sb strings.Builder
	sb.WriteString(prompt.GameMaster)
	sb.WriteString("\n\n## Current Game State\n\n")
	sb.WriteString(serializeState(state))
	return sb.String()
}

// serializeState renders state as structured plain text. Core sections
// (Campaign, Player Character, and Current Location) are always written when
// a game state is available; optional sections (NPCs, Quests, Inventory,
// World Facts) and optional fields are only included when there is data to show.
func serializeState(state *game.GameState) string {
	if state == nil {
		return "(no game state available)\n"
	}

	var sb strings.Builder

	// Campaign
	sb.WriteString("### Campaign\n")
	sb.WriteString(fmt.Sprintf("- Name: %s\n", state.Campaign.Name))
	if state.Campaign.Genre != "" {
		sb.WriteString(fmt.Sprintf("- Genre: %s\n", state.Campaign.Genre))
	}
	if state.Campaign.Tone != "" {
		sb.WriteString(fmt.Sprintf("- Tone: %s\n", state.Campaign.Tone))
	}
	if len(state.Campaign.Themes) > 0 {
		sb.WriteString(fmt.Sprintf("- Themes: %s\n", strings.Join(state.Campaign.Themes, ", ")))
	}
	if state.Campaign.Description != "" {
		sb.WriteString(fmt.Sprintf("- Description: %s\n", state.Campaign.Description))
	}
	sb.WriteString("\n")

	// Player character
	sb.WriteString("### Player Character\n")
	sb.WriteString(fmt.Sprintf("- Name: %s\n", state.Player.Name))
	sb.WriteString(fmt.Sprintf("- Level: %d\n", state.Player.Level))
	sb.WriteString(fmt.Sprintf("- HP: %d/%d\n", state.Player.HP, state.Player.MaxHP))
	if state.Player.Description != "" {
		sb.WriteString(fmt.Sprintf("- Description: %s\n", state.Player.Description))
	}
	if state.Player.Status != "" {
		sb.WriteString(fmt.Sprintf("- Status: %s\n", state.Player.Status))
	}
	sb.WriteString("\n")

	// Current location
	sb.WriteString("### Current Location\n")
	sb.WriteString(fmt.Sprintf("- Name: %s\n", state.CurrentLocation.Name))
	if state.CurrentLocation.Region != "" {
		sb.WriteString(fmt.Sprintf("- Region: %s\n", state.CurrentLocation.Region))
	}
	if state.CurrentLocation.LocationType != "" {
		sb.WriteString(fmt.Sprintf("- Type: %s\n", state.CurrentLocation.LocationType))
	}
	if state.CurrentLocation.Description != "" {
		sb.WriteString(fmt.Sprintf("- Description: %s\n", state.CurrentLocation.Description))
	}
	wroteExitsHeader := false
	for _, conn := range state.CurrentLocationConnections {
		if conn.Description == "" {
			continue
		}
		if !wroteExitsHeader {
			sb.WriteString("- Exits:\n")
			wroteExitsHeader = true
		}
		if conn.TravelTime != "" {
			sb.WriteString(fmt.Sprintf("  - %s (travel time: %s)\n", conn.Description, conn.TravelTime))
		} else {
			sb.WriteString(fmt.Sprintf("  - %s\n", conn.Description))
		}
	}
	sb.WriteString("\n")

	// NPCs present
	if len(state.NearbyNPCs) > 0 {
		sb.WriteString("### NPCs Present\n")
		for _, npc := range state.NearbyNPCs {
			line := fmt.Sprintf("- %s", npc.Name)
			if !npc.Alive {
				line += " (dead)"
			}
			if npc.Description != "" {
				line += fmt.Sprintf(": %s", npc.Description)
			}
			sb.WriteString(line + "\n")
		}
		sb.WriteString("\n")
	}

	// Active quests
	if len(state.ActiveQuests) > 0 {
		sb.WriteString("### Active Quests\n")
		for _, quest := range state.ActiveQuests {
			sb.WriteString(fmt.Sprintf("- %s", quest.Title))
			if quest.QuestType != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", quest.QuestType))
			}
			sb.WriteString("\n")
			if quest.Description != "" {
				sb.WriteString(fmt.Sprintf("  %s\n", quest.Description))
			}
			if objectives, ok := state.ActiveQuestObjectives[quest.ID]; ok {
				for _, obj := range objectives {
					check := "[ ]"
					if obj.Completed {
						check = "[x]"
					}
					sb.WriteString(fmt.Sprintf("  %s %s\n", check, obj.Description))
				}
			}
		}
		sb.WriteString("\n")
	}

	// Player inventory
	if len(state.PlayerInventory) > 0 {
		sb.WriteString("### Player Inventory\n")
		for _, item := range state.PlayerInventory {
			line := fmt.Sprintf("- %s", item.Name)
			if item.Quantity > 1 {
				line += fmt.Sprintf(" (x%d)", item.Quantity)
			}
			if item.Description != "" {
				line += fmt.Sprintf(": %s", item.Description)
			}
			sb.WriteString(line + "\n")
		}
		sb.WriteString("\n")
	}

	// World facts
	if len(state.WorldFacts) > 0 {
		sb.WriteString("### World Facts\n")
		for _, fact := range state.WorldFacts {
			sb.WriteString(fmt.Sprintf("- %s\n", fact.Fact))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
