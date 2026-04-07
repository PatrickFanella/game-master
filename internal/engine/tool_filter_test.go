package engine

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/game"
	"github.com/PatrickFanella/game-master/internal/llm"
	"github.com/PatrickFanella/game-master/internal/tools"
)

// testRegistry returns a *tools.Registry with metadata set for every tool
// used in the test suite. The handlers are no-ops since tests only exercise
// the filtering logic.
func testRegistry() *tools.Registry {
	noop := func(_ context.Context, _ map[string]any) (*tools.ToolResult, error) {
		return &tools.ToolResult{Success: true}, nil
	}

	metas := []tools.ToolMeta{
		// base
		{Name: "skill_check", Category: tools.CategoryBase},
		{Name: "roll_dice", Category: tools.CategoryBase},
		{Name: "move_player", Category: tools.CategoryBase},
		{Name: "describe_scene", Category: tools.CategoryBase},
		{Name: "present_choices", Category: tools.CategoryBase},
		{Name: "npc_dialogue", Category: tools.CategoryBase},
		{Name: "establish_fact", Category: tools.CategoryBase},
		{Name: "revise_fact", Category: tools.CategoryBase},
		{Name: "update_npc", Category: tools.CategoryBase},
		{Name: "add_item", Category: tools.CategoryBase},
		{Name: "remove_item", Category: tools.CategoryBase},
		{Name: "modify_item", Category: tools.CategoryBase},
		{Name: "create_item", Category: tools.CategoryBase},
		{Name: "generate_name", Category: tools.CategoryBase},
		{Name: "search_memory", Category: tools.CategoryBase},
		// combat (initiate_combat is exploration; dual-membership handled by filter)
		{Name: "initiate_combat", Category: tools.CategoryExploration},
		{Name: "combat_round", Category: tools.CategoryCombat},
		{Name: "apply_damage", Category: tools.CategoryCombat},
		{Name: "apply_condition", Category: tools.CategoryCombat},
		{Name: "resolve_combat", Category: tools.CategoryCombat},
		{Name: "add_ability", Category: tools.CategoryCombat},
		{Name: "remove_ability", Category: tools.CategoryCombat},
		{Name: "update_player_status", Category: tools.CategoryCombat},
		// exploration
		{Name: "create_npc", Category: tools.CategoryExploration},
		{Name: "create_location", Category: tools.CategoryExploration},
		{Name: "create_city", Category: tools.CategoryExploration},
		{Name: "create_faction", Category: tools.CategoryExploration},
		{Name: "create_language", Category: tools.CategoryExploration},
		{Name: "create_culture", Category: tools.CategoryExploration},
		{Name: "create_belief_system", Category: tools.CategoryExploration},
		{Name: "create_economic_system", Category: tools.CategoryExploration},
		{Name: "create_lore", Category: tools.CategoryExploration},
		{Name: "establish_relationship", Category: tools.CategoryExploration},
		{Name: "reveal_location", Category: tools.CategoryExploration},
		// quest
		{Name: "create_quest", Category: tools.CategoryQuest},
		{Name: "create_subquest", Category: tools.CategoryQuest},
		{Name: "update_quest", Category: tools.CategoryQuest},
		{Name: "complete_objective", Category: tools.CategoryQuest},
		{Name: "branch_quest", Category: tools.CategoryQuest},
		{Name: "link_quest_entity", Category: tools.CategoryQuest},
		// progression
		{Name: "add_experience", Category: tools.CategoryProgression},
		{Name: "level_up", Category: tools.CategoryProgression},
		{Name: "update_player_stats", Category: tools.CategoryProgression},
	}

	reg := tools.NewRegistry()
	for _, m := range metas {
		m.Definition = llm.Tool{Name: m.Name}
		_ = reg.RegisterWithMeta(m, noop)
	}
	return reg
}

// allTestTools returns a superset of tool names to simulate the full registry.
func allTestTools() []llm.Tool {
	names := []string{
		// base
		"skill_check", "roll_dice", "move_player", "describe_scene",
		"present_choices", "npc_dialogue", "establish_fact", "revise_fact",
		"update_npc", "add_item", "remove_item", "modify_item", "create_item",
		"generate_name", "search_memory",
		// combat
		"initiate_combat", "combat_round", "apply_damage", "apply_condition",
		"resolve_combat", "add_ability", "remove_ability",
		// exploration
		"create_npc", "create_location", "create_city", "create_faction",
		"create_language", "create_culture", "create_belief_system",
		"create_economic_system", "create_lore", "establish_relationship", "reveal_location",
		// quest
		"create_quest", "create_subquest", "update_quest", "complete_objective",
		"branch_quest", "link_quest_entity",
		// progression
		"add_experience", "level_up", "update_player_stats", "update_player_status",
	}
	tools := make([]llm.Tool, len(names))
	for i, n := range names {
		tools[i] = llm.Tool{Name: n}
	}
	return tools
}

