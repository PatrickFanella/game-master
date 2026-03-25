package combat

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestCombatantValidate(t *testing.T) {
	valid := Combatant{
		EntityID:   uuid.New(),
		EntityType: CombatantTypePlayer,
		Name:       "Hero",
		HP:         10,
		MaxHP:      10,
		Initiative: 5,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid combatant: %v", err)
	}

	tests := []struct {
		name string
		mod  func(*Combatant)
	}{
		{"nil entity_id", func(c *Combatant) { c.EntityID = uuid.Nil }},
		{"invalid entity_type", func(c *Combatant) { c.EntityType = "monster" }},
		{"empty name", func(c *Combatant) { c.Name = "" }},
		{"zero max_hp", func(c *Combatant) { c.MaxHP = 0 }},
		{"negative max_hp", func(c *Combatant) { c.MaxHP = -1 }},
		{"negative hp", func(c *Combatant) { c.HP = -1 }},
		{"hp exceeds max_hp", func(c *Combatant) { c.HP = 11 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := valid
			tt.mod(&c)
			if err := c.Validate(); err == nil {
				t.Error("expected error")
			}
		})
	}

	// NPC entity type should also be valid
	npc := valid
	npc.EntityType = CombatantTypeNPC
	if err := npc.Validate(); err != nil {
		t.Errorf("NPC combatant should be valid: %v", err)
	}
}

func TestCombatStateValidate(t *testing.T) {
	valid := CombatState{
		ID:         uuid.New(),
		CampaignID: uuid.New(),
		Combatants: []Combatant{
			{EntityID: uuid.New(), EntityType: CombatantTypePlayer, Name: "Hero", MaxHP: 10},
		},
		RoundNumber: 1,
		Status:      CombatStatusActive,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid combat state: %v", err)
	}

	tests := []struct {
		name string
		mod  func(*CombatState)
	}{
		{"nil id", func(cs *CombatState) { cs.ID = uuid.Nil }},
		{"nil campaign_id", func(cs *CombatState) { cs.CampaignID = uuid.Nil }},
		{"no combatants", func(cs *CombatState) { cs.Combatants = nil }},
		{"empty combatants", func(cs *CombatState) { cs.Combatants = []Combatant{} }},
		{"invalid combatant", func(cs *CombatState) {
			cs.Combatants = []Combatant{{EntityID: uuid.Nil}}
		}},
		{"negative round", func(cs *CombatState) { cs.RoundNumber = -1 }},
		{"invalid status", func(cs *CombatState) { cs.Status = "invalid" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := valid
			cs.Combatants = make([]Combatant, len(valid.Combatants))
			copy(cs.Combatants, valid.Combatants)
			tt.mod(&cs)
			if err := cs.Validate(); err == nil {
				t.Error("expected error")
			}
		})
	}

	// Round zero should be valid (initial state before first round)
	zeroRound := valid
	zeroRound.RoundNumber = 0
	if err := zeroRound.Validate(); err != nil {
		t.Errorf("round 0 should be valid: %v", err)
	}
}

type fixedInitiativeRoller struct {
	rolls    []int
	rollIdx  int
	ties     []int
	tieIdx   int
	defaultN int
}

func (f *fixedInitiativeRoller) RollD20() int {
	if f.rollIdx >= len(f.rolls) {
		return 1
	}
	v := f.rolls[f.rollIdx]
	f.rollIdx++
	return v
}

func (f *fixedInitiativeRoller) Intn(n int) int {
	f.defaultN = n
	if f.tieIdx >= len(f.ties) {
		return 0
	}
	v := f.ties[f.tieIdx]
	f.tieIdx++
	if v < 0 {
		return 0
	}
	if n > 0 && v >= n {
		return n - 1
	}
	return v
}

