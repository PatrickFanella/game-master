package combat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makeTestPlayer(hp, maxHP int, stats json.RawMessage) Combatant {
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

func makeTestNPC(name string, hp, maxHP int, stats json.RawMessage) Combatant {
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

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return data
}

// ---------------------------------------------------------------------------
// InitiateCombat
// ---------------------------------------------------------------------------

func TestNarrativeResolver_InitiateCombat(t *testing.T) {
	stats := mustJSON(t, map[string]int{"dexterity": 14})
	player := makeTestPlayer(20, 20, stats)
	enemy := makeTestNPC("Goblin", 10, 10, stats)

	campaignID := uuid.New()
	roller := &fixedInitiativeRoller{rolls: []int{15, 10}}
	resolver := newNarrativeCombatResolverWithRoller(campaignID, roller)

	state, err := resolver.InitiateCombat(context.Background(),
		[]Combatant{player, enemy},
		Environment{Description: "Dark cave"},
	)
	if err != nil {
		t.Fatalf("InitiateCombat: %v", err)
	}

	if state.ID == uuid.Nil {
		t.Error("state ID should not be nil")
	}
	if state.CampaignID != campaignID {
		t.Errorf("campaign_id = %v, want %v", state.CampaignID, campaignID)
	}
	if state.Status != CombatStatusActive {
		t.Errorf("status = %q, want active", state.Status)
	}
	if len(state.InitiativeOrder) != 2 {
		t.Fatalf("initiative order length = %d, want 2", len(state.InitiativeOrder))
	}
	if state.RoundNumber != 0 {
		t.Errorf("round number = %d, want 0", state.RoundNumber)
	}
	if state.Environment.Description != "Dark cave" {
		t.Errorf("environment = %q, want %q", state.Environment.Description, "Dark cave")
	}
}

func TestNarrativeResolver_InitiateCombatNoCombatants(t *testing.T) {
	resolver := NewNarrativeCombatResolver(uuid.New())
	_, err := resolver.InitiateCombat(context.Background(), nil, Environment{})
	if err == nil {
		t.Fatal("expected error for empty combatants")
	}
}

func TestNarrativeResolver_InitiateCombatInvalidCombatant(t *testing.T) {
	resolver := NewNarrativeCombatResolver(uuid.New())
	invalid := Combatant{EntityID: uuid.Nil}
	_, err := resolver.InitiateCombat(context.Background(), []Combatant{invalid}, Environment{})
	if err == nil {
		t.Fatal("expected error for invalid combatant")
	}
}

// ---------------------------------------------------------------------------
// ProcessRound
// ---------------------------------------------------------------------------

func TestNarrativeResolver_ProcessRoundNilState(t *testing.T) {
	resolver := NewNarrativeCombatResolver(uuid.New())
	_, err := resolver.ProcessRound(context.Background(), PlayerAction{}, nil)
	if err == nil {
		t.Fatal("expected error for nil state")
	}
}

func TestNarrativeResolver_ProcessRoundInactiveState(t *testing.T) {
	resolver := NewNarrativeCombatResolver(uuid.New())
	state := &CombatState{Status: CombatStatusCompleted}
	_, err := resolver.ProcessRound(context.Background(), PlayerAction{}, state)
	if err == nil {
		t.Fatal("expected error for inactive state")
	}
}

func TestNarrativeResolver_ProcessRoundPlayerHits(t *testing.T) {
	playerStats := mustJSON(t, map[string]int{"strength": 14})
	enemyStats := mustJSON(t, map[string]int{"strength": 10})
	player := makeTestPlayer(20, 20, playerStats)
	enemy := makeTestNPC("Goblin", 10, 10, enemyStats)

	// Rolls: 2 initiative (RollD20), then 1 player check (hit), 1 enemy check (miss)
	roller := &fixedInitiativeRoller{rolls: []int{10, 8, 15, 5}}
	resolver := newNarrativeCombatResolverWithRoller(uuid.New(), roller)

	state, err := resolver.InitiateCombat(context.Background(),
		[]Combatant{player, enemy},
		Environment{Description: "Forest"},
	)
	if err != nil {
		t.Fatalf("InitiateCombat: %v", err)
	}

	enemyID := enemy.EntityID
	details := mustJSON(t, actionDetails{
		Skill:       "strength",
		Difficulty:  10,
		DamageOnHit: 5,
		DamageType:  "slashing",
	})
	action := PlayerAction{
		CombatantID: player.EntityID,
		ActionType:  ActionTypeAttack,
		TargetID:    &enemyID,
		Description: "Swings sword at Goblin",
		Details:     details,
	}

	result, err := resolver.ProcessRound(context.Background(), action, state)
	if err != nil {
		t.Fatalf("ProcessRound: %v", err)
	}

	if result.RoundNumber != 1 {
		t.Errorf("round = %d, want 1", result.RoundNumber)
	}
	if len(result.ActionsTaken) < 1 {
		t.Fatal("expected at least one action")
	}

	// Player hit with 5 damage → enemy HP should be 5
	enemyCombatant := combatantByEntityID(state, enemy.EntityID)
	if enemyCombatant == nil {
		t.Fatal("enemy combatant not found")
	}
	if enemyCombatant.HP != 5 {
		t.Errorf("enemy HP = %d, want 5", enemyCombatant.HP)
	}
}

func TestNarrativeResolver_ProcessRoundSurprise(t *testing.T) {
	stats := mustJSON(t, map[string]int{"strength": 10})
	player := makeTestPlayer(20, 20, stats)
	enemy := makeTestNPC("Goblin", 10, 10, stats)
	enemy.Surprised = true

	roller := &fixedInitiativeRoller{rolls: []int{10, 8, 15}}
	resolver := newNarrativeCombatResolverWithRoller(uuid.New(), roller)

	state, err := resolver.InitiateCombat(context.Background(),
		[]Combatant{player, enemy},
		Environment{Description: "Ambush"},
	)
	if err != nil {
		t.Fatalf("InitiateCombat: %v", err)
	}

	action := PlayerAction{
		CombatantID: player.EntityID,
		ActionType:  ActionTypeAttack,
		Description: "Strikes from hiding",
		Details:     mustJSON(t, actionDetails{Skill: "strength", Difficulty: 10}),
	}

	result, err := resolver.ProcessRound(context.Background(), action, state)
	if err != nil {
		t.Fatalf("ProcessRound: %v", err)
	}

	// The surprised enemy should not have acted.
	for _, a := range result.ActionsTaken {
		if a.ActorID == enemy.EntityID {
			t.Error("surprised enemy should not have acted")
		}
	}
	if !state.SurpriseRoundActive {
		t.Error("surprise round should be active in round 1")
	}
}

func TestNarrativeResolver_ProcessRoundClearsStaleSurprise(t *testing.T) {
	stats := mustJSON(t, map[string]int{"strength": 10})
	player := makeTestPlayer(20, 20, stats)
	enemy := makeTestNPC("Goblin", 10, 10, stats)
	// No combatants are surprised.

	roller := &fixedInitiativeRoller{rolls: []int{10, 8, 15, 5}}
	resolver := newNarrativeCombatResolverWithRoller(uuid.New(), roller)

	state, err := resolver.InitiateCombat(context.Background(),
		[]Combatant{player, enemy},
		Environment{Description: "Open field"},
	)
	if err != nil {
		t.Fatalf("InitiateCombat: %v", err)
	}

	// Simulate a stale SurpriseRoundActive value from deserialization.
	state.SurpriseRoundActive = true

	action := PlayerAction{
		CombatantID: player.EntityID,
		ActionType:  ActionTypeAttack,
		Description: "Attacks",
		Details:     mustJSON(t, actionDetails{Skill: "strength", Difficulty: 10}),
	}

	_, err = resolver.ProcessRound(context.Background(), action, state)
	if err != nil {
		t.Fatalf("ProcessRound: %v", err)
	}

	// SurpriseRoundActive should be cleared since no combatants are surprised.
	if state.SurpriseRoundActive {
		t.Error("SurpriseRoundActive should be false when no combatants are surprised")
	}
}

// ---------------------------------------------------------------------------
// ResolveCombat
// ---------------------------------------------------------------------------

func TestNarrativeResolver_ResolveCombatNilState(t *testing.T) {
	resolver := NewNarrativeCombatResolver(uuid.New())
	_, err := resolver.ResolveCombat(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil state")
	}
}

func TestNarrativeResolver_ResolveCombatPlayerVictory(t *testing.T) {
	player := makeTestPlayer(10, 20, nil)
	enemy := makeTestNPC("Goblin", 0, 10, nil)
	enemy.Status = CombatantStatusDead

	state := baseCombatState(player, enemy)
	state.Status = CombatStatusCompleted

	resolver := NewNarrativeCombatResolver(uuid.New())
	outcome, err := resolver.ResolveCombat(context.Background(), state)
	if err != nil {
		t.Fatalf("ResolveCombat: %v", err)
	}

	if outcome.Winner == nil {
		t.Fatal("expected a winner")
	}
	if *outcome.Winner != player.EntityID {
		t.Errorf("winner = %v, want %v", *outcome.Winner, player.EntityID)
	}
	if len(outcome.Casualties) != 1 {
		t.Fatalf("casualties = %d, want 1", len(outcome.Casualties))
	}
	if outcome.Casualties[0] != enemy.EntityID {
		t.Errorf("casualty = %v, want %v", outcome.Casualties[0], enemy.EntityID)
	}
	if outcome.XPEarned != enemy.MaxHP {
		t.Errorf("xp = %d, want %d", outcome.XPEarned, enemy.MaxHP)
	}
}

func TestNarrativeResolver_ResolveCombatPlayerDefeat(t *testing.T) {
	player := makeTestPlayer(0, 20, nil)
	player.Status = CombatantStatusDead
	enemy := makeTestNPC("Goblin", 5, 10, nil)

	state := baseCombatState(player, enemy)

	resolver := NewNarrativeCombatResolver(uuid.New())
	outcome, err := resolver.ResolveCombat(context.Background(), state)
	if err != nil {
		t.Fatalf("ResolveCombat: %v", err)
	}

	if outcome.Winner == nil {
		t.Fatal("expected a winner")
	}
	if *outcome.Winner != enemy.EntityID {
		t.Errorf("winner = %v, want %v", *outcome.Winner, enemy.EntityID)
	}
	if outcome.XPEarned != 0 {
		t.Errorf("xp = %d, want 0 (no dead NPCs)", outcome.XPEarned)
	}
}

// ---------------------------------------------------------------------------
// parseActionDetails
// ---------------------------------------------------------------------------

func TestParseActionDetails_Defaults(t *testing.T) {
	ad := parseActionDetails(nil)
	if ad.Skill != defaultActionSkill {
		t.Errorf("skill = %q, want %q", ad.Skill, defaultActionSkill)
	}
	if ad.Difficulty != defaultActionDC {
		t.Errorf("difficulty = %d, want %d", ad.Difficulty, defaultActionDC)
	}
	if ad.DamageType != defaultDamageType {
		t.Errorf("damage_type = %q, want %q", ad.DamageType, defaultDamageType)
	}
}

func TestParseActionDetails_Custom(t *testing.T) {
	details := mustJSON(t, actionDetails{
		Skill:       "dexterity",
		Difficulty:  15,
		DamageOnHit: 8,
		DamageType:  "fire",
	})
	ad := parseActionDetails(details)
	if ad.Skill != "dexterity" {
		t.Errorf("skill = %q, want dexterity", ad.Skill)
	}
	if ad.Difficulty != 15 {
		t.Errorf("difficulty = %d, want 15", ad.Difficulty)
	}
	if ad.DamageOnHit != 8 {
		t.Errorf("damage_on_hit = %d, want 8", ad.DamageOnHit)
	}
	if ad.DamageType != "fire" {
		t.Errorf("damage_type = %q, want fire", ad.DamageType)
	}
}

func TestParseActionDetails_InvalidJSON(t *testing.T) {
	ad := parseActionDetails(json.RawMessage(`{invalid`))
	if ad.Skill != defaultActionSkill {
		t.Errorf("expected defaults on invalid JSON, got skill = %q", ad.Skill)
	}
}

// ---------------------------------------------------------------------------
// resolverStatModifier
// ---------------------------------------------------------------------------

func TestResolverStatModifier(t *testing.T) {
	tests := []struct {
		name     string
		stats    json.RawMessage
		skill    string
		expected int
	}{
		{"nil stats", nil, "strength", 0},
		{"empty stats", json.RawMessage(`{}`), "strength", 0},
		{"strength 14", mustJSON(t, map[string]int{"strength": 14}), "strength", 2},
		{"strength 10", mustJSON(t, map[string]int{"strength": 10}), "strength", 0},
		{"strength 8", mustJSON(t, map[string]int{"strength": 8}), "strength", -1},
		{"case insensitive", mustJSON(t, map[string]int{"Strength": 16}), "strength", 3},
		{"missing skill", mustJSON(t, map[string]int{"dexterity": 14}), "strength", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Combatant{Stats: tt.stats}
			got := resolverStatModifier(c, tt.skill)
			if got != tt.expected {
				t.Errorf("resolverStatModifier = %d, want %d", got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Integration test: 3-round combat to completion
// ---------------------------------------------------------------------------

func TestNarrativeResolver_ThreeRoundCombat(t *testing.T) {
	// Set up combatants.
	// Player: strength 14 → +2 modifier, HP 20
	// Enemy:  strength 12 → +1 modifier, HP 15
	playerStats := mustJSON(t, map[string]int{"strength": 14, "dexterity": 12})
	enemyStats := mustJSON(t, map[string]int{"strength": 12, "dexterity": 10})

	player := makeTestPlayer(20, 20, playerStats)
	enemy := makeTestNPC("Orc", 15, 15, enemyStats)

	// Dice rolls in order (RollD20 calls via the rolls array):
	// InitiateCombat:
	//   initiative: RollD20 for player=10, RollD20 for enemy=8
	//   (tie-break uses Intn which reads from the ties array, defaults to 0)
	// ProcessRound 1 (round 0→1, skips initiative re-roll):
	//   player check: RollD20=15 → 15+2=17 >= DC 10 → Hit
	//   enemy check:  RollD20=5  → 5+1=6   < DC 10 → Miss
	// ProcessRound 2 (round 1→2, no re-roll):
	//   player check: RollD20=15 → 15+2=17 >= DC 10 → Hit
	//   enemy check:  RollD20=15 → 15+1=16 >= DC 10 → Hit
	// ProcessRound 3 (round 2→3, no re-roll):
	//   player check: RollD20=15 → 15+2=17 >= DC 10 → Hit → enemy HP=0, dead
	//   (enemy dead, no check)
	roller := &fixedInitiativeRoller{
		rolls: []int{10, 8, 15, 5, 15, 15, 15},
	}
	resolver := newNarrativeCombatResolverWithRoller(uuid.New(), roller)
	ctx := context.Background()

	// --- InitiateCombat ---
	state, err := resolver.InitiateCombat(ctx,
		[]Combatant{player, enemy},
		Environment{Description: "Rocky hillside"},
	)
	if err != nil {
		t.Fatalf("InitiateCombat: %v", err)
	}
	if len(state.InitiativeOrder) != 2 {
		t.Fatalf("initiative order len = %d, want 2", len(state.InitiativeOrder))
	}
	if state.RoundNumber != 0 {
		t.Fatalf("round = %d, want 0 after InitiateCombat", state.RoundNumber)
	}

	enemyID := enemy.EntityID
	details := mustJSON(t, actionDetails{
		Skill:       "strength",
		Difficulty:  10,
		DamageOnHit: 5,
		DamageType:  "slashing",
	})
	action := PlayerAction{
		CombatantID: player.EntityID,
		ActionType:  ActionTypeAttack,
		TargetID:    &enemyID,
		Description: "Slashes the orc",
		Details:     details,
	}

	// --- Round 1 ---
	r1, err := resolver.ProcessRound(ctx, action, state)
	if err != nil {
		t.Fatalf("Round 1: %v", err)
	}
	if r1.RoundNumber != 1 {
		t.Errorf("round = %d, want 1", r1.RoundNumber)
	}
	enemyComb := combatantByEntityID(state, enemy.EntityID)
	playerComb := combatantByEntityID(state, player.EntityID)
	if enemyComb.HP != 10 {
		t.Errorf("after round 1: enemy HP = %d, want 10", enemyComb.HP)
	}
	if playerComb.HP != 20 {
		t.Errorf("after round 1: player HP = %d, want 20 (enemy missed)", playerComb.HP)
	}
	if state.Status != CombatStatusActive {
		t.Errorf("after round 1: status = %q, want active", state.Status)
	}

	// --- Round 2 ---
	r2, err := resolver.ProcessRound(ctx, action, state)
	if err != nil {
		t.Fatalf("Round 2: %v", err)
	}
	if r2.RoundNumber != 2 {
		t.Errorf("round = %d, want 2", r2.RoundNumber)
	}
	if enemyComb.HP != 5 {
		t.Errorf("after round 2: enemy HP = %d, want 5", enemyComb.HP)
	}
	// Enemy hit with defaultNPCDamageOnHit (3).
	if playerComb.HP != 17 {
		t.Errorf("after round 2: player HP = %d, want 17", playerComb.HP)
	}
	if state.Status != CombatStatusActive {
		t.Errorf("after round 2: status = %q, want active", state.Status)
	}

	// --- Round 3 ---
	r3, err := resolver.ProcessRound(ctx, action, state)
	if err != nil {
		t.Fatalf("Round 3: %v", err)
	}
	if r3.RoundNumber != 3 {
		t.Errorf("round = %d, want 3", r3.RoundNumber)
	}
	if enemyComb.HP != 0 {
		t.Errorf("after round 3: enemy HP = %d, want 0", enemyComb.HP)
	}
	if enemyComb.Status != CombatantStatusDead {
		t.Errorf("after round 3: enemy status = %q, want dead", enemyComb.Status)
	}
	if state.Status != CombatStatusCompleted {
		t.Errorf("after round 3: combat status = %q, want completed", state.Status)
	}

	// --- ResolveCombat ---
	outcome, err := resolver.ResolveCombat(ctx, state)
	if err != nil {
		t.Fatalf("ResolveCombat: %v", err)
	}

	if outcome.Winner == nil {
		t.Fatal("expected a winner")
	}
	if *outcome.Winner != player.EntityID {
		t.Errorf("winner = %v, want player %v", *outcome.Winner, player.EntityID)
	}
	if len(outcome.Casualties) != 1 {
		t.Fatalf("casualties = %d, want 1", len(outcome.Casualties))
	}
	if outcome.Casualties[0] != enemy.EntityID {
		t.Errorf("casualty = %v, want enemy %v", outcome.Casualties[0], enemy.EntityID)
	}
	if outcome.XPEarned != 15 {
		t.Errorf("xp = %d, want 15 (enemy MaxHP)", outcome.XPEarned)
	}
	if outcome.Narrative == "" {
		t.Error("expected non-empty narrative")
	}

	// Verify narratives from each round are non-empty.
	for i, rr := range []*RoundResult{r1, r2, r3} {
		if rr.Narrative == "" {
			t.Errorf("round %d narrative is empty", i+1)
		}
		if rr.UpdatedState == nil {
			t.Errorf("round %d UpdatedState is nil", i+1)
		}
	}
}

// ---------------------------------------------------------------------------
// Interface swappability
// ---------------------------------------------------------------------------

func TestNarrativeResolver_ImplementsInterface(t *testing.T) {
	// Verify that NarrativeCombatResolver can be assigned to a CombatResolver
	// variable, confirming interface compliance.
	var _ CombatResolver = NewNarrativeCombatResolver(uuid.New())
}
