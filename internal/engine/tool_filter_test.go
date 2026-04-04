package engine

import (
	"testing"

	"github.com/PatrickFanella/game-master/internal/game"
	"github.com/PatrickFanella/game-master/internal/llm"
)

// ---------------------------------------------------------------------------
// Mock implementation – compile-time interface check
// ---------------------------------------------------------------------------

// mockToolFilter is a minimal stub used to verify that the ToolFilter
// interface can be satisfied.
type mockToolFilter struct{}

var _ ToolFilter = (*mockToolFilter)(nil)

func (f *mockToolFilter) Filter(_ *game.GameState, allTools []llm.Tool) []llm.Tool {
	return allTools
}

// ---------------------------------------------------------------------------
// ToolFilter interface tests
// ---------------------------------------------------------------------------

func TestToolFilter_MockSatisfiesInterface(t *testing.T) {
	filter := &mockToolFilter{}

	state := &game.GameState{}
	tools := []llm.Tool{
		{Name: "move", Description: "Move to a location"},
		{Name: "attack", Description: "Attack a target"},
	}

	got := filter.Filter(state, tools)
	if len(got) != len(tools) {
		t.Errorf("expected %d tools, got %d", len(tools), len(got))
	}
	for i, tool := range got {
		if tool.Name != tools[i].Name {
			t.Errorf("tool[%d] name mismatch: got %q, want %q", i, tool.Name, tools[i].Name)
		}
	}
}

func TestToolFilter_FilterCanReturnSubset(t *testing.T) {
	combatFilter := &combatOnlyFilter{}
	state := &game.GameState{}
	allTools := []llm.Tool{
		{Name: "move"},
		{Name: "attack"},
		{Name: "rest"},
	}

	got := combatFilter.Filter(state, allTools)
	if len(got) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(got))
	}
	if got[0].Name != "attack" {
		t.Errorf("expected tool 'attack', got %q", got[0].Name)
	}
}

// combatOnlyFilter is a second mock that only passes through the "attack" tool.
type combatOnlyFilter struct{}

func (f *combatOnlyFilter) Filter(_ *game.GameState, allTools []llm.Tool) []llm.Tool {
	var out []llm.Tool
	for _, t := range allTools {
		if t.Name == "attack" {
			out = append(out, t)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// GamePhase enum tests
// ---------------------------------------------------------------------------

func TestGamePhase_Values(t *testing.T) {
	tests := []struct {
		phase    GamePhase
		expected int
	}{
		{PhaseExploration, 0},
		{PhaseCombat, 1},
		{PhaseSocial, 2},
		{PhaseRest, 3},
	}

	for _, tt := range tests {
		if int(tt.phase) != tt.expected {
			t.Errorf("phase %v: expected integer value %d, got %d", tt.phase, tt.expected, int(tt.phase))
		}
	}
}

func TestGamePhase_Distinct(t *testing.T) {
	phases := []GamePhase{PhaseExploration, PhaseCombat, PhaseSocial, PhaseRest}
	seen := make(map[GamePhase]struct{}, len(phases))
	for _, p := range phases {
		if _, dup := seen[p]; dup {
			t.Errorf("duplicate GamePhase value: %d", int(p))
		}
		seen[p] = struct{}{}
	}
}

func TestGamePhase_String(t *testing.T) {
	tests := []struct {
		phase    GamePhase
		expected string
	}{
		{PhaseExploration, "Exploration"},
		{PhaseCombat, "Combat"},
		{PhaseSocial, "Social"},
		{PhaseRest, "Rest"},
	}

	for _, tt := range tests {
		got := tt.phase.String()
		if got != tt.expected {
			t.Errorf("GamePhase(%d).String() = %q, want %q", int(tt.phase), got, tt.expected)
		}
	}
}

func TestGamePhase_String_Unknown(t *testing.T) {
	unknown := GamePhase(99)
	got := unknown.String()
	if got != "Unknown" {
		t.Errorf("unexpected String() for unknown phase: got %q, want %q", got, "Unknown")
	}
}
