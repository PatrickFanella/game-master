package combat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Contract test suite
// ---------------------------------------------------------------------------

// RunCombatResolverContractTests runs the contract test suite that any
// CombatResolver implementation must pass. These tests define expected
// behaviors: combat initiates with the correct combatant count, rounds
// process in initiative order, damage reduces HP, HP reaching zero
// triggers death or unconsciousness, conditions expire after their
// duration, and combat resolves with proper consequences.
//
// The newResolver function is called once per sub-test to create a fresh,
// deterministically configured resolver. Implementations should configure
// their resolver so that attacks generally succeed (both player and NPC),
// enabling damage-related contract assertions to be verified.
func RunCombatResolverContractTests(t *testing.T, newResolver func(t *testing.T) CombatResolver) {
	t.Helper()

	t.Run("InitiateCombat_CorrectCombatantCount", func(t *testing.T) {
		resolver := newResolver(t)
		ctx := context.Background()

		player := contractPlayer(t, 20, 20)
		npc1 := contractNPC(t, "Goblin", 10, 10)
		npc2 := contractNPC(t, "Orc", 15, 15)

		state, err := resolver.InitiateCombat(ctx,
			[]Combatant{player, npc1, npc2},
			Environment{Description: "Forest clearing"},
		)
		if err != nil {
			t.Fatalf("InitiateCombat: %v", err)
		}
		if len(state.Combatants) != 3 {
			t.Errorf("combatant count = %d, want 3", len(state.Combatants))
		}
		if state.Status != CombatStatusActive {
			t.Errorf("status = %q, want %q", state.Status, CombatStatusActive)
		}
		if state.ID == uuid.Nil {
			t.Error("state ID should not be nil")
		}
		if len(state.InitiativeOrder) != 3 {
			t.Errorf("initiative order length = %d, want 3", len(state.InitiativeOrder))
		}
	})

	t.Run("ProcessRound_RespectsInitiativeOrder", func(t *testing.T) {
		resolver := newResolver(t)
		ctx := context.Background()

		player := contractPlayer(t, 20, 20)
		npc := contractNPC(t, "Goblin", 20, 20)

		state, err := resolver.InitiateCombat(ctx,
			[]Combatant{player, npc},
			Environment{Description: "Arena"},
		)
		if err != nil {
			t.Fatalf("InitiateCombat: %v", err)
		}

		// Verify initiative order contains all combatant IDs.
		if len(state.InitiativeOrder) != 2 {
			t.Fatalf("initiative order length = %d, want 2", len(state.InitiativeOrder))
		}
		ids := make(map[uuid.UUID]bool, len(state.InitiativeOrder))
		for _, id := range state.InitiativeOrder {
			ids[id] = true
		}
		if !ids[player.EntityID] {
			t.Error("initiative order missing player")
		}
		if !ids[npc.EntityID] {
			t.Error("initiative order missing NPC")
		}

		// Verify combatants are sorted by initiative (descending).
		for i := 1; i < len(state.Combatants); i++ {
			if state.Combatants[i].Initiative > state.Combatants[i-1].Initiative {
				t.Errorf("combatants not sorted by initiative: index %d (%d) > index %d (%d)",
					i, state.Combatants[i].Initiative, i-1, state.Combatants[i-1].Initiative)
			}
		}

		// Process a round and verify it advances.
		npcID := npc.EntityID
		action := contractAttackAction(t, player.EntityID, &npcID)
		result, err := resolver.ProcessRound(ctx, action, state)
		if err != nil {
			t.Fatalf("ProcessRound: %v", err)
		}

		if result.RoundNumber < 1 {
			t.Errorf("round number = %d, want >= 1", result.RoundNumber)
		}
		if result.UpdatedState == nil {
			t.Fatal("UpdatedState should not be nil")
		}
		if len(result.ActionsTaken) == 0 {
			t.Error("expected at least one action taken")
		}

		// Verify that actions were executed in initiative order.
		initOrderIndex := make(map[uuid.UUID]int, len(state.InitiativeOrder))
		for idx, id := range state.InitiativeOrder {
			initOrderIndex[id] = idx
		}

		prevIdx := -1
		for i, act := range result.ActionsTaken {
			idx, ok := initOrderIndex[act.ActorID]
			if !ok {
				t.Fatalf("action %d has actor %v not present in initiative order", i, act.ActorID)
			}
			if idx < prevIdx {
				t.Errorf("actions not in initiative order: action %d actor %v (initiative index %d) occurs after actor with lower initiative index %d", i, act.ActorID, idx, prevIdx)
			}
			prevIdx = idx
		}
	})

	t.Run("DamageReducesHP", func(t *testing.T) {
		resolver := newResolver(t)
		ctx := context.Background()

		player := contractPlayer(t, 30, 30)
		npc := contractNPC(t, "Goblin", 20, 20)

		state, err := resolver.InitiateCombat(ctx,
			[]Combatant{player, npc},
			Environment{Description: "Dungeon"},
		)
		if err != nil {
			t.Fatalf("InitiateCombat: %v", err)
		}

		npcID := npc.EntityID
		action := contractAttackAction(t, player.EntityID, &npcID)

		beforeHP := make(map[uuid.UUID]int)
		for _, c := range state.Combatants {
			beforeHP[c.EntityID] = c.HP
		}

		result, err := resolver.ProcessRound(ctx, action, state)
		if err != nil {
			t.Fatalf("ProcessRound: %v", err)
		}
		state = result.UpdatedState

		// Aggregate damage per target.
		totalDamage := make(map[uuid.UUID]int)
		for _, dmg := range result.DamageDealt {
			totalDamage[dmg.TargetID] += dmg.Amount
		}

		// For each target that received damage, verify HP was reduced correctly.
		for targetID, dmg := range totalDamage {
			before, ok := beforeHP[targetID]
			if !ok {
				t.Errorf("DamageDealt references unknown target %v not in combatants list", targetID)
				continue
			}
			after := contractFindHP(state, targetID)
			if after == -1 {
				t.Errorf("target %v not found in updated state after round", targetID)
				continue
			}
			expected := before - dmg
			if expected < 0 {
				expected = 0
			}
			if after != expected {
				t.Errorf("target %v HP = %d, want %d (was %d, took %d damage)",
					targetID, after, expected, before, dmg)
			}
		}

		// Ensure damage was actually dealt.
		if len(result.DamageDealt) == 0 {
			t.Error("expected at least one damage record")
		}
	})

	t.Run("HPZeroTriggersNPCDeath", func(t *testing.T) {
		resolver := newResolver(t)
		ctx := context.Background()

		player := contractPlayer(t, 50, 50)
		npc := contractNPC(t, "Weakling", 5, 5)

		state, err := resolver.InitiateCombat(ctx,
			[]Combatant{player, npc},
			Environment{Description: "Battlefield"},
		)
		if err != nil {
			t.Fatalf("InitiateCombat: %v", err)
		}

		npcID := npc.EntityID
		action := contractAttackAction(t, player.EntityID, &npcID)

		for round := 0; round < 10; round++ {
			if state.Status != CombatStatusActive {
				break
			}
			result, err := resolver.ProcessRound(ctx, action, state)
			if err != nil {
				t.Fatalf("ProcessRound round %d: %v", round+1, err)
			}
			state = result.UpdatedState
		}

		npcCombatant := contractFindCombatant(state, npc.EntityID)
		if npcCombatant == nil {
			t.Fatal("NPC combatant not found")
		}
		if npcCombatant.HP != 0 {
			t.Fatalf("NPC HP = %d, want 0 (should have been killed)", npcCombatant.HP)
		}
		if npcCombatant.Status != CombatantStatusDead {
			t.Errorf("NPC at 0 HP: status = %q, want %q", npcCombatant.Status, CombatantStatusDead)
		}
	})

	t.Run("HPZeroTriggersPlayerUnconsciousness", func(t *testing.T) {
		resolver := newResolver(t)
		ctx := context.Background()

		// Player has just enough HP to be knocked out by NPC damage.
		player := contractPlayer(t, 3, 20)
		npc := contractNPC(t, "Brute", 100, 100)

		state, err := resolver.InitiateCombat(ctx,
			[]Combatant{player, npc},
			Environment{Description: "Arena"},
		)
		if err != nil {
			t.Fatalf("InitiateCombat: %v", err)
		}

		npcID := npc.EntityID
		action := contractAttackAction(t, player.EntityID, &npcID)

		for round := 0; round < 10; round++ {
			if state.Status != CombatStatusActive {
				break
			}
			result, err := resolver.ProcessRound(ctx, action, state)
			if err != nil {
				t.Fatalf("ProcessRound round %d: %v", round+1, err)
			}
			state = result.UpdatedState
		}

		playerCombatant := contractFindCombatant(state, player.EntityID)
		if playerCombatant == nil {
			t.Fatal("player combatant not found")
		}
		if playerCombatant.HP != 0 {
			t.Fatalf("player HP = %d, want 0 (should have been knocked unconscious)", playerCombatant.HP)
		}
		if playerCombatant.Status != CombatantStatusUnconscious {
			t.Errorf("player at 0 HP: status = %q, want %q",
				playerCombatant.Status, CombatantStatusUnconscious)
		}
	})

	t.Run("ConditionsExpireAfterDuration", func(t *testing.T) {
		resolver := newResolver(t)
		ctx := context.Background()

		player := contractPlayer(t, 50, 50)
		npc := contractNPC(t, "Goblin", 50, 50)

		state, err := resolver.InitiateCombat(ctx,
			[]Combatant{player, npc},
			Environment{Description: "Cave"},
		)
		if err != nil {
			t.Fatalf("InitiateCombat: %v", err)
		}

		// Add a condition with duration 2 to the NPC.
		npcCombatant := contractFindCombatant(state, npc.EntityID)
		if npcCombatant == nil {
			t.Fatal("NPC combatant not found")
		}
		AddCondition(npcCombatant, "test_condition", 2)
		if !HasCondition(npcCombatant, "test_condition") {
			t.Fatal("condition should be present after adding")
		}

		npcID := npc.EntityID
		action := contractAttackAction(t, player.EntityID, &npcID)

		// Round 1: condition ticks from duration 2 to 1.
		result1, err := resolver.ProcessRound(ctx, action, state)
		if err != nil {
			t.Fatalf("ProcessRound round 1: %v", err)
		}
		state = result1.UpdatedState
		npcCombatant = contractFindCombatant(state, npc.EntityID)
		if npcCombatant == nil {
			t.Fatal("NPC not found after round 1")
		}
		if !HasCondition(npcCombatant, "test_condition") {
			t.Error("condition should still be present after round 1 (duration was 2)")
		}

		// Round 2: condition ticks from duration 1 to 0 and expires.
		if state.Status != CombatStatusActive {
			t.Fatal("combat should still be active for round 2")
		}
		result2, err := resolver.ProcessRound(ctx, action, state)
		if err != nil {
			t.Fatalf("ProcessRound round 2: %v", err)
		}
		state = result2.UpdatedState
		npcCombatant = contractFindCombatant(state, npc.EntityID)
		if npcCombatant == nil {
			t.Fatal("NPC not found after round 2")
		}
		if HasCondition(npcCombatant, "test_condition") {
			t.Error("condition should have expired after 2 rounds")
		}
	})

	t.Run("ResolveCombat_ProducesOutcome", func(t *testing.T) {
		resolver := newResolver(t)
		ctx := context.Background()

		player := contractPlayer(t, 30, 30)
		npc := contractNPC(t, "Goblin", 5, 10)

		state, err := resolver.InitiateCombat(ctx,
			[]Combatant{player, npc},
			Environment{Description: "Town square"},
		)
		if err != nil {
			t.Fatalf("InitiateCombat: %v", err)
		}

		npcID := npc.EntityID
		action := contractAttackAction(t, player.EntityID, &npcID)

		// Run rounds until combat ends.
		for round := 0; round < 10; round++ {
			if state.Status != CombatStatusActive {
				break
			}
			result, err := resolver.ProcessRound(ctx, action, state)
			if err != nil {
				t.Fatalf("ProcessRound round %d: %v", round+1, err)
			}
			state = result.UpdatedState
		}

		outcome, err := resolver.ResolveCombat(ctx, state)
		if err != nil {
			t.Fatalf("ResolveCombat: %v", err)
		}

		if outcome.Narrative == "" {
			t.Error("outcome narrative should not be empty")
		}

		// NPC must have been killed for this contract test to be meaningful.
		npcCombatant := contractFindCombatant(state, npc.EntityID)
		if npcCombatant == nil {
			t.Fatal("NPC combatant not found in state")
		}
		if npcCombatant.Status != CombatantStatusDead {
			t.Fatalf("NPC status = %q, want %q; combat should have resulted in NPC death",
				npcCombatant.Status, CombatantStatusDead)
		}

		if outcome.Winner == nil {
			t.Error("expected a winner when NPC is dead")
		}
		if len(outcome.Casualties) == 0 {
			t.Error("expected casualties when NPC is dead")
		}
		if outcome.XPEarned <= 0 {
			t.Error("expected positive XP earned when NPC is dead")
		}
		// Verify dead NPC appears in casualties list.
		found := false
		for _, id := range outcome.Casualties {
			if id == npc.EntityID {
				found = true
				break
			}
		}
		if !found {
			t.Error("dead NPC should be in casualties list")
		}

		// ResolveCombat sets state to completed (mutation is part of the
		// contract — the interface does not return an updated state).
		if state.Status != CombatStatusCompleted {
			t.Errorf("combat status = %q, want %q after ResolveCombat",
				state.Status, CombatStatusCompleted)
		}
	})
}

