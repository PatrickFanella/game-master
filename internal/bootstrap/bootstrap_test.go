package bootstrap_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/bootstrap"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

// stubQuerier is a minimal in-memory implementation of statedb.Querier used
// for unit-testing the bootstrap package without a real database.
type stubQuerier struct {
	users             []statedb.User
	campaigns         []statedb.Campaign
	locations         []statedb.Location
	createUserFn      func(ctx context.Context, name string) (statedb.User, error)
	getUserByNameFn   func(ctx context.Context, name string) (statedb.User, error)
	createCampFn      func(ctx context.Context, arg statedb.CreateCampaignParams) (statedb.Campaign, error)
	createLocFn       func(ctx context.Context, arg statedb.CreateLocationParams) (statedb.Location, error)
}

// Minimal implementations of statedb.Querier that the bootstrap package uses.

func (s *stubQuerier) ListUsers(ctx context.Context) ([]statedb.User, error) {
	return s.users, nil
}

func (s *stubQuerier) GetUserByName(ctx context.Context, name string) (statedb.User, error) {
	if s.getUserByNameFn != nil {
		return s.getUserByNameFn(ctx, name)
	}
	for _, u := range s.users {
		if u.Name == name {
			return u, nil
		}
	}
	return statedb.User{}, pgx.ErrNoRows
}

func (s *stubQuerier) CreateUser(ctx context.Context, name string) (statedb.User, error) {
	if s.createUserFn != nil {
		return s.createUserFn(ctx, name)
	}
	u := statedb.User{Name: name}
	u.ID = pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	s.users = append(s.users, u)
	return u, nil
}

