package game

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"

	"github.com/PatrickFanella/game-master/internal/dbutil"
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
	campaign, err := sm.queries.CreateCampaign(ctx, statedb.CreateCampaignParams{
		Name:        params.Name,
		Description: pgtype.Text{String: params.Description, Valid: params.Description != ""},
		Genre:       pgtype.Text{String: params.Genre, Valid: params.Genre != ""},
		Tone:        pgtype.Text{String: params.Tone, Valid: params.Tone != ""},
		Themes:      params.Themes,
		Status:      "active",
		CreatedBy:   dbutil.ToPgtype(params.UserID),
	})
	if err != nil {
		return nil, fmt.Errorf("create campaign: %w", err)
	}
	c := campaignToDomain(campaign)
	return &c, nil
}

func (sm *pgStateManager) GatherState(ctx context.Context, campaignID uuid.UUID) (*GameState, error) {
	state := &GameState{
		CurrentLocationConnections: []domain.LocationConnection{},
		NearbyNPCs:                 []domain.NPC{},
		ActiveQuests:               []domain.Quest{},
		ActiveQuestObjectives:      make(map[uuid.UUID][]domain.QuestObjective),
		PlayerInventory:            []domain.Item{},
		WorldFacts:                 []domain.WorldFact{},
	}

	pgCampaignID := dbutil.ToPgtype(campaignID)

	// Campaign must be loaded first — everything else depends on it.
	campaign, err := sm.queries.GetCampaignByID(ctx, pgCampaignID)
	if err != nil {
		return nil, fmt.Errorf("gather state campaign: %w", err)
	}
	state.Campaign = campaignToDomain(campaign)

	// Round 1: fan out independent queries that only need campaign ID.
	g, gCtx := errgroup.WithContext(ctx)

	var playerCharacters []statedb.PlayerCharacter
	var quests []statedb.Quest
	var worldFacts []statedb.WorldFact

	g.Go(func() error {
		var err error
		playerCharacters, err = sm.queries.GetPlayerCharacterByCampaign(gCtx, pgCampaignID)
		return err
	})
	g.Go(func() error {
		var err error
		quests, err = sm.queries.ListActiveQuests(gCtx, pgCampaignID)
		return err
	})
	g.Go(func() error {
		var err error
		worldFacts, err = sm.queries.ListActiveFactsByCampaign(gCtx, pgCampaignID)
		return err
	})
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("gather state: %w", err)
	}

	// Convert world facts.
	for _, fact := range worldFacts {
		state.WorldFacts = append(state.WorldFacts, worldFactToDomain(fact))
	}

	// Process player character.
	if len(playerCharacters) > 0 {
		// SQL orders by created_at ASC; use the most recently created character.
		player := playerCharacterToDomain(playerCharacters[len(playerCharacters)-1])
		state.Player = player

		// Round 2: fan out queries that depend on the player character.
		if player.CurrentLocationID != nil {
			g2, g2Ctx := errgroup.WithContext(ctx)

			var location statedb.Location
			var connections []statedb.GetConnectionsFromLocationRow
			var nearbyNPCs []statedb.Npc
			var items []statedb.Item

			pgLocationID := dbutil.ToPgtype(*player.CurrentLocationID)

			g2.Go(func() error {
				var err error
				location, err = sm.queries.GetLocationByID(g2Ctx, statedb.GetLocationByIDParams{ID: pgLocationID, CampaignID: pgCampaignID})
				return err
			})
			g2.Go(func() error {
				var err error
				connections, err = sm.queries.GetConnectionsFromLocation(g2Ctx, statedb.GetConnectionsFromLocationParams{
					CampaignID: pgCampaignID,
					LocationID: pgLocationID,
				})
				return err
			})
			g2.Go(func() error {
				var err error
				nearbyNPCs, err = sm.queries.ListAliveNPCsByLocation(g2Ctx, statedb.ListAliveNPCsByLocationParams{
					CampaignID: pgCampaignID,
					LocationID: pgLocationID,
				})
				return err
			})
			g2.Go(func() error {
				var err error
				items, err = sm.queries.ListItemsByPlayer(g2Ctx, statedb.ListItemsByPlayerParams{
					CampaignID:        pgCampaignID,
					PlayerCharacterID: dbutil.ToPgtype(player.ID),
				})
				return err
			})
			if err := g2.Wait(); err != nil {
				return nil, fmt.Errorf("gather state: %w", err)
			}

			state.CurrentLocation = locationToDomain(location)
			for _, c := range connections {
				state.CurrentLocationConnections = append(state.CurrentLocationConnections, locationConnectionToDomain(c))
			}
			for _, npc := range nearbyNPCs {
				state.NearbyNPCs = append(state.NearbyNPCs, npcToDomain(npc))
			}
			for _, item := range items {
				state.PlayerInventory = append(state.PlayerInventory, itemToDomain(item))
			}
		} else {
			// No location — only fetch inventory.
			items, err := sm.queries.ListItemsByPlayer(ctx, statedb.ListItemsByPlayerParams{
				CampaignID:        pgCampaignID,
				PlayerCharacterID: dbutil.ToPgtype(player.ID),
			})
			if err != nil {
				return nil, fmt.Errorf("gather state inventory: %w", err)
			}
			for _, item := range items {
				state.PlayerInventory = append(state.PlayerInventory, itemToDomain(item))
			}
		}
	}

	// Quest objectives depend on quests — must be sequential after Round 1.
	questIDs := make([]pgtype.UUID, 0, len(quests))
	for _, quest := range quests {
		state.ActiveQuests = append(state.ActiveQuests, questToDomain(quest))
		questIDs = append(questIDs, quest.ID)
	}
	if len(questIDs) > 0 {
		objectives, err := sm.queries.ListObjectivesByQuests(ctx, questIDs)
		if err != nil {
			return nil, fmt.Errorf("gather state quest objectives: %w", err)
		}
		for _, objective := range objectives {
			questID := dbutil.FromPgtype(objective.QuestID)
			state.ActiveQuestObjectives[questID] = append(
				state.ActiveQuestObjectives[questID],
				questObjectiveToDomain(objective),
			)
		}
	}

	return state, nil
}

