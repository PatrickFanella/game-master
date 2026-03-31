package game

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	pgvector "github.com/pgvector/pgvector-go"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/internal/tools"
)

// worldService consolidates world-building persistence for expanded world tools
// and memory storage.
type worldService struct {
	queries statedb.Querier
}

// NewWorldService creates a service that satisfies world tool store interfaces
// and tools.MemoryStore.
func NewWorldService(q statedb.Querier) *worldService {
	return &worldService{queries: q}
}

// --- tools.LanguageStore methods ---

func (s *worldService) CreateLanguage(ctx context.Context, params tools.CreateLanguageParams) (uuid.UUID, error) {
	lang, err := s.queries.CreateLanguage(ctx, statedb.CreateLanguageParams{
		CampaignID:         dbutil.ToPgtype(params.CampaignID),
		Name:               params.Name,
		Description:        pgtype.Text{String: params.Description, Valid: true},
		Phonology:          params.PhonologicalRules,
		Naming:             params.NamingConventions,
		Vocabulary:         params.SampleVocabulary,
		SpokenByFactionIds: dbutil.UUIDsToPgtype(params.SpokenByFactionIDs),
		SpokenByCultureIds: dbutil.UUIDsToPgtype(params.SpokenByCultureIDs),
	})
	if err != nil {
		return uuid.Nil, err
	}
	return dbutil.FromPgtype(lang.ID), nil
}

func (s *worldService) FactionBelongsToCampaign(ctx context.Context, factionID, campaignID uuid.UUID) (bool, error) {
	faction, err := s.queries.GetFactionByID(ctx, dbutil.ToPgtype(factionID))
	if err != nil {
		return false, fmt.Errorf("get faction: %w", err)
	}
	return faction.CampaignID == dbutil.ToPgtype(campaignID), nil
}

func (s *worldService) CultureBelongsToCampaign(ctx context.Context, cultureID, campaignID uuid.UUID) (bool, error) {
	culture, err := s.queries.GetCultureByID(ctx, dbutil.ToPgtype(cultureID))
	if err != nil {
		return false, fmt.Errorf("get culture: %w", err)
	}
	return culture.CampaignID == dbutil.ToPgtype(campaignID), nil
}

// --- tools.MemoryStore methods ---

func (s *worldService) CreateMemory(ctx context.Context, params tools.CreateMemoryParams) error {
	_, err := s.queries.CreateMemory(ctx, statedb.CreateMemoryParams{
		CampaignID: dbutil.ToPgtype(params.CampaignID),
		Content:    params.Content,
		Embedding:  pgvector.NewVector(params.Embedding),
		MemoryType: params.MemoryType,
		Metadata:   params.Metadata,
	})
	return err
}

// --- tools.BeliefSystemStore methods ---

func (s *worldService) CreateBeliefSystem(ctx context.Context, arg statedb.CreateBeliefSystemParams) (statedb.BeliefSystem, error) {
	return s.queries.CreateBeliefSystem(ctx, arg)
}

func (s *worldService) CreateFact(ctx context.Context, arg statedb.CreateFactParams) (statedb.WorldFact, error) {
	return s.queries.CreateFact(ctx, arg)
}

func (s *worldService) GetFactionByID(ctx context.Context, id pgtype.UUID) (statedb.Faction, error) {
	return s.queries.GetFactionByID(ctx, id)
}

func (s *worldService) GetCultureByID(ctx context.Context, id pgtype.UUID) (statedb.Culture, error) {
	return s.queries.GetCultureByID(ctx, id)
}

// --- tools.CultureStore methods ---

func (s *worldService) CreateCulture(ctx context.Context, arg statedb.CreateCultureParams) (statedb.Culture, error) {
	return s.queries.CreateCulture(ctx, arg)
}

func (s *worldService) GetLanguageByID(ctx context.Context, id pgtype.UUID) (statedb.Language, error) {
	return s.queries.GetLanguageByID(ctx, id)
}

func (s *worldService) GetBeliefSystemByID(ctx context.Context, id pgtype.UUID) (statedb.BeliefSystem, error) {
	return s.queries.GetBeliefSystemByID(ctx, id)
}

// --- tools.EconomicSystemStore methods ---

func (s *worldService) CreateEconomicSystem(ctx context.Context, arg statedb.CreateEconomicSystemParams) (statedb.EconomicSystem, error) {
	return s.queries.CreateEconomicSystem(ctx, arg)
}

// --- tools.FactionStore methods ---

func (s *worldService) CreateFaction(ctx context.Context, arg statedb.CreateFactionParams) (statedb.Faction, error) {
	return s.queries.CreateFaction(ctx, arg)
}

func (s *worldService) CreateFactionRelationship(ctx context.Context, arg statedb.CreateFactionRelationshipParams) (statedb.FactionRelationship, error) {
	return s.queries.CreateFactionRelationship(ctx, arg)
}

func (s *worldService) GetLocationByID(ctx context.Context, id pgtype.UUID) (statedb.Location, error) {
	return s.queries.GetLocationByID(ctx, id)
}

// --- tools.CityStore methods ---

func (s *worldService) CreateLocation(ctx context.Context, arg statedb.CreateLocationParams) (statedb.Location, error) {
	return s.queries.CreateLocation(ctx, arg)
}

func (s *worldService) UpdateLocation(ctx context.Context, arg statedb.UpdateLocationParams) (statedb.Location, error) {
	return s.queries.UpdateLocation(ctx, arg)
}

func (s *worldService) ListLocationsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.Location, error) {
	return s.queries.ListLocationsByCampaign(ctx, campaignID)
}

func (s *worldService) CreateConnection(ctx context.Context, arg statedb.CreateConnectionParams) (statedb.LocationConnection, error) {
	return s.queries.CreateConnection(ctx, arg)
}

// --- tools.QuestStore methods ---

func (s *worldService) CreateQuest(ctx context.Context, arg statedb.CreateQuestParams) (statedb.Quest, error) {
	return s.queries.CreateQuest(ctx, arg)
}

func (s *worldService) GetQuestByID(ctx context.Context, id pgtype.UUID) (statedb.Quest, error) {
	return s.queries.GetQuestByID(ctx, id)
}

func (s *worldService) CreateObjective(ctx context.Context, arg statedb.CreateObjectiveParams) (statedb.QuestObjective, error) {
	return s.queries.CreateObjective(ctx, arg)
}

func (s *worldService) ListObjectivesByQuest(ctx context.Context, questID pgtype.UUID) ([]statedb.QuestObjective, error) {
	return s.queries.ListObjectivesByQuest(ctx, questID)
}

func (s *worldService) CompleteObjective(ctx context.Context, id pgtype.UUID) (statedb.QuestObjective, error) {
	return s.queries.CompleteObjective(ctx, id)
}

func (s *worldService) UpdateQuestStatus(ctx context.Context, arg statedb.UpdateQuestStatusParams) (statedb.Quest, error) {
	return s.queries.UpdateQuestStatus(ctx, arg)
}

func (s *worldService) CreateRelationship(ctx context.Context, arg statedb.CreateRelationshipParams) (statedb.EntityRelationship, error) {
	return s.queries.CreateRelationship(ctx, arg)
}
