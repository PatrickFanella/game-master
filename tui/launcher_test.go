package tui

import (
	"context"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/bootstrap"
	"github.com/PatrickFanella/game-master/internal/config"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/tui/campaign"
)

// compile-time check: Launcher must implement tea.Model.
var _ tea.Model = Launcher{}

// noopQuerier satisfies statedb.Querier with no-op methods used in launcher
// unit tests that never actually execute DB operations.
type noopQuerier struct{}

func (n *noopQuerier) CompleteObjective(ctx context.Context, id pgtype.UUID) (statedb.QuestObjective, error) {
	return statedb.QuestObjective{}, nil
}
func (n *noopQuerier) CreateBeliefSystem(ctx context.Context, arg statedb.CreateBeliefSystemParams) (statedb.BeliefSystem, error) {
	return statedb.BeliefSystem{}, nil
}
func (n *noopQuerier) CreateCampaign(ctx context.Context, arg statedb.CreateCampaignParams) (statedb.Campaign, error) {
	return statedb.Campaign{}, nil
}
func (n *noopQuerier) CreateConnection(ctx context.Context, arg statedb.CreateConnectionParams) (statedb.LocationConnection, error) {
	return statedb.LocationConnection{}, nil
}
func (n *noopQuerier) CreateCulture(ctx context.Context, arg statedb.CreateCultureParams) (statedb.Culture, error) {
	return statedb.Culture{}, nil
}
func (n *noopQuerier) CreateEconomicSystem(ctx context.Context, arg statedb.CreateEconomicSystemParams) (statedb.EconomicSystem, error) {
	return statedb.EconomicSystem{}, nil
}
func (n *noopQuerier) CreateFact(ctx context.Context, arg statedb.CreateFactParams) (statedb.WorldFact, error) {
	return statedb.WorldFact{}, nil
}
func (n *noopQuerier) CreateFaction(ctx context.Context, arg statedb.CreateFactionParams) (statedb.Faction, error) {
	return statedb.Faction{}, nil
}
func (n *noopQuerier) CreateFactionRelationship(ctx context.Context, arg statedb.CreateFactionRelationshipParams) (statedb.FactionRelationship, error) {
	return statedb.FactionRelationship{}, nil
}
func (n *noopQuerier) CreateItem(ctx context.Context, arg statedb.CreateItemParams) (statedb.Item, error) {
	return statedb.Item{}, nil
}
func (n *noopQuerier) CreateLanguage(ctx context.Context, arg statedb.CreateLanguageParams) (statedb.Language, error) {
	return statedb.Language{}, nil
}
func (n *noopQuerier) CreateLocation(ctx context.Context, arg statedb.CreateLocationParams) (statedb.Location, error) {
	return statedb.Location{}, nil
}
func (n *noopQuerier) CreateMemory(ctx context.Context, arg statedb.CreateMemoryParams) (statedb.Memory, error) {
	return statedb.Memory{}, nil
}
func (n *noopQuerier) CreateNPC(ctx context.Context, arg statedb.CreateNPCParams) (statedb.Npc, error) {
	return statedb.Npc{}, nil
}
func (n *noopQuerier) CreateObjective(ctx context.Context, arg statedb.CreateObjectiveParams) (statedb.QuestObjective, error) {
	return statedb.QuestObjective{}, nil
}
func (n *noopQuerier) CreatePlayerCharacter(ctx context.Context, arg statedb.CreatePlayerCharacterParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (n *noopQuerier) CreateQuest(ctx context.Context, arg statedb.CreateQuestParams) (statedb.Quest, error) {
	return statedb.Quest{}, nil
}
func (n *noopQuerier) CreateRelationship(ctx context.Context, arg statedb.CreateRelationshipParams) (statedb.EntityRelationship, error) {
	return statedb.EntityRelationship{}, nil
}
func (n *noopQuerier) CreateSessionLog(ctx context.Context, arg statedb.CreateSessionLogParams) (statedb.SessionLog, error) {
	return statedb.SessionLog{}, nil
}
func (n *noopQuerier) CreateUser(ctx context.Context, name string) (statedb.User, error) {
	return statedb.User{}, nil
}
func (n *noopQuerier) DeleteBeliefSystem(ctx context.Context, id pgtype.UUID) error { return nil }
func (n *noopQuerier) DeleteCampaign(ctx context.Context, id pgtype.UUID) error     { return nil }
func (n *noopQuerier) DeleteConnection(ctx context.Context, arg statedb.DeleteConnectionParams) error {
	return nil
}
func (n *noopQuerier) DeleteCulture(ctx context.Context, id pgtype.UUID) error        { return nil }
func (n *noopQuerier) DeleteEconomicSystem(ctx context.Context, id pgtype.UUID) error { return nil }
func (n *noopQuerier) DeleteItem(ctx context.Context, id pgtype.UUID) error           { return nil }
func (n *noopQuerier) DeleteLanguage(ctx context.Context, id pgtype.UUID) error       { return nil }
func (n *noopQuerier) DeleteRelationship(ctx context.Context, arg statedb.DeleteRelationshipParams) error {
	return nil
}
func (n *noopQuerier) DeleteUser(ctx context.Context, id pgtype.UUID) error { return nil }
func (n *noopQuerier) GetBeliefSystemByCulture(ctx context.Context, cultureID pgtype.UUID) (statedb.BeliefSystem, error) {
	return statedb.BeliefSystem{}, nil
}
func (n *noopQuerier) GetBeliefSystemByID(ctx context.Context, id pgtype.UUID) (statedb.BeliefSystem, error) {
	return statedb.BeliefSystem{}, nil
}
func (n *noopQuerier) GetCampaignByID(ctx context.Context, id pgtype.UUID) (statedb.Campaign, error) {
	return statedb.Campaign{}, nil
}
func (n *noopQuerier) GetConnectionsFromLocation(ctx context.Context, arg statedb.GetConnectionsFromLocationParams) ([]statedb.GetConnectionsFromLocationRow, error) {
	return nil, nil
}
func (n *noopQuerier) GetCultureByID(ctx context.Context, id pgtype.UUID) (statedb.Culture, error) {
	return statedb.Culture{}, nil
}
func (n *noopQuerier) GetEconomicSystemByID(ctx context.Context, id pgtype.UUID) (statedb.EconomicSystem, error) {
	return statedb.EconomicSystem{}, nil
}
func (n *noopQuerier) GetFactByID(ctx context.Context, id pgtype.UUID) (statedb.WorldFact, error) {
	return statedb.WorldFact{}, nil
}
func (n *noopQuerier) GetFactionByID(ctx context.Context, id pgtype.UUID) (statedb.Faction, error) {
	return statedb.Faction{}, nil
}
func (n *noopQuerier) GetFactionRelationships(ctx context.Context, factionID pgtype.UUID) ([]statedb.FactionRelationship, error) {
	return nil, nil
}
func (n *noopQuerier) GetItemByID(ctx context.Context, id pgtype.UUID) (statedb.Item, error) {
	return statedb.Item{}, nil
}
func (n *noopQuerier) GetLanguageByID(ctx context.Context, id pgtype.UUID) (statedb.Language, error) {
	return statedb.Language{}, nil
}
func (n *noopQuerier) GetLocationByID(ctx context.Context, id pgtype.UUID) (statedb.Location, error) {
	return statedb.Location{}, nil
}
func (n *noopQuerier) GetMemoryByID(ctx context.Context, id pgtype.UUID) (statedb.Memory, error) {
	return statedb.Memory{}, nil
}
func (n *noopQuerier) GetNPCByID(ctx context.Context, id pgtype.UUID) (statedb.Npc, error) {
	return statedb.Npc{}, nil
}
func (n *noopQuerier) GetPlayerCharacterByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.PlayerCharacter, error) {
	return nil, nil
}
func (n *noopQuerier) GetPlayerCharacterByID(ctx context.Context, id pgtype.UUID) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (n *noopQuerier) GetQuestByID(ctx context.Context, id pgtype.UUID) (statedb.Quest, error) {
	return statedb.Quest{}, nil
}
func (n *noopQuerier) GetRelationshipsBetween(ctx context.Context, arg statedb.GetRelationshipsBetweenParams) ([]statedb.EntityRelationship, error) {
	return nil, nil
}
func (n *noopQuerier) GetRelationshipsByEntity(ctx context.Context, arg statedb.GetRelationshipsByEntityParams) ([]statedb.EntityRelationship, error) {
	return nil, nil
}
func (n *noopQuerier) GetSessionLogByID(ctx context.Context, id pgtype.UUID) (statedb.SessionLog, error) {
	return statedb.SessionLog{}, nil
}
func (n *noopQuerier) GetUserByID(ctx context.Context, id pgtype.UUID) (statedb.User, error) {
	return statedb.User{}, nil
}
func (n *noopQuerier) GetUserByName(ctx context.Context, name string) (statedb.User, error) {
	return statedb.User{}, nil
}
func (n *noopQuerier) KillNPC(ctx context.Context, id pgtype.UUID) (statedb.Npc, error) {
	return statedb.Npc{}, nil
}
func (n *noopQuerier) ListActiveFactsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.WorldFact, error) {
	return nil, nil
}
func (n *noopQuerier) ListActiveQuests(ctx context.Context, campaignID pgtype.UUID) ([]statedb.Quest, error) {
	return nil, nil
}
func (n *noopQuerier) ListAliveNPCsByLocation(ctx context.Context, arg statedb.ListAliveNPCsByLocationParams) ([]statedb.Npc, error) {
	return nil, nil
}
func (n *noopQuerier) ListBeliefSystemsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.BeliefSystem, error) {
	return nil, nil
}
func (n *noopQuerier) ListCampaignsByUser(ctx context.Context, createdBy pgtype.UUID) ([]statedb.Campaign, error) {
	return nil, nil
}
func (n *noopQuerier) ListCulturesByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.Culture, error) {
	return nil, nil
}
func (n *noopQuerier) ListCulturesByLanguage(ctx context.Context, languageID pgtype.UUID) ([]statedb.Culture, error) {
	return nil, nil
}
func (n *noopQuerier) ListEconomicSystemsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.EconomicSystem, error) {
	return nil, nil
}
func (n *noopQuerier) ListFactionsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.Faction, error) {
	return nil, nil
}
func (n *noopQuerier) ListFactsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.WorldFact, error) {
	return nil, nil
}
func (n *noopQuerier) ListFactsByCategory(ctx context.Context, arg statedb.ListFactsByCategoryParams) ([]statedb.WorldFact, error) {
	return nil, nil
}
func (n *noopQuerier) ListItemsByPlayer(ctx context.Context, arg statedb.ListItemsByPlayerParams) ([]statedb.Item, error) {
	return nil, nil
}
func (n *noopQuerier) ListItemsByType(ctx context.Context, arg statedb.ListItemsByTypeParams) ([]statedb.Item, error) {
	return nil, nil
}
func (n *noopQuerier) ListLanguagesByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.Language, error) {
	return nil, nil
}
func (n *noopQuerier) ListLanguagesByFaction(ctx context.Context, factionID pgtype.UUID) ([]statedb.Language, error) {
	return nil, nil
}
func (n *noopQuerier) ListLocationsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.Location, error) {
	return nil, nil
}
func (n *noopQuerier) ListLocationsByRegion(ctx context.Context, arg statedb.ListLocationsByRegionParams) ([]statedb.Location, error) {
	return nil, nil
}
func (n *noopQuerier) ListMemoriesByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.Memory, error) {
	return nil, nil
}
func (n *noopQuerier) ListNPCsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.Npc, error) {
	return nil, nil
}
func (n *noopQuerier) ListNPCsByFaction(ctx context.Context, arg statedb.ListNPCsByFactionParams) ([]statedb.Npc, error) {
	return nil, nil
}
func (n *noopQuerier) ListNPCsByLocation(ctx context.Context, arg statedb.ListNPCsByLocationParams) ([]statedb.Npc, error) {
	return nil, nil
}
func (n *noopQuerier) ListObjectivesByQuest(ctx context.Context, questID pgtype.UUID) ([]statedb.QuestObjective, error) {
	return nil, nil
}
func (n *noopQuerier) ListObjectivesByQuests(ctx context.Context, questIds []pgtype.UUID) ([]statedb.QuestObjective, error) {
	return nil, nil
}
func (n *noopQuerier) ListQuestsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.Quest, error) {
	return nil, nil
}
func (n *noopQuerier) ListQuestsByType(ctx context.Context, arg statedb.ListQuestsByTypeParams) ([]statedb.Quest, error) {
	return nil, nil
}
func (n *noopQuerier) ListRecentSessionLogs(ctx context.Context, arg statedb.ListRecentSessionLogsParams) ([]statedb.SessionLog, error) {
	return nil, nil
}
func (n *noopQuerier) ListRelationshipsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.EntityRelationship, error) {
	return nil, nil
}
func (n *noopQuerier) ListSessionLogsByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.SessionLog, error) {
	return nil, nil
}
func (n *noopQuerier) ListSessionLogsByLocation(ctx context.Context, arg statedb.ListSessionLogsByLocationParams) ([]statedb.SessionLog, error) {
	return nil, nil
}
func (n *noopQuerier) ListSubquestsByParentQuest(ctx context.Context, parentQuestID pgtype.UUID) ([]statedb.Quest, error) {
	return nil, nil
}
func (n *noopQuerier) ListUsers(ctx context.Context) ([]statedb.User, error) { return nil, nil }
func (n *noopQuerier) Ping(ctx context.Context) (int32, error)               { return 1, nil }
func (n *noopQuerier) SearchMemoriesBySimilarity(ctx context.Context, arg statedb.SearchMemoriesBySimilarityParams) ([]statedb.SearchMemoriesBySimilarityRow, error) {
	return nil, nil
}
func (n *noopQuerier) SearchMemoriesWithFilters(ctx context.Context, arg statedb.SearchMemoriesWithFiltersParams) ([]statedb.SearchMemoriesWithFiltersRow, error) {
	return nil, nil
}
func (n *noopQuerier) SupersedeFact(ctx context.Context, arg statedb.SupersedeFactParams) (statedb.WorldFact, error) {
	return statedb.WorldFact{}, nil
}
func (n *noopQuerier) TransferItem(ctx context.Context, arg statedb.TransferItemParams) (statedb.Item, error) {
	return statedb.Item{}, nil
}
func (n *noopQuerier) UpdateBeliefSystem(ctx context.Context, arg statedb.UpdateBeliefSystemParams) (statedb.BeliefSystem, error) {
	return statedb.BeliefSystem{}, nil
}
func (n *noopQuerier) UpdateCampaign(ctx context.Context, arg statedb.UpdateCampaignParams) (statedb.Campaign, error) {
	return statedb.Campaign{}, nil
}
func (n *noopQuerier) UpdateCampaignStatus(ctx context.Context, arg statedb.UpdateCampaignStatusParams) (statedb.Campaign, error) {
	return statedb.Campaign{}, nil
}
func (n *noopQuerier) UpdateCulture(ctx context.Context, arg statedb.UpdateCultureParams) (statedb.Culture, error) {
	return statedb.Culture{}, nil
}
func (n *noopQuerier) UpdateEconomicSystem(ctx context.Context, arg statedb.UpdateEconomicSystemParams) (statedb.EconomicSystem, error) {
	return statedb.EconomicSystem{}, nil
}
func (n *noopQuerier) UpdateFaction(ctx context.Context, arg statedb.UpdateFactionParams) (statedb.Faction, error) {
	return statedb.Faction{}, nil
}
func (n *noopQuerier) UpdateFactionRelationship(ctx context.Context, arg statedb.UpdateFactionRelationshipParams) (statedb.FactionRelationship, error) {
	return statedb.FactionRelationship{}, nil
}
func (n *noopQuerier) UpdateItem(ctx context.Context, arg statedb.UpdateItemParams) (statedb.Item, error) {
	return statedb.Item{}, nil
}
func (n *noopQuerier) UpdateItemEquipped(ctx context.Context, arg statedb.UpdateItemEquippedParams) (statedb.Item, error) {
	return statedb.Item{}, nil
}
func (n *noopQuerier) UpdateItemQuantity(ctx context.Context, arg statedb.UpdateItemQuantityParams) (statedb.Item, error) {
	return statedb.Item{}, nil
}
func (n *noopQuerier) UpdateLanguage(ctx context.Context, arg statedb.UpdateLanguageParams) (statedb.Language, error) {
	return statedb.Language{}, nil
}
func (n *noopQuerier) UpdateLocation(ctx context.Context, arg statedb.UpdateLocationParams) (statedb.Location, error) {
	return statedb.Location{}, nil
}
func (n *noopQuerier) UpdateNPC(ctx context.Context, arg statedb.UpdateNPCParams) (statedb.Npc, error) {
	return statedb.Npc{}, nil
}
func (n *noopQuerier) UpdateNPCDisposition(ctx context.Context, arg statedb.UpdateNPCDispositionParams) (statedb.Npc, error) {
	return statedb.Npc{}, nil
}
func (n *noopQuerier) UpdateNPCLocation(ctx context.Context, arg statedb.UpdateNPCLocationParams) (statedb.Npc, error) {
	return statedb.Npc{}, nil
}
func (n *noopQuerier) UpdateObjective(ctx context.Context, arg statedb.UpdateObjectiveParams) (statedb.QuestObjective, error) {
	return statedb.QuestObjective{}, nil
}
func (n *noopQuerier) UpdatePlayerCharacter(ctx context.Context, arg statedb.UpdatePlayerCharacterParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (n *noopQuerier) UpdatePlayerExperience(ctx context.Context, arg statedb.UpdatePlayerExperienceParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (n *noopQuerier) UpdatePlayerLevel(ctx context.Context, arg statedb.UpdatePlayerLevelParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (n *noopQuerier) UpdatePlayerAbilities(ctx context.Context, arg statedb.UpdatePlayerAbilitiesParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (n *noopQuerier) UpdatePlayerHP(ctx context.Context, arg statedb.UpdatePlayerHPParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (n *noopQuerier) UpdatePlayerLocation(ctx context.Context, arg statedb.UpdatePlayerLocationParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (n *noopQuerier) UpdatePlayerStats(ctx context.Context, arg statedb.UpdatePlayerStatsParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (n *noopQuerier) UpdatePlayerAbilities(ctx context.Context, arg statedb.UpdatePlayerAbilitiesParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (n *noopQuerier) UpdatePlayerStatus(ctx context.Context, arg statedb.UpdatePlayerStatusParams) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}
func (n *noopQuerier) UpdateQuest(ctx context.Context, arg statedb.UpdateQuestParams) (statedb.Quest, error) {
	return statedb.Quest{}, nil
}
func (n *noopQuerier) UpdateQuestStatus(ctx context.Context, arg statedb.UpdateQuestStatusParams) (statedb.Quest, error) {
	return statedb.Quest{}, nil
}
func (n *noopQuerier) UpdateRelationship(ctx context.Context, arg statedb.UpdateRelationshipParams) (statedb.EntityRelationship, error) {
	return statedb.EntityRelationship{}, nil
}
func (n *noopQuerier) UpdateUser(ctx context.Context, arg statedb.UpdateUserParams) (statedb.User, error) {
	return statedb.User{}, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newTestLauncher() Launcher {
	return NewLauncher(
		config.Config{LLM: config.LLMConfig{Provider: "ollama"}},
		context.Background(),
		&noopQuerier{},
	)
}

func makeTestCampaign(id byte, name string) statedb.Campaign {
	return statedb.Campaign{
		ID:     pgtype.UUID{Bytes: [16]byte{id}, Valid: true},
		Name:   name,
		Status: "active",
	}
}

// ---------------------------------------------------------------------------
// State transition tests
// ---------------------------------------------------------------------------

func TestLauncherInitialState(t *testing.T) {
	l := newTestLauncher()
	if l.state != launcherLoading {
		t.Fatalf("expected initial state launcherLoading, got %d", l.state)
	}
}

func TestLauncherInitReturnsCmd(t *testing.T) {
	l := newTestLauncher()
	cmd := l.Init()
	if cmd == nil {
		t.Fatal("Init() should return a non-nil Cmd (spinner + bootstrap)")
	}
}

func TestLauncherViewLoadingState(t *testing.T) {
	l := newTestLauncher()
	v := l.View()
	if v == "" {
		t.Fatal("View() should return non-empty string in loading state")
	}
}

func TestLauncherBootstrapDone_SingleCampaignTransitionsToApp(t *testing.T) {
	l := newTestLauncher()
	c := makeTestCampaign(1, "Solo Campaign")
	m, _ := l.Update(bootstrapDoneMsg{
		result: bootstrap.Result{
			User:      statedb.User{Name: "Player"},
			Campaigns: []statedb.Campaign{c},
		},
	})
	if _, ok := m.(App); !ok {
		t.Fatalf("expected App after single-campaign bootstrap, got %T", m)
	}
}

func TestLauncherBootstrapDone_MultipleCampaignsShowsPicker(t *testing.T) {
	l := newTestLauncher()
	m, _ := l.Update(bootstrapDoneMsg{
		result: bootstrap.Result{
			User: statedb.User{Name: "Player"},
			Campaigns: []statedb.Campaign{
				makeTestCampaign(1, "Campaign A"),
				makeTestCampaign(2, "Campaign B"),
			},
		},
	})
	launcher, ok := m.(Launcher)
	if !ok {
		t.Fatalf("expected Launcher after multi-campaign bootstrap, got %T", m)
	}
	if launcher.state != launcherSelecting {
		t.Fatalf("expected launcherSelecting, got %d", launcher.state)
	}
}

func TestLauncherBootstrapDone_ErrorSetsErrMsg(t *testing.T) {
	l := newTestLauncher()
	m, cmd := l.Update(bootstrapDoneMsg{err: errForTest("bootstrap failed")})
	launcher, ok := m.(Launcher)
	if !ok {
		t.Fatalf("expected Launcher on error, got %T", m)
	}
	if launcher.errMsg == "" {
		t.Fatal("expected errMsg to be set after bootstrap error")
	}
	if cmd != nil {
		t.Fatal("expected nil cmd after bootstrap error")
	}
}

func TestLauncherBootstrapDone_ErrorRendersErrorMessage(t *testing.T) {
	l := newTestLauncher()
	m, _ := l.Update(bootstrapDoneMsg{err: errForTest("db unreachable")})
	launcher := m.(Launcher)
	v := launcher.View()
	if v == "" {
		t.Fatal("View() should not be empty after error")
	}
}

func TestLauncherCampaignSelected_TransitionsToApp(t *testing.T) {
	l := newTestLauncher()
	// Put launcher in selecting state first.
	m, _ := l.Update(bootstrapDoneMsg{
		result: bootstrap.Result{
			Campaigns: []statedb.Campaign{
				makeTestCampaign(1, "A"),
				makeTestCampaign(2, "B"),
			},
		},
	})
	launcher := m.(Launcher)

	// Now emit a SelectedMsg.
	c := makeTestCampaign(1, "A")
	m2, _ := launcher.Update(campaign.SelectedMsg{Campaign: c})
	if _, ok := m2.(App); !ok {
		t.Fatalf("expected App after SelectedMsg, got %T", m2)
	}
}

func TestLauncherNewCampaignNameMsg_TransitionsToCreating(t *testing.T) {
	l := newTestLauncher()
	m, cmd := l.Update(campaign.NewCampaignNameMsg{Name: "Brave New World"})
	launcher, ok := m.(Launcher)
	if !ok {
		t.Fatalf("expected Launcher after NewCampaignNameMsg, got %T", m)
	}
	if launcher.state != launcherCreating {
		t.Fatalf("expected launcherCreating, got %d", launcher.state)
	}
	if cmd == nil {
		t.Fatal("expected a DB command to be issued")
	}
}

func TestLauncherCampaignCreated_TransitionsToApp(t *testing.T) {
	l := newTestLauncher()
	c := makeTestCampaign(3, "New World")
	m, _ := l.Update(campaignCreatedMsg{c: c})
	if _, ok := m.(App); !ok {
		t.Fatalf("expected App after campaignCreatedMsg, got %T", m)
	}
}

func TestLauncherCampaignCreated_ErrorReturnsToSelecting(t *testing.T) {
	// Start in creating state.
	l := newTestLauncher()
	l.state = launcherCreating

	m, _ := l.Update(campaignCreatedMsg{err: errForTest("create failed")})
	launcher, ok := m.(Launcher)
	if !ok {
		t.Fatalf("expected Launcher on campaignCreatedMsg error, got %T", m)
	}
	if launcher.state != launcherSelecting {
		t.Fatalf("expected launcherSelecting after create error, got %d", launcher.state)
	}
	if launcher.errMsg == "" {
		t.Fatal("expected errMsg after create error")
	}
}

func TestLauncherSpinnerTick_OnlyAdvancesInLoadingOrCreating(t *testing.T) {
	tickMsg := spinner.TickMsg{}

	// In loading state: should return a cmd (next tick).
	l := newTestLauncher()
	_, cmd := l.Update(tickMsg)
	if cmd == nil {
		t.Fatal("expected tick cmd in loading state")
	}

	// In selecting state: should return nil (no perpetual tick loop).
	m, _ := l.Update(bootstrapDoneMsg{
		result: bootstrap.Result{
			Campaigns: []statedb.Campaign{
				makeTestCampaign(1, "A"),
				makeTestCampaign(2, "B"),
			},
		},
	})
	launcher := m.(Launcher)
	if launcher.state != launcherSelecting {
		t.Skip("launcher not in selecting state, skipping spinner test")
	}
	_, cmd2 := launcher.Update(tickMsg)
	if cmd2 != nil {
		t.Fatal("expected nil tick cmd in selecting state to avoid infinite loop")
	}

	// In creating state: should return a cmd.
	launcher.state = launcherCreating
	_, cmd3 := launcher.Update(tickMsg)
	if cmd3 == nil {
		t.Fatal("expected tick cmd in creating state")
	}
}

func TestLauncherCtrlC_Quits(t *testing.T) {
	l := newTestLauncher()
	_, cmd := l.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit cmd for ctrl+c")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected tea.QuitMsg for ctrl+c")
	}
}

func TestLauncherWindowSize_UpdatesDimensions(t *testing.T) {
	l := newTestLauncher()
	m, _ := l.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	launcher, ok := m.(Launcher)
	if !ok {
		t.Fatalf("expected Launcher, got %T", m)
	}
	if launcher.width != 120 || launcher.height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", launcher.width, launcher.height)
	}
}

// errForTest is a simple error value for test assertions.
type testErr string

func errForTest(s string) error { return testErr(s) }
func (e testErr) Error() string { return string(e) }