func TestRollInitiativeSortsByInitiativeDescending(t *testing.T) {
	stats12, _ := json.Marshal(map[string]int{"dexterity": 12})
	stats14, _ := json.Marshal(map[string]int{"dexterity": 14})
	stats10, _ := json.Marshal(map[string]int{"dexterity": 10})

	c1 := Combatant{EntityID: uuid.New(), EntityType: CombatantTypePlayer, Name: "A", HP: 10, MaxHP: 10, Stats: stats12}
	c2 := Combatant{EntityID: uuid.New(), EntityType: CombatantTypeNPC, Name: "B", HP: 10, MaxHP: 10, Stats: stats14}
	c3 := Combatant{EntityID: uuid.New(), EntityType: CombatantTypeNPC, Name: "C", HP: 10, MaxHP: 10, Stats: stats10}

	state := &CombatState{
		ID:         uuid.New(),
		CampaignID: uuid.New(),
		Combatants: []Combatant{c1, c2, c3},
		Status:     CombatStatusActive,
	}

	roller := &fixedInitiativeRoller{rolls: []int{10, 8, 15}}
	if err := rollInitiativeWithRoller(state, roller); err != nil {
		t.Fatalf("roll initiative: %v", err)
	}

	if len(state.InitiativeOrder) != 3 {
		t.Fatalf("initiative order length = %d, want 3", len(state.InitiativeOrder))
	}
	if state.Combatants[0].EntityID != c3.EntityID || state.Combatants[1].EntityID != c1.EntityID || state.Combatants[2].EntityID != c2.EntityID {
		t.Fatalf("unexpected initiative order: got [%s %s %s]", state.Combatants[0].Name, state.Combatants[1].Name, state.Combatants[2].Name)
	}
}

func TestRollInitiativeTieBreakByDexterityThenRandom(t *testing.T) {
	stats14, _ := json.Marshal(map[string]int{"dexterity": 14})
	stats12, _ := json.Marshal(map[string]int{"dexterity": 12})
	stats10, _ := json.Marshal(map[string]int{"dexterity": 10})

	highDex := Combatant{EntityID: uuid.New(), EntityType: CombatantTypePlayer, Name: "HighDex", HP: 10, MaxHP: 10, Stats: stats14}
	lowDex := Combatant{EntityID: uuid.New(), EntityType: CombatantTypeNPC, Name: "LowDex", HP: 10, MaxHP: 10, Stats: stats12}
	tieA := Combatant{EntityID: uuid.New(), EntityType: CombatantTypeNPC, Name: "TieA", HP: 10, MaxHP: 10, Stats: stats10}
	tieB := Combatant{EntityID: uuid.New(), EntityType: CombatantTypeNPC, Name: "TieB", HP: 10, MaxHP: 10, Stats: stats10}

	state := &CombatState{
		ID:         uuid.New(),
		CampaignID: uuid.New(),
		Combatants: []Combatant{tieA, lowDex, highDex, tieB},
		Status:     CombatStatusActive,
	}

	// Everyone rolls 10. highDex wins first tie-break by dexterity.
	// tieB gets higher random tie-break than tieA.
	roller := &fixedInitiativeRoller{
		rolls: []int{10, 10, 10, 10},
		ties:  []int{100, 200, 300, 400},
	}
	if err := rollInitiativeWithRoller(state, roller); err != nil {
		t.Fatalf("roll initiative: %v", err)
	}

	got := []uuid.UUID{state.Combatants[0].EntityID, state.Combatants[1].EntityID, state.Combatants[2].EntityID, state.Combatants[3].EntityID}
	want := []uuid.UUID{highDex.EntityID, lowDex.EntityID, tieB.EntityID, tieA.EntityID}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("initiative tie-break order mismatch at %d: got %v want %v", i, got, want)
		}
	}
}

func TestCombatantsForCurrentRoundSkipsSurprisedDuringSurpriseRound(t *testing.T) {
	ready := Combatant{EntityID: uuid.New(), EntityType: CombatantTypePlayer, Name: "Ready", HP: 10, MaxHP: 10}
	surprised := Combatant{EntityID: uuid.New(), EntityType: CombatantTypeNPC, Name: "Surprised", HP: 10, MaxHP: 10, Surprised: true}

	state := &CombatState{
		ID:                  uuid.New(),
		CampaignID:          uuid.New(),
		Combatants:          []Combatant{ready, surprised},
		SurpriseRoundActive: true,
		Status:              CombatStatusActive,
	}

	roundCombatants := CombatantsForCurrentRound(state)
	if len(roundCombatants) != 1 {
		t.Fatalf("combatants for surprise round = %d, want 1", len(roundCombatants))
	}
	if roundCombatants[0].EntityID != ready.EntityID {
		t.Fatalf("unexpected combatant in surprise round: got %s", roundCombatants[0].Name)
	}

	state.SurpriseRoundActive = false
	roundCombatants = CombatantsForCurrentRound(state)
	if len(roundCombatants) != 2 {
		t.Fatalf("combatants after surprise round = %d, want 2", len(roundCombatants))
	}
}