func toolNames(tools []llm.Tool) map[string]struct{} {
	m := make(map[string]struct{}, len(tools))
	for _, t := range tools {
		m[t.Name] = struct{}{}
	}
	return m
}

func hasAll(t *testing.T, tools []llm.Tool, names ...string) {
	t.Helper()
	m := toolNames(tools)
	for _, name := range names {
		if _, ok := m[name]; !ok {
			t.Errorf("expected tool %q in filtered set, not found (got %d tools)", name, len(tools))
		}
	}
}

func hasNone(t *testing.T, tools []llm.Tool, names ...string) {
	t.Helper()
	m := toolNames(tools)
	for _, name := range names {
		if _, ok := m[name]; ok {
			t.Errorf("tool %q should NOT be in filtered set", name)
		}
	}
}

// --- Interface compliance ---

var _ ToolFilter = (*PhaseToolFilter)(nil)

// --- GamePhase ---

func TestGamePhase_String(t *testing.T) {
	if PhaseExploration.String() != "exploration" {
		t.Errorf("got %q", PhaseExploration.String())
	}
	if PhaseCombat.String() != "combat" {
		t.Errorf("got %q", PhaseCombat.String())
	}
	if GamePhase(99).String() != "unknown" {
		t.Errorf("got %q", GamePhase(99).String())
	}
}

// --- DetectPhase ---

func TestDetectPhase_NilState(t *testing.T) {
	if got := DetectPhase(nil); got != PhaseExploration {
		t.Errorf("nil state: got %v, want exploration", got)
	}
}

func TestDetectPhase_Exploration(t *testing.T) {
	state := &game.GameState{Player: domain.PlayerCharacter{Status: "active"}}
	if got := DetectPhase(state); got != PhaseExploration {
		t.Errorf("got %v, want exploration", got)
	}
}

func TestDetectPhase_Combat(t *testing.T) {
	state := &game.GameState{Player: domain.PlayerCharacter{Status: "in_combat"}}
	if got := DetectPhase(state); got != PhaseCombat {
		t.Errorf("got %v, want combat", got)
	}
}

// --- FilterTools: nil state returns all ---

func TestFilter_NilState_ReturnsAll(t *testing.T) {
	f := NewPhaseToolFilter(testRegistry())
	all := allTestTools()
	got := f.Filter(nil, all)
	if len(got) != len(all) {
		t.Errorf("nil state: got %d tools, want %d", len(got), len(all))
	}
}

// --- FilterTools: base tools always present ---

func TestFilter_BaseToolsAlwaysPresent(t *testing.T) {
	f := NewPhaseToolFilter(testRegistry())
	state := &game.GameState{Player: domain.PlayerCharacter{Status: "active"}}
	got := f.Filter(state, allTestTools())

	hasAll(t, got,
		"skill_check", "roll_dice", "move_player", "describe_scene",
		"present_choices", "npc_dialogue", "establish_fact", "update_npc",
		"add_item", "remove_item",
	)
}

// --- FilterTools: combat phase ---

func TestFilter_CombatPhase_HasCombatTools(t *testing.T) {
	f := NewPhaseToolFilter(testRegistry())
	state := &game.GameState{
		Player:    domain.PlayerCharacter{Status: "in_combat"},
		RulesMode: "light",
	}
	got := f.Filter(state, allTestTools())

	hasAll(t, got, "combat_round", "apply_damage", "apply_condition", "resolve_combat")
}

func TestFilter_CombatPhase_NoExplorationTools(t *testing.T) {
	f := NewPhaseToolFilter(testRegistry())
	state := &game.GameState{
		Player:    domain.PlayerCharacter{Status: "in_combat"},
		RulesMode: "light",
	}
	got := f.Filter(state, allTestTools())

	hasNone(t, got, "create_npc", "create_location", "create_faction", "create_language")
}

// --- FilterTools: exploration phase ---

func TestFilter_Exploration_HasWorldBuildingTools(t *testing.T) {
	f := NewPhaseToolFilter(testRegistry())
	state := &game.GameState{
		Player:    domain.PlayerCharacter{Status: "active"},
		RulesMode: "light",
	}
	got := f.Filter(state, allTestTools())

	hasAll(t, got, "create_npc", "create_location", "create_faction",
		"create_language", "initiate_combat")
}

