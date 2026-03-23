package game

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

// mockQuerier implements statedb.Querier for testing.
type mockQuerier struct {
	users       map[string]statedb.User
	nextUserID  pgtype.UUID
	createCount int

	// Injectable errors for testing error paths.
	getByNameErr error
	createErr    error
}

func newMockQuerier() *mockQuerier {
	return &mockQuerier{
		users:      make(map[string]statedb.User),
		nextUserID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
	}
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

func (m *mockQuerier) GetCampaignByID(_ context.Context, _ pgtype.UUID) (statedb.Campaign, error) {
	return statedb.Campaign{}, pgx.ErrNoRows
}

func (m *mockQuerier) ListCampaignsByUser(_ context.Context, _ pgtype.UUID) ([]statedb.Campaign, error) {
	return nil, nil
}

func (m *mockQuerier) UpdateCampaign(_ context.Context, _ statedb.UpdateCampaignParams) (statedb.Campaign, error) {
	return statedb.Campaign{}, pgx.ErrNoRows
}

func (m *mockQuerier) UpdateCampaignStatus(_ context.Context, _ statedb.UpdateCampaignStatusParams) (statedb.Campaign, error) {
	return statedb.Campaign{}, pgx.ErrNoRows
}

func (m *mockQuerier) DeleteCampaign(_ context.Context, _ pgtype.UUID) error {
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

func (m *mockQuerier) ListActiveFactsByCampaign(_ context.Context, _ pgtype.UUID) ([]statedb.WorldFact, error) {
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

func (m *mockQuerier) CreateLocation(_ context.Context, _ statedb.CreateLocationParams) (statedb.Location, error) {
	return statedb.Location{}, pgx.ErrNoRows
}

func (m *mockQuerier) CreateItem(_ context.Context, _ statedb.CreateItemParams) (statedb.Item, error) {
	return statedb.Item{}, pgx.ErrNoRows
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

func (m *mockQuerier) GetItemByID(_ context.Context, _ pgtype.UUID) (statedb.Item, error) {
	return statedb.Item{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetLocationByID(_ context.Context, _ pgtype.UUID) (statedb.Location, error) {
	return statedb.Location{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetNPCByID(_ context.Context, _ pgtype.UUID) (statedb.Npc, error) {
	return statedb.Npc{}, pgx.ErrNoRows
}

func (m *mockQuerier) ListLocationsByCampaign(_ context.Context, _ pgtype.UUID) ([]statedb.Location, error) {
	return nil, nil
}

func (m *mockQuerier) ListItemsByPlayer(_ context.Context, _ statedb.ListItemsByPlayerParams) ([]statedb.Item, error) {
	return nil, nil
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

func (m *mockQuerier) ListObjectivesByQuest(_ context.Context, _ pgtype.UUID) ([]statedb.QuestObjective, error) {
	return nil, nil
}

func (m *mockQuerier) ListQuestsByCampaign(_ context.Context, _ pgtype.UUID) ([]statedb.Quest, error) {
	return nil, nil
}

func (m *mockQuerier) ListActiveQuests(_ context.Context, _ pgtype.UUID) ([]statedb.Quest, error) {
	return nil, nil
}

func (m *mockQuerier) ListQuestsByType(_ context.Context, _ statedb.ListQuestsByTypeParams) ([]statedb.Quest, error) {
	return nil, nil
}

func (m *mockQuerier) ListSubquestsByParentQuest(_ context.Context, _ pgtype.UUID) ([]statedb.Quest, error) {
	return nil, nil
}

func (m *mockQuerier) ListNPCsByFaction(_ context.Context, _ statedb.ListNPCsByFactionParams) ([]statedb.Npc, error) {
	return nil, nil
}

func (m *mockQuerier) ListAliveNPCsByLocation(_ context.Context, _ statedb.ListAliveNPCsByLocationParams) ([]statedb.Npc, error) {
	return nil, nil
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
	return nil, nil
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

func (m *mockQuerier) GetPlayerCharacterByCampaign(_ context.Context, _ pgtype.UUID) ([]statedb.PlayerCharacter, error) {
	return nil, nil
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
