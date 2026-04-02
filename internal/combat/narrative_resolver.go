package combat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Default values for narrative combat resolution.
const (
	defaultActionSkill    = "strength"
	defaultActionDC       = 10
	defaultNPCDamageOnHit = 3
	defaultDamageType     = "physical"
)

// actionDetails contains optional parameters extracted from
// PlayerAction.Details to drive skill checks and damage application.
type actionDetails struct {
	Skill       string `json:"skill"`
	Difficulty  int    `json:"difficulty"`
	DamageOnHit int    `json:"damage_on_hit"`
	DamageType  string `json:"damage_type"`
}

// NarrativeCombatResolver is the default CombatResolver implementation
// that uses d20 skill checks for action resolution, applies damage and
// conditions using the combat package utilities, and produces narrative
// descriptions of each round.
type NarrativeCombatResolver struct {
	roller     initiativeRoller
	campaignID uuid.UUID
}

// NewNarrativeCombatResolver creates a NarrativeCombatResolver with the
// default random dice roller. campaignID associates new encounters with a
// specific campaign.
func NewNarrativeCombatResolver(campaignID uuid.UUID) *NarrativeCombatResolver {
	return &NarrativeCombatResolver{roller: newDefaultInitiativeRoller(), campaignID: campaignID}
}

// newNarrativeCombatResolverWithRoller creates a resolver with an injected
// roller for deterministic testing.
func newNarrativeCombatResolverWithRoller(campaignID uuid.UUID, roller initiativeRoller) *NarrativeCombatResolver {
	return &NarrativeCombatResolver{roller: roller, campaignID: campaignID}
}

// Verify interface compliance at compile time.
var _ CombatResolver = (*NarrativeCombatResolver)(nil)

// InitiateCombat sets up a new combat encounter, rolls initiative for all
// combatants, and returns the initial combat state.
func (r *NarrativeCombatResolver) InitiateCombat(ctx context.Context, combatants []Combatant, environment Environment) (*CombatState, error) {
	if len(combatants) == 0 {
		return nil, fmt.Errorf("at least one combatant is required")
	}

	for i := range combatants {
		if err := combatants[i].Validate(); err != nil {
			return nil, fmt.Errorf("combatant %d: %w", i, err)
		}
	}

	state := &CombatState{
		ID:          uuid.New(),
		CampaignID:  r.campaignID,
		Combatants:  combatants,
		Environment: environment,
		Status:      CombatStatusActive,
	}

	if err := rollInitiativeWithRoller(state, r.roller); err != nil {
		return nil, fmt.Errorf("roll initiative: %w", err)
	}

	return state, nil
}

