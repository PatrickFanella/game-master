package game

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

// mockQuerier implements statedb.Querier for testing.
type mockQuerier struct {
	users       map[string]statedb.User
	nextUserID  pgtype.UUID
	createCount int

	campaign           statedb.Campaign
	playerCharacters   []statedb.PlayerCharacter
	location           statedb.Location
	connections        []statedb.GetConnectionsFromLocationRow
	npcs               []statedb.Npc
	quests             []statedb.Quest
	objectivesByQuest  map[[16]byte][]statedb.QuestObjective
	objectivesByQuests []statedb.QuestObjective
	items              []statedb.Item
	worldFacts         []statedb.WorldFact
	npcByID            map[[16]byte]mockNPCRecord
	recentSessionLogs  []statedb.SessionLog
	lastSessionLog     *statedb.CreateSessionLogParams

	// Injectable errors for testing error paths.
	getByNameErr             error
	createErr                error
	getCampaignErr           error
	getPlayerErr             error
	getLocationErr           error
	getConnectionsErr        error
	getNPCsErr               error
	getQuestsErr             error
	getObjectivesErr         error
	getObjectivesByQuestsErr error
	getItemsErr              error
	getWorldFactsErr         error
	getNPCByIDErr            error
	createSessionLogErr      error
	listRecentSessionLogsErr error
}

func newMockQuerier() *mockQuerier {
	return &mockQuerier{
		users:             make(map[string]statedb.User),
		nextUserID:        pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		objectivesByQuest: make(map[[16]byte][]statedb.QuestObjective),
		npcByID:           make(map[[16]byte]mockNPCRecord),
	}
}

type mockNPCRecord struct {
	npc statedb.Npc
	err error
}

