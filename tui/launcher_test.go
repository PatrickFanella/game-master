package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PatrickFanella/game-master/internal/bootstrap"
	"github.com/PatrickFanella/game-master/internal/config"
	"github.com/PatrickFanella/game-master/internal/llm"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/internal/world"
	"github.com/PatrickFanella/game-master/tui/campaign"
)

// compile-time check: Launcher must implement tea.Model.
var _ tea.Model = Launcher{}

const loadCampaignCmdBatchIndex = 1

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

func (n *noopQuerier) CreateQuestHistoryEntry(ctx context.Context, arg statedb.CreateQuestHistoryEntryParams) (statedb.QuestHistory, error) {
	return statedb.QuestHistory{}, nil
}

func (n *noopQuerier) CreateQuestNote(ctx context.Context, arg statedb.CreateQuestNoteParams) (statedb.QuestNote, error) {
	return statedb.QuestNote{}, nil
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
func (n *noopQuerier) DeleteQuestNote(ctx context.Context, arg statedb.DeleteQuestNoteParams) error {
	return nil
}
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

func (n *noopQuerier) GetLocationByID(ctx context.Context, arg statedb.GetLocationByIDParams) (statedb.Location, error) {
	return statedb.Location{}, nil
}

func (n *noopQuerier) GetMemoryByID(ctx context.Context, id pgtype.UUID) (statedb.Memory, error) {
	return statedb.Memory{}, nil
}

func (n *noopQuerier) GetNPCByID(ctx context.Context, arg statedb.GetNPCByIDParams) (statedb.Npc, error) {
	return statedb.Npc{}, nil
}

func (n *noopQuerier) GetPlayerCharacterByCampaign(ctx context.Context, campaignID pgtype.UUID) ([]statedb.PlayerCharacter, error) {
	return nil, nil
}

func (n *noopQuerier) GetPlayerCharacterByID(ctx context.Context, id pgtype.UUID) (statedb.PlayerCharacter, error) {
	return statedb.PlayerCharacter{}, nil
}

func (n *noopQuerier) GetQuestByID(ctx context.Context, arg statedb.GetQuestByIDParams) (statedb.Quest, error) {
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
func (n *noopQuerier) GetFactPlayerKnown(_ context.Context, _ pgtype.UUID) (bool, error) {
	return false, nil
}
func (n *noopQuerier) ListPlayerKnownFacts(_ context.Context, _ pgtype.UUID) ([]statedb.WorldFact, error) {
	return nil, nil
}
func (n *noopQuerier) SetFactPlayerKnown(_ context.Context, _ pgtype.UUID) error { return nil }
func (n *noopQuerier) ListPlayerAwareRelationships(_ context.Context, _ pgtype.UUID) ([]statedb.EntityRelationship, error) {
	return nil, nil
}
func (n *noopQuerier) SetRelationshipPlayerAware(_ context.Context, _ pgtype.UUID) error { return nil }
func (n *noopQuerier) ListPlayerKnownLanguages(_ context.Context, _ pgtype.UUID) ([]statedb.Language, error) {
	return nil, nil
}
func (n *noopQuerier) SetLanguagePlayerKnown(_ context.Context, _ pgtype.UUID) error { return nil }
func (n *noopQuerier) ListPlayerKnownCultures(_ context.Context, _ pgtype.UUID) ([]statedb.Culture, error) {
	return nil, nil
}
func (n *noopQuerier) SetCulturePlayerKnown(_ context.Context, _ pgtype.UUID) error { return nil }
func (n *noopQuerier) ListPlayerKnownBeliefSystems(_ context.Context, _ pgtype.UUID) ([]statedb.BeliefSystem, error) {
	return nil, nil
}
func (n *noopQuerier) SetBeliefSystemPlayerKnown(_ context.Context, _ pgtype.UUID) error { return nil }
func (n *noopQuerier) ListPlayerKnownEconomicSystems(_ context.Context, _ pgtype.UUID) ([]statedb.EconomicSystem, error) {
	return nil, nil
}
func (n *noopQuerier) SetEconomicSystemPlayerKnown(_ context.Context, _ pgtype.UUID) error {
	return nil
}
func (n *noopQuerier) ListPlayerKnownLocations(_ context.Context, _ pgtype.UUID) ([]statedb.Location, error) {
	return nil, nil
}
func (n *noopQuerier) ListPlayerVisitedLocations(_ context.Context, _ pgtype.UUID) ([]statedb.Location, error) {
	return nil, nil
}
func (n *noopQuerier) ListQuestHistory(_ context.Context, _ pgtype.UUID) ([]statedb.QuestHistory, error) {
	return nil, nil
}
func (n *noopQuerier) ListQuestNotes(_ context.Context, _ pgtype.UUID) ([]statedb.QuestNote, error) {
	return nil, nil
}
func (n *noopQuerier) SetLocationPlayerKnown(_ context.Context, _ pgtype.UUID) error  { return nil }
func (n *noopQuerier) SetLocationPlayerVisited(_ context.Context, _ pgtype.UUID) error { return nil }

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

func (n *noopQuerier) UpdateItemProperties(ctx context.Context, arg statedb.UpdateItemPropertiesParams) (statedb.Item, error) {
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

func newTestLauncherWithEngine(engine *mockGameEngine) Launcher {
	return NewLauncherWithEngine(
		config.Config{LLM: config.LLMConfig{Provider: "ollama"}},
		context.Background(),
		&noopQuerier{},
		engine,
	)
}

func makeTestCampaign(id byte, name string) statedb.Campaign {
	return statedb.Campaign{
		ID:     pgtype.UUID{Bytes: [16]byte{id}, Valid: true},
		Name:   name,
		Status: "active",
	}
}

type stubLLMProvider struct{}

func (stubLLMProvider) Complete(context.Context, []llm.Message, []llm.Tool) (*llm.Response, error) {
	return &llm.Response{}, nil
}

func (stubLLMProvider) Stream(context.Context, []llm.Message, []llm.Tool) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 1)
	ch <- llm.StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func newTestLauncherWithProvider(provider llm.Provider) Launcher {
	return NewLauncherWithEngine(
		config.Config{LLM: config.LLMConfig{Provider: "ollama"}},
		context.Background(),
		&noopQuerier{},
		nil,
		WithLLMProvider(provider),
	)
}

func bootstrapToSelecting(t *testing.T, l Launcher) Launcher {
	t.Helper()

	m, _ := l.Update(bootstrapDoneMsg{result: bootstrap.Result{
		User: statedb.User{
			ID:   pgtype.UUID{Bytes: [16]byte{99}, Valid: true},
			Name: "Player",
		},
		Campaigns: []statedb.Campaign{
			makeTestCampaign(1, "A"),
			makeTestCampaign(2, "B"),
		},
	}})

	launcher, ok := m.(Launcher)
	if !ok {
		t.Fatalf("expected Launcher after bootstrap, got %T", m)
	}
	if launcher.state != launcherSelecting {
		t.Fatalf("expected launcherSelecting after bootstrap, got %d", launcher.state)
	}

	return launcher
}

func testCampaignProfile() world.CampaignProfile {
	return world.CampaignProfile{
		Genre:               "Fantasy",
		Tone:                "Hopeful",
		Themes:              []string{"friendship"},
		WorldType:           "frontier",
		DangerLevel:         "moderate",
		PoliticalComplexity: "simple",
	}
}

func testCharacterProfile() world.CharacterProfile {
	return world.CharacterProfile{
		Name:        "Mira",
		Concept:     "Ranger",
		Background:  "Frontier scout",
		Personality: "Steady",
		Motivations: []string{"Protect the wilds"},
		Strengths:   []string{"Tracking"},
		Weaknesses:  []string{"Reckless"},
	}
}

func testCampaignProposal() world.CampaignProposal {
	return world.CampaignProposal{
		Name:    "Skyreach",
		Summary: "Defend a frontier city perched above a storm sea.",
		Profile: testCampaignProfile(),
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

func TestLauncherBootstrapDone_SingleCampaignShowsPicker(t *testing.T) {
	l := newTestLauncher()
	c := makeTestCampaign(1, "Solo Campaign")
	m, _ := l.Update(bootstrapDoneMsg{
		result: bootstrap.Result{
			User:      statedb.User{Name: "Player"},
			Campaigns: []statedb.Campaign{c},
		},
	})
	launcher, ok := m.(Launcher)
	if !ok {
		t.Fatalf("expected Launcher after single-campaign bootstrap, got %T", m)
	}
	if launcher.state != launcherSelecting {
		t.Fatalf("expected launcherSelecting, got %d", launcher.state)
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

func TestLauncherSelectingState_RendersErrorBanner(t *testing.T) {
	l := newTestLauncher()
	m, _ := l.Update(bootstrapDoneMsg{
		result: bootstrap.Result{
			Campaigns: []statedb.Campaign{
				makeTestCampaign(1, "A"),
				makeTestCampaign(2, "B"),
			},
		},
	})
	launcher := m.(Launcher)
	launcher.errMsg = "Load campaign failed: network timeout"
	v := launcher.View()
	if !strings.Contains(v, "Load campaign failed: network timeout") {
		t.Fatalf("expected selecting view to render error banner, got %q", v)
	}
}

func TestLauncherCampaignSelected_TransitionsToApp(t *testing.T) {
	mockEngine := &mockGameEngine{}
	l := newTestLauncherWithEngine(mockEngine)
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
	m2, cmd := launcher.Update(campaign.SelectedMsg{Campaign: c})
	if cmd == nil {
		t.Fatal("expected load campaign cmd after SelectedMsg")
	}
	launcherAfterSelect, ok := m2.(Launcher)
	if !ok {
		t.Fatalf("expected Launcher while loading campaign, got %T", m2)
	}
	if launcherAfterSelect.state != launcherLoadingCampaign {
		t.Fatalf("expected launcherLoadingCampaign, got %d", launcherAfterSelect.state)
	}
	rawMsg := cmd()
	batchMsg, ok := rawMsg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg from cmd, got %T", rawMsg)
	}
	if len(batchMsg) < 2 {
		t.Fatalf("expected at least 2 commands in batch, got %d", len(batchMsg))
	}
	// Launcher batches spinner tick first, then runLoadCampaign.
	loadMsgRaw := batchMsg[loadCampaignCmdBatchIndex]()
	loadMsg, ok := loadMsgRaw.(campaignLoadedMsg)
	if !ok {
		t.Fatalf("expected campaignLoadedMsg from batch[1], got %T", loadMsgRaw)
	}
	m3, _ := launcherAfterSelect.Update(loadMsg)
	if len(mockEngine.loadedCampaignIDs) != 1 {
		t.Fatalf("expected engine.LoadCampaign to be called once, got %d", len(mockEngine.loadedCampaignIDs))
	}
	if _, ok := m3.(App); !ok {
		t.Fatalf("expected App after campaignLoadedMsg, got %T", m3)
	}
}

func TestLauncherCampaignSelected_LoadCampaignErrorRefreshesPicker(t *testing.T) {
	launcher := bootstrapToSelecting(t, newTestLauncher())
	c := makeTestCampaign(1, "A")

	m, _ := launcher.Update(campaign.SelectedMsg{Campaign: c})
	loading := m.(Launcher)

	m2, cmd := loading.Update(campaignLoadedMsg{c: c, err: errForTest("load failed")})
	reloaded := m2.(Launcher)

	if reloaded.state != launcherLoading {
		t.Fatalf("expected launcherLoading after load error, got %d", reloaded.state)
	}
	if cmd == nil {
		t.Fatal("expected bootstrap refresh cmd after load error")
	}
	if reloaded.errMsg == "" {
		t.Fatal("expected errMsg after load error")
	}
}

func TestLauncherNewCampaignMsg_TransitionsToChooseMethod(t *testing.T) {
	launcher := bootstrapToSelecting(t, newTestLauncher())

	m, _ := launcher.Update(campaign.NewCampaignMsg{})
	launcher2 := m.(Launcher)

	if launcher2.state != launcherChooseMethod {
		t.Fatalf("expected launcherChooseMethod, got %d", launcher2.state)
	}
}

func TestLauncherMethodChosenDescribe_TransitionsToInterviewing(t *testing.T) {
	launcher := bootstrapToSelecting(t, newTestLauncherWithProvider(stubLLMProvider{}))

	m, _ := launcher.Update(campaign.NewCampaignMsg{})
	launcher = m.(Launcher)

	m2, _ := launcher.Update(campaign.MethodChosenMsg{Method: campaign.MethodDescribe})
	launcher2 := m2.(Launcher)

	if launcher2.state != launcherInterviewing {
		t.Fatalf("expected launcherInterviewing, got %d", launcher2.state)
	}
}

func TestLauncherMethodChosenAttributes_TransitionsToAttributes(t *testing.T) {
	launcher := bootstrapToSelecting(t, newTestLauncher())

	m, _ := launcher.Update(campaign.NewCampaignMsg{})
	launcher = m.(Launcher)

	m2, _ := launcher.Update(campaign.MethodChosenMsg{Method: campaign.MethodAttributes})
	launcher2 := m2.(Launcher)

	if launcher2.state != launcherAttributes {
		t.Fatalf("expected launcherAttributes, got %d", launcher2.state)
	}
}

func TestLauncherProposalsGenerated_TransitionsToProposals(t *testing.T) {
	launcher := bootstrapToSelecting(t, newTestLauncherWithProvider(stubLLMProvider{}))

	m, _ := launcher.Update(campaign.NewCampaignMsg{})
	launcher = m.(Launcher)
	m, _ = launcher.Update(campaign.MethodChosenMsg{Method: campaign.MethodAttributes})
	launcher = m.(Launcher)

	m, cmd := launcher.Update(campaign.AttributesReadyMsg{Genre: "Fantasy", SettingStyle: "Frontier", Tone: "Heroic"})
	loading := m.(Launcher)
	if loading.state != launcherGeneratingProposals {
		t.Fatalf("expected launcherGeneratingProposals, got %d", loading.state)
	}
	if cmd == nil {
		t.Fatal("expected proposal generation cmd")
	}

	m2, _ := loading.Update(proposalsGeneratedMsg{proposals: []world.CampaignProposal{testCampaignProposal()}})
	launcher2 := m2.(Launcher)

	if launcher2.state != launcherProposals {
		t.Fatalf("expected launcherProposals, got %d", launcher2.state)
	}
}

func TestLauncherProposalsGenerated_ErrorReturnsPrefilledAttributes(t *testing.T) {
	launcher := bootstrapToSelecting(t, newTestLauncherWithProvider(stubLLMProvider{}))

	m, _ := launcher.Update(campaign.NewCampaignMsg{})
	launcher = m.(Launcher)
	m, _ = launcher.Update(campaign.MethodChosenMsg{Method: campaign.MethodAttributes})
	launcher = m.(Launcher)
	m, _ = launcher.Update(campaign.AttributesReadyMsg{
		Genre:        "Fantasy",
		SettingStyle: "Open Wilderness",
		Tone:         "Mysterious",
	})
	loading := m.(Launcher)

	m2, _ := loading.Update(proposalsGeneratedMsg{err: errForTest("parse failure")})
	launcher2 := m2.(Launcher)
	if launcher2.state != launcherAttributes {
		t.Fatalf("expected launcherAttributes, got %d", launcher2.state)
	}
	if launcher2.proposalAttrs.Genre != "Fantasy" || launcher2.proposalAttrs.SettingStyle != "Open Wilderness" || launcher2.proposalAttrs.Tone != "Mysterious" {
		t.Fatalf("proposal attrs not preserved: %+v", launcher2.proposalAttrs)
	}
	if launcher2.errMsg == "" {
		t.Fatal("expected error message after proposal generation failure")
	}
	if launcher2.view == nil {
		t.Fatal("expected attributes view after proposal generation failure")
	}
	launcher2.width = 100
	launcher2.height = 30
	if view := launcher2.View(); !strings.Contains(view, "Generate proposals failed: parse failure") {
		t.Fatalf("expected inline proposal error in view, got %q", view)
	}
}

func TestLauncherProposalSelected_TransitionsToCharMethod(t *testing.T) {
	launcher := bootstrapToSelecting(t, newTestLauncher())

	m, _ := launcher.Update(campaign.NewCampaignMsg{})
	launcher = m.(Launcher)
	m, _ = launcher.Update(proposalsGeneratedMsg{proposals: []world.CampaignProposal{testCampaignProposal()}})
	launcher = m.(Launcher)

	m2, _ := launcher.Update(campaign.ProposalSelectedMsg{Proposal: testCampaignProposal()})
	launcher2 := m2.(Launcher)

	if launcher2.state != launcherCharMethod {
		t.Fatalf("expected launcherCharMethod, got %d", launcher2.state)
	}
}

func TestLauncherCharacterReady_TransitionsToConfirmation(t *testing.T) {
	launcher := bootstrapToSelecting(t, newTestLauncher())
	proposal := testCampaignProposal()
	character := testCharacterProfile()

	m, _ := launcher.Update(campaign.NewCampaignMsg{})
	launcher = m.(Launcher)
	m, _ = launcher.Update(campaign.ProposalSelectedMsg{Proposal: proposal})
	launcher = m.(Launcher)

	m2, _ := launcher.Update(campaign.CharacterReadyMsg{Profile: character})
	launcher2 := m2.(Launcher)

	if launcher2.state != launcherConfirmation {
		t.Fatalf("expected launcherConfirmation, got %d", launcher2.state)
	}
}

func TestLauncherConfirmed_TransitionsToWorldBuilding(t *testing.T) {
	launcher := bootstrapToSelecting(t, newTestLauncherWithProvider(stubLLMProvider{}))
	proposal := testCampaignProposal()
	character := testCharacterProfile()

	m, _ := launcher.Update(campaign.NewCampaignMsg{})
	launcher = m.(Launcher)
	m, _ = launcher.Update(campaign.ProposalSelectedMsg{Proposal: proposal})
	launcher = m.(Launcher)
	m, _ = launcher.Update(campaign.CharacterReadyMsg{Profile: character})
	launcher = m.(Launcher)

	m2, _ := launcher.Update(campaign.ConfirmedMsg{
		Name:             proposal.Name,
		Summary:          proposal.Summary,
		Profile:          proposal.Profile,
		CharacterProfile: character,
	})
	launcher2 := m2.(Launcher)

	if launcher2.state != launcherWorldBuilding {
		t.Fatalf("expected launcherWorldBuilding, got %d", launcher2.state)
	}
}

func TestLauncherWorldReady_TransitionsToApp(t *testing.T) {
	launcher := bootstrapToSelecting(t, newTestLauncher())
	result := world.OrchestratorResult{
		Campaign: makeTestCampaign(7, "Skyreach"),
		Scene: &world.SceneResult{
			Narrative: "Storm clouds gather over the harbor.",
			Choices:   []string{"Investigate the docks", "Seek shelter"},
		},
	}

	m, cmd := launcher.Update(campaign.WorldReadyMsg{Result: &result})
	if cmd == nil {
		t.Fatal("expected load campaign cmd after world ready")
	}
	loading := m.(Launcher)
	if loading.state != launcherLoadingCampaign {
		t.Fatalf("expected launcherLoadingCampaign, got %d", loading.state)
	}

	rawMsg := cmd()
	batchMsg, ok := rawMsg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg from cmd, got %T", rawMsg)
	}
	if len(batchMsg) < 2 {
		t.Fatalf("expected at least 2 commands in batch, got %d", len(batchMsg))
	}

	loadMsgRaw := batchMsg[loadCampaignCmdBatchIndex]()
	loadMsg, ok := loadMsgRaw.(campaignLoadedMsg)
	if !ok {
		t.Fatalf("expected campaignLoadedMsg from batch[1], got %T", loadMsgRaw)
	}

	m2, _ := loading.Update(loadMsg)
	if _, ok := m2.(App); !ok {
		t.Fatalf("expected App after campaignLoadedMsg, got %T", m2)
	}
}

func TestLauncherBootstrapDone_EmptyCampaignsShowsPicker(t *testing.T) {
	l := newTestLauncher()
	m, _ := l.Update(bootstrapDoneMsg{
		result: bootstrap.Result{
			User:      statedb.User{Name: "Player"},
			Campaigns: nil,
		},
	})
	launcher, ok := m.(Launcher)
	if !ok {
		t.Fatalf("expected Launcher after empty-campaign bootstrap, got %T", m)
	}
	if launcher.state != launcherSelecting {
		t.Fatalf("expected launcherSelecting, got %d", launcher.state)
	}
}

func TestLauncherCampaignSelected_NoEngineStillTransitionsToApp(t *testing.T) {
	l := newTestLauncher()
	m, _ := l.Update(bootstrapDoneMsg{
		result: bootstrap.Result{
			Campaigns: []statedb.Campaign{
				makeTestCampaign(1, "A"),
				makeTestCampaign(2, "B"),
			},
		},
	})
	launcher := m.(Launcher)
	c := makeTestCampaign(1, "A")
	m2, _ := launcher.Update(campaign.SelectedMsg{Campaign: c})
	launcher2 := m2.(Launcher)
	m3, _ := launcher2.Update(campaignLoadedMsg{c: c})
	if _, ok := m3.(App); !ok {
		t.Fatalf("expected App after campaignLoadedMsg, got %T", m3)
	}
}

func TestLauncherBackFromChooseMethod_ReloadsPicker(t *testing.T) {
	launcher := bootstrapToSelecting(t, newTestLauncher())

	m, _ := launcher.Update(campaign.NewCampaignMsg{})
	launcher = m.(Launcher)

	m2, cmd := launcher.Update(campaign.BackMsg{})
	reloaded := m2.(Launcher)

	if reloaded.state != launcherLoading {
		t.Fatalf("expected launcherLoading after back, got %d", reloaded.state)
	}
	if cmd == nil {
		t.Fatal("expected bootstrap refresh cmd after back from choose method")
	}
}

func TestLauncherBackFromCharForm_ReturnsToCharMethod(t *testing.T) {
	launcher := bootstrapToSelecting(t, newTestLauncher())
	proposal := testCampaignProposal()

	m, _ := launcher.Update(campaign.NewCampaignMsg{})
	launcher = m.(Launcher)
	m, _ = launcher.Update(campaign.ProposalSelectedMsg{Proposal: proposal})
	launcher = m.(Launcher)
	m, _ = launcher.Update(campaign.CharMethodChosenMsg{Method: campaign.MethodAttributes})
	launcher = m.(Launcher)

	if launcher.state != launcherCharForm {
		t.Fatalf("expected launcherCharForm, got %d", launcher.state)
	}

	m2, _ := launcher.Update(campaign.BackMsg{})
	launcher2 := m2.(Launcher)

	if launcher2.state != launcherCharMethod {
		t.Fatalf("expected launcherCharMethod after back, got %d", launcher2.state)
	}
}

func TestLauncherChangeFromConfirmation_ReturnsToCharMethod(t *testing.T) {
	launcher := bootstrapToSelecting(t, newTestLauncher())
	proposal := testCampaignProposal()

	m, _ := launcher.Update(campaign.NewCampaignMsg{})
	launcher = m.(Launcher)
	m, _ = launcher.Update(campaign.ProposalSelectedMsg{Proposal: proposal})
	launcher = m.(Launcher)
	m, _ = launcher.Update(campaign.CharacterReadyMsg{Profile: testCharacterProfile()})
	launcher = m.(Launcher)

	if launcher.state != launcherConfirmation {
		t.Fatalf("expected launcherConfirmation, got %d", launcher.state)
	}

	m2, _ := launcher.Update(campaign.ChangeMsg{})
	launcher2 := m2.(Launcher)

	if launcher2.state != launcherCharMethod {
		t.Fatalf("expected launcherCharMethod after change, got %d", launcher2.state)
	}
}

func TestLauncherWorldError_ReloadsPicker(t *testing.T) {
	launcher := bootstrapToSelecting(t, newTestLauncher())
	launcher.state = launcherWorldBuilding

	m, cmd := launcher.Update(campaign.WorldErrorMsg{Err: errForTest("world failed")})
	reloaded := m.(Launcher)

	if reloaded.state != launcherLoading {
		t.Fatalf("expected launcherLoading after world error, got %d", reloaded.state)
	}
	if cmd == nil {
		t.Fatal("expected bootstrap refresh cmd after world error")
	}
}

func TestLauncherCampaignLoadedError_ReloadsPicker(t *testing.T) {
	launcher := bootstrapToSelecting(t, newTestLauncher())
	launcher.state = launcherLoadingCampaign

	m, cmd := launcher.Update(campaignLoadedMsg{c: makeTestCampaign(9, "Broken Save"), err: errForTest("load failed")})
	reloaded := m.(Launcher)

	if reloaded.state != launcherLoading {
		t.Fatalf("expected launcherLoading after campaignLoadedMsg error, got %d", reloaded.state)
	}
	if cmd == nil {
		t.Fatal("expected bootstrap refresh cmd after campaignLoadedMsg error")
	}
}

func TestLauncherSpinnerTick_OnlyAdvancesInRootSpinnerStates(t *testing.T) {
	tickMsg := spinner.TickMsg{}

	l := newTestLauncher()
	_, cmd := l.Update(tickMsg)
	if cmd == nil {
		t.Fatal("expected tick cmd in loading state")
	}

	launcher := bootstrapToSelecting(t, l)
	_, cmd2 := launcher.Update(tickMsg)
	if cmd2 != nil {
		t.Fatal("expected nil tick cmd in selecting state to avoid infinite loop")
	}

	launcher.state = launcherGeneratingProposals
	_, cmd3 := launcher.Update(tickMsg)
	if cmd3 == nil {
		t.Fatal("expected tick cmd in generating-proposals state")
	}

	launcher.state = launcherLoadingCampaign
	_, cmd4 := launcher.Update(tickMsg)
	if cmd4 == nil {
		t.Fatal("expected tick cmd in loading-campaign state")
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