func TestStartNextRoundRerollConfigDefaultAndEnabled(t *testing.T) {
	stats10, _ := json.Marshal(map[string]int{"dexterity": 10})
	stats12, _ := json.Marshal(map[string]int{"dexterity": 12})

	a := Combatant{EntityID: uuid.New(), EntityType: CombatantTypePlayer, Name: "A", HP: 10, MaxHP: 10, Stats: stats10}
	b := Combatant{EntityID: uuid.New(), EntityType: CombatantTypeNPC, Name: "B", HP: 10, MaxHP: 10, Stats: stats12, Surprised: true}

	stateNoReroll := &CombatState{
		ID:         uuid.New(),
		CampaignID: uuid.New(),
		Combatants: []Combatant{a, b},
		Status:     CombatStatusActive,
	}
	if stateNoReroll.InitiativeRerollEachRound {
		t.Fatal("default reroll config should be false")
	}

	firstRoller := &fixedInitiativeRoller{rolls: []int{10, 12}}
	if err := startNextRoundWithRoller(stateNoReroll, firstRoller); err != nil {
		t.Fatalf("start round 1: %v", err)
	}
	firstOrder := append([]uuid.UUID(nil), stateNoReroll.InitiativeOrder...)
	firstInitiatives := []int{stateNoReroll.Combatants[0].Initiative, stateNoReroll.Combatants[1].Initiative}
	if !stateNoReroll.SurpriseRoundActive {
		t.Fatal("surprise round should be active in round 1 when a combatant is surprised")
	}

	secondRoller := &fixedInitiativeRoller{rolls: []int{1, 20}}
	if err := startNextRoundWithRoller(stateNoReroll, secondRoller); err != nil {
		t.Fatalf("start round 2: %v", err)
	}
	if stateNoReroll.SurpriseRoundActive {
		t.Fatal("surprise round should be inactive after round 1")
	}
	for i := range firstOrder {
		if stateNoReroll.InitiativeOrder[i] != firstOrder[i] {
			t.Fatalf("initiative order changed without reroll: got %v want %v", stateNoReroll.InitiativeOrder, firstOrder)
		}
	}
	if stateNoReroll.Combatants[0].Initiative != firstInitiatives[0] || stateNoReroll.Combatants[1].Initiative != firstInitiatives[1] {
		t.Fatal("initiative values should not change when reroll disabled")
	}

	stateReroll := &CombatState{
		ID:                        uuid.New(),
		CampaignID:                uuid.New(),
		Combatants:                []Combatant{a, b},
		InitiativeRerollEachRound: true,
		Status:                    CombatStatusActive,
	}
	roundOneRoller := &fixedInitiativeRoller{rolls: []int{10, 10}}
	if err := startNextRoundWithRoller(stateReroll, roundOneRoller); err != nil {
		t.Fatalf("start reroll state round 1: %v", err)
	}
	roundOneInitiatives := []int{stateReroll.Combatants[0].Initiative, stateReroll.Combatants[1].Initiative}

	roundTwoRoller := &fixedInitiativeRoller{rolls: []int{1, 20}}
	if err := startNextRoundWithRoller(stateReroll, roundTwoRoller); err != nil {
		t.Fatalf("start reroll state round 2: %v", err)
	}
	if stateReroll.Combatants[0].Initiative == roundOneInitiatives[0] && stateReroll.Combatants[1].Initiative == roundOneInitiatives[1] {
		t.Fatal("initiative values should change when reroll enabled")
	}
}