func (m *mockQuerier) GetUserByName(_ context.Context, name string) (statedb.User, error) {
	if m.getByNameErr != nil {
		return statedb.User{}, m.getByNameErr
	}
	u, ok := m.users[name]
	if !ok {
		return statedb.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func (m *mockQuerier) CreateUser(_ context.Context, name string) (statedb.User, error) {
	if m.createErr != nil {
		return statedb.User{}, m.createErr
	}
	m.createCount++
	u := statedb.User{ID: m.nextUserID, Name: name}
	m.users[name] = u
	return u, nil
}

func (m *mockQuerier) GetUserByID(_ context.Context, id pgtype.UUID) (statedb.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return statedb.User{}, pgx.ErrNoRows
}

func (m *mockQuerier) ListUsers(_ context.Context) ([]statedb.User, error) {
	var out []statedb.User
	for _, u := range m.users {
		out = append(out, u)
	}
	return out, nil
}

func (m *mockQuerier) UpdateUser(_ context.Context, arg statedb.UpdateUserParams) (statedb.User, error) {
	return statedb.User{}, pgx.ErrNoRows
}

func (m *mockQuerier) DeleteUser(_ context.Context, _ pgtype.UUID) error {
	return nil
}

func (m *mockQuerier) Ping(_ context.Context) (int32, error) {
	return 1, nil
}

func (m *mockQuerier) CreateCampaign(_ context.Context, _ statedb.CreateCampaignParams) (statedb.Campaign, error) {
	return statedb.Campaign{}, pgx.ErrNoRows
}

func (m *mockQuerier) CreateBeliefSystem(_ context.Context, _ statedb.CreateBeliefSystemParams) (statedb.BeliefSystem, error) {
	return statedb.BeliefSystem{}, pgx.ErrNoRows
}

func (m *mockQuerier) CreateCulture(_ context.Context, _ statedb.CreateCultureParams) (statedb.Culture, error) {
	return statedb.Culture{}, pgx.ErrNoRows
}

func (m *mockQuerier) CreateEconomicSystem(_ context.Context, _ statedb.CreateEconomicSystemParams) (statedb.EconomicSystem, error) {
	return statedb.EconomicSystem{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetCampaignByID(_ context.Context, _ pgtype.UUID) (statedb.Campaign, error) {
	if m.getCampaignErr != nil {
		return statedb.Campaign{}, m.getCampaignErr
	}
	if !m.campaign.ID.Valid {
		return statedb.Campaign{}, pgx.ErrNoRows
	}
	return m.campaign, nil
}

func (m *mockQuerier) GetBeliefSystemByID(_ context.Context, _ pgtype.UUID) (statedb.BeliefSystem, error) {
	return statedb.BeliefSystem{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetBeliefSystemByCulture(_ context.Context, _ pgtype.UUID) (statedb.BeliefSystem, error) {
	return statedb.BeliefSystem{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetCultureByID(_ context.Context, _ pgtype.UUID) (statedb.Culture, error) {
	return statedb.Culture{}, pgx.ErrNoRows
}

func (m *mockQuerier) ListCulturesByLanguage(_ context.Context, _ pgtype.UUID) ([]statedb.Culture, error) {
	return nil, nil
}

func (m *mockQuerier) GetEconomicSystemByID(_ context.Context, _ pgtype.UUID) (statedb.EconomicSystem, error) {
	return statedb.EconomicSystem{}, pgx.ErrNoRows
}

func (m *mockQuerier) ListCampaignsByUser(_ context.Context, _ pgtype.UUID) ([]statedb.Campaign, error) {
	return nil, nil
}

func (m *mockQuerier) ListBeliefSystemsByCampaign(_ context.Context, _ pgtype.UUID) ([]statedb.BeliefSystem, error) {
	return nil, nil
}

func (m *mockQuerier) ListCulturesByCampaign(_ context.Context, _ pgtype.UUID) ([]statedb.Culture, error) {
	return nil, nil
}

func (m *mockQuerier) ListEconomicSystemsByCampaign(_ context.Context, _ pgtype.UUID) ([]statedb.EconomicSystem, error) {
	return nil, nil
}

func (m *mockQuerier) UpdateCampaign(_ context.Context, _ statedb.UpdateCampaignParams) (statedb.Campaign, error) {
	return statedb.Campaign{}, pgx.ErrNoRows
}

func (m *mockQuerier) UpdateBeliefSystem(_ context.Context, _ statedb.UpdateBeliefSystemParams) (statedb.BeliefSystem, error) {
	return statedb.BeliefSystem{}, pgx.ErrNoRows
}

func (m *mockQuerier) UpdateCulture(_ context.Context, _ statedb.UpdateCultureParams) (statedb.Culture, error) {
	return statedb.Culture{}, pgx.ErrNoRows
}

func (m *mockQuerier) UpdateEconomicSystem(_ context.Context, _ statedb.UpdateEconomicSystemParams) (statedb.EconomicSystem, error) {
	return statedb.EconomicSystem{}, pgx.ErrNoRows
}

func (m *mockQuerier) UpdateCampaignStatus(_ context.Context, _ statedb.UpdateCampaignStatusParams) (statedb.Campaign, error) {
	return statedb.Campaign{}, pgx.ErrNoRows
}

func (m *mockQuerier) DeleteBeliefSystem(_ context.Context, _ pgtype.UUID) error {
	return nil
}

func (m *mockQuerier) DeleteCampaign(_ context.Context, _ pgtype.UUID) error {
	return nil
}

func (m *mockQuerier) DeleteCulture(_ context.Context, _ pgtype.UUID) error {
	return nil
}

func (m *mockQuerier) DeleteEconomicSystem(_ context.Context, _ pgtype.UUID) error {
	return nil
}

func (m *mockQuerier) DeleteLanguage(_ context.Context, _ pgtype.UUID) error {
	return nil
}

func (m *mockQuerier) CreateFaction(_ context.Context, _ statedb.CreateFactionParams) (statedb.Faction, error) {
	return statedb.Faction{}, pgx.ErrNoRows
}

func (m *mockQuerier) CreateFact(_ context.Context, _ statedb.CreateFactParams) (statedb.WorldFact, error) {
	return statedb.WorldFact{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetFactionByID(_ context.Context, _ pgtype.UUID) (statedb.Faction, error) {
	return statedb.Faction{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetFactByID(_ context.Context, _ pgtype.UUID) (statedb.WorldFact, error) {
	return statedb.WorldFact{}, pgx.ErrNoRows
}

func (m *mockQuerier) ListFactionsByCampaign(_ context.Context, _ pgtype.UUID) ([]statedb.Faction, error) {
	return nil, nil
}

func (m *mockQuerier) ListFactsByCampaign(_ context.Context, _ pgtype.UUID) ([]statedb.WorldFact, error) {
	return nil, nil
}

func (m *mockQuerier) ListFactsByCategory(_ context.Context, _ statedb.ListFactsByCategoryParams) ([]statedb.WorldFact, error) {
	return nil, nil
}

func (m *mockQuerier) UpdateFaction(_ context.Context, _ statedb.UpdateFactionParams) (statedb.Faction, error) {
	return statedb.Faction{}, pgx.ErrNoRows
}

func (m *mockQuerier) CreateFactionRelationship(_ context.Context, _ statedb.CreateFactionRelationshipParams) (statedb.FactionRelationship, error) {
	return statedb.FactionRelationship{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetFactionRelationships(_ context.Context, _ pgtype.UUID) ([]statedb.FactionRelationship, error) {
	return nil, nil
}

func (m *mockQuerier) UpdateFactionRelationship(_ context.Context, _ statedb.UpdateFactionRelationshipParams) (statedb.FactionRelationship, error) {
	return statedb.FactionRelationship{}, pgx.ErrNoRows
}

func (m *mockQuerier) CreateRelationship(_ context.Context, _ statedb.CreateRelationshipParams) (statedb.EntityRelationship, error) {
	return statedb.EntityRelationship{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetRelationshipsByEntity(_ context.Context, _ statedb.GetRelationshipsByEntityParams) ([]statedb.EntityRelationship, error) {
	return nil, nil
}

func (m *mockQuerier) GetRelationshipsBetween(_ context.Context, _ statedb.GetRelationshipsBetweenParams) ([]statedb.EntityRelationship, error) {
	return nil, nil
}

func (m *mockQuerier) ListRelationshipsByCampaign(_ context.Context, _ pgtype.UUID) ([]statedb.EntityRelationship, error) {
	return nil, nil
}

func (m *mockQuerier) UpdateRelationship(_ context.Context, _ statedb.UpdateRelationshipParams) (statedb.EntityRelationship, error) {
	return statedb.EntityRelationship{}, pgx.ErrNoRows
}

func (m *mockQuerier) DeleteRelationship(_ context.Context, _ statedb.DeleteRelationshipParams) error {
	return nil
}

func (m *mockQuerier) CreateLocation(_ context.Context, _ statedb.CreateLocationParams) (statedb.Location, error) {
	return statedb.Location{}, pgx.ErrNoRows
}

func (m *mockQuerier) CreateItem(_ context.Context, _ statedb.CreateItemParams) (statedb.Item, error) {
	return statedb.Item{}, pgx.ErrNoRows
}

func (m *mockQuerier) CreateLanguage(_ context.Context, _ statedb.CreateLanguageParams) (statedb.Language, error) {
	return statedb.Language{}, pgx.ErrNoRows
}

func (m *mockQuerier) CreateNPC(_ context.Context, _ statedb.CreateNPCParams) (statedb.Npc, error) {
	return statedb.Npc{}, pgx.ErrNoRows
}

func (m *mockQuerier) CreateObjective(_ context.Context, _ statedb.CreateObjectiveParams) (statedb.QuestObjective, error) {
	return statedb.QuestObjective{}, pgx.ErrNoRows
}

func (m *mockQuerier) CreateQuest(_ context.Context, _ statedb.CreateQuestParams) (statedb.Quest, error) {
	return statedb.Quest{}, pgx.ErrNoRows
}

func (m *mockQuerier) CreateSessionLog(_ context.Context, arg statedb.CreateSessionLogParams) (statedb.SessionLog, error) {
	if m.createSessionLogErr != nil {
		return statedb.SessionLog{}, m.createSessionLogErr
	}
	m.lastSessionLog = &arg
	return statedb.SessionLog{}, nil
}

func (m *mockQuerier) GetItemByID(_ context.Context, _ pgtype.UUID) (statedb.Item, error) {
	return statedb.Item{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetLanguageByID(_ context.Context, _ pgtype.UUID) (statedb.Language, error) {
	return statedb.Language{}, pgx.ErrNoRows
}

func (m *mockQuerier) ListLanguagesByFaction(_ context.Context, _ pgtype.UUID) ([]statedb.Language, error) {
	return nil, nil
}

func (m *mockQuerier) GetLocationByID(_ context.Context, _ pgtype.UUID) (statedb.Location, error) {
	if m.getLocationErr != nil {
		return statedb.Location{}, m.getLocationErr
	}
	if !m.location.ID.Valid {
		return statedb.Location{}, pgx.ErrNoRows
	}
	return m.location, nil
}

func (m *mockQuerier) GetNPCByID(_ context.Context, id pgtype.UUID) (statedb.Npc, error) {
	if m.getNPCByIDErr != nil {
		return statedb.Npc{}, m.getNPCByIDErr
	}
	record, ok := m.npcByID[id.Bytes]
	if !ok {
		return statedb.Npc{}, pgx.ErrNoRows
	}
	if record.err != nil {
		return statedb.Npc{}, record.err
	}
	return record.npc, nil
}

func (m *mockQuerier) ListLocationsByCampaign(_ context.Context, _ pgtype.UUID) ([]statedb.Location, error) {
	return nil, nil
}

func (m *mockQuerier) ListLanguagesByCampaign(_ context.Context, _ pgtype.UUID) ([]statedb.Language, error) {
	return nil, nil
}

func (m *mockQuerier) ListItemsByPlayer(_ context.Context, _ statedb.ListItemsByPlayerParams) ([]statedb.Item, error) {
	if m.getItemsErr != nil {
		return nil, m.getItemsErr
	}
	return m.items, nil
}

func (m *mockQuerier) ListItemsByType(_ context.Context, _ statedb.ListItemsByTypeParams) ([]statedb.Item, error) {
	return nil, nil
}

func (m *mockQuerier) ListLocationsByRegion(_ context.Context, _ statedb.ListLocationsByRegionParams) ([]statedb.Location, error) {
	return nil, nil
}

func (m *mockQuerier) ListNPCsByCampaign(_ context.Context, _ pgtype.UUID) ([]statedb.Npc, error) {
	return nil, nil
}

func (m *mockQuerier) ListNPCsByLocation(_ context.Context, _ statedb.ListNPCsByLocationParams) ([]statedb.Npc, error) {
	return nil, nil
}

func (m *mockQuerier) ListObjectivesByQuest(_ context.Context, questID pgtype.UUID) ([]statedb.QuestObjective, error) {
	if m.getObjectivesErr != nil {
		return nil, m.getObjectivesErr
	}
	return m.objectivesByQuest[questID.Bytes], nil
}

func (m *mockQuerier) ListObjectivesByQuests(_ context.Context, _ []pgtype.UUID) ([]statedb.QuestObjective, error) {
	if m.getObjectivesByQuestsErr != nil {
		return nil, m.getObjectivesByQuestsErr
	}
	return m.objectivesByQuests, nil
}

func (m *mockQuerier) ListQuestsByCampaign(_ context.Context, _ pgtype.UUID) ([]statedb.Quest, error) {
	return nil, nil
}

func (m *mockQuerier) ListActiveQuests(_ context.Context, _ pgtype.UUID) ([]statedb.Quest, error) {
	if m.getQuestsErr != nil {
		return nil, m.getQuestsErr
	}
	return m.quests, nil
}

func (m *mockQuerier) ListQuestsByType(_ context.Context, _ statedb.ListQuestsByTypeParams) ([]statedb.Quest, error) {
	return nil, nil
}

func (m *mockQuerier) ListRecentSessionLogs(_ context.Context, _ statedb.ListRecentSessionLogsParams) ([]statedb.SessionLog, error) {
	if m.listRecentSessionLogsErr != nil {
		return nil, m.listRecentSessionLogsErr
	}
	return m.recentSessionLogs, nil
}

func (m *mockQuerier) ListSessionLogsByCampaign(_ context.Context, _ pgtype.UUID) ([]statedb.SessionLog, error) {
	return nil, nil
}

func (m *mockQuerier) ListSessionLogsByLocation(_ context.Context, _ statedb.ListSessionLogsByLocationParams) ([]statedb.SessionLog, error) {
	return nil, nil
}

func (m *mockQuerier) ListSubquestsByParentQuest(_ context.Context, _ pgtype.UUID) ([]statedb.Quest, error) {
	return nil, nil
}

func (m *mockQuerier) ListNPCsByFaction(_ context.Context, _ statedb.ListNPCsByFactionParams) ([]statedb.Npc, error) {
	return nil, nil
}

func (m *mockQuerier) ListAliveNPCsByLocation(_ context.Context, _ statedb.ListAliveNPCsByLocationParams) ([]statedb.Npc, error) {
	if m.getNPCsErr != nil {
		return nil, m.getNPCsErr
	}
	return m.npcs, nil
}

func (m *mockQuerier) UpdateLocation(_ context.Context, _ statedb.UpdateLocationParams) (statedb.Location, error) {
	return statedb.Location{}, pgx.ErrNoRows
}

func (m *mockQuerier) UpdateItem(_ context.Context, _ statedb.UpdateItemParams) (statedb.Item, error) {
	return statedb.Item{}, pgx.ErrNoRows
}

func (m *mockQuerier) UpdateItemEquipped(_ context.Context, _ statedb.UpdateItemEquippedParams) (statedb.Item, error) {
	return statedb.Item{}, pgx.ErrNoRows
}

func (m *mockQuerier) UpdateItemQuantity(_ context.Context, _ statedb.UpdateItemQuantityParams) (statedb.Item, error) {
	return statedb.Item{}, pgx.ErrNoRows
}

func (m *mockQuerier) UpdateLanguage(_ context.Context, _ statedb.UpdateLanguageParams) (statedb.Language, error) {
	return statedb.Language{}, pgx.ErrNoRows
}

func (m *mockQuerier) UpdateNPC(_ context.Context, _ statedb.UpdateNPCParams) (statedb.Npc, error) {
	return statedb.Npc{}, pgx.ErrNoRows
}

func (m *mockQuerier) UpdateNPCDisposition(_ context.Context, _ statedb.UpdateNPCDispositionParams) (statedb.Npc, error) {
	return statedb.Npc{}, pgx.ErrNoRows
}

func (m *mockQuerier) UpdateNPCLocation(_ context.Context, _ statedb.UpdateNPCLocationParams) (statedb.Npc, error) {
	return statedb.Npc{}, pgx.ErrNoRows
}

func (m *mockQuerier) KillNPC(_ context.Context, _ pgtype.UUID) (statedb.Npc, error) {
	return statedb.Npc{}, pgx.ErrNoRows
}

func (m *mockQuerier) CompleteObjective(_ context.Context, _ pgtype.UUID) (statedb.QuestObjective, error) {
	return statedb.QuestObjective{}, pgx.ErrNoRows
}

func (m *mockQuerier) CreateConnection(_ context.Context, _ statedb.CreateConnectionParams) (statedb.LocationConnection, error) {
	return statedb.LocationConnection{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetConnectionsFromLocation(_ context.Context, _ statedb.GetConnectionsFromLocationParams) ([]statedb.GetConnectionsFromLocationRow, error) {
	if m.getConnectionsErr != nil {
		return nil, m.getConnectionsErr
	}
	return m.connections, nil
}

func (m *mockQuerier) DeleteConnection(_ context.Context, _ statedb.DeleteConnectionParams) error {
	return nil
}

func (m *mockQuerier) CreatePlayerCharacter(_ context.Context, _ statedb.CreatePlayerCharacterParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetPlayerCharacterByID(_ context.Context, _ pgtype.UUID) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetQuestByID(_ context.Context, _ pgtype.UUID) (statedb.Quest, error) {
	return statedb.Quest{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetSessionLogByID(_ context.Context, _ pgtype.UUID) (statedb.SessionLog, error) {
	return statedb.SessionLog{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetPlayerCharacterByCampaign(_ context.Context, _ pgtype.UUID) ([]statedb.PlayerCharacter, error) {
	if m.getPlayerErr != nil {
		return nil, m.getPlayerErr
	}
	return m.playerCharacters, nil
}

func (m *mockQuerier) UpdatePlayerCharacter(_ context.Context, _ statedb.UpdatePlayerCharacterParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, pgx.ErrNoRows
}

func (m *mockQuerier) UpdatePlayerStats(_ context.Context, _ statedb.UpdatePlayerStatsParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, pgx.ErrNoRows
}

func (m *mockQuerier) UpdatePlayerHP(_ context.Context, _ statedb.UpdatePlayerHPParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, pgx.ErrNoRows
}

func (m *mockQuerier) UpdatePlayerExperience(_ context.Context, _ statedb.UpdatePlayerExperienceParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, pgx.ErrNoRows
}

func (m *mockQuerier) UpdatePlayerLocation(_ context.Context, _ statedb.UpdatePlayerLocationParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, pgx.ErrNoRows
}

func (m *mockQuerier) UpdatePlayerStatus(_ context.Context, _ statedb.UpdatePlayerStatusParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, pgx.ErrNoRows
}

func (m *mockQuerier) UpdateQuest(_ context.Context, _ statedb.UpdateQuestParams) (statedb.Quest, error) {
	return statedb.Quest{}, pgx.ErrNoRows
}

func (m *mockQuerier) UpdateQuestStatus(_ context.Context, _ statedb.UpdateQuestStatusParams) (statedb.Quest, error) {
	return statedb.Quest{}, pgx.ErrNoRows
}

func (m *mockQuerier) UpdateObjective(_ context.Context, _ statedb.UpdateObjectiveParams) (statedb.QuestObjective, error) {
	return statedb.QuestObjective{}, pgx.ErrNoRows
}

func (m *mockQuerier) DeleteItem(_ context.Context, _ pgtype.UUID) error {
	return nil
}

func (m *mockQuerier) TransferItem(_ context.Context, _ statedb.TransferItemParams) (statedb.Item, error) {
	return statedb.Item{}, pgx.ErrNoRows
}

func (m *mockQuerier) SupersedeFact(_ context.Context, _ statedb.SupersedeFactParams) (statedb.WorldFact, error) {
	return statedb.WorldFact{}, pgx.ErrNoRows
}

func (m *mockQuerier) ListActiveFactsByCampaign(_ context.Context, _ pgtype.UUID) ([]statedb.WorldFact, error) {
	if m.getWorldFactsErr != nil {
		return nil, m.getWorldFactsErr
	}
	return m.worldFacts, nil
}

func (m *mockQuerier) CreateMemory(_ context.Context, _ statedb.CreateMemoryParams) (statedb.Memory, error) {
	return statedb.Memory{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetMemoryByID(_ context.Context, _ pgtype.UUID) (statedb.Memory, error) {
	return statedb.Memory{}, pgx.ErrNoRows
}

func (m *mockQuerier) ListMemoriesByCampaign(_ context.Context, _ pgtype.UUID) ([]statedb.Memory, error) {
	return nil, nil
}

func (m *mockQuerier) SearchMemoriesBySimilarity(_ context.Context, _ statedb.SearchMemoriesBySimilarityParams) ([]statedb.SearchMemoriesBySimilarityRow, error) {
	return nil, nil
}

func (m *mockQuerier) SearchMemoriesWithFilters(_ context.Context, _ statedb.SearchMemoriesWithFiltersParams) ([]statedb.SearchMemoriesWithFiltersRow, error) {
	return nil, nil
}

func TestGetOrCreateDefaultUser_Creates(t *testing.T) {
	mq := newMockQuerier()
	sm := newStateManagerWithQuerier(mq)

	u, err := sm.GetOrCreateDefaultUser(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Name != "Player" {
		t.Fatalf("expected name Player, got %q", u.Name)
	}
	if mq.createCount != 1 {
		t.Fatalf("expected 1 create call, got %d", mq.createCount)
	}
}

func TestGetOrCreateDefaultUser_ReturnsExisting(t *testing.T) {
	mq := newMockQuerier()
	mq.users["Player"] = statedb.User{
		ID:   pgtype.UUID{Bytes: [16]byte{42}, Valid: true},
		Name: "Player",
	}
	sm := newStateManagerWithQuerier(mq)

	u, err := sm.GetOrCreateDefaultUser(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Name != "Player" {
		t.Fatalf("expected name Player, got %q", u.Name)
	}
	if mq.createCount != 0 {
		t.Fatalf("should not create when user exists, got %d create calls", mq.createCount)
	}
}

func TestGetOrCreateDefaultUser_LookupError(t *testing.T) {
	dbErr := errors.New("connection refused")
	mq := newMockQuerier()
	mq.getByNameErr = dbErr
	sm := newStateManagerWithQuerier(mq)

	_, err := sm.GetOrCreateDefaultUser(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected wrapped db error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "get default user") {
		t.Fatalf("expected context in error, got: %v", err)
	}
	if mq.createCount != 0 {
		t.Fatal("should not attempt create after lookup error")
	}
}

func TestGetOrCreateDefaultUser_CreateError(t *testing.T) {
	dbErr := errors.New("unique violation")
	mq := newMockQuerier()
	mq.createErr = dbErr
	sm := newStateManagerWithQuerier(mq)

	_, err := sm.GetOrCreateDefaultUser(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected wrapped db error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "create default user") {
		t.Fatalf("expected context in error, got: %v", err)
	}
}

func TestGetOrCreateDefaultUser_ConvertsUUID(t *testing.T) {
	expectedID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	mq := newMockQuerier()
	mq.nextUserID = pgtype.UUID{Bytes: expectedID, Valid: true}
	sm := newStateManagerWithQuerier(mq)

	u, err := sm.GetOrCreateDefaultUser(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID != expectedID {
		t.Fatalf("expected UUID %v, got %v", expectedID, u.ID)
	}
}

func TestGetOrCreateDefaultUser_CalledTwice(t *testing.T) {
	mq := newMockQuerier()
	sm := newStateManagerWithQuerier(mq)

	u1, err := sm.GetOrCreateDefaultUser(context.Background())
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	u2, err := sm.GetOrCreateDefaultUser(context.Background())
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if u1.Name != u2.Name {
		t.Fatalf("expected same user, got %q and %q", u1.Name, u2.Name)
	}
	if mq.createCount != 1 {
		t.Fatalf("should create only once, got %d", mq.createCount)
	}
}

func TestGatherState_AssemblesCompleteState(t *testing.T) {
	mq := newMockQuerier()
	campaignID := uuid.New()
	userID := uuid.New()
	playerID := uuid.New()
	locationID := uuid.New()
	npcID := uuid.New()
	questID := uuid.New()
	objectiveID := uuid.New()
	itemID := uuid.New()
	factID := uuid.New()

	mq.campaign = statedb.Campaign{
		ID:        dbutil.ToPgtype(campaignID),
		Name:      "Campaign",
		Status:    "active",
		CreatedBy: dbutil.ToPgtype(userID),
	}
	mq.playerCharacters = []statedb.PlayerCharacter{{
		ID:                dbutil.ToPgtype(playerID),
		CampaignID:        dbutil.ToPgtype(campaignID),
		UserID:            dbutil.ToPgtype(userID),
		Name:              "Hero",
		Stats:             []byte(`{"str":14}`),
		Hp:                10,
		MaxHp:             12,
		Level:             2,
		Status:            "healthy",
		Abilities:         []byte(`["dash"]`),
		CurrentLocationID: dbutil.ToPgtype(locationID),
	}}
	mq.location = statedb.Location{
		ID:          dbutil.ToPgtype(locationID),
		CampaignID:  dbutil.ToPgtype(campaignID),
		Name:        "Town Square",
		Description: pgtype.Text{String: "Busy center", Valid: true},
	}
	mq.connections = []statedb.GetConnectionsFromLocationRow{{
		ID:             dbutil.ToPgtype(uuid.New()),
		FromLocationID: dbutil.ToPgtype(locationID),
		ToLocationID:   dbutil.ToPgtype(uuid.New()),
		Description:    pgtype.Text{String: "Road north", Valid: true},
		CampaignID:     dbutil.ToPgtype(campaignID),
	}}
	mq.npcs = []statedb.Npc{{
		ID:          dbutil.ToPgtype(npcID),
		CampaignID:  dbutil.ToPgtype(campaignID),
		Name:        "Guard",
		Description: pgtype.Text{String: "Watchful", Valid: true},
		Disposition: 15,
		LocationID:  dbutil.ToPgtype(locationID),
		Alive:       true,
	}}
	mq.quests = []statedb.Quest{{
		ID:          dbutil.ToPgtype(questID),
		CampaignID:  dbutil.ToPgtype(campaignID),
		Title:       "Find Relic",
		Description: pgtype.Text{String: "Seek the old relic", Valid: true},
		QuestType:   "short_term",
		Status:      "active",
	}}
	mq.objectivesByQuests = []statedb.QuestObjective{{
		ID:          dbutil.ToPgtype(objectiveID),
		QuestID:     dbutil.ToPgtype(questID),
		Description: "Search the ruins",
		OrderIndex:  1,
	}}
	mq.items = []statedb.Item{{
		ID:                dbutil.ToPgtype(itemID),
		CampaignID:        dbutil.ToPgtype(campaignID),
		PlayerCharacterID: dbutil.ToPgtype(playerID),
		Name:              "Potion",
		ItemType:          "consumable",
		Quantity:          2,
	}}
	mq.worldFacts = []statedb.WorldFact{{
		ID:         dbutil.ToPgtype(factID),
		CampaignID: dbutil.ToPgtype(campaignID),
		Fact:       "The moon is dimming",
		Category:   "lore",
		Source:     "oracle",
	}}

	sm := newStateManagerWithQuerier(mq)
	state, err := sm.GatherState(context.Background(), campaignID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state.Campaign.ID != campaignID {
		t.Fatalf("expected campaign %v, got %v", campaignID, state.Campaign.ID)
	}
	if state.Player.ID != playerID {
		t.Fatalf("expected player %v, got %v", playerID, state.Player.ID)
	}
	if state.CurrentLocation.ID != locationID {
		t.Fatalf("expected location %v, got %v", locationID, state.CurrentLocation.ID)
	}
	if len(state.CurrentLocationConnections) != 1 {
		t.Fatalf("expected 1 location connection, got %d", len(state.CurrentLocationConnections))
	}
	if len(state.NearbyNPCs) != 1 || state.NearbyNPCs[0].ID != npcID {
		t.Fatalf("expected guard npc in state, got %+v", state.NearbyNPCs)
	}
	if len(state.ActiveQuests) != 1 || state.ActiveQuests[0].ID != questID {
		t.Fatalf("expected active quest in state, got %+v", state.ActiveQuests)
	}
	objectives := state.ActiveQuestObjectives[questID]
	if len(objectives) != 1 || objectives[0].ID != objectiveID {
		t.Fatalf("expected quest objective in state, got %+v", objectives)
	}
	if len(state.PlayerInventory) != 1 || state.PlayerInventory[0].ID != itemID {
		t.Fatalf("expected inventory item in state, got %+v", state.PlayerInventory)
	}
	if len(state.WorldFacts) != 1 || state.WorldFacts[0].ID != factID {
		t.Fatalf("expected world fact in state, got %+v", state.WorldFacts)
	}
}

func TestSaveSessionLogPersistsMappedFields(t *testing.T) {
	q := newMockQuerier()
	sm := newStateManagerWithQuerier(q)
	campaignID := uuid.New()
	locationID := uuid.New()
	npcID := uuid.New()

	err := sm.SaveSessionLog(context.Background(), domain.SessionLog{
		CampaignID:   campaignID,
		TurnNumber:   4,
		PlayerInput:  "inspect the sigil",
		InputType:    domain.GameAction,
		LLMResponse:  "The sigil glows faintly.",
		ToolCalls:    []byte(`[{"tool":"describe_scene"}]`),
		LocationID:   &locationID,
		NPCsInvolved: []uuid.UUID{npcID},
	})
	if err != nil {
		t.Fatalf("SaveSessionLog() error = %v", err)
	}
	if q.lastSessionLog == nil {
		t.Fatal("expected session log insert to be recorded")
	}
	if got := dbutil.FromPgtype(q.lastSessionLog.CampaignID); got != campaignID {
		t.Fatalf("expected campaign id %s, got %s", campaignID, got)
	}
	if q.lastSessionLog.TurnNumber != 4 {
		t.Fatalf("expected turn number 4, got %d", q.lastSessionLog.TurnNumber)
	}
	if q.lastSessionLog.InputType != string(domain.GameAction) {
		t.Fatalf("expected input type %q, got %q", domain.GameAction, q.lastSessionLog.InputType)
	}
	if got := dbutil.FromPgtype(q.lastSessionLog.LocationID); got != locationID {
		t.Fatalf("expected location id %s, got %s", locationID, got)
	}
	if len(q.lastSessionLog.NpcsInvolved) != 1 || dbutil.FromPgtype(q.lastSessionLog.NpcsInvolved[0]) != npcID {
		t.Fatal("expected npc ids to be converted to pgtype UUIDs")
	}
}

func TestSaveSessionLogRejectsInvalidDomainLog(t *testing.T) {
	q := newMockQuerier()
	sm := newStateManagerWithQuerier(q)

	err := sm.SaveSessionLog(context.Background(), domain.SessionLog{})
	if err == nil {
		t.Fatal("expected validation error for invalid session log")
	}
	if q.lastSessionLog != nil {
		t.Fatal("expected invalid log not to hit the database")
	}
}

func TestGatherState_HandlesMissingDataGracefully(t *testing.T) {
	mq := newMockQuerier()
	campaignID := uuid.New()
	userID := uuid.New()
	mq.campaign = statedb.Campaign{
		ID:        dbutil.ToPgtype(campaignID),
		Name:      "New Campaign",
		Status:    "active",
		CreatedBy: dbutil.ToPgtype(userID),
	}

	sm := newStateManagerWithQuerier(mq)
	state, err := sm.GatherState(context.Background(), campaignID)
	if err != nil {
		t.Fatalf("unexpected error for sparse campaign data: %v", err)
	}

	if state.Campaign.ID != campaignID {
		t.Fatalf("expected campaign to be loaded")
	}
	if state.Player.ID != uuid.Nil {
		t.Fatalf("expected Player.ID to be uuid.Nil when no player characters exist")
	}
	if state.CurrentLocation.ID != uuid.Nil {
		t.Fatalf("expected empty location when player has none")
	}
	if len(state.NearbyNPCs) != 0 {
		t.Fatalf("expected no nearby npcs, got %d", len(state.NearbyNPCs))
	}
	if len(state.ActiveQuests) != 0 {
		t.Fatalf("expected no active quests, got %d", len(state.ActiveQuests))
	}
	if len(state.ActiveQuestObjectives) != 0 {
		t.Fatalf("expected no quest objectives, got %d", len(state.ActiveQuestObjectives))
	}
	if len(state.PlayerInventory) != 0 {
		t.Fatalf("expected no inventory, got %d", len(state.PlayerInventory))
	}
	if len(state.WorldFacts) != 0 {
		t.Fatalf("expected no world facts, got %d", len(state.WorldFacts))
	}

	if state.NearbyNPCs == nil || state.ActiveQuests == nil || state.PlayerInventory == nil || state.WorldFacts == nil || state.CurrentLocationConnections == nil {
		t.Fatalf("expected empty slices to be initialized, got nil collection")
	}
}

func TestGatherState_SelectsNewestPlayerCharacter(t *testing.T) {
	mq := newMockQuerier()
	campaignID := uuid.New()
	userID := uuid.New()
	oldPlayerID := uuid.New()
	newPlayerID := uuid.New()

	mq.campaign = statedb.Campaign{
		ID:        dbutil.ToPgtype(campaignID),
		Name:      "Campaign",
		Status:    "active",
		CreatedBy: dbutil.ToPgtype(userID),
	}
	mq.playerCharacters = []statedb.PlayerCharacter{
		{
			ID:         dbutil.ToPgtype(oldPlayerID),
			CampaignID: dbutil.ToPgtype(campaignID),
			UserID:     dbutil.ToPgtype(userID),
			Name:       "Old Hero",
		},
		{
			ID:         dbutil.ToPgtype(newPlayerID),
			CampaignID: dbutil.ToPgtype(campaignID),
			UserID:     dbutil.ToPgtype(userID),
			Name:       "New Hero",
		},
	}

	sm := newStateManagerWithQuerier(mq)
	state, err := sm.GatherState(context.Background(), campaignID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Player.ID != newPlayerID {
		t.Fatalf("expected newest player character %v, got %v", newPlayerID, state.Player.ID)
	}
}

func TestGatherState_MissingCampaignReturnsError(t *testing.T) {
	mq := newMockQuerier()
	sm := newStateManagerWithQuerier(mq)

	_, err := sm.GatherState(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error when campaign is missing")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected wrapped pgx.ErrNoRows, got %v", err)
	}
}

func TestGatherState_MissingLocationReturnsError(t *testing.T) {
	mq := newMockQuerier()
	campaignID := uuid.New()
	userID := uuid.New()
	locationID := uuid.New()

	mq.campaign = statedb.Campaign{
		ID:        dbutil.ToPgtype(campaignID),
		Name:      "Campaign",
		Status:    "active",
		CreatedBy: dbutil.ToPgtype(userID),
	}
	mq.playerCharacters = []statedb.PlayerCharacter{{
		ID:                dbutil.ToPgtype(uuid.New()),
		CampaignID:        dbutil.ToPgtype(campaignID),
		UserID:            dbutil.ToPgtype(userID),
		Name:              "Hero",
		CurrentLocationID: dbutil.ToPgtype(locationID),
	}}

	sm := newStateManagerWithQuerier(mq)
	_, err := sm.GatherState(context.Background(), campaignID)
	if err == nil {
		t.Fatal("expected error when current location does not exist")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected wrapped pgx.ErrNoRows, got %v", err)
	}
}
