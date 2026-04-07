package game

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

// StateLoader extracts campaign state assembly into named query methods,
// making each query independently testable while preserving the phased
// concurrency of the original GatherState.
type StateLoader struct {
	queries statedb.Querier
	db      statedb.DBTX // for campaign_time raw SQL
}

// NewStateLoader creates a StateLoader backed by the given querier and raw DB handle.
func NewStateLoader(queries statedb.Querier, db statedb.DBTX) *StateLoader {
	return &StateLoader{queries: queries, db: db}
}

// Load assembles all relevant campaign state in three phases:
//  1. Campaign (must succeed — everything else depends on it)
//  2. Independent fan-out (player character, quests, world facts)
//  3. Dependent fan-out (location, connections, NPCs, inventory, quest objectives)
func (l *StateLoader) Load(ctx context.Context, campaignID uuid.UUID) (*GameState, error) {
	pgID := dbutil.ToPgtype(campaignID)
	state := &GameState{
		CurrentLocationConnections: []domain.LocationConnection{},
		NearbyNPCs:                 []domain.NPC{},
		ActiveQuests:               []domain.Quest{},
		ActiveQuestObjectives:      make(map[uuid.UUID][]domain.QuestObjective),
		PlayerInventory:            []domain.Item{},
		WorldFacts:                 []domain.WorldFact{},
	}

	// Phase 1: Campaign (must succeed — everything depends on it).
	if err := l.loadCampaign(ctx, pgID, state); err != nil {
		return nil, fmt.Errorf("gather state campaign: %w", err)
	}

	// Phase 2: Independent fan-out — only need campaign ID.
	var quests []statedb.Quest
	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error { return l.loadPlayerCharacter(gCtx, pgID, state) })
	g.Go(func() error {
		var err error
		quests, err = l.queries.ListActiveQuests(gCtx, pgID)
		return err
	})
	g.Go(func() error { return l.loadWorldFacts(gCtx, pgID, state) })
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("gather state: %w", err)
	}

	// Convert quests after fan-out completes.
	for _, q := range quests {
		state.ActiveQuests = append(state.ActiveQuests, questToDomain(q))
	}

	// Phase 3: Dependent fan-out — needs player location + quests from phase 2.
	if state.Player.CurrentLocationID != nil {
		g2, g2Ctx := errgroup.WithContext(ctx)
		g2.Go(func() error { return l.loadLocation(g2Ctx, pgID, state) })
		g2.Go(func() error { return l.loadConnections(g2Ctx, pgID, state) })
		g2.Go(func() error { return l.loadNearbyNPCs(g2Ctx, pgID, state) })
		g2.Go(func() error { return l.loadInventory(g2Ctx, pgID, state) })
		if err := g2.Wait(); err != nil {
			return nil, fmt.Errorf("gather state: %w", err)
		}
	} else if state.Player.ID != uuid.Nil {
		// Player exists but has no location — only fetch inventory.
		if err := l.loadInventory(ctx, pgID, state); err != nil {
			return nil, fmt.Errorf("gather state inventory: %w", err)
		}
	}

	// Quest objectives depend on quests — sequential after phase 2.
	if err := l.loadQuestObjectives(ctx, quests, state); err != nil {
		return nil, err
	}

	// Derive combat flag from player status.
	state.CombatActive = state.Player.Status == "in_combat"

	// Campaign time — raw SQL since campaign_time is not in sqlc yet.
	if l.db != nil {
		ct, err := loadCampaignTime(ctx, l.db, pgID)
		if err == nil {
			state.Time = ct
		}
		// Ignore errors (table might not exist during migration rollout).
	}

	return state, nil
}

func (l *StateLoader) loadCampaign(ctx context.Context, pgID pgtype.UUID, state *GameState) error {
	campaign, err := l.queries.GetCampaignByID(ctx, pgID)
	if err != nil {
		return err
	}
	state.Campaign = campaignToDomain(campaign)
	state.RulesMode = string(state.Campaign.RulesMode)
	if state.RulesMode == "" {
		state.RulesMode = string(domain.RulesModeNarrative)
	}
	return nil
}

