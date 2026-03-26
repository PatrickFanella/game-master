package combat

import "fmt"

// ---------------------------------------------------------------------------
// Condition constants
// ---------------------------------------------------------------------------

// PermanentDuration is used as the DurationRounds value for conditions that
// do not expire naturally and must be removed explicitly.
const PermanentDuration = -1

// Named condition constants used for standard condition effects.
const (
	// ConditionStunned causes the combatant to skip their turn.
	ConditionStunned = "stunned"
	// ConditionProne causes the combatant to have disadvantage on attack rolls.
	ConditionProne = "prone"
	// ConditionPoisoned causes the combatant to have disadvantage on attack rolls.
	ConditionPoisoned = "poisoned"
	// ConditionBlinded causes the combatant to have disadvantage on attack rolls.
	ConditionBlinded = "blinded"
	// ConditionFrightened causes the combatant to have disadvantage on attack rolls.
	ConditionFrightened = "frightened"
	// ConditionParalyzed causes the combatant to skip their turn.
	ConditionParalyzed = "paralyzed"
	// ConditionExhausted is a generic exhaustion condition.
	ConditionExhausted = "exhausted"
	// ConditionCharmed affects the combatant's target selection.
	ConditionCharmed = "charmed"
)

// ---------------------------------------------------------------------------
// HP management
// ---------------------------------------------------------------------------

// ApplyDamage applies damage to the combatant, clamping HP to [0, MaxHP].
// When HP reaches zero: NPC combatants become Dead; player combatants become
// Unconscious. Damage values of zero or less are ignored.
func ApplyDamage(c *Combatant, damage int) {
	if damage <= 0 || c.Status == CombatantStatusDead {
		return
	}
	c.HP -= damage
	if c.HP < 0 {
		c.HP = 0
	}
	if c.HP == 0 {
		switch c.EntityType {
		case CombatantTypeNPC:
			c.Status = CombatantStatusDead
		case CombatantTypePlayer:
			if c.Status != CombatantStatusDead {
				c.Status = CombatantStatusUnconscious
			}
		}
	}
}

// ApplyHealing restores HP to the combatant, clamped to MaxHP. Dead
// combatants cannot be healed. An unconscious player who receives healing
// above 0 HP regains consciousness and has their death saving throws cleared.
// Healing values of zero or less are ignored.
func ApplyHealing(c *Combatant, healing int) {
	if healing <= 0 || c.Status == CombatantStatusDead {
		return
	}
	c.HP += healing
	if c.HP > c.MaxHP {
		c.HP = c.MaxHP
	}
	if c.Status == CombatantStatusUnconscious && c.HP > 0 {
		c.Status = CombatantStatusAlive
		c.DeathSavingThrows = nil
	}
}

// ---------------------------------------------------------------------------
// Condition management
// ---------------------------------------------------------------------------

// AddCondition applies the named condition to the combatant with the given
// duration. If the condition is already present its duration is replaced.
// Use PermanentDuration (-1) for conditions that do not expire naturally.
func AddCondition(c *Combatant, name string, durationRounds int) {
	for i := range c.Conditions {
		if c.Conditions[i].Name == name {
			c.Conditions[i].DurationRounds = durationRounds
			return
		}
	}
	c.Conditions = append(c.Conditions, ActiveCondition{Name: name, DurationRounds: durationRounds})
}

// RemoveCondition removes the named condition from the combatant. It is a
// no-op if the condition is not present.
func RemoveCondition(c *Combatant, name string) {
	n := 0
	for i := range c.Conditions {
		if c.Conditions[i].Name != name {
			c.Conditions[n] = c.Conditions[i]
			n++
		}
	}
	c.Conditions = c.Conditions[:n]
}

// HasCondition returns true if the combatant currently has the named
// condition active.
func HasCondition(c *Combatant, name string) bool {
	for i := range c.Conditions {
		if c.Conditions[i].Name == name {
			return true
		}
	}
	return false
}

