package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/combat"
	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/llm"
)

const (
	initiateCombatToolName = "initiate_combat"
	combatPlayerStatus     = "in_combat"
)

// InitiateCombatNPCParams contains NPC fields required when creating enemy records.
type InitiateCombatNPCParams struct {
	CampaignID  uuid.UUID
	Name        string
	Description string
	LocationID  *uuid.UUID
	HP          int
	Stats       json.RawMessage
	Abilities   json.RawMessage
}

// InitiateCombatLogEntry contains persisted context for a newly-started encounter.
type InitiateCombatLogEntry struct {
	CampaignID             uuid.UUID
	LocationID             *uuid.UUID
	EnemyNPCIDs            []uuid.UUID
	EnvironmentDescription string
	OpeningDescription     string
}

// InitiateCombatStore provides persistence needed for initiate_combat.
type InitiateCombatStore interface {
	GetPlayerCharacterByID(ctx context.Context, playerCharacterID uuid.UUID) (*domain.PlayerCharacter, error)
	ListNPCsByCampaign(ctx context.Context, campaignID uuid.UUID) ([]domain.NPC, error)
	CreateNPC(ctx context.Context, params InitiateCombatNPCParams) (*domain.NPC, error)
	UpdatePlayerStatus(ctx context.Context, playerCharacterID uuid.UUID, status string) error
	LogCombatStart(ctx context.Context, entry InitiateCombatLogEntry) error
}

// InitiateCombatTool returns the initiate_combat tool definition and JSON schema.
func InitiateCombatTool() llm.Tool {
	return llm.Tool{
		Name:        initiateCombatToolName,
		Description: "Start combat by creating enemies, building combatants, rolling initiative, and entering combat mode.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"enemies": map[string]any{
					"type":        "array",
					"description": "Enemy combatants to include in this encounter.",
					"minItems":    1,
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{
								"type":        "string",
								"description": "Enemy name.",
							},
							"description": map[string]any{
								"type":        "string",
								"description": "Enemy description.",
							},
							"hp": map[string]any{
								"type":        "integer",
								"description": "Enemy max/current HP at combat start.",
							},
							"stats": map[string]any{
								"type":        "object",
								"description": "Enemy stats used for mechanics like initiative.",
							},
							"abilities": map[string]any{
								"type":        "array",
								"description": "Enemy abilities available during combat.",
								"items":       map[string]any{},
							},
						},
						"required":             []string{"name", "description", "hp", "stats", "abilities"},
						"additionalProperties": false,
					},
				},
				"environment": map[string]any{
					"type":        "string",
					"description": "Terrain and combat conditions description.",
				},
				"surprise": map[string]any{
					"type":        "string",
					"description": "Optional side that has surprise. One of: players, enemies.",
				},
			},
			"required":             []string{"enemies", "environment"},
			"additionalProperties": false,
		},
	}
}

// RegisterInitiateCombat registers the initiate_combat tool and handler.
func RegisterInitiateCombat(reg *Registry, store InitiateCombatStore) error {
	if store == nil {
		return errors.New("initiate_combat store is required")
	}
	return reg.Register(InitiateCombatTool(), NewInitiateCombatHandler(store).Handle)
}

// InitiateCombatHandler executes initiate_combat tool calls.
type InitiateCombatHandler struct {
	store InitiateCombatStore
}

// NewInitiateCombatHandler creates a new initiate_combat handler.
func NewInitiateCombatHandler(store InitiateCombatStore) *InitiateCombatHandler {
	return &InitiateCombatHandler{store: store}
}

