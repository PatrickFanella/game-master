package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/combat"
	"github.com/PatrickFanella/game-master/internal/llm"
)

const combatRoundToolName = "combat_round"

// CombatRoundTool returns the combat_round tool definition and JSON schema.
func CombatRoundTool() llm.Tool {
	return llm.Tool{
		Name:        combatRoundToolName,
		Description: "Process one round of combat. Resolves the player action and enemy actions via skill checks, applies damage and conditions, and returns the round narrative and updated combat state.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"player_action": map[string]any{
					"type":        "string",
					"description": "Natural language description of the player's combat action.",
				},
				"action_type": map[string]any{
					"type":        "string",
					"description": "Category of the player's action: attack, defend, spell, item, move, flee, or custom.",
				},
				"target_id": map[string]any{
					"type":        "string",
					"description": "UUID of the target combatant for the player's action.",
				},
				"skill": map[string]any{
					"type":        "string",
					"description": "Skill or stat key used for the player's action check (e.g. strength, dexterity).",
				},
				"difficulty": map[string]any{
					"type":        "integer",
					"description": "Difficulty class (DC) for the player's action check.",
				},
				"damage_on_hit": map[string]any{
					"type":        "integer",
					"description": "Damage dealt if the player's action succeeds. Zero or omitted for non-damaging actions.",
				},
				"damage_type": map[string]any{
					"type":        "string",
					"description": "Type of damage dealt (e.g. slashing, fire, psychic).",
				},
				"enemy_actions": map[string]any{
					"type":        "array",
					"description": "Enemy actions for this round, generated based on enemy personalities and tactics.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"enemy_id": map[string]any{
								"type":        "string",
								"description": "UUID of the enemy combatant acting.",
							},
							"action_type": map[string]any{
								"type":        "string",
								"description": "Category of the enemy's action.",
							},
							"target_id": map[string]any{
								"type":        "string",
								"description": "UUID of the target for the enemy's action.",
							},
							"description": map[string]any{
								"type":        "string",
								"description": "Narrative description of the enemy's action.",
							},
							"skill": map[string]any{
								"type":        "string",
								"description": "Skill or stat key for the enemy's action check.",
							},
							"difficulty": map[string]any{
								"type":        "integer",
								"description": "DC for the enemy's action check.",
							},
							"damage_on_hit": map[string]any{
								"type":        "integer",
								"description": "Damage dealt if the enemy's action succeeds.",
							},
							"damage_type": map[string]any{
								"type":        "string",
								"description": "Type of damage dealt.",
							},
						},
						"required": []string{"enemy_id", "action_type", "description", "skill", "difficulty"},
					},
				},
				"combat_state": map[string]any{
					"type":        "object",
					"description": "Current combat state from initiate_combat or a previous combat_round call.",
				},
			},
			"required":             []string{"player_action", "action_type", "skill", "difficulty", "enemy_actions", "combat_state"},
			"additionalProperties": false,
		},
	}
}

// RegisterCombatRound registers the combat_round tool and handler.
func RegisterCombatRound(reg *Registry, roller DiceRoller) error {
	handler := NewCombatRoundHandler(roller)
	return reg.Register(CombatRoundTool(), handler.Handle)
}

// CombatRoundHandler executes combat_round tool calls.
type CombatRoundHandler struct {
	roller DiceRoller
}

// NewCombatRoundHandler creates a new combat_round handler.
func NewCombatRoundHandler(roller DiceRoller) *CombatRoundHandler {
	if roller == nil {
		roller = newRandomRoller()
	}
	return &CombatRoundHandler{roller: roller}
}

