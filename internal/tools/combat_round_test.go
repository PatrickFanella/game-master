package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/combat"
)

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func TestRegisterCombatRound(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterCombatRound(reg, nil); err != nil {
		t.Fatalf("register combat_round: %v", err)
	}

	registered := reg.List()
	if len(registered) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(registered))
	}
	if registered[0].Name != combatRoundToolName {
		t.Fatalf("tool name = %q, want %q", registered[0].Name, combatRoundToolName)
	}
	required, ok := registered[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", registered[0].Parameters["required"])
	}
	if len(required) != 6 {
		t.Fatalf("required field count = %d, want 6", len(required))
	}
}

// ---------------------------------------------------------------------------
// Helpers for building test fixtures
// ---------------------------------------------------------------------------

func baseCombatStateArgs(playerID, enemyID uuid.UUID) map[string]any {
	return map[string]any{
		"id":          uuid.New().String(),
		"campaign_id": uuid.New().String(),
		"round_number": 0,
		"combatants": []any{
			map[string]any{
				"entity_id":   playerID.String(),
				"entity_type": "player",
				"name":        "Aria",
				"hp":          24,
				"max_hp":      30,
				"stats":       map[string]any{"strength": float64(16), "dexterity": float64(14)},
				"conditions":  []any{},
				"initiative":  12,
				"status":      "alive",
				"surprised":   false,
			},
			map[string]any{
				"entity_id":   enemyID.String(),
				"entity_type": "npc",
				"name":        "Goblin Scout",
				"hp":          9,
				"max_hp":      9,
				"stats":       map[string]any{"dexterity": float64(14)},
				"conditions":  []any{},
				"initiative":  10,
				"status":      "alive",
				"surprised":   false,
			},
		},
		"initiative_order":            []any{playerID.String(), enemyID.String()},
		"environment":                 map[string]any{"description": "Dense forest trail"},
		"status":                      "active",
		"initiative_reroll_each_round": false,
		"track_death_saving_throws":   false,
		"surprise_round_active":       false,
	}
}

func defaultRoundArgs(playerID, enemyID uuid.UUID) map[string]any {
	return map[string]any{
		"player_action": "I swing my sword at the goblin",
		"action_type":   "attack",
		"target_id":     enemyID.String(),
		"skill":         "strength",
		"difficulty":    13,
		"damage_on_hit": 8,
		"damage_type":   "slashing",
		"enemy_actions": []any{
			map[string]any{
				"enemy_id":     enemyID.String(),
				"action_type":  "attack",
				"target_id":    playerID.String(),
				"description":  "The goblin thrusts with its spear",
				"skill":        "dexterity",
				"difficulty":   14,
				"damage_on_hit": 5,
				"damage_type":  "piercing",
			},
		},
		"combat_state": baseCombatStateArgs(playerID, enemyID),
	}
}

// ---------------------------------------------------------------------------
// Player hits and damages enemy
// ---------------------------------------------------------------------------

func TestCombatRoundPlayerHitEnemyDamaged(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()

	// Rolls: player d20=15 (hit DC13), enemy d20=8 (miss DC14)
	roller := &stubRoller{rolls: []int{14, 7}}
	h := NewCombatRoundHandler(roller)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	result, err := h.Handle(ctx, defaultRoundArgs(playerID, enemyID))
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success=true")
	}

	// Verify round number advanced.
	rn, ok := result.Data["round_number"].(int)
	if !ok || rn != 1 {
		t.Fatalf("round_number = %v, want 1", result.Data["round_number"])
	}

	// Verify damage was dealt to enemy.
	combatants, ok := result.Data["combatants"].([]map[string]any)
	if !ok {
		t.Fatalf("combatants type = %T", result.Data["combatants"])
	}
	var enemyHP int
	for _, c := range combatants {
		if c["entity_id"] == enemyID.String() {
			enemyHP, _ = c["hp"].(int)
		}
	}
	if enemyHP != 1 { // 9 - 8 = 1
		t.Fatalf("enemy HP = %d, want 1", enemyHP)
	}

	// Verify player HP unchanged (enemy missed).
	var playerHP int
	for _, c := range combatants {
		if c["entity_id"] == playerID.String() {
			playerHP, _ = c["hp"].(int)
		}
	}
	if playerHP != 24 {
		t.Fatalf("player HP = %d, want 24", playerHP)
	}

	// Verify narrative contains hit/miss info.
	if !strings.Contains(result.Narrative, "Hit!") {
		t.Fatalf("narrative should contain Hit!, got %q", result.Narrative)
	}
	if !strings.Contains(result.Narrative, "Miss!") {
		t.Fatalf("narrative should contain Miss!, got %q", result.Narrative)
	}

	// Combat should not be over.
	if co, _ := result.Data["combat_over"].(bool); co {
		t.Fatal("combat_over should be false")
	}
}

