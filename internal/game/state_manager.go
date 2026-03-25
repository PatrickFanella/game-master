package game

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/PatrickFanella/game-master/internal/domain"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

// pgStateManager implements StateManager using pgx and sqlc.
type pgStateManager struct {
	db      statedb.DBTX
	queries statedb.Querier
}

// NewStateManager creates a new StateManager backed by the given database connection.
func NewStateManager(db statedb.DBTX) StateManager {
	return &pgStateManager{
		db:      db,
		queries: statedb.New(db),
	}
}

// newStateManagerWithQuerier is used for testing with a mock Querier.
func newStateManagerWithQuerier(q statedb.Querier) *pgStateManager {
	return &pgStateManager{queries: q}
}

func (sm *pgStateManager) GetOrCreateDefaultUser(ctx context.Context) (*domain.User, error) {
	const defaultName = "Player"

	u, err := sm.queries.GetUserByName(ctx, defaultName)
	if err == nil {
		return userToDomain(u), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get default user: %w", err)
	}

	u, err = sm.queries.CreateUser(ctx, defaultName)
	if err != nil {
		return nil, fmt.Errorf("create default user: %w", err)
	}
	return userToDomain(u), nil
}

func (sm *pgStateManager) CreateCampaign(ctx context.Context, params CreateCampaignParams) (*domain.Campaign, error) {
	return nil, fmt.Errorf("CreateCampaign: not yet implemented (requires campaign queries)")
}

func (sm *pgStateManager) LoadCampaign(ctx context.Context, id uuid.UUID) (*GameState, error) {
	return sm.GatherState(ctx, id)
}

func (sm *pgStateManager) GetGameState(ctx context.Context, campaignID uuid.UUID) (*GameState, error) {
	return sm.GatherState(ctx, campaignID)
}

func (sm *pgStateManager) GatherState(ctx context.Context, campaignID uuid.UUID) (*GameState, error) {
	state := &GameState{
		ActiveQuestObjectives: make(map[uuid.UUID][]domain.QuestObjective),
	}

	pgCampaignID := uuidToPgtype(campaignID)

	campaign, err := sm.queries.GetCampaignByID(ctx, pgCampaignID)
	if err != nil {
		return nil, fmt.Errorf("gather state campaign: %w", err)
	}
	state.Campaign = campaignToDomain(campaign)

	playerCharacters, err := sm.queries.GetPlayerCharacterByCampaign(ctx, pgCampaignID)
	if err != nil {
		return nil, fmt.Errorf("gather state player: %w", err)
	}
	if len(playerCharacters) > 0 {
		player := playerCharacterToDomain(playerCharacters[0])
		state.Player = player

		if player.CurrentLocationID != nil {
			location, err := sm.queries.GetLocationByID(ctx, uuidToPgtype(*player.CurrentLocationID))
			if err != nil {
				return nil, fmt.Errorf("gather state location: %w", err)
			}
			state.CurrentLocation = locationToDomain(location)

			connections, err := sm.queries.GetConnectionsFromLocation(ctx, statedb.GetConnectionsFromLocationParams{
				CampaignID: pgCampaignID,
				LocationID: uuidToPgtype(*player.CurrentLocationID),
			})
			if err != nil {
				return nil, fmt.Errorf("gather state location connections: %w", err)
			}
			for _, c := range connections {
				state.CurrentLocationConnections = append(state.CurrentLocationConnections, locationConnectionToDomain(c))
			}

			npcs, err := sm.queries.ListAliveNPCsByLocation(ctx, statedb.ListAliveNPCsByLocationParams{
				CampaignID: pgCampaignID,
				LocationID: uuidToPgtype(*player.CurrentLocationID),
			})
			if err != nil {
				return nil, fmt.Errorf("gather state location npcs: %w", err)
			}
			for _, npc := range npcs {
				state.NearbyNPCs = append(state.NearbyNPCs, npcToDomain(npc))
			}
		}

		items, err := sm.queries.ListItemsByPlayer(ctx, statedb.ListItemsByPlayerParams{
			CampaignID:        pgCampaignID,
			PlayerCharacterID: uuidToPgtype(player.ID),
		})
		if err != nil {
			return nil, fmt.Errorf("gather state inventory: %w", err)
		}
		for _, item := range items {
			state.PlayerInventory = append(state.PlayerInventory, itemToDomain(item))
		}
	}

	quests, err := sm.queries.ListActiveQuests(ctx, pgCampaignID)
	if err != nil {
		return nil, fmt.Errorf("gather state quests: %w", err)
	}
	for _, quest := range quests {
		q := questToDomain(quest)
		state.ActiveQuests = append(state.ActiveQuests, q)

		objectives, err := sm.queries.ListObjectivesByQuest(ctx, quest.ID)
		if err != nil {
			return nil, fmt.Errorf("gather state quest objectives: %w", err)
		}

		if len(objectives) == 0 {
			continue
		}
		domainObjectives := make([]domain.QuestObjective, 0, len(objectives))
		for _, objective := range objectives {
			domainObjectives = append(domainObjectives, questObjectiveToDomain(objective))
		}
		state.ActiveQuestObjectives[q.ID] = domainObjectives
	}

	worldFacts, err := sm.queries.ListActiveFactsByCampaign(ctx, pgCampaignID)
	if err != nil {
		return nil, fmt.Errorf("gather state world facts: %w", err)
	}
	for _, fact := range worldFacts {
		state.WorldFacts = append(state.WorldFacts, worldFactToDomain(fact))
	}

	return state, nil
}

func (sm *pgStateManager) SaveSessionLog(ctx context.Context, log domain.SessionLog) error {
	return fmt.Errorf("SaveSessionLog: not yet implemented (requires session_log queries)")
}
