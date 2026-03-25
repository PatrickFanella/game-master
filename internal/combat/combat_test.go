package combat

import (
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