func (h *InitiateCombatHandler) Handle(ctx context.Context, args map[string]any) (*ToolResult, error) {
	if h == nil {
		return nil, errors.New("initiate_combat handler is nil")
	}
	if h.store == nil {
		return nil, errors.New("initiate_combat store is required")
	}

	playerCharacterID, ok := CurrentPlayerCharacterIDFromContext(ctx)
	if !ok {
		return nil, errors.New("initiate_combat requires current player character id in context")
	}

	enemies, err := parseEnemyInputs(args, "enemies")
	if err != nil {
		return nil, err
	}
	environmentDescription, err := parseStringArg(args, "environment")
	if err != nil {
		return nil, err
	}
	surpriseSide, err := parseOptionalSurpriseArg(args, "surprise")
	if err != nil {
		return nil, err
	}

	player, err := h.store.GetPlayerCharacterByID(ctx, playerCharacterID)
	if err != nil {
		return nil, fmt.Errorf("get player character: %w", err)
	}
	if player == nil {
		return nil, errors.New("player character not found")
	}

	combatants := make([]combat.Combatant, 0, 1+len(enemies))
	combatants = append(combatants, buildPlayerCombatant(*player))
	enemyNPCIDs := make([]uuid.UUID, 0, len(enemies))

	currentLocationID, hasLocation := CurrentLocationIDFromContext(ctx)
	var locationID *uuid.UUID
	if hasLocation {
		locationID = &currentLocationID
	}

	existingNPCs, err := h.store.ListNPCsByCampaign(ctx, player.CampaignID)
	if err != nil {
		return nil, fmt.Errorf("list campaign npcs: %w", err)
	}
	npcByLowerName := make(map[string]*domain.NPC, len(existingNPCs))
	for i := range existingNPCs {
		npc := existingNPCs[i]
		npcByLowerName[strings.ToLower(npc.Name)] = &npc
	}

	for i, enemy := range enemies {
		npc := npcByLowerName[strings.ToLower(enemy.Name)]
		if npc == nil {
			createdNPC, createErr := h.store.CreateNPC(ctx, InitiateCombatNPCParams{
				CampaignID:  player.CampaignID,
				Name:        enemy.Name,
				Description: enemy.Description,
				LocationID:  locationID,
				HP:          enemy.HP,
				Stats:       enemy.Stats,
				Abilities:   enemy.Abilities,
			})
			if createErr != nil {
				return nil, fmt.Errorf("create enemy npc %q: %w", enemy.Name, createErr)
			}
			npc = createdNPC
			npcByLowerName[strings.ToLower(enemy.Name)] = npc
		}
		if npc == nil {
			return nil, fmt.Errorf("enemy npc %q could not be resolved", enemy.Name)
		}

		enemyCombatant := combat.Combatant{
			EntityID:   npc.ID,
			EntityType: combat.CombatantTypeNPC,
			Name:       enemy.Name,
			HP:         enemy.HP,
			MaxHP:      enemy.HP,
			Stats:      enemy.Stats,
		}
		if surpriseSide == "players" {
			enemyCombatant.Surprised = true
		}
		if err := enemyCombatant.Validate(); err != nil {
			return nil, fmt.Errorf("validate enemy combatant %d: %w", i, err)
		}

		combatants = append(combatants, enemyCombatant)
		enemyNPCIDs = append(enemyNPCIDs, npc.ID)
	}

	if surpriseSide == "enemies" {
		combatants[0].Surprised = true
	}

	state := &combat.CombatState{
		ID:         uuid.New(),
		CampaignID: player.CampaignID,
		Combatants: combatants,
		Environment: combat.Environment{
			LocationID:  locationID,
			Description: environmentDescription,
		},
		Status: combat.CombatStatusActive,
	}

	if err := combat.RollInitiative(state); err != nil {
		return nil, fmt.Errorf("roll initiative: %w", err)
	}
	if err := state.Validate(); err != nil {
		return nil, fmt.Errorf("validate combat state: %w", err)
	}

	initiativeOrder := makeInitiativeOrder(state)
	opening := buildOpeningDescription(*player, enemies, environmentDescription, surpriseSide)

	if err := h.store.UpdatePlayerStatus(ctx, playerCharacterID, combatPlayerStatus); err != nil {
		return nil, fmt.Errorf("set combat mode: %w", err)
	}

	if err := h.store.LogCombatStart(ctx, InitiateCombatLogEntry{
		CampaignID:             player.CampaignID,
		LocationID:             locationID,
		EnemyNPCIDs:            enemyNPCIDs,
		EnvironmentDescription: environmentDescription,
		OpeningDescription:     opening,
	}); err != nil {
		return nil, fmt.Errorf("log combat start: %w", err)
	}

	data := map[string]any{
		"combat_state_id":     state.ID.String(),
		"initiative_order":    initiativeOrder,
		"environment":         environmentDescription,
		"surprise":            surpriseSide,
		"enemy_count":         len(enemies),
		"opening_description": opening,
		"mode":                "combat",
	}

	return &ToolResult{
		Success:   true,
		Data:      data,
		Narrative: opening,
	}, nil
}

type enemyInput struct {
	Name        string
	Description string
	HP          int
	Stats       json.RawMessage
	Abilities   json.RawMessage
}