// ProcessRound advances combat by one round. It resolves the player's action
// and generates NPC actions, applying damage and conditions as appropriate.
func (r *NarrativeCombatResolver) ProcessRound(ctx context.Context, playerAction PlayerAction, combatState *CombatState) (*RoundResult, error) {
	if combatState == nil {
		return nil, fmt.Errorf("combat state is required")
	}
	if combatState.Status != CombatStatusActive {
		return nil, fmt.Errorf("combat is not active")
	}

	// Advance the round. Skip re-rolling initiative on round 1 when
	// initiative was already rolled by InitiateCombat.
	if combatState.RoundNumber == 0 && len(combatState.InitiativeOrder) > 0 {
		combatState.RoundNumber = 1
		combatState.SurpriseRoundActive = hasSurprisedCombatant(combatState.Combatants)
		TickAllConditions(combatState)
	} else {
		if err := startNextRoundWithRoller(combatState, r.roller); err != nil {
			return nil, fmt.Errorf("start next round: %w", err)
		}
	}

	var actions []CombatAction
	var damage []DamageRecord
	var condChanges []ConditionChange
	var narrativeParts []string

	// Resolve the player's action.
	playerCombatant := combatantByEntityID(combatState, playerAction.CombatantID)
	if playerCombatant != nil && playerCombatant.Status == CombatantStatusAlive {
		if combatState.SurpriseRoundActive && playerCombatant.Surprised {
			narrativeParts = append(narrativeParts, fmt.Sprintf("%s is surprised and cannot act.", playerCombatant.Name))
		} else if SkipsTurn(playerCombatant) {
			narrativeParts = append(narrativeParts, fmt.Sprintf("%s is unable to act this round.", playerCombatant.Name))
		} else {
			ad := parseActionDetails(playerAction.Details)
			modifier := resolverStatModifier(playerCombatant, ad.Skill)
			disadvantage := HasAttackDisadvantage(playerCombatant)
			roll, total, success := r.resolveCheck(modifier, ad.Difficulty, disadvantage)

			action := CombatAction{
				ActorID:     playerAction.CombatantID,
				ActionType:  playerAction.ActionType,
				TargetID:    playerAction.TargetID,
				Description: playerAction.Description,
			}
			actions = append(actions, action)

			if success {
				narrativeParts = append(narrativeParts,
					fmt.Sprintf("%s: %s (d20: %d + %d = %d vs DC %d — Hit!)",
						playerCombatant.Name, playerAction.Description, roll, modifier, total, ad.Difficulty))
				if ad.DamageOnHit > 0 && playerAction.TargetID != nil {
					dmg, cond := resolverApplyDamage(combatState, playerAction.CombatantID, *playerAction.TargetID, ad.DamageOnHit, ad.DamageType)
					damage = append(damage, dmg)
					narrativeParts = append(narrativeParts, resolverDamageNarrative(dmg, combatState))
					condChanges = append(condChanges, cond...)
				}
			} else {
				narrativeParts = append(narrativeParts,
					fmt.Sprintf("%s: %s (d20: %d + %d = %d vs DC %d — Miss!)",
						playerCombatant.Name, playerAction.Description, roll, modifier, total, ad.Difficulty))
			}
		}
	}

	// Resolve NPC actions: each alive NPC attacks the first alive player.
	playerTarget := firstAliveCombatantByType(combatState, CombatantTypePlayer)
	for i := range combatState.Combatants {
		npc := &combatState.Combatants[i]
		if npc.EntityType != CombatantTypeNPC || npc.Status != CombatantStatusAlive {
			continue
		}
		if combatState.SurpriseRoundActive && npc.Surprised {
			narrativeParts = append(narrativeParts, fmt.Sprintf("%s is surprised and cannot act.", npc.Name))
			continue
		}
		if SkipsTurn(npc) {
			narrativeParts = append(narrativeParts, fmt.Sprintf("%s is unable to act this round.", npc.Name))
			continue
		}
		if playerTarget == nil {
			continue
		}

		modifier := resolverStatModifier(npc, defaultActionSkill)
		disadvantage := HasAttackDisadvantage(npc)
		roll, total, success := r.resolveCheck(modifier, defaultActionDC, disadvantage)

		targetID := playerTarget.EntityID
		action := CombatAction{
			ActorID:     npc.EntityID,
			ActionType:  ActionTypeAttack,
			TargetID:    &targetID,
			Description: fmt.Sprintf("%s attacks %s", npc.Name, playerTarget.Name),
		}
		actions = append(actions, action)

		if success {
			narrativeParts = append(narrativeParts,
				fmt.Sprintf("%s attacks %s (d20: %d + %d = %d vs DC %d — Hit!)",
					npc.Name, playerTarget.Name, roll, modifier, total, defaultActionDC))
			dmg, cond := resolverApplyDamage(combatState, npc.EntityID, targetID, defaultNPCDamageOnHit, defaultDamageType)
			damage = append(damage, dmg)
			narrativeParts = append(narrativeParts, resolverDamageNarrative(dmg, combatState))
			condChanges = append(condChanges, cond...)
			// Re-check the player target in case they fell.
			if playerTarget.Status != CombatantStatusAlive {
				playerTarget = firstAliveCombatantByType(combatState, CombatantTypePlayer)
			}
		} else {
			narrativeParts = append(narrativeParts,
				fmt.Sprintf("%s attacks %s (d20: %d + %d = %d vs DC %d — Miss!)",
					npc.Name, playerTarget.Name, roll, modifier, total, defaultActionDC))
		}
	}

	// Remove dead combatants from initiative order.
	pruneDeadFromInitiative(combatState)

	// Check if combat should end.
	if !hasAliveCombatant(combatState, CombatantTypeNPC) {
		combatState.Status = CombatStatusCompleted
		narrativeParts = append(narrativeParts, "All enemies have been defeated! Combat is over.")
	} else if !hasAliveCombatant(combatState, CombatantTypePlayer) {
		combatState.Status = CombatStatusCompleted
		narrativeParts = append(narrativeParts, "The player has fallen! Combat is over.")
	}

	narrative := fmt.Sprintf("Round %d: %s", combatState.RoundNumber, strings.Join(narrativeParts, " "))
	combatState.Narrative = narrative

	return &RoundResult{
		RoundNumber:       combatState.RoundNumber,
		ActionsTaken:      actions,
		DamageDealt:       damage,
		ConditionsChanged: condChanges,
		Narrative:         narrative,
		UpdatedState:      combatState,
	}, nil
}