func (s *stubQuerier) ListCampaignsByUser(ctx context.Context, createdBy pgtype.UUID) ([]statedb.Campaign, error) {
	var out []statedb.Campaign
	for _, c := range s.campaigns {
		if c.CreatedBy == createdBy {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *stubQuerier) CreateCampaign(ctx context.Context, arg statedb.CreateCampaignParams) (statedb.Campaign, error) {
	if s.createCampFn != nil {
		return s.createCampFn(ctx, arg)
	}
	c := statedb.Campaign{
		Name:      arg.Name,
		Status:    arg.Status,
		CreatedBy: arg.CreatedBy,
	}
	c.ID = pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	s.campaigns = append(s.campaigns, c)
	return c, nil
}

func (s *stubQuerier) CreateLocation(ctx context.Context, arg statedb.CreateLocationParams) (statedb.Location, error) {
	if s.createLocFn != nil {
		return s.createLocFn(ctx, arg)
	}
	l := statedb.Location{
		CampaignID:   arg.CampaignID,
		Name:         arg.Name,
		Description:  arg.Description,
		LocationType: arg.LocationType,
	}
	l.ID = pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	s.locations = append(s.locations, l)
	return l, nil
}

// Remaining Querier methods are no-ops for this stub.

func (s *stubQuerier) CompleteObjective(ctx context.Context, id pgtype.UUID) (statedb.QuestObjective, error) {
	return statedb.QuestObjective{}, nil
}
func (s *stubQuerier) CreateBeliefSystem(ctx context.Context, arg statedb.CreateBeliefSystemParams) (statedb.BeliefSystem, error) {
	return statedb.BeliefSystem{}, nil
}
func (s *stubQuerier) CreateConnection(ctx context.Context, arg statedb.CreateConnectionParams) (statedb.LocationConnection, error) {
	return statedb.LocationConnection{}, nil
}
func (s *stubQuerier) CreateCulture(ctx context.Context, arg statedb.CreateCultureParams) (statedb.Culture, error) {
	return statedb.Culture{}, nil
}
func (s *stubQuerier) CreateEconomicSystem(ctx context.Context, arg statedb.CreateEconomicSystemParams) (statedb.EconomicSystem, error) {
	return statedb.EconomicSystem{}, nil
}
func (s *stubQuerier) CreateFact(ctx context.Context, arg statedb.CreateFactParams) (statedb.WorldFact, error) {
	return statedb.WorldFact{}, nil
}
func (s *stubQuerier) CreateFaction(ctx context.Context, arg statedb.CreateFactionParams) (statedb.Faction, error) {
	return statedb.Faction{}, nil
}
func (s *stubQuerier) CreateFactionRelationship(ctx context.Context, arg statedb.CreateFactionRelationshipParams) (statedb.FactionRelationship, error) {
	return statedb.FactionRelationship{}, nil
}
func (s *stubQuerier) CreateItem(ctx context.Context, arg statedb.CreateItemParams) (statedb.Item, error) {
	return statedb.Item{}, nil
}
func (s *stubQuerier) CreateLanguage(ctx context.Context, arg statedb.CreateLanguageParams) (statedb.Language, error) {
	return statedb.Language{}, nil
}
func (s *stubQuerier) CreateMemory(ctx context.Context, arg statedb.CreateMemoryParams) (statedb.Memory, error) {
	return statedb.Memory{}, nil
}
func (s *stubQuerier) CreateNPC(ctx context.Context, arg statedb.CreateNPCParams) (statedb.Npc, error) {
	return statedb.Npc{}, nil
}
func (s *stubQuerier) CreateObjective(ctx context.Context, arg statedb.CreateObjectiveParams) (statedb.QuestObjective, error) {
	return statedb.QuestObjective{}, nil
}
func (s *stubQuerier) CreatePlayerCharacter(ctx context.Context, arg statedb.CreatePlayerCharacterParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (s *stubQuerier) CreateQuest(ctx context.Context, arg statedb.CreateQuestParams) (statedb.Quest, error) {
	return statedb.Quest{}, nil
}
func (s *stubQuerier) CreateRelationship(ctx context.Context, arg statedb.CreateRelationshipParams) (statedb.EntityRelationship, error) {
	return statedb.EntityRelationship{}, nil
}
func (s *stubQuerier) CreateSessionLog(ctx context.Context, arg statedb.CreateSessionLogParams) (statedb.SessionLog, error) {
	return statedb.SessionLog{}, nil
}
func (s *stubQuerier) DeleteBeliefSystem(ctx context.Context, id pgtype.UUID) error { return nil }
func (s *stubQuerier) DeleteCampaign(ctx context.Context, id pgtype.UUID) error     { return nil }
func (s *stubQuerier) DeleteConnection(ctx context.Context, arg statedb.DeleteConnectionParams) error {
	return nil
}
func (s *stubQuerier) DeleteCulture(ctx context.Context, id pgtype.UUID) error        { return nil }
func (s *stubQuerier) DeleteEconomicSystem(ctx context.Context, id pgtype.UUID) error { return nil }
func (s *stubQuerier) DeleteItem(ctx context.Context, id pgtype.UUID) error           { return nil }
func (s *stubQuerier) DeleteLanguage(ctx context.Context, id pgtype.UUID) error       { return nil }
func (s *stubQuerier) DeleteRelationship(ctx context.Context, arg statedb.DeleteRelationshipParams) error {
	return nil
}
func (s *stubQuerier) DeleteUser(ctx context.Context, id pgtype.UUID) error { return nil }
func (s *stubQuerier) GetBeliefSystemByCulture(ctx context.Context, cultureID pgtype.UUID) (statedb.BeliefSystem, error) {
	return statedb.BeliefSystem{}, nil
}
func (s *stubQuerier) GetBeliefSystemByID(ctx context.Context, id pgtype.UUID) (statedb.BeliefSystem, error) {
	return statedb.BeliefSystem{}, nil
}
func (s *stubQuerier) GetCampaignByID(ctx context.Context, id pgtype.UUID) (statedb.Campaign, error) {
	return statedb.Campaign{}, nil
}
func (s *stubQuerier) GetConnectionsFromLocation(ctx context.Context, arg statedb.GetConnectionsFromLocationParams) ([]statedb.GetConnectionsFromLocationRow, error) {
	return nil, nil
}
func (s *stubQuerier) GetCultureByID(ctx context.Context, id pgtype.UUID) (statedb.Culture, error) {
	return statedb.Culture{}, nil
}
func (s *stubQuerier) GetEconomicSystemByID(ctx context.Context, id pgtype.UUID) (statedb.EconomicSystem, error) {
	return statedb.EconomicSystem{}, nil
}
func (s *stubQuerier) GetFactByID(ctx context.Context, id pgtype.UUID) (statedb.WorldFact, error) {
	return statedb.WorldFact{}, nil
}
func (s *stubQuerier) GetFactionByID(ctx context.Context, id pgtype.UUID) (statedb.Faction, error) {
	return statedb.Faction{}, nil
}
func (s *stubQuerier) GetFactionRelationships(ctx context.Context, factionID pgtype.UUID) ([]statedb.FactionRelationship, error) {
	return nil, nil
}
func (s *stubQuerier) GetItemByID(ctx context.Context, id pgtype.UUID) (statedb.Item, error) {
	return statedb.Item{}, nil
}
func (s *stubQuerier) GetLanguageByID(ctx context.Context, id pgtype.UUID) (statedb.Language, error) {
	return statedb.Language{}, nil
}
func (s *stubQuerier) GetLocationByID(ctx context.Context, id pgtype.UUID) (statedb.Location, error) {
	return statedb.Location{}, nil
}
func (s *stubQuerier) GetMemoryByID(ctx context.Context, id pgtype.UUID) (statedb.Memory, error) {
	return statedb.Memory{}, nil
}
func (s *stubQuerier) GetNPCByID(ctx context.Context, id pgtype.UUID) (statedb.Npc, error) {
	return statedb.Npc{}, nil
}
func (s *stubQuerier) GetPlayerCharacterByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.PlayerCharacter, error) {
	return nil, nil
}
func (s *stubQuerier) GetPlayerCharacterByID(ctx context.Context, id pgtype.UUID) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (s *stubQuerier) GetQuestByID(ctx context.Context, id pgtype.UUID) (statedb.Quest, error) {
	return statedb.Quest{}, nil
}
func (s *stubQuerier) GetRelationshipsBetween(ctx context.Context, arg statedb.GetRelationshipsBetweenParams) ([]statedb.EntityRelationship, error) {
	return nil, nil
}
func (s *stubQuerier) GetRelationshipsByEntity(ctx context.Context, arg statedb.GetRelationshipsByEntityParams) ([]statedb.EntityRelationship, error) {
	return nil, nil
}
func (s *stubQuerier) GetSessionLogByID(ctx context.Context, id pgtype.UUID) (statedb.SessionLog, error) {
	return statedb.SessionLog{}, nil
}
func (s *stubQuerier) GetUserByID(ctx context.Context, id pgtype.UUID) (statedb.User, error) {
	return statedb.User{}, nil
}
func (s *stubQuerier) KillNPC(ctx context.Context, id pgtype.UUID) (statedb.Npc, error) {
	return statedb.Npc{}, nil
}
func (s *stubQuerier) ListActiveFactsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.WorldFact, error) {
	return nil, nil
}
func (s *stubQuerier) ListActiveQuests(ctx context.Context, campaignID pgtype.UUID) ([]statedb.Quest, error) {
	return nil, nil
}
func (s *stubQuerier) ListAliveNPCsByLocation(ctx context.Context, arg statedb.ListAliveNPCsByLocationParams) ([]statedb.Npc, error) {
	return nil, nil
}
func (s *stubQuerier) ListBeliefSystemsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.BeliefSystem, error) {
	return nil, nil
}
func (s *stubQuerier) ListCulturesByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.Culture, error) {
	return nil, nil
}
func (s *stubQuerier) ListCulturesByLanguage(ctx context.Context, languageID pgtype.UUID) ([]statedb.Culture, error) {
	return nil, nil
}
func (s *stubQuerier) ListEconomicSystemsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.EconomicSystem, error) {
	return nil, nil
}
func (s *stubQuerier) ListFactionsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.Faction, error) {
	return nil, nil
}
func (s *stubQuerier) ListFactsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.WorldFact, error) {
	return nil, nil
}
func (s *stubQuerier) ListFactsByCategory(ctx context.Context, arg statedb.ListFactsByCategoryParams) ([]statedb.WorldFact, error) {
	return nil, nil
}
func (s *stubQuerier) ListItemsByPlayer(ctx context.Context, arg statedb.ListItemsByPlayerParams) ([]statedb.Item, error) {
	return nil, nil
}
func (s *stubQuerier) ListItemsByType(ctx context.Context, arg statedb.ListItemsByTypeParams) ([]statedb.Item, error) {
	return nil, nil
}
func (s *stubQuerier) ListLanguagesByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.Language, error) {
	return nil, nil
}
func (s *stubQuerier) ListLanguagesByFaction(ctx context.Context, factionID pgtype.UUID) ([]statedb.Language, error) {
	return nil, nil
}
func (s *stubQuerier) ListLocationsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.Location, error) {
	return nil, nil
}
func (s *stubQuerier) ListLocationsByRegion(ctx context.Context, arg statedb.ListLocationsByRegionParams) ([]statedb.Location, error) {
	return nil, nil
}
func (s *stubQuerier) ListMemoriesByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.Memory, error) {
	return nil, nil
}
func (s *stubQuerier) ListNPCsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.Npc, error) {
	return nil, nil
}
func (s *stubQuerier) ListNPCsByFaction(ctx context.Context, arg statedb.ListNPCsByFactionParams) ([]statedb.Npc, error) {
	return nil, nil
}
func (s *stubQuerier) ListNPCsByLocation(ctx context.Context, arg statedb.ListNPCsByLocationParams) ([]statedb.Npc, error) {
	return nil, nil
}
func (s *stubQuerier) ListObjectivesByQuest(ctx context.Context, questID pgtype.UUID) ([]statedb.QuestObjective, error) {
	return nil, nil
}
func (s *stubQuerier) ListObjectivesByQuests(ctx context.Context, questIds []pgtype.UUID) ([]statedb.QuestObjective, error) {
	return nil, nil
}
func (s *stubQuerier) ListQuestsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.Quest, error) {
	return nil, nil
}
func (s *stubQuerier) ListQuestsByType(ctx context.Context, arg statedb.ListQuestsByTypeParams) ([]statedb.Quest, error) {
	return nil, nil
}
func (s *stubQuerier) ListRecentSessionLogs(ctx context.Context, arg statedb.ListRecentSessionLogsParams) ([]statedb.SessionLog, error) {
	return nil, nil
}
func (s *stubQuerier) ListRelationshipsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.EntityRelationship, error) {
	return nil, nil
}
func (s *stubQuerier) ListSessionLogsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.SessionLog, error) {
	return nil, nil
}
func (s *stubQuerier) ListSessionLogsByLocation(ctx context.Context, arg statedb.ListSessionLogsByLocationParams) ([]statedb.SessionLog, error) {
	return nil, nil
}
func (s *stubQuerier) ListSubquestsByParentQuest(ctx context.Context, parentQuestID pgtype.UUID) ([]statedb.Quest, error) {
	return nil, nil
}
func (s *stubQuerier) Ping(ctx context.Context) (int32, error) { return 1, nil }
func (s *stubQuerier) SearchMemoriesBySimilarity(ctx context.Context, arg statedb.SearchMemoriesBySimilarityParams) ([]statedb.SearchMemoriesBySimilarityRow, error) {
	return nil, nil
}
func (s *stubQuerier) SearchMemoriesWithFilters(ctx context.Context, arg statedb.SearchMemoriesWithFiltersParams) ([]statedb.SearchMemoriesWithFiltersRow, error) {
	return nil, nil
}
func (s *stubQuerier) SupersedeFact(ctx context.Context, arg statedb.SupersedeFactParams) (statedb.WorldFact, error) {
	return statedb.WorldFact{}, nil
}
func (s *stubQuerier) TransferItem(ctx context.Context, arg statedb.TransferItemParams) (statedb.Item, error) {
	return statedb.Item{}, nil
}
func (s *stubQuerier) UpdateBeliefSystem(ctx context.Context, arg statedb.UpdateBeliefSystemParams) (statedb.BeliefSystem, error) {
	return statedb.BeliefSystem{}, nil
}
func (s *stubQuerier) UpdateCampaign(ctx context.Context, arg statedb.UpdateCampaignParams) (statedb.Campaign, error) {
	return statedb.Campaign{}, nil
}
func (s *stubQuerier) UpdateCampaignStatus(ctx context.Context, arg statedb.UpdateCampaignStatusParams) (statedb.Campaign, error) {
	return statedb.Campaign{}, nil
}
func (s *stubQuerier) UpdateCulture(ctx context.Context, arg statedb.UpdateCultureParams) (statedb.Culture, error) {
	return statedb.Culture{}, nil
}
func (s *stubQuerier) UpdateEconomicSystem(ctx context.Context, arg statedb.UpdateEconomicSystemParams) (statedb.EconomicSystem, error) {
	return statedb.EconomicSystem{}, nil
}
func (s *stubQuerier) UpdateFaction(ctx context.Context, arg statedb.UpdateFactionParams) (statedb.Faction, error) {
	return statedb.Faction{}, nil
}
func (s *stubQuerier) UpdateFactionRelationship(ctx context.Context, arg statedb.UpdateFactionRelationshipParams) (statedb.FactionRelationship, error) {
	return statedb.FactionRelationship{}, nil
}
func (s *stubQuerier) UpdateItem(ctx context.Context, arg statedb.UpdateItemParams) (statedb.Item, error) {
	return statedb.Item{}, nil
}
func (s *stubQuerier) UpdateItemEquipped(ctx context.Context, arg statedb.UpdateItemEquippedParams) (statedb.Item, error) {
	return statedb.Item{}, nil
}
func (s *stubQuerier) UpdateItemQuantity(ctx context.Context, arg statedb.UpdateItemQuantityParams) (statedb.Item, error) {
	return statedb.Item{}, nil
}
func (s *stubQuerier) UpdateLanguage(ctx context.Context, arg statedb.UpdateLanguageParams) (statedb.Language, error) {
	return statedb.Language{}, nil
}
func (s *stubQuerier) UpdateLocation(ctx context.Context, arg statedb.UpdateLocationParams) (statedb.Location, error) {
	return statedb.Location{}, nil
}
func (s *stubQuerier) UpdateNPC(ctx context.Context, arg statedb.UpdateNPCParams) (statedb.Npc, error) {
	return statedb.Npc{}, nil
}
func (s *stubQuerier) UpdateNPCDisposition(ctx context.Context, arg statedb.UpdateNPCDispositionParams) (statedb.Npc, error) {
	return statedb.Npc{}, nil
}
func (s *stubQuerier) UpdateNPCLocation(ctx context.Context, arg statedb.UpdateNPCLocationParams) (statedb.Npc, error) {
	return statedb.Npc{}, nil
}
func (s *stubQuerier) UpdateObjective(ctx context.Context, arg statedb.UpdateObjectiveParams) (statedb.QuestObjective, error) {
	return statedb.QuestObjective{}, nil
}
func (s *stubQuerier) UpdatePlayerCharacter(ctx context.Context, arg statedb.UpdatePlayerCharacterParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (s *stubQuerier) UpdatePlayerExperience(ctx context.Context, arg statedb.UpdatePlayerExperienceParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (s *stubQuerier) UpdatePlayerLevel(ctx context.Context, arg statedb.UpdatePlayerLevelParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (s *stubQuerier) UpdatePlayerAbilities(ctx context.Context, arg statedb.UpdatePlayerAbilitiesParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (s *stubQuerier) UpdatePlayerHP(ctx context.Context, arg statedb.UpdatePlayerHPParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (s *stubQuerier) UpdatePlayerLocation(ctx context.Context, arg statedb.UpdatePlayerLocationParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (s *stubQuerier) UpdatePlayerStats(ctx context.Context, arg statedb.UpdatePlayerStatsParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (s *stubQuerier) UpdatePlayerAbilities(ctx context.Context, arg statedb.UpdatePlayerAbilitiesParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (s *stubQuerier) UpdatePlayerStatus(ctx context.Context, arg statedb.UpdatePlayerStatusParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (s *stubQuerier) UpdateQuest(ctx context.Context, arg statedb.UpdateQuestParams) (statedb.Quest, error) {
	return statedb.Quest{}, nil
}
func (s *stubQuerier) UpdateQuestStatus(ctx context.Context, arg statedb.UpdateQuestStatusParams) (statedb.Quest, error) {
	return statedb.Quest{}, nil
}
func (s *stubQuerier) UpdateRelationship(ctx context.Context, arg statedb.UpdateRelationshipParams) (statedb.EntityRelationship, error) {
	return statedb.EntityRelationship{}, nil
}
func (s *stubQuerier) UpdateUser(ctx context.Context, arg statedb.UpdateUserParams) (statedb.User, error) {
	return statedb.User{}, nil
}

// --- Tests ---

func TestRun_FirstBootCreatesUserAndCampaign(t *testing.T) {
	q := &stubQuerier{}
	result, err := bootstrap.Run(context.Background(), q)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.User.Name != bootstrap.DefaultUserName {
		t.Errorf("expected user %q, got %q", bootstrap.DefaultUserName, result.User.Name)
	}
	if len(result.Campaigns) != 1 {
		t.Fatalf("expected 1 campaign, got %d", len(result.Campaigns))
	}
	if result.Campaigns[0].Name != bootstrap.DefaultCampaignName {
		t.Errorf("expected campaign %q, got %q", bootstrap.DefaultCampaignName, result.Campaigns[0].Name)
	}

	// Verify the starting location was created.
	if len(q.locations) != 1 {
		t.Fatalf("expected 1 location, got %d", len(q.locations))
	}
	if q.locations[0].Name != bootstrap.DefaultLocationName {
		t.Errorf("expected location %q, got %q", bootstrap.DefaultLocationName, q.locations[0].Name)
	}
}

func TestRun_ExistingUserReused(t *testing.T) {
	existingUser := statedb.User{
		Name: bootstrap.DefaultUserName,
	}
	existingUser.ID = pgtype.UUID{Bytes: [16]byte{99}, Valid: true}

	q := &stubQuerier{users: []statedb.User{existingUser}}
	result, err := bootstrap.Run(context.Background(), q)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// Should not create a new user – ID must match the existing one.
	if result.User.ID != existingUser.ID {
		t.Errorf("expected existing user ID to be reused")
	}
	// Only one user should exist.
	if len(q.users) != 1 {
		t.Errorf("expected 1 user in store, got %d", len(q.users))
	}
}

func TestRun_ExistingCampaignsReturned(t *testing.T) {
	userID := pgtype.UUID{Bytes: [16]byte{5}, Valid: true}
	existingUser := statedb.User{Name: bootstrap.DefaultUserName, ID: userID}
	campaignID := pgtype.UUID{Bytes: [16]byte{6}, Valid: true}
	existingCampaign := statedb.Campaign{
		ID:        campaignID,
		Name:      "Existing Adventure",
		Status:    "active",
		CreatedBy: userID,
	}

	q := &stubQuerier{
		users:     []statedb.User{existingUser},
		campaigns: []statedb.Campaign{existingCampaign},
	}

	result, err := bootstrap.Run(context.Background(), q)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(result.Campaigns) != 1 {
		t.Fatalf("expected 1 existing campaign, got %d", len(result.Campaigns))
	}
	if result.Campaigns[0].ID != campaignID {
		t.Errorf("expected existing campaign ID to be returned")
	}
	// Should not create additional campaigns.
	if len(q.campaigns) != 1 {
		t.Errorf("expected 1 campaign in store, got %d", len(q.campaigns))
	}
}

func TestRun_CreateUserError(t *testing.T) {
	q := &stubQuerier{
		createUserFn: func(_ context.Context, _ string) (statedb.User, error) {
			return statedb.User{}, errors.New("db error")
		},
	}
	_, err := bootstrap.Run(context.Background(), q)
	if err == nil {
		t.Fatal("expected error when CreateUser fails")
	}
}

func TestRun_GetUserByNameNonNoRowsError(t *testing.T) {
	q := &stubQuerier{
		getUserByNameFn: func(_ context.Context, _ string) (statedb.User, error) {
			return statedb.User{}, errors.New("connection error")
		},
	}
	_, err := bootstrap.Run(context.Background(), q)
	if err == nil {
		t.Fatal("expected error when GetUserByName fails with non-NoRows error")
	}
}

func TestRun_CreateCampaignError(t *testing.T) {
	q := &stubQuerier{
		createCampFn: func(_ context.Context, _ statedb.CreateCampaignParams) (statedb.Campaign, error) {
			return statedb.Campaign{}, errors.New("db error")
		},
	}
	_, err := bootstrap.Run(context.Background(), q)
	if err == nil {
		t.Fatal("expected error when CreateCampaign fails")
	}
}

func TestCreateCampaign_CreatesLocationForCampaign(t *testing.T) {
	userID := pgtype.UUID{Bytes: [16]byte{7}, Valid: true}
	q := &stubQuerier{}

	camp, err := bootstrap.CreateCampaign(context.Background(), q, userID, "My Campaign")
	if err != nil {
		t.Fatalf("CreateCampaign returned error: %v", err)
	}

	if camp.Name != "My Campaign" {
		t.Errorf("expected campaign name %q, got %q", "My Campaign", camp.Name)
	}
	if len(q.locations) != 1 {
		t.Fatalf("expected 1 location, got %d", len(q.locations))
	}
	if q.locations[0].CampaignID != camp.ID {
		t.Errorf("expected location to belong to the new campaign")
	}
}

func TestCreateCampaign_EmptyNameReturnsError(t *testing.T) {
	userID := pgtype.UUID{Bytes: [16]byte{8}, Valid: true}
	q := &stubQuerier{}

	_, err := bootstrap.CreateCampaign(context.Background(), q, userID, "")
	if err == nil {
		t.Fatal("expected error for empty campaign name")
	}
}

func TestCreateCampaign_WhitespaceOnlyNameReturnsError(t *testing.T) {
	userID := pgtype.UUID{Bytes: [16]byte{9}, Valid: true}
	q := &stubQuerier{}

	_, err := bootstrap.CreateCampaign(context.Background(), q, userID, "   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only campaign name")
	}
}

func TestCreateCampaign_TrimsWhitespace(t *testing.T) {
	userID := pgtype.UUID{Bytes: [16]byte{10}, Valid: true}
	q := &stubQuerier{}

	camp, err := bootstrap.CreateCampaign(context.Background(), q, userID, "  My Adventure  ")
	if err != nil {
		t.Fatalf("CreateCampaign returned error: %v", err)
	}
	if camp.Name != "My Adventure" {
		t.Errorf("expected trimmed name %q, got %q", "My Adventure", camp.Name)
	}
}