// ---------------------------------------------------------------------------
// Default implementation: NarrativeCombatResolver contract compliance
// ---------------------------------------------------------------------------

// TestNarrativeResolver_ContractSuite verifies that the default
// NarrativeCombatResolver passes all contract tests.
func TestNarrativeResolver_ContractSuite(t *testing.T) {
	RunCombatResolverContractTests(t, func(t *testing.T) CombatResolver {
		t.Helper()
		// Use a deterministic roller where all d20 rolls return 15,
		// guaranteeing hits for both player and NPC actions (15 + modifier
		// always exceeds the default DC of 10).
		roller := &fixedInitiativeRoller{
			rolls: contractRepeatedRolls(15, 50),
		}
		return newNarrativeCombatResolverWithRoller(uuid.New(), roller)
	})
}

// ---------------------------------------------------------------------------
// Contract test helpers
// ---------------------------------------------------------------------------

func contractPlayer(t *testing.T, hp, maxHP int) Combatant {
	t.Helper()
	stats, err := json.Marshal(map[string]int{"strength": 14, "dexterity": 12})
	if err != nil {
		t.Fatalf("marshal player stats: %v", err)
	}
	return Combatant{
		EntityID:   uuid.New(),
		EntityType: CombatantTypePlayer,
		Name:       "Hero",
		HP:         hp,
		MaxHP:      maxHP,
		Stats:      stats,
		Status:     CombatantStatusAlive,
	}
}