// Handle executes the combat_round tool.
func (h *CombatRoundHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("combat_round handler is nil")
	}

	// Parse player action parameters.
	playerAction, err := parseStringArg(args, "player_action")
	if err != nil {
		return nil, err
	}
	actionType, err := parseStringArg(args, "action_type")
	if err != nil {
		return nil, err
	}
	skill, err := parseStringArg(args, "skill")
	if err != nil {
		return nil, err
	}
	dc, err := parseIntArg(args, "difficulty")
	if err != nil {
		return nil, err
	}

	damageOnHit := 0
	if _, ok := args["damage_on_hit"]; ok {
		damageOnHit, err = parseIntArg(args, "damage_on_hit")
		if err != nil {
			return nil, err
		}
	}

	damageType := ""
	if raw, ok := args["damage_type"]; ok {
		if s, ok := raw.(string); ok {
			damageType = s
		}
	}

	var targetID *uuid.UUID
	if raw, ok := args["target_id"]; ok {
		if s, ok := raw.(string); ok && s != "" {
			id, parseErr := uuid.Parse(s)
			if parseErr != nil {
				return nil, fmt.Errorf("target_id must be a valid UUID")
			}
			targetID = &id
		}
	}

	if damageOnHit > 0 && targetID == nil {
		return nil, errors.New("target_id is required when damage_on_hit is specified")
	}

	// Parse enemy actions.
	enemyActions, err := parseCombatRoundEnemyActions(args, "enemy_actions")
	if err != nil {
		return nil, err
	}

	// Parse and advance combat state.
	state, err := parseCombatStateArg(args, "combat_state")
	if err != nil {
		return nil, err
	}

	// Advance the round. Avoid re-rolling initiative when entering round 1
	// if initiative has already been rolled (e.g., by initiate_combat).
	// This mirrors the surprise-round activation logic from
	// combat.StartNextRound but skips the initiative re-roll.
	if state.RoundNumber == 0 && len(state.InitiativeOrder) > 0 {
		state.RoundNumber = 1
		// Activate surprise round for round 1 when any combatant is surprised.
		for i := range state.Combatants {
			if state.Combatants[i].Surprised {
				state.SurpriseRoundActive = true
				break
			}
		}
		combat.TickAllConditions(state)
	} else {
		if err := combat.StartNextRound(state); err != nil {
			return nil, fmt.Errorf("start next round: %w", err)
		}
	}

	var actionsTaken []combat.CombatAction
	var damageDealt []combat.DamageRecord
	var conditionsChanged []combat.ConditionChange
	var narrativeParts []string

	// Identify the player combatant.
	playerEntityID, hasPlayerID := CurrentPlayerCharacterIDFromContext(ctx)
	var playerCombatant *combat.Combatant
	if hasPlayerID {
		playerCombatant = combatantByID(state, playerEntityID)
	}
	if playerCombatant == nil {
		playerCombatant = firstCombatantByType(state, combat.CombatantTypePlayer)
	}

	// Resolve the player's action.
	if playerCombatant != nil && playerCombatant.Status == combat.CombatantStatusAlive {
		if state.SurpriseRoundActive && playerCombatant.Surprised {
			narrativeParts = append(narrativeParts, fmt.Sprintf("%s is surprised and cannot act this round.", playerCombatant.Name))
		} else if combat.SkipsTurn(playerCombatant) {
			narrativeParts = append(narrativeParts, fmt.Sprintf("%s is unable to act this round.", playerCombatant.Name))
		} else {
			disadvantage := combat.HasAttackDisadvantage(playerCombatant)
			modifier := combatantStatModifier(playerCombatant, skill)
			roll, total, success := h.resolveCheck(modifier, dc, disadvantage)

			action := combat.CombatAction{
				ActorID:     playerCombatant.EntityID,
				ActionType:  combat.ActionType(actionType),
				TargetID:    targetID,
				Description: playerAction,
			}
			actionsTaken = append(actionsTaken, action)

			if success {
				narrativeParts = append(narrativeParts,
					fmt.Sprintf("%s: %s (d20: %d + %d = %d vs DC %d — Hit!)",
						playerCombatant.Name, playerAction, roll, modifier, total, dc))
				if damageOnHit > 0 && targetID != nil {
					dmg, cond := applyActionDamage(state, playerCombatant.EntityID, *targetID, damageOnHit, damageType)
					damageDealt = append(damageDealt, dmg)
					narrativeParts = append(narrativeParts, formatDamageNarrative(dmg, state))
					conditionsChanged = append(conditionsChanged, cond...)
				}
			} else {
				narrativeParts = append(narrativeParts,
					fmt.Sprintf("%s: %s (d20: %d + %d = %d vs DC %d — Miss!)",
						playerCombatant.Name, playerAction, roll, modifier, total, dc))
			}
		}
	}

	// Resolve enemy actions.
	for _, ea := range enemyActions {
		enemy := combatantByID(state, ea.EnemyID)
		if enemy == nil || enemy.Status != combat.CombatantStatusAlive {
			continue
		}
		if state.SurpriseRoundActive && enemy.Surprised {
			narrativeParts = append(narrativeParts, fmt.Sprintf("%s is surprised and cannot act this round.", enemy.Name))
			continue
		}
		if combat.SkipsTurn(enemy) {
			narrativeParts = append(narrativeParts, fmt.Sprintf("%s is unable to act this round.", enemy.Name))
			continue
		}

		disadvantage := combat.HasAttackDisadvantage(enemy)
		modifier := combatantStatModifier(enemy, ea.Skill)
		roll, total, success := h.resolveCheck(modifier, ea.DC, disadvantage)

		action := combat.CombatAction{
			ActorID:     ea.EnemyID,
			ActionType:  combat.ActionType(ea.ActionType),
			TargetID:    ea.TargetID,
			Description: ea.Description,
		}
		actionsTaken = append(actionsTaken, action)

		if success {
			narrativeParts = append(narrativeParts,
				fmt.Sprintf("%s: %s (d20: %d + %d = %d vs DC %d — Hit!)",
					enemy.Name, ea.Description, roll, modifier, total, ea.DC))
			if ea.DamageOnHit > 0 && ea.TargetID != nil {
				dmg, cond := applyActionDamage(state, ea.EnemyID, *ea.TargetID, ea.DamageOnHit, ea.DamageType)
				damageDealt = append(damageDealt, dmg)
				narrativeParts = append(narrativeParts, formatDamageNarrative(dmg, state))
				conditionsChanged = append(conditionsChanged, cond...)
			}
		} else {
			narrativeParts = append(narrativeParts,
				fmt.Sprintf("%s: %s (d20: %d + %d = %d vs DC %d — Miss!)",
					enemy.Name, ea.Description, roll, modifier, total, ea.DC))
		}
	}

	// Remove dead combatants from initiative order.
	alive := make(map[uuid.UUID]bool, len(state.Combatants))
	for i := range state.Combatants {
		if state.Combatants[i].Status != combat.CombatantStatusDead {
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

	// Check if combat is over.
	combatOver := !hasAliveCombatantOfType(state, combat.CombatantTypeNPC) ||
		!hasAliveCombatantOfType(state, combat.CombatantTypePlayer)
	if combatOver {
		state.Status = combat.CombatStatusCompleted
		if hasAliveCombatantOfType(state, combat.CombatantTypePlayer) {
			narrativeParts = append(narrativeParts, "All enemies have been defeated! Combat is over.")
		} else {
			narrativeParts = append(narrativeParts, "The player has fallen! Combat is over.")
		}
	}

	narrative := fmt.Sprintf("Round %d: %s", state.RoundNumber, strings.Join(narrativeParts, " "))
	state.Narrative = narrative

	data := map[string]any{
		"round_number":       state.RoundNumber,
		"actions":            formatActionSummaries(actionsTaken),
		"damage_dealt":       formatDamageSummaries(damageDealt),
		"conditions_changed": formatConditionSummaries(conditionsChanged),
		"combatants":         formatCombatantHP(state),
		"combat_state":       combatStateToMap(state),
		"combat_over":        combatOver,
	}

	return &ToolResult{
		Success:   true,
		Data:      data,
		Narrative: narrative,
	}, nil
}

// resolveCheck rolls d20 + modifier vs DC, respecting criticals and disadvantage.
func (h *CombatRoundHandler) resolveCheck(modifier, dc int, disadvantage bool) (roll, total int, success bool) {
	roll = h.roller.Intn(20) + 1
	if disadvantage {
		second := h.roller.Intn(20) + 1
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

// ---------------------------------------------------------------------------
// Combat state serialization helpers
// ---------------------------------------------------------------------------

type combatStateJSON struct {
	ID                        string            `json:"id"`
	CampaignID                string            `json:"campaign_id"`
	Combatants                []combatantJSON   `json:"combatants"`
	InitiativeOrder           []string          `json:"initiative_order"`
	RoundNumber               int               `json:"round_number"`
	Environment               json.RawMessage   `json:"environment"`
	Status                    string            `json:"status"`
	Narrative                 string            `json:"narrative"`
	InitiativeRerollEachRound bool              `json:"initiative_reroll_each_round"`
	TrackDeathSavingThrows    bool              `json:"track_death_saving_throws"`
	SurpriseRoundActive       bool              `json:"surprise_round_active"`
	ActiveEffects             json.RawMessage   `json:"active_effects,omitempty"`
}

type combatantJSON struct {
	EntityID          string              `json:"entity_id"`
	EntityType        string              `json:"entity_type"`
	Name              string              `json:"name"`
	HP                int                 `json:"hp"`
	MaxHP             int                 `json:"max_hp"`
	Stats             json.RawMessage     `json:"stats"`
	Conditions        []conditionJSON     `json:"conditions"`
	Initiative        int                 `json:"initiative"`
	Status            string              `json:"status"`
	Surprised         bool                `json:"surprised"`
	DeathSavingThrows *deathSavingJSON    `json:"death_saving_throws,omitempty"`
}

type deathSavingJSON struct {
	Successes int `json:"successes"`
	Failures  int `json:"failures"`
}

type conditionJSON struct {
	Name           string `json:"name"`
	DurationRounds int    `json:"duration_rounds"`
}

type environmentJSON struct {
	LocationID  string          `json:"location_id,omitempty"`
	Description string          `json:"description"`
	Properties  json.RawMessage `json:"properties,omitempty"`
}

func parseCombatStateArg(args map[string]any, key string) (*combat.CombatState, error) {
	raw, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	stateMap, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", key)
	}

	jsonBytes, err := json.Marshal(stateMap)
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", key, err)
	}

	var csj combatStateJSON
	if err := json.Unmarshal(jsonBytes, &csj); err != nil {
		return nil, fmt.Errorf("parse %s: %w", key, err)
	}

	id, err := uuid.Parse(csj.ID)
	if err != nil {
		return nil, fmt.Errorf("%s.id must be a valid UUID", key)
	}
	campaignID, err := uuid.Parse(csj.CampaignID)
	if err != nil {
		return nil, fmt.Errorf("%s.campaign_id must be a valid UUID", key)
	}

	combatants := make([]combat.Combatant, 0, len(csj.Combatants))
	for i, cj := range csj.Combatants {
		entityID, parseErr := uuid.Parse(cj.EntityID)
		if parseErr != nil {
			return nil, fmt.Errorf("%s.combatants[%d].entity_id must be a valid UUID", key, i)
		}
		conditions := make([]combat.ActiveCondition, len(cj.Conditions))
		for j, condJ := range cj.Conditions {
			conditions[j] = combat.ActiveCondition{
				Name:           condJ.Name,
				DurationRounds: condJ.DurationRounds,
			}
		}
		c := combat.Combatant{
			EntityID:   entityID,
			EntityType: combat.CombatantType(cj.EntityType),
			Name:       cj.Name,
			HP:         cj.HP,
			MaxHP:      cj.MaxHP,
			Stats:      cj.Stats,
			Conditions: conditions,
			Initiative: cj.Initiative,
			Status:     combat.CombatantStatus(cj.Status),
			Surprised:  cj.Surprised,
		}
		if cj.DeathSavingThrows != nil {
			c.DeathSavingThrows = &combat.DeathSavingThrows{
				Successes: cj.DeathSavingThrows.Successes,
				Failures:  cj.DeathSavingThrows.Failures,
			}
		}
		combatants = append(combatants, c)
	}

	initiativeOrder := make([]uuid.UUID, 0, len(csj.InitiativeOrder))
	for i, s := range csj.InitiativeOrder {
		oid, parseErr := uuid.Parse(s)
		if parseErr != nil {
			return nil, fmt.Errorf("%s.initiative_order[%d] must be a valid UUID", key, i)
		}
		initiativeOrder = append(initiativeOrder, oid)
	}

	var env combat.Environment
	if csj.Environment != nil {
		var envObj environmentJSON
		if err := json.Unmarshal(csj.Environment, &envObj); err != nil {
			return nil, fmt.Errorf("%s.environment: %w", key, err)
		}
		env.Description = envObj.Description
		env.Properties = envObj.Properties
		if envObj.LocationID != "" {
			lid, parseErr := uuid.Parse(envObj.LocationID)
			if parseErr != nil {
				return nil, fmt.Errorf("%s.environment.location_id must be a valid UUID", key)
			}
			env.LocationID = &lid
		}
	}

	status := combat.CombatStatus(csj.Status)
	if status == "" {
		status = combat.CombatStatusActive
	}

	return &combat.CombatState{
		ID:                        id,
		CampaignID:                campaignID,
		Combatants:                combatants,
		InitiativeOrder:           initiativeOrder,
		InitiativeRerollEachRound: csj.InitiativeRerollEachRound,
		TrackDeathSavingThrows:    csj.TrackDeathSavingThrows,
		SurpriseRoundActive:       csj.SurpriseRoundActive,
		RoundNumber:               csj.RoundNumber,
		ActiveEffects:             csj.ActiveEffects,
		Environment:               env,
		Status:                    status,
		Narrative:                 csj.Narrative,
	}, nil
}

func combatStateToMap(state *combat.CombatState) map[string]any {
	combatants := make([]map[string]any, 0, len(state.Combatants))
	for _, c := range state.Combatants {
		conditions := make([]map[string]any, 0, len(c.Conditions))
		for _, cond := range c.Conditions {
			conditions = append(conditions, map[string]any{
				"name":            cond.Name,
				"duration_rounds": cond.DurationRounds,
			})
		}
		cm := map[string]any{
			"entity_id":   c.EntityID.String(),
			"entity_type": string(c.EntityType),
			"name":        c.Name,
			"hp":          c.HP,
			"max_hp":      c.MaxHP,
			"stats":       c.Stats,
			"conditions":  conditions,
			"initiative":  c.Initiative,
			"status":      string(c.Status),
			"surprised":   c.Surprised,
		}
		if c.DeathSavingThrows != nil {
			cm["death_saving_throws"] = map[string]any{
				"successes": c.DeathSavingThrows.Successes,
				"failures":  c.DeathSavingThrows.Failures,
			}
		}
		combatants = append(combatants, cm)
	}

	initOrder := make([]string, 0, len(state.InitiativeOrder))
	for _, id := range state.InitiativeOrder {
		initOrder = append(initOrder, id.String())
	}

	envMap := map[string]any{
		"description": state.Environment.Description,
	}
	if state.Environment.LocationID != nil {
		envMap["location_id"] = state.Environment.LocationID.String()
	}
	if state.Environment.Properties != nil {
		envMap["properties"] = state.Environment.Properties
	}

	m := map[string]any{
		"id":                          state.ID.String(),
		"campaign_id":                 state.CampaignID.String(),
		"combatants":                  combatants,
		"initiative_order":            initOrder,
		"round_number":                state.RoundNumber,
		"environment":                 envMap,
		"status":                      string(state.Status),
		"narrative":                   state.Narrative,
		"initiative_reroll_each_round": state.InitiativeRerollEachRound,
		"track_death_saving_throws":   state.TrackDeathSavingThrows,
		"surprise_round_active":       state.SurpriseRoundActive,
	}
	if state.ActiveEffects != nil {
		m["active_effects"] = state.ActiveEffects
	}
	return m
}

// ---------------------------------------------------------------------------
// Enemy action parsing
// ---------------------------------------------------------------------------

type roundEnemyAction struct {
	EnemyID     uuid.UUID
	ActionType  string
	TargetID    *uuid.UUID
	Description string
	Skill       string
	DC          int
	DamageOnHit int
	DamageType  string
}

func parseCombatRoundEnemyActions(args map[string]any, key string) ([]roundEnemyAction, error) {
	raw, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}

	actions := make([]roundEnemyAction, 0, len(items))
	for i, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an object", key, i)
		}

		enemyID, err := parseUUIDArg(obj, "enemy_id")
		if err != nil {
			return nil, fmt.Errorf("%s[%d].%w", key, i, err)
		}
		at, err := parseStringArg(obj, "action_type")
		if err != nil {
			return nil, fmt.Errorf("%s[%d].%w", key, i, err)
		}
		desc, err := parseStringArg(obj, "description")
		if err != nil {
			return nil, fmt.Errorf("%s[%d].%w", key, i, err)
		}
		sk, err := parseStringArg(obj, "skill")
		if err != nil {
			return nil, fmt.Errorf("%s[%d].%w", key, i, err)
		}
		diff, err := parseIntArg(obj, "difficulty")
		if err != nil {
			return nil, fmt.Errorf("%s[%d].%w", key, i, err)
		}

		var tid *uuid.UUID
		if raw, ok := obj["target_id"]; ok {
			if s, ok := raw.(string); ok && s != "" {
				parsed, parseErr := uuid.Parse(s)
				if parseErr != nil {
					return nil, fmt.Errorf("%s[%d].target_id must be a valid UUID", key, i)
				}
				tid = &parsed
			}
		}

		dmgOnHit := 0
		if _, exists := obj["damage_on_hit"]; exists {
			v, parseErr := parseIntArg(obj, "damage_on_hit")
			if parseErr != nil {
				return nil, fmt.Errorf("%s[%d].%w", key, i, parseErr)
			}
			dmgOnHit = v
		}

		if dmgOnHit > 0 && tid == nil {
			return nil, fmt.Errorf("%s[%d].target_id is required when damage_on_hit is specified", key, i)
		}

		dmgType := ""
		if raw, ok := obj["damage_type"]; ok {
			if s, ok := raw.(string); ok {
				dmgType = s
			}
		}

		actions = append(actions, roundEnemyAction{
			EnemyID:     enemyID,
			ActionType:  at,
			TargetID:    tid,
			Description: desc,
			Skill:       sk,
			DC:          diff,
			DamageOnHit: dmgOnHit,
			DamageType:  dmgType,
		})
	}
	return actions, nil
}

