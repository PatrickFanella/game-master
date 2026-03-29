package game

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/progression"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/internal/tools"
)

type combatService struct {
	queries statedb.Querier
}

// NewCombatService creates a service that satisfies tools.InitiateCombatStore.
func NewCombatService(q statedb.Querier) *combatService {
	return &combatService{queries: q}
}

func (s *combatService) GetPlayerCharacterByID(ctx context.Context, playerCharacterID uuid.UUID) (*domain.PlayerCharacter, error) {
	pc, err := s.queries.GetPlayerCharacterByID(ctx, dbutil.ToPgtype(playerCharacterID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	domainPC := playerCharacterToDomain(pc)
	return &domainPC, nil
}

func (s *combatService) ListNPCsByCampaign(ctx context.Context, campaignID uuid.UUID) ([]domain.NPC, error) {
	npcs, err := s.queries.ListNPCsByCampaign(ctx, dbutil.ToPgtype(campaignID))
	if err != nil {
		return nil, fmt.Errorf("list npcs by campaign: %w", err)
	}
	out := make([]domain.NPC, 0, len(npcs))
	for _, npc := range npcs {
		out = append(out, npcToDomain(npc))
	}
	return out, nil
}

func (s *combatService) CreateNPC(ctx context.Context, params tools.InitiateCombatNPCParams) (*domain.NPC, error) {
	properties := map[string]any{}
	if len(params.Abilities) > 0 {
		var abilities []any
		if err := json.Unmarshal(params.Abilities, &abilities); err == nil {
			properties["abilities"] = abilities
		}
	}
	propertiesJSON, err := json.Marshal(properties)
	if err != nil {
		return nil, fmt.Errorf("marshal npc properties: %w", err)
	}

	hp := params.HP
	npc, err := s.queries.CreateNPC(ctx, statedb.CreateNPCParams{
		CampaignID:  dbutil.ToPgtype(params.CampaignID),
		Name:        params.Name,
		Description: stringToPgText(params.Description),
		Personality: stringToPgText(""),
		Disposition: 0,
		LocationID:  dbutil.ToPgtype(uuidOrNil(params.LocationID)),
		Alive:       true,
		Hp:          intOrNullInt4(&hp),
		Stats:       params.Stats,
		Properties:  propertiesJSON,
	})
	if err != nil {
		return nil, err
	}

	domainNPC := npcToDomain(npc)
	return &domainNPC, nil
}

func (s *combatService) UpdatePlayerStatus(ctx context.Context, playerCharacterID uuid.UUID, status string) error {
	_, err := s.queries.UpdatePlayerStatus(ctx, statedb.UpdatePlayerStatusParams{
		ID:     dbutil.ToPgtype(playerCharacterID),
		Status: status,
	})
	return err
}

func (s *combatService) LogCombatStart(ctx context.Context, entry tools.InitiateCombatLogEntry) error {
	recentLogs, err := s.queries.ListRecentSessionLogs(ctx, statedb.ListRecentSessionLogsParams{
		CampaignID: dbutil.ToPgtype(entry.CampaignID),
		LimitCount: 1,
	})
	if err != nil {
		return fmt.Errorf("list recent session logs: %w", err)
	}

	turnNumber := int32(1)
	if len(recentLogs) > 0 {
		turnNumber = recentLogs[0].TurnNumber + 1
	}

	playerInput := fmt.Sprintf("Combat initiated. Environment: %s", entry.EnvironmentDescription)

	_, err = s.queries.CreateSessionLog(ctx, statedb.CreateSessionLogParams{
		CampaignID:   dbutil.ToPgtype(entry.CampaignID),
		TurnNumber:   turnNumber,
		PlayerInput:  playerInput,
		InputType:    string(domain.Narrative),
		LlmResponse:  entry.OpeningDescription,
		ToolCalls:    []byte("[]"),
		LocationID:   dbutil.ToPgtype(uuidOrNil(entry.LocationID)),
		NpcsInvolved: dbutil.UUIDsToPgtype(entry.EnemyNPCIDs),
	})
	if err != nil {
		return fmt.Errorf("create session log: %w", err)
	}
	return nil
}

var _ tools.InitiateCombatStore = (*combatService)(nil)
var _ tools.ResolveCombatStore = (*combatService)(nil)
var _ tools.UpdatePlayerStatsStore = (*combatService)(nil)
var _ tools.AddAbilityStore = (*combatService)(nil)
var _ tools.RemoveAbilityStore = (*combatService)(nil)

// --- tools.ResolveCombatStore methods ---

func (s *combatService) UpdatePlayerHP(ctx context.Context, playerCharacterID uuid.UUID, hp, maxHP int) error {
	_, err := s.queries.UpdatePlayerHP(ctx, statedb.UpdatePlayerHPParams{
		ID:    dbutil.ToPgtype(playerCharacterID),
		Hp:    int32(hp),
		MaxHp: int32(maxHP),
	})
	return err
}

func (s *combatService) UpdatePlayerLocation(ctx context.Context, playerCharacterID uuid.UUID, locationID uuid.UUID) error {
	_, err := s.queries.UpdatePlayerLocation(ctx, statedb.UpdatePlayerLocationParams{
		ID:                dbutil.ToPgtype(playerCharacterID),
		CurrentLocationID: dbutil.ToPgtype(locationID),
	})
	return err
}

func (s *combatService) UpdatePlayerStats(ctx context.Context, playerCharacterID uuid.UUID, stats json.RawMessage) error {
	_, err := s.queries.UpdatePlayerStats(ctx, statedb.UpdatePlayerStatsParams{
		ID:    dbutil.ToPgtype(playerCharacterID),
		Stats: stats,
	})
	return err
}

func (s *combatService) UpdatePlayerAbilities(ctx context.Context, playerCharacterID uuid.UUID, abilities json.RawMessage) error {
	_, err := s.queries.UpdatePlayerAbilities(ctx, statedb.UpdatePlayerAbilitiesParams{
		ID:        dbutil.ToPgtype(playerCharacterID),
		Abilities: abilities,
	})
	return err
}

func (s *combatService) AddPlayerExperience(ctx context.Context, playerCharacterID uuid.UUID, xpAmount int) error {
	pc, err := s.GetPlayerCharacterByID(ctx, playerCharacterID)
	if err != nil {
		return fmt.Errorf("get player character: %w", err)
	}
	if pc == nil {
		return fmt.Errorf("player character %s not found", playerCharacterID)
	}
	newExperience := pc.Experience + xpAmount
	newLevel := progression.LevelFromExperience(newExperience, progression.DefaultMaxLevel)
	_, err = s.queries.UpdatePlayerExperience(ctx, statedb.UpdatePlayerExperienceParams{
		ID:         dbutil.ToPgtype(playerCharacterID),
		Experience: int32(newExperience),
		Level:      int32(newLevel),
	})
	return err
}

func (s *combatService) CreatePlayerItem(ctx context.Context, playerCharacterID uuid.UUID, name, description, itemType, rarity string, quantity int) (uuid.UUID, error) {
	pc, err := s.GetPlayerCharacterByID(ctx, playerCharacterID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("get player character: %w", err)
	}
	if pc == nil {
		return uuid.Nil, fmt.Errorf("player character %s not found", playerCharacterID)
	}
	item, err := s.queries.CreateItem(ctx, statedb.CreateItemParams{
		CampaignID:        dbutil.ToPgtype(pc.CampaignID),
		PlayerCharacterID: dbutil.ToPgtype(playerCharacterID),
		Name:              name,
		Description:       stringToPgText(description),
		ItemType:          itemType,
		Rarity:            rarity,
		Properties:        []byte("{}"),
		Equipped:          false,
		Quantity:          int32(quantity),
	})
	if err != nil {
		return uuid.Nil, err
	}
	return dbutil.FromPgtype(item.ID), nil
}

func (s *combatService) MarkNPCDead(ctx context.Context, npcID uuid.UUID) error {
	_, err := s.queries.KillNPC(ctx, dbutil.ToPgtype(npcID))
	return err
}

func (s *combatService) GetNPCByID(ctx context.Context, npcID uuid.UUID) (*domain.NPC, error) {
	npc, err := s.queries.GetNPCByID(ctx, dbutil.ToPgtype(npcID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	domainNPC := npcToDomain(npc)
	return &domainNPC, nil
}

func (s *combatService) UpdateNPCDisposition(ctx context.Context, npcID uuid.UUID, newDisposition int) error {
	_, err := s.queries.UpdateNPCDisposition(ctx, statedb.UpdateNPCDispositionParams{
		ID:          dbutil.ToPgtype(npcID),
		Disposition: int32(newDisposition),
	})
	return err
}