func parseEnemyInputs(args map[string]any, key string) ([]enemyInput, error) {
	raw, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%s must contain at least one enemy", key)
	}

	enemies := make([]enemyInput, 0, len(items))
	for i, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an object", key, i)
		}

		name, err := parseStringArg(obj, "name")
		if err != nil {
			return nil, fmt.Errorf("%s[%d].%w", key, i, err)
		}
		description, err := parseStringArg(obj, "description")
		if err != nil {
			return nil, fmt.Errorf("%s[%d].%w", key, i, err)
		}
		hp, err := parseIntArg(obj, "hp")
		if err != nil {
			return nil, fmt.Errorf("%s[%d].%w", key, i, err)
		}
		if hp <= 0 {
			return nil, fmt.Errorf("%s[%d].hp must be greater than 0", key, i)
		}

		statsObj, err := parseJSONObjectArg(obj, "stats")
		if err != nil {
			return nil, fmt.Errorf("%s[%d].%w", key, i, err)
		}
		statsJSON, err := json.Marshal(statsObj)
		if err != nil {
			return nil, fmt.Errorf("%s[%d].marshal stats: %w", key, i, err)
		}

		rawAbilities, ok := obj["abilities"]
		if !ok {
			return nil, fmt.Errorf("%s[%d].abilities is required", key, i)
		}
		abilities, ok := rawAbilities.([]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d].abilities must be an array", key, i)
		}
		abilitiesJSON, err := json.Marshal(abilities)
		if err != nil {
			return nil, fmt.Errorf("%s[%d].marshal abilities: %w", key, i, err)
		}

		enemies = append(enemies, enemyInput{
			Name:        name,
			Description: description,
			HP:          hp,
			Stats:       statsJSON,
			Abilities:   abilitiesJSON,
		})
	}

	return enemies, nil
}

func parseOptionalSurpriseArg(args map[string]any, key string) (string, error) {
	raw, ok := args[key]
	if !ok {
		return "", nil
	}
	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}
	if value != "players" && value != "enemies" {
		return "", fmt.Errorf("%s must be one of: players, enemies", key)
	}
	return value, nil
}

func buildPlayerCombatant(player domain.PlayerCharacter) combat.Combatant {
	maxHP := player.MaxHP
	if maxHP <= 0 {
		if player.HP > 0 {
			maxHP = player.HP
		} else {
			maxHP = 1
		}
	}

	hp := player.HP
	if hp < 0 {
		hp = 0
	}
	if hp > maxHP {
		hp = maxHP
	}

	stats := player.Stats
	if len(stats) == 0 {
		stats = json.RawMessage(`{}`)
	}

	return combat.Combatant{
		EntityID:   player.ID,
		EntityType: combat.CombatantTypePlayer,
		Name:       player.Name,
		HP:         hp,
		MaxHP:      maxHP,
		Stats:      stats,
	}
}

func makeInitiativeOrder(state *combat.CombatState) []map[string]any {
	byID := make(map[uuid.UUID]combat.Combatant, len(state.Combatants))
	for _, c := range state.Combatants {
		byID[c.EntityID] = c
	}

	order := make([]map[string]any, 0, len(state.InitiativeOrder))
	for _, id := range state.InitiativeOrder {
		combatant, ok := byID[id]
		if !ok {
			continue
		}
		order = append(order, map[string]any{
			"entity_id":  combatant.EntityID.String(),
			"name":       combatant.Name,
			"type":       string(combatant.EntityType),
			"initiative": combatant.Initiative,
			"surprised":  combatant.Surprised,
			"status":     string(combatant.Status),
			"current_hp": combatant.HP,
			"max_hp":     combatant.MaxHP,
		})
	}
	return order
}

func buildOpeningDescription(player domain.PlayerCharacter, enemies []enemyInput, environmentDescription, surprise string) string {
	enemyCount := len(enemies)
	enemyLabel := "enemy"
	if enemyCount != 1 {
		enemyLabel = "enemies"
	}

	surpriseText := ""
	switch surprise {
	case "players":
		surpriseText = " Players have surprise; the enemy side is surprised."
	case "enemies":
		surpriseText = " Enemies have surprise; the player side is surprised."
	}

	return fmt.Sprintf(
		"Combat begins! %s faces %d %s. Environment: %s.%s",
		player.Name,
		enemyCount,
		enemyLabel,
		environmentDescription,
		surpriseText,
	)
}