func contractNPC(t *testing.T, name string, hp, maxHP int) Combatant {
	t.Helper()
	stats, err := json.Marshal(map[string]int{"strength": 12, "dexterity": 10})
	if err != nil {
		t.Fatalf("marshal NPC stats: %v", err)
	}
	return Combatant{
		EntityID:   uuid.New(),
		EntityType: CombatantTypeNPC,
		Name:       name,
		HP:         hp,
		MaxHP:      maxHP,
		Stats:      stats,
		Status:     CombatantStatusAlive,
	}
}

func contractAttackAction(t *testing.T, combatantID uuid.UUID, targetID *uuid.UUID) PlayerAction {
	t.Helper()
	details, err := json.Marshal(map[string]any{
		"skill":       "strength",
		"difficulty":  10,
		"damage_on_hit": 5,
		"damage_type": "slashing",
	})
	if err != nil {
		t.Fatalf("marshal action details: %v", err)
	}
	return PlayerAction{
		CombatantID: combatantID,
		ActionType:  ActionTypeAttack,
		TargetID:    targetID,
		Description: "Contract test attack",
		Details:     details,
	}
}

func contractFindCombatant(state *CombatState, id uuid.UUID) *Combatant {
	if state == nil {
		return nil
	}
	for i := range state.Combatants {
		if state.Combatants[i].EntityID == id {
			return &state.Combatants[i]
		}
	}
	return nil
}

func contractFindHP(state *CombatState, id uuid.UUID) int {
	c := contractFindCombatant(state, id)
	if c == nil {
		return -1
	}
	return c.HP
}

func contractRepeatedRolls(value, count int) []int {
	rolls := make([]int, count)
	for i := range rolls {
		rolls[i] = value
	}
	return rolls
}