func TestFilter_Exploration_NoCombatRoundTools(t *testing.T) {
	f := NewPhaseToolFilter(testRegistry())
	state := &game.GameState{Player: domain.PlayerCharacter{Status: "active"}}
	got := f.Filter(state, allTestTools())

	hasNone(t, got, "combat_round", "apply_damage", "resolve_combat")
}

// --- FilterTools: quest tools ---

func TestFilter_QuestToolsWhenActiveQuests(t *testing.T) {
	f := NewPhaseToolFilter(testRegistry())
	state := &game.GameState{
		Player:       domain.PlayerCharacter{Status: "active"},
		ActiveQuests: []domain.Quest{{ID: uuid.New(), Title: "Find the sword"}},
	}
	got := f.Filter(state, allTestTools())

	hasAll(t, got, "create_quest", "update_quest", "complete_objective", "branch_quest")
}

func TestFilter_QuestToolsWhenNPCsPresent(t *testing.T) {
	f := NewPhaseToolFilter(testRegistry())
	state := &game.GameState{
		Player:     domain.PlayerCharacter{Status: "active"},
		NearbyNPCs: []domain.NPC{{ID: uuid.New(), Name: "Bartender"}},
	}
	got := f.Filter(state, allTestTools())

	hasAll(t, got, "create_quest", "update_quest")
}

func TestFilter_NoQuestToolsWhenNoQuestsOrNPCs(t *testing.T) {
	f := NewPhaseToolFilter(testRegistry())
	state := &game.GameState{Player: domain.PlayerCharacter{Status: "active"}}
	got := f.Filter(state, allTestTools())

	hasNone(t, got, "create_quest", "update_quest", "complete_objective")
}

// --- FilterTools: progression tools ---

func TestFilter_AddExperienceAlwaysPresent(t *testing.T) {
	f := NewPhaseToolFilter(testRegistry())
	state := &game.GameState{
		Player:    domain.PlayerCharacter{Status: "active", Level: 1, Experience: 0},
		RulesMode: "light",
	}
	got := f.Filter(state, allTestTools())

	hasAll(t, got, "add_experience")
}

func TestFilter_ProgressionToolsNearLevelUp(t *testing.T) {
	f := NewPhaseToolFilter(testRegistry())
	// Level 1 threshold is 100, 50% = 50 XP
	state := &game.GameState{
		Player:    domain.PlayerCharacter{Status: "active", Level: 1, Experience: 60},
		RulesMode: "light",
	}
	got := f.Filter(state, allTestTools())

	hasAll(t, got, "level_up", "update_player_stats")
}

func TestFilter_NoProgressionToolsFarFromLevelUp(t *testing.T) {
	f := NewPhaseToolFilter(testRegistry())
	// Level 1 needs 100 XP, player has 10 (well below 50%)
	state := &game.GameState{
		Player: domain.PlayerCharacter{Status: "active", Level: 1, Experience: 10},
	}
	got := f.Filter(state, allTestTools())

	hasNone(t, got, "level_up", "update_player_stats")
}

// --- FilterTools: tool count reduction ---

func TestFilter_ReducesToolCount(t *testing.T) {
	f := NewPhaseToolFilter(testRegistry())
	all := allTestTools()
	state := &game.GameState{Player: domain.PlayerCharacter{Status: "active"}}
	got := f.Filter(state, all)

	if len(got) >= len(all) {
		t.Errorf("filtering should reduce tools: got %d, total %d", len(got), len(all))
	}
	if len(got) > 35 {
		t.Errorf("filtered set too large: %d (target ~15-30)", len(got))
	}
}

// --- nearLevelThreshold ---

func TestNearLevelThreshold_AtHalfway(t *testing.T) {
	state := &game.GameState{Player: domain.PlayerCharacter{Level: 1, Experience: 50}}
	if !nearLevelThreshold(state) {
		t.Error("should be near threshold at 50% (50/100)")
	}
}

func TestNearLevelThreshold_BelowHalfway(t *testing.T) {
	state := &game.GameState{Player: domain.PlayerCharacter{Level: 1, Experience: 10}}
	if nearLevelThreshold(state) {
		t.Error("should NOT be near threshold at 10% (10/100)")
	}
}

func TestNearLevelThreshold_MaxLevel(t *testing.T) {
	state := &game.GameState{Player: domain.PlayerCharacter{Level: 99, Experience: 9999}}
	if !nearLevelThreshold(state) {
		t.Error("max level should always return true")
	}
}
