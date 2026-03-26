package combat

import (
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Enums
// ---------------------------------------------------------------------------

// CombatantType identifies whether a combatant is a player character or NPC.
type CombatantType string

const (
	// CombatantTypePlayer represents a player-controlled character.
	CombatantTypePlayer CombatantType = "player"
	// CombatantTypeNPC represents a non-player character.
	CombatantTypeNPC CombatantType = "npc"
)

// CombatantStatus tracks the vital state of a combatant within an encounter.
type CombatantStatus string

const (
	// CombatantStatusAlive indicates the combatant is conscious and able to act.
	// Note: the Go zero value for CombatantStatus is "" (empty string), not
	// CombatantStatusAlive. Validate normalizes an empty status so callers do
	// not need to set it explicitly on construction.
	CombatantStatusAlive CombatantStatus = "alive"
	// CombatantStatusUnconscious indicates a player character has reached 0 HP
	// and is incapacitated but not yet dead.
	CombatantStatusUnconscious CombatantStatus = "unconscious"
	// CombatantStatusDead indicates the combatant has been killed (NPC at 0 HP,
	// or a player who failed three death saving throws).
	CombatantStatusDead CombatantStatus = "dead"
)

// CombatStatus tracks the current phase of a combat encounter.
type CombatStatus string

const (
	// CombatStatusActive indicates combat is ongoing.
	CombatStatusActive CombatStatus = "active"
	// CombatStatusCompleted indicates combat has ended normally.
	CombatStatusCompleted CombatStatus = "completed"
	// CombatStatusFled indicates one or more parties fled.
	CombatStatusFled CombatStatus = "fled"
)

// ActionType categorizes the kind of action a combatant can take during a
// round.
type ActionType string

const (
	// ActionTypeAttack is a physical or ranged attack.
	ActionTypeAttack ActionType = "attack"
	// ActionTypeDefend is a defensive or guarding action.
	ActionTypeDefend ActionType = "defend"
	// ActionTypeSpell is a magical ability or spell cast.
	ActionTypeSpell ActionType = "spell"
	// ActionTypeItem is the use of an inventory item.
	ActionTypeItem ActionType = "item"
	// ActionTypeMove is a positional or tactical movement.
	ActionTypeMove ActionType = "move"
	// ActionTypeFlee is an attempt to escape combat.
	ActionTypeFlee ActionType = "flee"
	// ActionTypeCustom covers narrative or system-specific actions not
	// captured by the other categories.
	ActionTypeCustom ActionType = "custom"
)

// ---------------------------------------------------------------------------
// Core types
// ---------------------------------------------------------------------------

// ActiveCondition represents a status effect currently applied to a combatant.
type ActiveCondition struct {
	// Name is the identifier of the condition (e.g. "stunned", "prone").
	Name string
	// DurationRounds is the number of rounds remaining. Use PermanentDuration
	// (-1) for conditions with no fixed expiry.
	DurationRounds int
}

// DeathSavingThrows tracks the state of death saving throws for an
// unconscious player combatant.
type DeathSavingThrows struct {
	// Successes is the count of successful saving throws (max 3).
	Successes int
	// Failures is the count of failed saving throws (max 3).
	Failures int
}

// Combatant represents a single participant in a combat encounter.
type Combatant struct {
	// EntityID is the unique identifier of the underlying entity.
	EntityID uuid.UUID
	// EntityType distinguishes player characters from NPCs.
	EntityType CombatantType
	// Name is the display name of the combatant.
	Name string
	// HP is the combatant's current hit points.
	HP int
	// MaxHP is the combatant's maximum hit points.
	MaxHP int
	// Stats holds game-system-agnostic attributes (e.g. strength, dexterity).
	Stats json.RawMessage
	// Conditions lists active status effects with their remaining durations.
	Conditions []ActiveCondition
	// Initiative determines action order within a round.
	Initiative int
	// Surprised indicates this combatant skips turns during the surprise round.
	Surprised bool
	// Status tracks whether the combatant is alive, unconscious, or dead.
	Status CombatantStatus
	// DeathSavingThrows tracks saving throw progress for unconscious players.
	// Nil when the combatant is alive or death saving throws are not in use.
	DeathSavingThrows *DeathSavingThrows
}

// Validate checks that the combatant has the minimum required fields.
// An empty Status is normalized to CombatantStatusAlive when HP > 0, or
// CombatantStatusDead when HP == 0, so callers do not need to set it
// explicitly on construction.
func (c *Combatant) Validate() error {
	if c.EntityID == uuid.Nil {
		return errors.New("combatant entity_id is required")
	}
	switch c.EntityType {
	case CombatantTypePlayer, CombatantTypeNPC:
	default:
		return errors.New("invalid combatant entity_type")
	}
	if c.Name == "" {
		return errors.New("combatant name is required")
	}
	if c.MaxHP <= 0 {
		return errors.New("combatant max_hp must be greater than zero")
	}
	if c.HP < 0 {
		return errors.New("combatant hp cannot be negative")
	}
	if c.HP > c.MaxHP {
		return errors.New("combatant hp cannot exceed max_hp")
	}
	// Normalize empty status to a sensible default based on current HP so that
	// downstream logic does not need to handle Status == "" specially.
	if c.Status == "" {
		if c.HP <= 0 {
			c.Status = CombatantStatusDead
		} else {
			c.Status = CombatantStatusAlive
		}
	}
	switch c.Status {
	case CombatantStatusAlive, CombatantStatusUnconscious, CombatantStatusDead:
	default:
		return errors.New("invalid combatant status")
	}
	if c.Status == CombatantStatusUnconscious && c.EntityType == CombatantTypeNPC {
		return errors.New("NPC combatants cannot be unconscious; they are either alive or dead")
	}
	return nil
}

// Environment describes the setting in which combat takes place.
type Environment struct {
	// LocationID optionally ties the encounter to a world location.
	LocationID *uuid.UUID
	// Description is a narrative summary of the environment.
	Description string
	// Properties holds game-system-agnostic environmental modifiers
	// (e.g. terrain type, weather, lighting).
	Properties json.RawMessage
}

// CombatState holds the full mutable state of an ongoing combat encounter.
type CombatState struct {
	// ID uniquely identifies this combat encounter.
	ID uuid.UUID
	// CampaignID ties the encounter to a campaign.
	CampaignID uuid.UUID
	// Combatants lists all participants and their current stats.
	Combatants []Combatant
	// InitiativeOrder stores combatant entity IDs ordered by current initiative.
	InitiativeOrder []uuid.UUID
	// InitiativeRerollEachRound controls whether initiative is re-rolled each round.
	// Defaults to false (roll once per combat).
	InitiativeRerollEachRound bool
	// TrackDeathSavingThrows enables death saving throw tracking for unconscious
	// player combatants. Defaults to false.
	TrackDeathSavingThrows bool
	// SurpriseRoundActive indicates whether round 1 is currently treated as a
	// surprise round.
	SurpriseRoundActive bool
	// RoundNumber is the current round (0 before the first round, then starts at 1).
	RoundNumber int
	// ActiveEffects holds persistent effects that span multiple rounds
	// (e.g. area-of-effect spells, environmental hazards).
	ActiveEffects json.RawMessage
	// Environment describes the combat setting.
	Environment Environment
	// Status indicates the current phase of the encounter.
	Status CombatStatus
	// Narrative is the running narrative description of combat events,
	// supporting narrative-driven resolution.
	Narrative string
}

// Validate checks that the combat state has the minimum required fields.
func (cs *CombatState) Validate() error {
	if cs.ID == uuid.Nil {
		return errors.New("combat state id is required")
	}
	if cs.CampaignID == uuid.Nil {
		return errors.New("combat state campaign_id is required")
	}
	if len(cs.Combatants) == 0 {
		return errors.New("combat state must have at least one combatant")
	}
	for i := range cs.Combatants {
		if err := cs.Combatants[i].Validate(); err != nil {
			return err
		}
	}
	if cs.RoundNumber < 0 {
		return errors.New("combat state round_number cannot be negative")
	}
	switch cs.Status {
	case CombatStatusActive, CombatStatusCompleted, CombatStatusFled:
	default:
		return errors.New("invalid combat state status")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Action / round types
// ---------------------------------------------------------------------------

// PlayerAction represents the action a player chooses to take during a
// combat round.
type PlayerAction struct {
	// CombatantID identifies which combatant is acting.
	CombatantID uuid.UUID
	// ActionType categorizes the action.
	ActionType ActionType
	// TargetID optionally identifies the target of the action.
	TargetID *uuid.UUID
	// Description is a free-text description of the intended action,
	// enabling narrative-style input.
	Description string
	// Details holds additional action parameters in a game-system-agnostic
	// format.
	Details json.RawMessage
}

// CombatAction records a single action taken during a round, including
// both player and NPC actions.
type CombatAction struct {
	// ActorID identifies who performed the action.
	ActorID uuid.UUID
	// ActionType categorizes the action performed.
	ActionType ActionType
	// TargetID optionally identifies the target.
	TargetID *uuid.UUID
	// Description is a narrative summary of the action and its effect.
	Description string
	// Details holds additional game-system-specific data.
	Details json.RawMessage
}

// DamageRecord logs a single instance of damage dealt during a round.
type DamageRecord struct {
	// SourceID identifies who dealt the damage.
	SourceID uuid.UUID
	// TargetID identifies who received the damage.
	TargetID uuid.UUID
	// Amount is the numeric damage value.
	Amount int
	// DamageType describes the nature of the damage (e.g. "slashing",
	// "fire", "psychic").
	DamageType string
}

// ConditionChange records the application or removal of a status condition
// during a round.
type ConditionChange struct {
	// EntityID identifies the affected combatant.
	EntityID uuid.UUID
	// Condition is the name of the status effect.
	Condition string
	// Applied is true when the condition was added and false when removed.
	Applied bool
}

// RoundResult captures everything that happened during a single combat
// round.
type RoundResult struct {
	// RoundNumber identifies which round this result belongs to.
	RoundNumber int
	// ActionsTaken lists all actions performed during the round.
	ActionsTaken []CombatAction
	// DamageDealt lists all damage instances that occurred.
	DamageDealt []DamageRecord
	// ConditionsChanged lists conditions that were applied or removed.
	ConditionsChanged []ConditionChange
	// Narrative is a human-readable summary of the round, supporting
	// narrative-driven resolution.
	Narrative string
	// UpdatedState is the full combat state after the round has been
	// resolved.
	UpdatedState *CombatState
}

// ---------------------------------------------------------------------------
// Outcome types
// ---------------------------------------------------------------------------

// LootEntry represents a single item gained from combat.
type LootEntry struct {
	// ItemID optionally references an existing item entity.
	ItemID *uuid.UUID
	// Name is the display name of the item.
	Name string
	// Description describes the looted item.
	Description string
	// Quantity is how many of this item were obtained.
	Quantity int
}

// CombatOutcome describes the final result of a combat encounter.
type CombatOutcome struct {
	// Winner optionally identifies the winning combatant. Nil indicates a
	// draw or inconclusive result (e.g. both sides fled).
	Winner *uuid.UUID
	// Casualties lists the entity IDs of combatants who were defeated.
	Casualties []uuid.UUID
	// Loot lists items gained from the encounter.
	Loot []LootEntry
	// XPEarned is the total experience points awarded.
	XPEarned int
	// Consequences holds game-system-agnostic data about lasting effects
	// of the combat (e.g. story flags, reputation changes).
	Consequences json.RawMessage
	// Narrative is a summary of the combat outcome for display or
	// further LLM processing.
	Narrative string
}
