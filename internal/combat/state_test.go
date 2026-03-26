package combat

import (
	"testing"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makePlayer(hp, maxHP int) *Combatant {
	return &Combatant{
		EntityID:   uuid.New(),
		EntityType: CombatantTypePlayer,
		Name:       "Player",
		HP:         hp,
		MaxHP:      maxHP,
		Status:     CombatantStatusAlive,
	}
}

func makeNPC(hp, maxHP int) *Combatant {
	return &Combatant{
		EntityID:   uuid.New(),
		EntityType: CombatantTypeNPC,
		Name:       "NPC",
		HP:         hp,
		MaxHP:      maxHP,
		Status:     CombatantStatusAlive,
	}
}

// ---------------------------------------------------------------------------
// HP management
// ---------------------------------------------------------------------------

func TestApplyDamageClampsToZero(t *testing.T) {
	c := makePlayer(5, 10)
	ApplyDamage(c, 20)
	if c.HP != 0 {
		t.Fatalf("HP = %d, want 0 after excess damage", c.HP)
	}
}

func TestApplyDamageReducesHP(t *testing.T) {
	c := makePlayer(10, 10)
	ApplyDamage(c, 3)
	if c.HP != 7 {
		t.Fatalf("HP = %d, want 7", c.HP)
	}
	if c.Status != CombatantStatusAlive {
		t.Fatalf("Status = %q, want alive", c.Status)
	}
}

func TestApplyDamageZeroOrNegativeIgnored(t *testing.T) {
	c := makePlayer(10, 10)
	ApplyDamage(c, 0)
	ApplyDamage(c, -5)
	if c.HP != 10 {
		t.Fatalf("HP = %d after zero/negative damage, want 10", c.HP)
	}
}

func TestApplyDamageNPCDiesAtZeroHP(t *testing.T) {
	c := makeNPC(5, 10)
	ApplyDamage(c, 5)
	if c.HP != 0 {
		t.Fatalf("HP = %d, want 0", c.HP)
	}
	if c.Status != CombatantStatusDead {
		t.Fatalf("NPC status = %q, want dead", c.Status)
	}
}

func TestApplyDamagePlayerFallsUnconsciousAtZeroHP(t *testing.T) {
	c := makePlayer(5, 10)
	ApplyDamage(c, 5)
	if c.HP != 0 {
		t.Fatalf("HP = %d, want 0", c.HP)
	}
	if c.Status != CombatantStatusUnconscious {
		t.Fatalf("player status = %q, want unconscious", c.Status)
	}
}

func TestApplyDamageIgnoredOnDeadCombatant(t *testing.T) {
	c := makeNPC(0, 10)
	c.Status = CombatantStatusDead
	ApplyDamage(c, 5)
	if c.HP != 0 {
		t.Fatalf("HP = %d, dead combatant should not receive damage", c.HP)
	}
}

func TestApplyHealingRestoresHP(t *testing.T) {
	c := makePlayer(5, 10)
	ApplyHealing(c, 3)
	if c.HP != 8 {
		t.Fatalf("HP = %d, want 8", c.HP)
	}
}

func TestApplyHealingClampsToMaxHP(t *testing.T) {
	c := makePlayer(8, 10)
	ApplyHealing(c, 100)
	if c.HP != 10 {
		t.Fatalf("HP = %d after over-healing, want 10", c.HP)
	}
}

func TestApplyHealingZeroOrNegativeIgnored(t *testing.T) {
	c := makePlayer(5, 10)
	ApplyHealing(c, 0)
	ApplyHealing(c, -3)
	if c.HP != 5 {
		t.Fatalf("HP = %d after zero/negative heal, want 5", c.HP)
	}
}

func TestApplyHealingIgnoredOnDeadCombatant(t *testing.T) {
	c := makePlayer(0, 10)
	c.Status = CombatantStatusDead
	ApplyHealing(c, 5)
	if c.HP != 0 {
		t.Fatalf("HP = %d, dead combatant should not be healed", c.HP)
	}
	if c.Status != CombatantStatusDead {
		t.Fatalf("Status = %q, dead combatant should remain dead", c.Status)
	}
}

func TestApplyHealingRestoresConsciousness(t *testing.T) {
	c := makePlayer(0, 10)
	c.Status = CombatantStatusUnconscious
	c.DeathSavingThrows = &DeathSavingThrows{Successes: 1, Failures: 1}
	ApplyHealing(c, 4)
	if c.HP != 4 {
		t.Fatalf("HP = %d, want 4", c.HP)
	}
	if c.Status != CombatantStatusAlive {
		t.Fatalf("Status = %q, want alive after healing", c.Status)
	}
	if c.DeathSavingThrows != nil {
		t.Fatal("DeathSavingThrows should be cleared after healing")
	}
}

// ---------------------------------------------------------------------------
// Condition management
// ---------------------------------------------------------------------------

func TestAddConditionAppends(t *testing.T) {
	c := makePlayer(10, 10)
	AddCondition(c, ConditionStunned, 2)
	if len(c.Conditions) != 1 {
		t.Fatalf("len(Conditions) = %d, want 1", len(c.Conditions))
	}
	if c.Conditions[0].Name != ConditionStunned || c.Conditions[0].DurationRounds != 2 {
		t.Fatalf("unexpected condition: %+v", c.Conditions[0])
	}
}

func TestAddConditionUpdatesDurationIfAlreadyPresent(t *testing.T) {
	c := makePlayer(10, 10)
	AddCondition(c, ConditionStunned, 1)
	AddCondition(c, ConditionStunned, 3)
	if len(c.Conditions) != 1 {
		t.Fatalf("len(Conditions) = %d, want 1 after update", len(c.Conditions))
	}
	if c.Conditions[0].DurationRounds != 3 {
		t.Fatalf("DurationRounds = %d, want 3", c.Conditions[0].DurationRounds)
	}
}

func TestAddConditionPermanentDuration(t *testing.T) {
	c := makePlayer(10, 10)
	AddCondition(c, ConditionProne, PermanentDuration)
	if c.Conditions[0].DurationRounds != PermanentDuration {
		t.Fatalf("DurationRounds = %d, want %d", c.Conditions[0].DurationRounds, PermanentDuration)
	}
}

func TestRemoveConditionRemovesTargetOnly(t *testing.T) {
	c := makePlayer(10, 10)
	AddCondition(c, ConditionStunned, 2)
	AddCondition(c, ConditionProne, 1)
	RemoveCondition(c, ConditionStunned)
	if len(c.Conditions) != 1 {
		t.Fatalf("len(Conditions) = %d, want 1 after removal", len(c.Conditions))
	}
	if c.Conditions[0].Name != ConditionProne {
		t.Fatalf("remaining condition = %q, want %q", c.Conditions[0].Name, ConditionProne)
	}
}

func TestRemoveConditionNoOpWhenAbsent(t *testing.T) {
	c := makePlayer(10, 10)
	AddCondition(c, ConditionProne, 2)
	RemoveCondition(c, ConditionStunned) // not present
	if len(c.Conditions) != 1 {
		t.Fatalf("len(Conditions) = %d, want 1 after no-op removal", len(c.Conditions))
	}
}

func TestHasCondition(t *testing.T) {
	c := makePlayer(10, 10)
	if HasCondition(c, ConditionStunned) {
		t.Fatal("HasCondition should return false before any condition is added")
	}
	AddCondition(c, ConditionStunned, 1)
	if !HasCondition(c, ConditionStunned) {
		t.Fatal("HasCondition should return true after adding condition")
	}
	RemoveCondition(c, ConditionStunned)
	if HasCondition(c, ConditionStunned) {
		t.Fatal("HasCondition should return false after removal")
	}
}

func TestTickConditionsDecrementsAndRemovesExpired(t *testing.T) {
	c := makePlayer(10, 10)
	AddCondition(c, ConditionStunned, 1)
	AddCondition(c, ConditionProne, 2)
	AddCondition(c, ConditionPoisoned, PermanentDuration)

	TickConditions(c)

	// stunned (duration 1) should expire.
	if HasCondition(c, ConditionStunned) {
		t.Error("stunned should have expired after tick")
	}
	// prone (duration 2→1) should still be present.
	if !HasCondition(c, ConditionProne) {
		t.Error("prone should still be active with duration 1")
	}
	for _, cond := range c.Conditions {
		if cond.Name == ConditionProne && cond.DurationRounds != 1 {
			t.Fatalf("prone DurationRounds = %d, want 1", cond.DurationRounds)
		}
	}
	// poisoned (permanent) should remain unchanged.
	if !HasCondition(c, ConditionPoisoned) {
		t.Error("poisoned (permanent) should still be active")
	}
	for _, cond := range c.Conditions {
		if cond.Name == ConditionPoisoned && cond.DurationRounds != PermanentDuration {
			t.Fatalf("permanent condition duration changed: got %d", cond.DurationRounds)
		}
	}
}

func TestTickConditionsRemovesAllExpired(t *testing.T) {
	c := makePlayer(10, 10)
	AddCondition(c, ConditionStunned, 1)
	AddCondition(c, ConditionBlinded, 1)
	TickConditions(c)
	if len(c.Conditions) != 0 {
		t.Fatalf("len(Conditions) = %d, want 0 after all conditions expire", len(c.Conditions))
	}
}

func TestTickAllConditionsTicksEveryCombatant(t *testing.T) {
	a := Combatant{EntityID: uuid.New(), EntityType: CombatantTypePlayer, Name: "A", HP: 10, MaxHP: 10}
	b := Combatant{EntityID: uuid.New(), EntityType: CombatantTypeNPC, Name: "B", HP: 10, MaxHP: 10}
	AddCondition(&a, ConditionStunned, 1)
	AddCondition(&b, ConditionProne, 1)

	state := baseCombatState(a, b)

	TickAllConditions(state)

	for i := range state.Combatants {
		if len(state.Combatants[i].Conditions) != 0 {
			t.Fatalf("combatant %s should have no conditions after tick, got %d",
				state.Combatants[i].Name, len(state.Combatants[i].Conditions))
		}
	}
}

// ---------------------------------------------------------------------------
// Condition effects
// ---------------------------------------------------------------------------

func TestSkipsTurnStunned(t *testing.T) {
	c := makePlayer(10, 10)
	if SkipsTurn(c) {
		t.Fatal("SkipsTurn should be false with no conditions")
	}
	AddCondition(c, ConditionStunned, 1)
	if !SkipsTurn(c) {
		t.Fatal("SkipsTurn should be true when stunned")
	}
}

func TestSkipsTurnParalyzed(t *testing.T) {
	c := makeNPC(10, 10)
	AddCondition(c, ConditionParalyzed, PermanentDuration)
	if !SkipsTurn(c) {
		t.Fatal("SkipsTurn should be true when paralyzed")
	}
}

func TestHasAttackDisadvantage(t *testing.T) {
	c := makePlayer(10, 10)
	if HasAttackDisadvantage(c) {
		t.Fatal("HasAttackDisadvantage should be false with no conditions")
	}
	for _, cond := range []string{ConditionProne, ConditionBlinded, ConditionPoisoned, ConditionFrightened} {
		cc := makePlayer(10, 10)
		AddCondition(cc, cond, 1)
		if !HasAttackDisadvantage(cc) {
			t.Fatalf("HasAttackDisadvantage should be true for condition %q", cond)
		}
	}
}

// ---------------------------------------------------------------------------
// Round-start condition ticking
// ---------------------------------------------------------------------------

func TestStartNextRoundTicksConditions(t *testing.T) {
	c := Combatant{EntityID: uuid.New(), EntityType: CombatantTypePlayer, Name: "P", HP: 10, MaxHP: 10}
	AddCondition(&c, ConditionStunned, 1)
	AddCondition(&c, ConditionProne, 2)

	state := baseCombatState(c)

	roller := &fixedInitiativeRoller{rolls: []int{10}}
	if err := startNextRoundWithRoller(state, roller); err != nil {
		t.Fatalf("startNextRound: %v", err)
	}

	// After first round starts, stunned (duration 1) should expire; prone (2→1) remains.
	comb := state.Combatants[0]
	if HasCondition(&comb, ConditionStunned) {
		t.Error("stunned should have been ticked off at round start")
	}
	if !HasCondition(&comb, ConditionProne) {
		t.Error("prone should still be active (duration 2→1)")
	}
}

// ---------------------------------------------------------------------------
// Death thresholds
// ---------------------------------------------------------------------------

func TestCombatantStatusValidate(t *testing.T) {
	valid := Combatant{
		EntityID:   uuid.New(),
		EntityType: CombatantTypePlayer,
		Name:       "Hero",
		HP:         0,
		MaxHP:      10,
		Status:     CombatantStatusUnconscious,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("unconscious combatant at 0 HP should be valid: %v", err)
	}

	deadNPC := Combatant{
		EntityID:   uuid.New(),
		EntityType: CombatantTypeNPC,
		Name:       "Goblin",
		HP:         0,
		MaxHP:      5,
		Status:     CombatantStatusDead,
	}
	if err := deadNPC.Validate(); err != nil {
		t.Fatalf("dead NPC at 0 HP should be valid: %v", err)
	}

	invalid := valid
	invalid.Status = "zombie"
	if err := invalid.Validate(); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

// ---------------------------------------------------------------------------
// Death saving throws
// ---------------------------------------------------------------------------

func makeDeathSaveState() *CombatState {
	return &CombatState{
		ID:                     uuid.New(),
		CampaignID:             uuid.New(),
		Status:                 CombatStatusActive,
		TrackDeathSavingThrows: true,
	}
}

func TestRollDeathSavingThrowRequiresTrackingEnabled(t *testing.T) {
	state := makeDeathSaveState()
	state.TrackDeathSavingThrows = false
	c := makePlayer(0, 10)
	c.Status = CombatantStatusUnconscious
	if _, _, err := RollDeathSavingThrow(state, c, 10); err == nil {
		t.Fatal("expected error when TrackDeathSavingThrows is false")
	}
}

func TestRollDeathSavingThrowRequiresUnconsciousPlayer(t *testing.T) {
	state := makeDeathSaveState()
	player := makePlayer(5, 10)
	if _, _, err := RollDeathSavingThrow(state, player, 10); err == nil {
		t.Fatal("expected error for conscious player")
	}

	npc := makeNPC(0, 10)
	npc.Status = CombatantStatusUnconscious
	if _, _, err := RollDeathSavingThrow(state, npc, 10); err == nil {
		t.Fatal("expected error for NPC")
	}
}

func TestRollDeathSavingThrowInvalidRoll(t *testing.T) {
	state := makeDeathSaveState()
	c := makePlayer(0, 10)
	c.Status = CombatantStatusUnconscious
	for _, roll := range []int{0, -1, 21, 100} {
		if _, _, err := RollDeathSavingThrow(state, c, roll); err == nil {
			t.Fatalf("expected error for invalid roll %d", roll)
		}
	}
}

func TestRollDeathSavingThrowNatural20(t *testing.T) {
	state := makeDeathSaveState()
	c := makePlayer(0, 10)
	c.Status = CombatantStatusUnconscious
	stabilized, died, err := RollDeathSavingThrow(state, c, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !stabilized || died {
		t.Fatalf("natural 20: want stabilized=true, died=false; got %v, %v", stabilized, died)
	}
	if c.HP != 1 {
		t.Fatalf("HP = %d after natural 20, want 1", c.HP)
	}
	if c.Status != CombatantStatusAlive {
		t.Fatalf("Status = %q after natural 20, want alive", c.Status)
	}
	if c.DeathSavingThrows != nil {
		t.Fatal("DeathSavingThrows should be cleared after natural 20")
	}
}

func TestRollDeathSavingThrowNatural1CountsAsTwoFailures(t *testing.T) {
	state := makeDeathSaveState()
	c := makePlayer(0, 10)
	c.Status = CombatantStatusUnconscious
	stabilized, died, err := RollDeathSavingThrow(state, c, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stabilized || died {
		t.Fatalf("single natural 1 should not resolve; got stabilized=%v, died=%v", stabilized, died)
	}
	if c.DeathSavingThrows == nil || c.DeathSavingThrows.Failures != 2 {
		t.Fatalf("expected 2 failures from natural 1, got %+v", c.DeathSavingThrows)
	}
}

func TestRollDeathSavingThrowThreeSuccessesStabilize(t *testing.T) {
	state := makeDeathSaveState()
	c := makePlayer(0, 10)
	c.Status = CombatantStatusUnconscious

	for i := 0; i < 3; i++ {
		stabilized, died, err := RollDeathSavingThrow(state, c, 15)
		if err != nil {
			t.Fatalf("roll %d: unexpected error: %v", i+1, err)
		}
		if i < 2 && (stabilized || died) {
			t.Fatalf("roll %d: should not resolve yet; stabilized=%v, died=%v", i+1, stabilized, died)
		}
		if i == 2 && !stabilized {
			t.Fatal("3 successes should stabilize the combatant")
		}
	}
	if c.DeathSavingThrows != nil {
		t.Fatal("DeathSavingThrows should be cleared after stabilization")
	}
	// Stabilized at 0 HP, still unconscious.
	if c.Status != CombatantStatusUnconscious {
		t.Fatalf("Status = %q after stabilization, want unconscious", c.Status)
	}
}

func TestRollDeathSavingThrowThreeFailuresDie(t *testing.T) {
	state := makeDeathSaveState()
	c := makePlayer(0, 10)
	c.Status = CombatantStatusUnconscious

	for i := 0; i < 3; i++ {
		stabilized, died, err := RollDeathSavingThrow(state, c, 5)
		if err != nil {
			t.Fatalf("roll %d: unexpected error: %v", i+1, err)
		}
		if i < 2 && (stabilized || died) {
			t.Fatalf("roll %d: should not resolve yet; stabilized=%v, died=%v", i+1, stabilized, died)
		}
		if i == 2 && !died {
			t.Fatal("3 failures should kill the combatant")
		}
	}
	if c.Status != CombatantStatusDead {
		t.Fatalf("Status = %q after 3 failures, want dead", c.Status)
	}
	if c.DeathSavingThrows != nil {
		t.Fatal("DeathSavingThrows should be cleared after death")
	}
}

func TestRollDeathSavingThrowMixedResults(t *testing.T) {
	state := makeDeathSaveState()
	c := makePlayer(0, 10)
	c.Status = CombatantStatusUnconscious

	// 2 successes, 2 failures — not resolved yet.
	rolls := []int{15, 5, 15, 5}
	for i, roll := range rolls {
		stabilized, died, err := RollDeathSavingThrow(state, c, roll)
		if err != nil {
			t.Fatalf("roll %d: unexpected error: %v", i+1, err)
		}
		if stabilized || died {
			t.Fatalf("roll %d: should not resolve yet; stabilized=%v, died=%v", i+1, stabilized, died)
		}
	}
	if c.DeathSavingThrows == nil || c.DeathSavingThrows.Successes != 2 || c.DeathSavingThrows.Failures != 2 {
		t.Fatalf("unexpected saves state: %+v", c.DeathSavingThrows)
	}
}