// ---------------------------------------------------------------------------
// Player misses
// ---------------------------------------------------------------------------

func TestCombatRoundPlayerMiss(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()

	// Rolls: player d20=5 (miss DC13), enemy d20=3 (miss DC14)
	roller := &stubRoller{rolls: []int{4, 2}}
	h := NewCombatRoundHandler(roller)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	result, err := h.Handle(ctx, defaultRoundArgs(playerID, enemyID))
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	// Verify no damage dealt.
	dmg, ok := result.Data["damage_dealt"].([]map[string]any)
	if !ok {
		t.Fatalf("damage_dealt type = %T", result.Data["damage_dealt"])
	}
	if len(dmg) != 0 {
		t.Fatalf("damage_dealt count = %d, want 0", len(dmg))
	}
}

// ---------------------------------------------------------------------------
// Enemy killed and removed from initiative
// ---------------------------------------------------------------------------

func TestCombatRoundEnemyKilled(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()

	// Enemy has only 5 HP, damage_on_hit=8 → killed.
	args := defaultRoundArgs(playerID, enemyID)
	state := args["combat_state"].(map[string]any)
	combatants := state["combatants"].([]any)
	enemyMap := combatants[1].(map[string]any)
	enemyMap["hp"] = 5
	enemyMap["max_hp"] = 5

	// Rolls: player d20=18 (hit), enemy d20 not needed (dead before acting).
	roller := &stubRoller{rolls: []int{17, 2}}
	h := NewCombatRoundHandler(roller)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	result, err := h.Handle(ctx, args)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	// Enemy should be dead.
	combatantsOut, ok := result.Data["combatants"].([]map[string]any)
	if !ok {
		t.Fatalf("combatants type = %T", result.Data["combatants"])
	}
	for _, c := range combatantsOut {
		if c["entity_id"] == enemyID.String() {
			if c["status"] != "dead" {
				t.Fatalf("enemy status = %q, want dead", c["status"])
			}
			if hp, _ := c["hp"].(int); hp != 0 {
				t.Fatalf("enemy HP = %d, want 0", hp)
			}
		}
	}

	// Initiative order should not include the dead enemy.
	updatedState, ok := result.Data["combat_state"].(map[string]any)
	if !ok {
		t.Fatalf("combat_state type = %T", result.Data["combat_state"])
	}
	initOrder, ok := updatedState["initiative_order"].([]string)
	if !ok {
		t.Fatalf("initiative_order type = %T", updatedState["initiative_order"])
	}
	for _, id := range initOrder {
		if id == enemyID.String() {
			t.Fatal("dead enemy should be removed from initiative_order")
		}
	}

	// Combat should be over.
	if co, _ := result.Data["combat_over"].(bool); !co {
		t.Fatal("combat_over should be true when all enemies are dead")
	}

	if !strings.Contains(result.Narrative, "defeated") {
		t.Fatalf("narrative should mention defeat, got %q", result.Narrative)
	}
}

// ---------------------------------------------------------------------------
// Stunned combatant skips turn
// ---------------------------------------------------------------------------

func TestCombatRoundStunnedCombatantSkipsTurn(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()

	args := defaultRoundArgs(playerID, enemyID)
	state := args["combat_state"].(map[string]any)
	combatants := state["combatants"].([]any)

	// Stun the enemy.
	enemyMap := combatants[1].(map[string]any)
	enemyMap["conditions"] = []any{
		map[string]any{"name": "stunned", "duration_rounds": 2},
	}

	// Roll: player d20=15 (hit), enemy is stunned so no roll needed.
	roller := &stubRoller{rolls: []int{14}}
	h := NewCombatRoundHandler(roller)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	result, err := h.Handle(ctx, args)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if !strings.Contains(result.Narrative, "unable to act") {
		t.Fatalf("narrative should indicate stunned combatant skipped, got %q", result.Narrative)
	}
}