func (l *StateLoader) loadPlayerCharacter(ctx context.Context, pgID pgtype.UUID, state *GameState) error {
	playerCharacters, err := l.queries.GetPlayerCharacterByCampaign(ctx, pgID)
	if err != nil {
		return err
	}
	if len(playerCharacters) > 0 {
		// SQL orders by created_at ASC; use the most recently created character.
		state.Player = playerCharacterToDomain(playerCharacters[len(playerCharacters)-1])
	}
	return nil
}

func (l *StateLoader) loadWorldFacts(ctx context.Context, pgID pgtype.UUID, state *GameState) error {
	worldFacts, err := l.queries.ListActiveFactsByCampaign(ctx, pgID)
	if err != nil {
		return err
	}
	for _, fact := range worldFacts {
		state.WorldFacts = append(state.WorldFacts, worldFactToDomain(fact))
	}
	return nil
}

func (l *StateLoader) loadLocation(ctx context.Context, pgID pgtype.UUID, state *GameState) error {
	pgLocationID := dbutil.ToPgtype(*state.Player.CurrentLocationID)
	location, err := l.queries.GetLocationByID(ctx, statedb.GetLocationByIDParams{
		ID:         pgLocationID,
		CampaignID: pgID,
	})
	if err != nil {
		return err
	}
	state.CurrentLocation = locationToDomain(location)
	return nil
}

func (l *StateLoader) loadConnections(ctx context.Context, pgID pgtype.UUID, state *GameState) error {
	pgLocationID := dbutil.ToPgtype(*state.Player.CurrentLocationID)
	connections, err := l.queries.GetConnectionsFromLocation(ctx, statedb.GetConnectionsFromLocationParams{
		CampaignID: pgID,
		LocationID: pgLocationID,
	})
	if err != nil {
		return err
	}
	for _, c := range connections {
		state.CurrentLocationConnections = append(state.CurrentLocationConnections, locationConnectionToDomain(c))
	}
	return nil
}

func (l *StateLoader) loadNearbyNPCs(ctx context.Context, pgID pgtype.UUID, state *GameState) error {
	pgLocationID := dbutil.ToPgtype(*state.Player.CurrentLocationID)
	nearbyNPCs, err := l.queries.ListAliveNPCsByLocation(ctx, statedb.ListAliveNPCsByLocationParams{
		CampaignID: pgID,
		LocationID: pgLocationID,
	})
	if err != nil {
		return err
	}
	for _, npc := range nearbyNPCs {
		state.NearbyNPCs = append(state.NearbyNPCs, npcToDomain(npc))
	}
	return nil
}

func (l *StateLoader) loadInventory(ctx context.Context, pgID pgtype.UUID, state *GameState) error {
	items, err := l.queries.ListItemsByPlayer(ctx, statedb.ListItemsByPlayerParams{
		CampaignID:        pgID,
		PlayerCharacterID: dbutil.ToPgtype(state.Player.ID),
	})
	if err != nil {
		return err
	}
	for _, item := range items {
		state.PlayerInventory = append(state.PlayerInventory, itemToDomain(item))
	}
	return nil
}

func (l *StateLoader) loadQuestObjectives(ctx context.Context, quests []statedb.Quest, state *GameState) error {
	questIDs := make([]pgtype.UUID, 0, len(quests))
	for _, quest := range quests {
		questIDs = append(questIDs, quest.ID)
	}
	if len(questIDs) == 0 {
		return nil
	}
	objectives, err := l.queries.ListObjectivesByQuests(ctx, questIDs)
	if err != nil {
		return fmt.Errorf("gather state quest objectives: %w", err)
	}
	for _, objective := range objectives {
		questID := dbutil.FromPgtype(objective.QuestID)
		state.ActiveQuestObjectives[questID] = append(
			state.ActiveQuestObjectives[questID],
			questObjectiveToDomain(objective),
		)
	}
	return nil
}