// TickConditions decrements the duration of all finite-duration conditions
// on the combatant and removes any that have reached zero. Conditions with
// PermanentDuration (-1) are not decremented.
func TickConditions(c *Combatant) {
	n := 0
	for i := range c.Conditions {
		if c.Conditions[i].DurationRounds == PermanentDuration {
			c.Conditions[n] = c.Conditions[i]
			n++
			continue
		}
		c.Conditions[i].DurationRounds--
		if c.Conditions[i].DurationRounds > 0 {
			c.Conditions[n] = c.Conditions[i]
			n++
		}
	}
	c.Conditions = c.Conditions[:n]
}

// TickAllConditions ticks condition durations for every combatant in the
// combat state. It is called at the start of each new round by StartNextRound.
func TickAllConditions(state *CombatState) {
	for i := range state.Combatants {
		TickConditions(&state.Combatants[i])
	}
}

// ---------------------------------------------------------------------------
// Condition effect queries
// ---------------------------------------------------------------------------

// SkipsTurn returns true if the combatant's active conditions prevent them
// from taking any action this turn (e.g. stunned, paralyzed).
func SkipsTurn(c *Combatant) bool {
	return HasCondition(c, ConditionStunned) || HasCondition(c, ConditionParalyzed)
}

// HasAttackDisadvantage returns true if the combatant has disadvantage on
// attack rolls due to one or more active conditions (e.g. prone, blinded,
// poisoned, frightened).
func HasAttackDisadvantage(c *Combatant) bool {
	return HasCondition(c, ConditionProne) ||
		HasCondition(c, ConditionBlinded) ||
		HasCondition(c, ConditionPoisoned) ||
		HasCondition(c, ConditionFrightened)
}

// ---------------------------------------------------------------------------
// Death saving throws
// ---------------------------------------------------------------------------

// RollDeathSavingThrow processes a death saving throw result for an
// unconscious player combatant. state must have TrackDeathSavingThrows
// enabled; roll must be in [1, 20].
//
// Special cases:
//   - Natural 20: the combatant regains 1 HP and consciousness.
//   - Natural 1: counts as two failures.
//
// Returns (stabilized, died):
//   - stabilized=true when the death save is resolved without death, either:
//   - the combatant rolls a natural 20 and regains 1 HP and consciousness, or
//   - the combatant accumulates 3 successes and is stabilized at 0 HP.
//   - died=true when the combatant accumulates 3 failures, setting status to Dead.
//
// An error is returned if death save tracking is disabled for the combat state,
// if the combatant is not an unconscious player, or if roll is outside [1, 20].
func RollDeathSavingThrow(state *CombatState, c *Combatant, roll int) (stabilized bool, died bool, err error) {
	if !state.TrackDeathSavingThrows {
		return false, false, fmt.Errorf("death saving throws are not enabled for this combat")
	}
	if c.EntityType != CombatantTypePlayer {
		return false, false, fmt.Errorf("death saving throws only apply to player combatants")
	}
	if c.Status != CombatantStatusUnconscious {
		return false, false, fmt.Errorf("combatant %s is not unconscious", c.Name)
	}
	if roll < 1 || roll > 20 {
		return false, false, fmt.Errorf("roll must be between 1 and 20, got %d", roll)
	}

	if c.DeathSavingThrows == nil {
		c.DeathSavingThrows = &DeathSavingThrows{}
	}

	switch {
	case roll == 20:
		// Natural 20: regain 1 HP and consciousness.
		c.HP = 1
		c.Status = CombatantStatusAlive
		c.DeathSavingThrows = nil
		return true, false, nil
	case roll >= 10:
		c.DeathSavingThrows.Successes++
	case roll == 1:
		// Natural 1: counts as two failures.
		c.DeathSavingThrows.Failures += 2
	default:
		c.DeathSavingThrows.Failures++
	}

	if c.DeathSavingThrows.Successes >= 3 {
		// Stabilized: stop making throws. Still unconscious at 0 HP.
		c.DeathSavingThrows = nil
		return true, false, nil
	}

	if c.DeathSavingThrows.Failures >= 3 {
		c.Status = CombatantStatusDead
		c.DeathSavingThrows = nil
		return false, true, nil
	}

	return false, false, nil
}