// ---------------------------------------------------------------------------
// Combatant helpers
// ---------------------------------------------------------------------------

func combatantByID(state *combat.CombatState, id uuid.UUID) *combat.Combatant {
	for i := range state.Combatants {
		if state.Combatants[i].EntityID == id {
			return &state.Combatants[i]
		}
	}
	return nil
}

func firstCombatantByType(state *combat.CombatState, entityType combat.CombatantType) *combat.Combatant {
	for i := range state.Combatants {
		if state.Combatants[i].EntityType == entityType {
			return &state.Combatants[i]
		}
	}
	return nil
}

func hasAliveCombatantOfType(state *combat.CombatState, entityType combat.CombatantType) bool {
	for i := range state.Combatants {
		if state.Combatants[i].EntityType == entityType && state.Combatants[i].Status == combat.CombatantStatusAlive {
			return true
		}
	}
	return false
}

// combatantStatModifier extracts the d20-style ability modifier for a given
// skill from the combatant's Stats JSON. Returns 0 when the stat is missing
// or unparseable.
func combatantStatModifier(c *combat.Combatant, skill string) int {
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

// ---------------------------------------------------------------------------
// Damage / condition application
// ---------------------------------------------------------------------------

func applyActionDamage(state *combat.CombatState, sourceID, targetID uuid.UUID, amount int, damageType string) (combat.DamageRecord, []combat.ConditionChange) {
	target := combatantByID(state, targetID)
	if target != nil {
		combat.ApplyDamage(target, amount)
	}

	dmg := combat.DamageRecord{
		SourceID:   sourceID,
		TargetID:   targetID,
		Amount:     amount,
		DamageType: damageType,
	}

	var changes []combat.ConditionChange
	if target != nil && target.Status == combat.CombatantStatusDead {
		changes = append(changes, combat.ConditionChange{
			EntityID:  targetID,
			Condition: "dead",
			Applied:   true,
		})
	} else if target != nil && target.Status == combat.CombatantStatusUnconscious {
		changes = append(changes, combat.ConditionChange{
			EntityID:  targetID,
			Condition: "unconscious",
			Applied:   true,
		})
	}
	return dmg, changes
}

func formatDamageNarrative(dmg combat.DamageRecord, state *combat.CombatState) string {
	target := combatantByID(state, dmg.TargetID)
	if target == nil {
		return ""
	}

	parts := []string{
		fmt.Sprintf("%s takes %d %s damage. (HP: %d/%d)", target.Name, dmg.Amount, dmg.DamageType, target.HP, target.MaxHP),
	}
	if target.Status == combat.CombatantStatusDead {
		parts = append(parts, fmt.Sprintf("%s has been defeated!", target.Name))
	} else if target.Status == combat.CombatantStatusUnconscious {
		parts = append(parts, fmt.Sprintf("%s falls unconscious!", target.Name))
	}
	return strings.Join(parts, " ")
}

// ---------------------------------------------------------------------------
// Response formatting
// ---------------------------------------------------------------------------

func formatActionSummaries(actions []combat.CombatAction) []map[string]any {
	out := make([]map[string]any, 0, len(actions))
	for _, a := range actions {
		s := map[string]any{
			"actor_id":    a.ActorID.String(),
			"action_type": string(a.ActionType),
			"description": a.Description,
		}
		if a.TargetID != nil {
			s["target_id"] = a.TargetID.String()
		}
		out = append(out, s)
	}
	return out
}

func formatDamageSummaries(damage []combat.DamageRecord) []map[string]any {
	out := make([]map[string]any, 0, len(damage))
	for _, d := range damage {
		out = append(out, map[string]any{
			"source_id":   d.SourceID.String(),
			"target_id":   d.TargetID.String(),
			"amount":      d.Amount,
			"damage_type": d.DamageType,
		})
	}
	return out
}

func formatConditionSummaries(changes []combat.ConditionChange) []map[string]any {
	out := make([]map[string]any, 0, len(changes))
	for _, c := range changes {
		out = append(out, map[string]any{
			"entity_id": c.EntityID.String(),
			"condition": c.Condition,
			"applied":   c.Applied,
		})
	}
	return out
}

func formatCombatantHP(state *combat.CombatState) []map[string]any {
	out := make([]map[string]any, 0, len(state.Combatants))
	for _, c := range state.Combatants {
		out = append(out, map[string]any{
			"entity_id": c.EntityID.String(),
			"name":      c.Name,
			"hp":        c.HP,
			"max_hp":    c.MaxHP,
			"status":    string(c.Status),
		})
	}
	return out
}