func (sm *pgStateManager) SaveSessionLog(ctx context.Context, log domain.SessionLog) error {
	if err := log.Validate(); err != nil {
		return fmt.Errorf("save session log validate: %w", err)
	}

	_, err := sm.queries.CreateSessionLog(ctx, statedb.CreateSessionLogParams{
		CampaignID:   dbutil.ToPgtype(log.CampaignID),
		TurnNumber:   int32(log.TurnNumber),
		PlayerInput:  log.PlayerInput,
		InputType:    string(log.InputType),
		LlmResponse:  log.LLMResponse,
		ToolCalls:    log.ToolCalls,
		LocationID:   dbutil.ToPgtype(uuidOrNil(log.LocationID)),
		NpcsInvolved: dbutil.UUIDsToPgtype(log.NPCsInvolved),
	})
	if err != nil {
		return fmt.Errorf("save session log: %w", err)
	}
	return nil
}

func uuidOrNil(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return *id
}


func (sm *pgStateManager) ListRecentSessionLogs(ctx context.Context, campaignID uuid.UUID, limit int) ([]domain.SessionLog, error) {
	logs, err := sm.queries.ListRecentSessionLogs(ctx, statedb.ListRecentSessionLogsParams{
		CampaignID: dbutil.ToPgtype(campaignID),
		LimitCount: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list recent session logs: %w", err)
	}
	return sessionLogsToDomain(logs), nil
}

func (sm *pgStateManager) GetCampaignByID(ctx context.Context, campaignID uuid.UUID) (*domain.Campaign, error) {
	campaign, err := sm.queries.GetCampaignByID(ctx, dbutil.ToPgtype(campaignID))
	if err != nil {
		return nil, fmt.Errorf("get campaign: %w", err)
	}
	c := campaignToDomain(campaign)
	return &c, nil
}

func sessionLogsToDomain(logs []statedb.SessionLog) []domain.SessionLog {
	if len(logs) == 0 {
		return nil
	}
	result := make([]domain.SessionLog, 0, len(logs))
	for i := len(logs) - 1; i >= 0; i-- {
		l := logs[i]
		result = append(result, domain.SessionLog{
			ID:           dbutil.FromPgtype(l.ID),
			CampaignID:   dbutil.FromPgtype(l.CampaignID),
			TurnNumber:   int(l.TurnNumber),
			PlayerInput:  l.PlayerInput,
			InputType:    domain.InputType(l.InputType),
			LLMResponse:  l.LlmResponse,
			ToolCalls:    append(json.RawMessage(nil), l.ToolCalls...),
			LocationID:   optionalUUID(l.LocationID),
			NPCsInvolved: pgUUIDsToUUIDs(l.NpcsInvolved),
			CreatedAt:    l.CreatedAt.Time,
		})
	}
	return result
}

func optionalUUID(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	value := dbutil.FromPgtype(id)
	return &value
}

func pgUUIDsToUUIDs(ids []pgtype.UUID) []uuid.UUID {
	if len(ids) == 0 {
		return nil
	}
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if !id.Valid {
			continue
		}
		out = append(out, dbutil.FromPgtype(id))
	}
	return out
}