// ResolveCombat determines the final outcome of the combat encounter based
// on which sides have surviving combatants.
func (r *NarrativeCombatResolver) ResolveCombat(ctx context.Context, combatState *CombatState) (*CombatOutcome, error) {
	if combatState == nil {
		return nil, fmt.Errorf("combat state is required")
	}

	playersAlive := hasAliveCombatant(combatState, CombatantTypePlayer)
	npcsAlive := hasAliveCombatant(combatState, CombatantTypeNPC)

	var winner *uuid.UUID
	var narrativeParts []string

	switch {
	case playersAlive && !npcsAlive:
		w := firstAliveCombatantByType(combatState, CombatantTypePlayer)
		if w != nil {
			id := w.EntityID
			winner = &id
		}
		narrativeParts = append(narrativeParts, "Victory! All enemies have been defeated.")
	case !playersAlive && npcsAlive:
		w := firstAliveCombatantByType(combatState, CombatantTypeNPC)
		if w != nil {
			id := w.EntityID
			winner = &id
		}
		narrativeParts = append(narrativeParts, "Defeat. The player has fallen in combat.")
	default:
		narrativeParts = append(narrativeParts, "Combat has ended inconclusively.")
	}

	var casualties []uuid.UUID
	xpEarned := 0
	for i := range combatState.Combatants {
		c := &combatState.Combatants[i]
		if c.Status == CombatantStatusDead {
			casualties = append(casualties, c.EntityID)
			if c.EntityType == CombatantTypeNPC {
				xpEarned += c.MaxHP
			}
		}
	}

	combatState.Status = CombatStatusCompleted

	return &CombatOutcome{
		Winner:     winner,
		Casualties: casualties,
		XPEarned:   xpEarned,
		Narrative:  strings.Join(narrativeParts, " "),
	}, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// resolveCheck rolls d20 + modifier vs DC, with critical hit/miss handling
// and optional disadvantage.
func (r *NarrativeCombatResolver) resolveCheck(modifier, dc int, disadvantage bool) (roll, total int, success bool) {
	roll = r.roller.RollD20()
	if disadvantage {
		second := r.roller.RollD20()
		if second < roll {
			roll = second
		}
	}
	total = roll + modifier
	success = total >= dc
	if roll == 20 {
		success = true
	}
	if roll == 1 {
		success = false
	}
	return roll, total, success
}

// parseActionDetails extracts action details from PlayerAction.Details JSON.
// Missing or unparseable fields fall back to defaults.
func parseActionDetails(details json.RawMessage) actionDetails {
	ad := actionDetails{
		Skill:      defaultActionSkill,
		Difficulty: defaultActionDC,
		DamageType: defaultDamageType,
	}
	if len(details) == 0 {
		return ad
	}
	var parsed actionDetails
	if err := json.Unmarshal(details, &parsed); err != nil {
		return ad
	}
	if parsed.Skill != "" {
		ad.Skill = parsed.Skill
	}
	if parsed.Difficulty > 0 {
		ad.Difficulty = parsed.Difficulty
	}
	if parsed.DamageOnHit > 0 {
		ad.DamageOnHit = parsed.DamageOnHit
	}
	if parsed.DamageType != "" {
		ad.DamageType = parsed.DamageType
	}
	return ad
}

// resolverStatModifier extracts the d20-style ability modifier for the
// named skill from the combatant's Stats JSON. Returns 0 when the stat is
// missing or unparseable.
func resolverStatModifier(c *Combatant, skill string) int {
	if c == nil || len(c.Stats) == 0 {
		return 0
	}
	var statsMap map[string]any
	if err := json.Unmarshal(c.Stats, &statsMap); err != nil {
		return 0
	}
	skill = strings.ToLower(skill)
	for k, v := range statsMap {
		if strings.ToLower(k) != skill {
			continue
		}
		fv, ok := v.(float64)
		if !ok {
			return 0
		}
		stat := int(fv)
		if stat >= 10 {
			return (stat - 10) / 2
		}
		return -((11 - stat) / 2)
	}
	return 0
}

// combatantByEntityID finds a combatant by entity ID in the combat state.
func combatantByEntityID(state *CombatState, id uuid.UUID) *Combatant {
	for i := range state.Combatants {
		if state.Combatants[i].EntityID == id {
			return &state.Combatants[i]
		}
	}
	return nil
}

// firstAliveCombatantByType returns the first alive combatant of the given
// type, or nil if none exist.
func firstAliveCombatantByType(state *CombatState, entityType CombatantType) *Combatant {
	for i := range state.Combatants {
		if state.Combatants[i].EntityType == entityType && state.Combatants[i].Status == CombatantStatusAlive {
			return &state.Combatants[i]
		}
	}
	return nil
}

// hasAliveCombatant returns true if there is at least one alive combatant of
// the given type.
func hasAliveCombatant(state *CombatState, entityType CombatantType) bool {
	return firstAliveCombatantByType(state, entityType) != nil
}

// pruneDeadFromInitiative removes dead combatants from the initiative order.
func pruneDeadFromInitiative(state *CombatState) {
	alive := make(map[uuid.UUID]bool, len(state.Combatants))
	for i := range state.Combatants {
		if state.Combatants[i].Status != CombatantStatusDead {
			alive[state.Combatants[i].EntityID] = true
		}
	}
	newOrder := make([]uuid.UUID, 0, len(state.InitiativeOrder))
	for _, id := range state.InitiativeOrder {
		if alive[id] {
			newOrder = append(newOrder, id)
		}
	}
	state.InitiativeOrder = newOrder
}

// resolverApplyDamage applies damage to the target combatant and returns
// the damage record and any condition changes.
func resolverApplyDamage(state *CombatState, sourceID, targetID uuid.UUID, amount int, damageType string) (DamageRecord, []ConditionChange) {
	target := combatantByEntityID(state, targetID)
	if target != nil {
		ApplyDamage(target, amount)
	}

	dmg := DamageRecord{
		SourceID:   sourceID,
		TargetID:   targetID,
		Amount:     amount,
		DamageType: damageType,
	}

	var changes []ConditionChange
	if target != nil && target.Status == CombatantStatusDead {
		changes = append(changes, ConditionChange{
			EntityID:  targetID,
			Condition: "dead",
			Applied:   true,
		})
	} else if target != nil && target.Status == CombatantStatusUnconscious {
		changes = append(changes, ConditionChange{
			EntityID:  targetID,
			Condition: "unconscious",
			Applied:   true,
		})
	}
	return dmg, changes
}

// resolverDamageNarrative builds a human-readable damage narrative string.
func resolverDamageNarrative(dmg DamageRecord, state *CombatState) string {
	target := combatantByEntityID(state, dmg.TargetID)
	if target == nil {
		return ""
	}
	parts := []string{
		fmt.Sprintf("%s takes %d %s damage. (HP: %d/%d)", target.Name, dmg.Amount, dmg.DamageType, target.HP, target.MaxHP),
	}
	switch target.Status {
	case CombatantStatusDead:
		parts = append(parts, fmt.Sprintf("%s has been defeated!", target.Name))
	case CombatantStatusUnconscious:
		parts = append(parts, fmt.Sprintf("%s falls unconscious!", target.Name))
	}
	return strings.Join(parts, " ")
}