// ---------------------------------------------------------------------------
// Nil handler
// ---------------------------------------------------------------------------

func TestCombatRoundNilHandler(t *testing.T) {
	var h *CombatRoundHandler
	_, err := h.Handle(context.Background(), map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "handler is nil") {
		t.Fatalf("expected nil handler error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Argument validation
// ---------------------------------------------------------------------------

func TestCombatRoundValidation(t *testing.T) {
	roller := &stubRoller{rolls: []int{10}}
	h := NewCombatRoundHandler(roller)
	ctx := context.Background()

	// Missing player_action.
	_, err := h.Handle(ctx, map[string]any{
		"action_type":   "attack",
		"skill":         "strength",
		"difficulty":    13,
		"enemy_actions": []any{},
		"combat_state":  baseCombatStateArgs(uuid.New(), uuid.New()),
	})
	if err == nil || !strings.Contains(err.Error(), "player_action") {
		t.Fatalf("expected player_action validation error, got %v", err)
	}

	// Missing combat_state.
	_, err = h.Handle(ctx, map[string]any{
		"player_action": "attack",
		"action_type":   "attack",
		"skill":         "strength",
		"difficulty":    13,
		"enemy_actions": []any{},
	})
	if err == nil || !strings.Contains(err.Error(), "combat_state") {
		t.Fatalf("expected combat_state validation error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Stat modifier extraction
// ---------------------------------------------------------------------------

func TestCombatantStatModifier(t *testing.T) {
	cases := []struct {
		name  string
		stats json.RawMessage
		skill string
		want  int
	}{
		{"strength 16 → +3", json.RawMessage(`{"strength":16}`), "strength", 3},
		{"dexterity 14 → +2", json.RawMessage(`{"dexterity":14}`), "dexterity", 2},
		{"dexterity 8 → -1", json.RawMessage(`{"dexterity":8}`), "dexterity", -1},
		{"missing stat → 0", json.RawMessage(`{"strength":16}`), "wisdom", 0},
		{"empty stats → 0", json.RawMessage(`{}`), "strength", 0},
		{"nil stats → 0", nil, "strength", 0},
		{"case insensitive", json.RawMessage(`{"Strength":18}`), "strength", 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &combat.Combatant{Stats: tc.stats}
			got := combatantStatModifier(c, tc.skill)
			if got != tc.want {
				t.Fatalf("combatantStatModifier = %d, want %d", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Combat state round-trip serialization
// ---------------------------------------------------------------------------

func TestCombatStateRoundTrip(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()
	locationID := uuid.New()

	stateArgs := baseCombatStateArgs(playerID, enemyID)
	// Set extra fields to verify lossless round-trip.
	stateArgs["initiative_reroll_each_round"] = true
	stateArgs["track_death_saving_throws"] = true
	stateArgs["surprise_round_active"] = true
	stateArgs["active_effects"] = map[string]any{"fog": true}
	stateArgs["environment"] = map[string]any{
		"description": "Dense forest trail",
		"location_id": locationID.String(),
		"properties":  map[string]any{"terrain": "forest"},
	}
	// Mark first combatant as surprised with death saving throws.
	combatantsSlice := stateArgs["combatants"].([]any)
	playerMap := combatantsSlice[0].(map[string]any)
	playerMap["surprised"] = true
	playerMap["death_saving_throws"] = map[string]any{"successes": 1, "failures": 2}
	enemyMap := combatantsSlice[1].(map[string]any)
	enemyMap["surprised"] = true

	args := map[string]any{"combat_state": stateArgs}

	state, err := parseCombatStateArg(args, "combat_state")
	if err != nil {
		t.Fatalf("parseCombatStateArg: %v", err)
	}
	if len(state.Combatants) != 2 {
		t.Fatalf("combatant count = %d, want 2", len(state.Combatants))
	}
	if state.Combatants[0].Name != "Aria" {
		t.Fatalf("first combatant name = %q, want Aria", state.Combatants[0].Name)
	}
	if state.RoundNumber != 0 {
		t.Fatalf("round_number = %d, want 0", state.RoundNumber)
	}
	if !state.InitiativeRerollEachRound {
		t.Fatal("InitiativeRerollEachRound should be true")
	}
	if !state.TrackDeathSavingThrows {
		t.Fatal("TrackDeathSavingThrows should be true")
	}
	if !state.SurpriseRoundActive {
		t.Fatal("SurpriseRoundActive should be true")
	}
	if state.ActiveEffects == nil {
		t.Fatal("ActiveEffects should not be nil")
	}
	if state.Environment.LocationID == nil || *state.Environment.LocationID != locationID {
		t.Fatalf("Environment.LocationID = %v, want %s", state.Environment.LocationID, locationID)
	}
	if state.Environment.Properties == nil {
		t.Fatal("Environment.Properties should not be nil")
	}
	if !state.Combatants[0].Surprised {
		t.Fatal("player combatant should be surprised")
	}
	if state.Combatants[0].DeathSavingThrows == nil {
		t.Fatal("player DeathSavingThrows should not be nil")
	}
	if state.Combatants[0].DeathSavingThrows.Successes != 1 {
		t.Fatalf("DeathSavingThrows.Successes = %d, want 1", state.Combatants[0].DeathSavingThrows.Successes)
	}
	if state.Combatants[0].DeathSavingThrows.Failures != 2 {
		t.Fatalf("DeathSavingThrows.Failures = %d, want 2", state.Combatants[0].DeathSavingThrows.Failures)
	}
	if !state.Combatants[1].Surprised {
		t.Fatal("enemy combatant should be surprised")
	}

	// Round-trip to map and back.
	m := combatStateToMap(state)
	state2, err := parseCombatStateArg(map[string]any{"s": m}, "s")
	if err != nil {
		t.Fatalf("round-trip parse: %v", err)
	}
	if state2.Combatants[0].EntityID != playerID {
		t.Fatalf("round-trip player ID mismatch")
	}
	if !state2.InitiativeRerollEachRound {
		t.Fatal("round-trip: InitiativeRerollEachRound lost")
	}
	if !state2.TrackDeathSavingThrows {
		t.Fatal("round-trip: TrackDeathSavingThrows lost")
	}
	if !state2.SurpriseRoundActive {
		t.Fatal("round-trip: SurpriseRoundActive lost")
	}
	if state2.ActiveEffects == nil {
		t.Fatal("round-trip: ActiveEffects lost")
	}
	if state2.Environment.LocationID == nil || *state2.Environment.LocationID != locationID {
		t.Fatal("round-trip: Environment.LocationID lost")
	}
	if state2.Environment.Properties == nil {
		t.Fatal("round-trip: Environment.Properties lost")
	}
	if !state2.Combatants[0].Surprised {
		t.Fatal("round-trip: player surprised lost")
	}
	if state2.Combatants[0].DeathSavingThrows == nil || state2.Combatants[0].DeathSavingThrows.Successes != 1 {
		t.Fatal("round-trip: player DeathSavingThrows lost")
	}
	if !state2.Combatants[1].Surprised {
		t.Fatal("round-trip: enemy surprised lost")
	}
}

// ---------------------------------------------------------------------------
// Empty enemy actions
// ---------------------------------------------------------------------------

func TestCombatRoundNoEnemyActions(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()

	args := defaultRoundArgs(playerID, enemyID)
	args["enemy_actions"] = []any{}

	// Roll: player d20=10 + mod 3 = 13 >= DC13 → hit.
	roller := &stubRoller{rolls: []int{9}} // Intn(20)=9 → d20=10
	h := NewCombatRoundHandler(roller)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	result, err := h.Handle(ctx, args)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success=true")
	}
	if result.Data["round_number"].(int) != 1 {
		t.Fatalf("round_number = %v, want 1", result.Data["round_number"])
	}
}

// ---------------------------------------------------------------------------
// Critical success (natural 20) and critical failure (natural 1)
// ---------------------------------------------------------------------------

func TestCombatRoundCriticalSuccessAndFailure(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()

	// Natural 20 always hits even against very high DC.
	args := defaultRoundArgs(playerID, enemyID)
	args["difficulty"] = 99
	args["enemy_actions"] = []any{}

	roller := &stubRoller{rolls: []int{19}} // natural 20 (roller returns 19, d20 = 19+1 = 20)
	h := NewCombatRoundHandler(roller)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	result, err := h.Handle(ctx, args)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !strings.Contains(result.Narrative, "Hit!") {
		t.Fatalf("natural 20 should be a hit, got %q", result.Narrative)
	}

	// Natural 1 always misses even against very low DC.
	args["difficulty"] = 1
	roller = &stubRoller{rolls: []int{0}} // d20 = 1 (Intn(20)=0 → 0+1=1)
	h = NewCombatRoundHandler(roller)
	result, err = h.Handle(ctx, args)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !strings.Contains(result.Narrative, "Miss!") {
		t.Fatalf("natural 1 should be a miss, got %q", result.Narrative)
	}
}

// ---------------------------------------------------------------------------
// Surprised combatants skip during surprise round
// ---------------------------------------------------------------------------

func TestCombatRoundSurprisedCombatantSkips(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()

	args := defaultRoundArgs(playerID, enemyID)
	state := args["combat_state"].(map[string]any)
	// Mark the enemy as surprised — this is round 0→1.
	combatantSlice := state["combatants"].([]any)
	enemyMap := combatantSlice[1].(map[string]any)
	enemyMap["surprised"] = true

	// Roll: player d20=15 (hit).
	roller := &stubRoller{rolls: []int{14}}
	h := NewCombatRoundHandler(roller)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	result, err := h.Handle(ctx, args)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if !strings.Contains(result.Narrative, "surprised") {
		t.Fatalf("narrative should indicate surprised combatant skipped, got %q", result.Narrative)
	}
}

// ---------------------------------------------------------------------------
// Damage without target_id validation
// ---------------------------------------------------------------------------

func TestCombatRoundDamageRequiresTargetID(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()

	roller := &stubRoller{rolls: []int{14}}
	h := NewCombatRoundHandler(roller)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	// Player damage_on_hit > 0 but no target_id.
	args := defaultRoundArgs(playerID, enemyID)
	delete(args, "target_id")
	args["damage_on_hit"] = 8

	_, err := h.Handle(ctx, args)
	if err == nil || !strings.Contains(err.Error(), "target_id is required") {
		t.Fatalf("expected target_id validation error, got %v", err)
	}

	// Enemy damage_on_hit > 0 but no target_id.
	args2 := defaultRoundArgs(playerID, enemyID)
	args2["damage_on_hit"] = 0
	args2["enemy_actions"] = []any{
		map[string]any{
			"enemy_id":     enemyID.String(),
			"action_type":  "attack",
			"description":  "swings sword",
			"skill":        "strength",
			"difficulty":   12,
			"damage_on_hit": 5,
			// no target_id
		},
	}
	_, err = h.Handle(ctx, args2)
	if err == nil || !strings.Contains(err.Error(), "target_id is required") {
		t.Fatalf("expected enemy target_id validation error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Round 1 preserves existing initiative order
// ---------------------------------------------------------------------------

func TestCombatRoundPreservesInitiativeOnRound1(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()

	args := defaultRoundArgs(playerID, enemyID)
	state := args["combat_state"].(map[string]any)
	// Set initiative: enemy first, then player.
	state["initiative_order"] = []any{enemyID.String(), playerID.String()}

	// Rolls: player d20=5 (miss), enemy d20=5 (miss).
	roller := &stubRoller{rolls: []int{4, 4}}
	h := NewCombatRoundHandler(roller)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	result, err := h.Handle(ctx, args)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	updatedState, ok := result.Data["combat_state"].(map[string]any)
	if !ok {
		t.Fatalf("combat_state type = %T", result.Data["combat_state"])
	}
	initOrder, ok := updatedState["initiative_order"].([]string)
	if !ok {
		t.Fatalf("initiative_order type = %T", updatedState["initiative_order"])
	}
	// Initiative should be preserved (enemy first, then player).
	if len(initOrder) != 2 || initOrder[0] != enemyID.String() || initOrder[1] != playerID.String() {
		t.Fatalf("initiative order should be preserved, got %v", initOrder)
	}
}
