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
