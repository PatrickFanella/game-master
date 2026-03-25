//go:build integration

package statedb_test

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	pgvector "github.com/pgvector/pgvector-go"
	"github.com/pressly/goose/v3"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

const (
	testDBName    = "game_master_test"
	testDBUser    = "game_master"
	testDBPass    = "game_master"
	migrationsDir = "../../../migrations"
	vectorDim     = 1536
)

var (
	testPool *pgxpool.Pool
	testDSN  string
)

// TestMain sets up a pgvector Postgres container, applies all migrations, and runs the tests.
func TestMain(m *testing.M) {
	// Use a bounded context for container startup and migration so CI never hangs indefinitely.
	setupCtx, setupCancel := context.WithTimeout(context.Background(), 5*time.Minute)

	var exitCode int
	defer func() {
		setupCancel()
		os.Exit(exitCode)
	}()

	container, err := tcpostgres.Run(setupCtx,
		"docker.io/pgvector/pgvector:pg16",
		tcpostgres.WithDatabase(testDBName),
		tcpostgres.WithUsername(testDBUser),
		tcpostgres.WithPassword(testDBPass),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		log.Printf("failed to start postgres container: %v", err)
		exitCode = 1
		return
	}
	defer func() {
		if err := container.Terminate(context.Background()); err != nil {
			log.Printf("failed to terminate container: %v", err)
		}
	}()

	testDSN, err = container.ConnectionString(setupCtx, "sslmode=disable")
	if err != nil {
		log.Printf("failed to get connection string: %v", err)
		exitCode = 1
		return
	}

	// Apply all migrations using goose.
	migrationsDB, err := sql.Open("pgx", testDSN)
	if err != nil {
		log.Printf("failed to open database for migrations: %v", err)
		exitCode = 1
		return
	}
	defer migrationsDB.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, migrationsDB, os.DirFS(migrationsDir))
	if err != nil {
		log.Printf("failed to create goose provider: %v", err)
		exitCode = 1
		return
	}
	if _, err := provider.Up(setupCtx); err != nil {
		log.Printf("failed to run migrations: %v", err)
		exitCode = 1
		return
	}

	// Create a pgxpool for use in tests.
	testPool, err = pgxpool.New(setupCtx, testDSN)
	if err != nil {
		log.Printf("failed to create connection pool: %v", err)
		exitCode = 1
		return
	}
	defer testPool.Close()

	exitCode = m.Run()
}

// newTx begins a transaction and returns a Queries instance backed by it.
// The transaction is automatically rolled back when the test ends, ensuring isolation.
func newTx(t *testing.T) *statedb.Queries {
	t.Helper()
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	return statedb.New(tx)
}

// makeVec returns a 1536-dimensional unit vector with 1.0 at position (idx % vectorDim).
func makeVec(idx int) pgvector.Vector {
	floats := make([]float32, vectorDim)
	floats[idx%vectorDim] = 1.0
	return pgvector.NewVector(floats)
}

// txt is a convenience helper to create a valid pgtype.Text.
func txt(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }

// int4 is a convenience helper to create a valid pgtype.Int4.
func int4(n int32) pgtype.Int4 { return pgtype.Int4{Int32: n, Valid: true} }

// ─── Helper creators ────────────────────────────────────────────────────────

func createUser(t *testing.T, q *statedb.Queries, name string) statedb.User {
	t.Helper()
	u, err := q.CreateUser(context.Background(), name)
	if err != nil {
		t.Fatalf("CreateUser(%q): %v", name, err)
	}
	return u
}

func createCampaign(t *testing.T, q *statedb.Queries, userID pgtype.UUID) statedb.Campaign {
	t.Helper()
	c, err := q.CreateCampaign(context.Background(), statedb.CreateCampaignParams{
		Name:      "Test Campaign",
		Status:    "active",
		CreatedBy: userID,
	})
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	return c
}

func createLocation(t *testing.T, q *statedb.Queries, campaignID pgtype.UUID, name string) statedb.Location {
	t.Helper()
	loc, err := q.CreateLocation(context.Background(), statedb.CreateLocationParams{
		CampaignID: campaignID,
		Name:       name,
	})
	if err != nil {
		t.Fatalf("CreateLocation(%q): %v", name, err)
	}
	return loc
}

func createFaction(t *testing.T, q *statedb.Queries, campaignID pgtype.UUID) statedb.Faction {
	t.Helper()
	f, err := q.CreateFaction(context.Background(), statedb.CreateFactionParams{
		CampaignID: campaignID,
		Name:       "Test Faction",
	})
	if err != nil {
		t.Fatalf("CreateFaction: %v", err)
	}
	return f
}

func createNPC(t *testing.T, q *statedb.Queries, campaignID pgtype.UUID) statedb.Npc {
	t.Helper()
	npc, err := q.CreateNPC(context.Background(), statedb.CreateNPCParams{
		CampaignID:  campaignID,
		Name:        "Test NPC",
		Disposition: 0,
		Alive:       true,
	})
	if err != nil {
		t.Fatalf("CreateNPC: %v", err)
	}
	return npc
}

func createPC(t *testing.T, q *statedb.Queries, campaignID, userID pgtype.UUID) statedb.PlayerCharacter {
	t.Helper()
	pc, err := q.CreatePlayerCharacter(context.Background(), statedb.CreatePlayerCharacterParams{
		CampaignID: campaignID,
		UserID:     userID,
		Name:       "Test PC",
		Hp:         100,
		MaxHp:      100,
		Experience: 0,
		Level:      1,
		Status:     "active",
	})
	if err != nil {
		t.Fatalf("CreatePlayerCharacter: %v", err)
	}
	return pc
}

func createQuest(t *testing.T, q *statedb.Queries, campaignID pgtype.UUID) statedb.Quest {
	t.Helper()
	quest, err := q.CreateQuest(context.Background(), statedb.CreateQuestParams{
		CampaignID: campaignID,
		Title:      "Test Quest",
		QuestType:  "main",
		Status:     "active",
	})
	if err != nil {
		t.Fatalf("CreateQuest: %v", err)
	}
	return quest
}

// ─── Tests ───────────────────────────────────────────────────────────────────

// TestIntegrationPing verifies the database is reachable.
func TestIntegrationPing(t *testing.T) {
	q := newTx(t)
	n, err := q.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if n != 1 {
		t.Errorf("Ping: expected 1, got %d", n)
	}
}

// TestIntegrationMigrationsDown verifies that all down migrations apply cleanly in reverse.
func TestIntegrationMigrationsDown(t *testing.T) {
	ctx := context.Background()

	// Create a fresh database within the same container for this test.
	conn, err := testPool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire connection: %v", err)
	}
	defer conn.Release()

	const downDB = "game_master_down_test"
	if _, err := conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %q", downDB)); err != nil {
		t.Fatalf("CREATE DATABASE: %v", err)
	}
	t.Cleanup(func() {
		c, err := testPool.Acquire(ctx)
		if err != nil {
			t.Logf("acquire connection for cleanup: %v", err)
			return
		}
		defer c.Release()

		// Terminate any remaining connections so that DROP DATABASE can succeed.
		if _, err := c.Exec(ctx, `
			SELECT pg_terminate_backend(pid)
			FROM pg_stat_activity
			WHERE datname = $1
				AND pid <> pg_backend_pid()
		`, downDB); err != nil {
			t.Logf("terminate connections to %s: %v", downDB, err)
		}

		if _, err := c.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %q", downDB)); err != nil {
			t.Logf("DROP DATABASE %s: %v", downDB, err)
		}
	})

	// Build a connection string for the new database.
	pgxCfg, err := pgx.ParseConfig(testDSN)
	if err != nil {
		t.Fatalf("parse DSN: %v", err)
	}
	pgxCfg.Database = downDB
	downDSN := stdlib.RegisterConnConfig(pgxCfg)

	downSQLDB, err := sql.Open("pgx", downDSN)
	if err != nil {
		t.Fatalf("open down DB: %v", err)
	}
	defer downSQLDB.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, downSQLDB, os.DirFS(migrationsDir))
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	// Apply all down migrations in reverse.
	if _, err := provider.DownTo(ctx, 0); err != nil {
		t.Fatalf("migrate down to 0: %v", err)
	}
}

// TestIntegrationUsers tests full CRUD for the users table.
func TestIntegrationUsers(t *testing.T) {
	ctx := context.Background()
	q := newTx(t)

	// Create
	user, err := q.CreateUser(ctx, "alice")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if user.Name != "alice" || !user.ID.Valid {
		t.Errorf("CreateUser: unexpected result %+v", user)
	}

	// GetByID
	got, err := q.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.ID != user.ID {
		t.Error("GetUserByID: ID mismatch")
	}

	// GetByName
	got, err = q.GetUserByName(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUserByName: %v", err)
	}
	if got.Name != "alice" {
		t.Errorf("GetUserByName: expected alice, got %q", got.Name)
	}

	// ListUsers
	users, err := q.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	found := false
	for _, u := range users {
		if u.ID == user.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("ListUsers: created user not found")
	}

	// UpdateUser
	updated, err := q.UpdateUser(ctx, statedb.UpdateUserParams{ID: user.ID, Name: "alice-v2"})
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if updated.Name != "alice-v2" {
		t.Errorf("UpdateUser: expected alice-v2, got %q", updated.Name)
	}

	// DeleteUser
	if err := q.DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if _, err := q.GetUserByID(ctx, user.ID); err == nil {
		t.Error("expected error after delete, got nil")
	}
}

// TestIntegrationCampaigns tests full CRUD for the campaigns table.
func TestIntegrationCampaigns(t *testing.T) {
	ctx := context.Background()
	q := newTx(t)
	user := createUser(t, q, "campaign-owner")

	// Create
	c, err := q.CreateCampaign(ctx, statedb.CreateCampaignParams{
		Name:        "Epic Adventure",
		Description: txt("A sweeping fantasy epic"),
		Genre:       txt("fantasy"),
		Tone:        txt("serious"),
		Themes:      []string{"heroism", "loss"},
		Status:      "active",
		CreatedBy:   user.ID,
	})
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	if c.Name != "Epic Adventure" || c.Status != "active" {
		t.Errorf("CreateCampaign: unexpected result")
	}

	// GetByID
	got, err := q.GetCampaignByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetCampaignByID: %v", err)
	}
	if got.ID != c.ID {
		t.Error("GetCampaignByID: ID mismatch")
	}

	// ListCampaignsByUser
	list, err := q.ListCampaignsByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListCampaignsByUser: %v", err)
	}
	if len(list) != 1 || list[0].ID != c.ID {
		t.Errorf("ListCampaignsByUser: unexpected result")
	}

	// UpdateCampaign
	updated, err := q.UpdateCampaign(ctx, statedb.UpdateCampaignParams{
		ID:   c.ID,
		Name: "Updated Adventure",
	})
	if err != nil {
		t.Fatalf("UpdateCampaign: %v", err)
	}
	if updated.Name != "Updated Adventure" {
		t.Errorf("UpdateCampaign: expected Updated Adventure, got %q", updated.Name)
	}

	// UpdateCampaignStatus
	updated, err = q.UpdateCampaignStatus(ctx, statedb.UpdateCampaignStatusParams{ID: c.ID, Status: "paused"})
	if err != nil {
		t.Fatalf("UpdateCampaignStatus: %v", err)
	}
	if updated.Status != "paused" {
		t.Errorf("UpdateCampaignStatus: expected paused, got %q", updated.Status)
	}

	// DeleteCampaign
	if err := q.DeleteCampaign(ctx, c.ID); err != nil {
		t.Fatalf("DeleteCampaign: %v", err)
	}
	if _, err := q.GetCampaignByID(ctx, c.ID); err == nil {
		t.Error("expected error after delete, got nil")
	}
}

// TestIntegrationLocations tests full CRUD for the locations table.
func TestIntegrationLocations(t *testing.T) {
	ctx := context.Background()
	q := newTx(t)
	user := createUser(t, q, "loc-user")
	camp := createCampaign(t, q, user.ID)

	// Create
	loc, err := q.CreateLocation(ctx, statedb.CreateLocationParams{
		CampaignID:   camp.ID,
		Name:         "The Dark Forest",
		Description:  txt("A dense, foreboding forest"),
		Region:       txt("Northlands"),
		LocationType: txt("wilderness"),
	})
	if err != nil {
		t.Fatalf("CreateLocation: %v", err)
	}
	if loc.Name != "The Dark Forest" {
		t.Errorf("CreateLocation: unexpected name %q", loc.Name)
	}

	// GetByID
	got, err := q.GetLocationByID(ctx, loc.ID)
	if err != nil {
		t.Fatalf("GetLocationByID: %v", err)
	}
	if got.ID != loc.ID {
		t.Error("GetLocationByID: ID mismatch")
	}

	// ListLocationsByCampaign
	locs, err := q.ListLocationsByCampaign(ctx, camp.ID)
	if err != nil {
		t.Fatalf("ListLocationsByCampaign: %v", err)
	}
	if len(locs) != 1 || locs[0].ID != loc.ID {
		t.Error("ListLocationsByCampaign: unexpected result")
	}

	// ListLocationsByRegion
	byRegion, err := q.ListLocationsByRegion(ctx, statedb.ListLocationsByRegionParams{
		CampaignID: camp.ID,
		Region:     txt("Northlands"),
	})
	if err != nil {
		t.Fatalf("ListLocationsByRegion: %v", err)
	}
	if len(byRegion) != 1 || byRegion[0].ID != loc.ID {
		t.Error("ListLocationsByRegion: unexpected result")
	}

	// UpdateLocation
	updated, err := q.UpdateLocation(ctx, statedb.UpdateLocationParams{
		ID:           loc.ID,
		Name:         "The Enchanted Forest",
		Region:       txt("Northlands"),
		LocationType: txt("magical wilderness"),
	})
	if err != nil {
		t.Fatalf("UpdateLocation: %v", err)
	}
	if updated.Name != "The Enchanted Forest" {
		t.Errorf("UpdateLocation: expected The Enchanted Forest, got %q", updated.Name)
	}
}

// TestIntegrationFactions tests full CRUD for factions and faction_relationships.
func TestIntegrationFactions(t *testing.T) {
	ctx := context.Background()
	q := newTx(t)
	user := createUser(t, q, "faction-user")
	camp := createCampaign(t, q, user.ID)

	// CreateFaction
	f1, err := q.CreateFaction(ctx, statedb.CreateFactionParams{
		CampaignID:  camp.ID,
		Name:        "The Order",
		Description: txt("A noble order"),
		Agenda:      txt("Protect the realm"),
		Territory:   txt("Capital City"),
	})
	if err != nil {
		t.Fatalf("CreateFaction: %v", err)
	}
	if f1.Name != "The Order" {
		t.Errorf("CreateFaction: unexpected name %q", f1.Name)
	}

	f2, err := q.CreateFaction(ctx, statedb.CreateFactionParams{
		CampaignID: camp.ID,
		Name:       "The Shadow Guild",
	})
	if err != nil {
		t.Fatalf("CreateFaction (f2): %v", err)
	}

	// GetFactionByID
	got, err := q.GetFactionByID(ctx, f1.ID)
	if err != nil {
		t.Fatalf("GetFactionByID: %v", err)
	}
	if got.ID != f1.ID {
		t.Error("GetFactionByID: ID mismatch")
	}

	// ListFactionsByCampaign
	factions, err := q.ListFactionsByCampaign(ctx, camp.ID)
	if err != nil {
		t.Fatalf("ListFactionsByCampaign: %v", err)
	}
	if len(factions) != 2 {
		t.Errorf("ListFactionsByCampaign: expected 2, got %d", len(factions))
	}

	// UpdateFaction
	updated, err := q.UpdateFaction(ctx, statedb.UpdateFactionParams{
		ID:          f1.ID,
		Name:        "The Holy Order",
		Description: txt("An even nobler order"),
		Agenda:      txt("Protect and serve"),
		Territory:   txt("Capital City"),
	})
	if err != nil {
		t.Fatalf("UpdateFaction: %v", err)
	}
	if updated.Name != "The Holy Order" {
		t.Errorf("UpdateFaction: expected The Holy Order, got %q", updated.Name)
	}

	// CreateFactionRelationship
	rel, err := q.CreateFactionRelationship(ctx, statedb.CreateFactionRelationshipParams{
		FactionID:        f1.ID,
		RelatedFactionID: f2.ID,
		RelationshipType: "rivalry",
		Description:      txt("Long-standing conflict"),
	})
	if err != nil {
		t.Fatalf("CreateFactionRelationship: %v", err)
	}

	// GetFactionRelationships
	rels, err := q.GetFactionRelationships(ctx, f1.ID)
	if err != nil {
		t.Fatalf("GetFactionRelationships: %v", err)
	}
	if len(rels) != 1 || rels[0].ID != rel.ID {
		t.Error("GetFactionRelationships: unexpected result")
	}

	// UpdateFactionRelationship
	updRel, err := q.UpdateFactionRelationship(ctx, statedb.UpdateFactionRelationshipParams{
		ID:               rel.ID,
		RelationshipType: "alliance",
		Description:      txt("Forged in battle"),
	})
	if err != nil {
		t.Fatalf("UpdateFactionRelationship: %v", err)
	}
	if updRel.RelationshipType != "alliance" {
		t.Errorf("UpdateFactionRelationship: expected alliance, got %q", updRel.RelationshipType)
	}
}

// TestIntegrationNPCs tests full CRUD and specialised updates for the npcs table.
func TestIntegrationNPCs(t *testing.T) {
	ctx := context.Background()
	q := newTx(t)
	user := createUser(t, q, "npc-user")
	camp := createCampaign(t, q, user.ID)
	loc := createLocation(t, q, camp.ID, "Tavern")
	faction := createFaction(t, q, camp.ID)

	// CreateNPC
	npc, err := q.CreateNPC(ctx, statedb.CreateNPCParams{
		CampaignID:  camp.ID,
		Name:        "Gandalf",
		Description: txt("A wise wizard"),
		Personality: txt("Enigmatic"),
		Disposition: 50,
		LocationID:  loc.ID,
		FactionID:   faction.ID,
		Alive:       true,
		Hp:          int4(80),
	})
	if err != nil {
		t.Fatalf("CreateNPC: %v", err)
	}
	if npc.Name != "Gandalf" || !npc.Alive {
		t.Errorf("CreateNPC: unexpected result")
	}

	// GetNPCByID
	got, err := q.GetNPCByID(ctx, npc.ID)
	if err != nil {
		t.Fatalf("GetNPCByID: %v", err)
	}
	if got.ID != npc.ID {
		t.Error("GetNPCByID: ID mismatch")
	}

	// ListNPCsByCampaign
	npcs, err := q.ListNPCsByCampaign(ctx, camp.ID)
	if err != nil {
		t.Fatalf("ListNPCsByCampaign: %v", err)
	}
	if len(npcs) != 1 || npcs[0].ID != npc.ID {
		t.Error("ListNPCsByCampaign: unexpected result")
	}

	// ListNPCsByLocation
	byLoc, err := q.ListNPCsByLocation(ctx, statedb.ListNPCsByLocationParams{
		CampaignID: camp.ID,
		LocationID: loc.ID,
	})
	if err != nil {
		t.Fatalf("ListNPCsByLocation: %v", err)
	}
	if len(byLoc) != 1 || byLoc[0].ID != npc.ID {
		t.Error("ListNPCsByLocation: unexpected result")
	}

	// ListNPCsByFaction
	byFaction, err := q.ListNPCsByFaction(ctx, statedb.ListNPCsByFactionParams{
		CampaignID: camp.ID,
		FactionID:  faction.ID,
	})
	if err != nil {
		t.Fatalf("ListNPCsByFaction: %v", err)
	}
	if len(byFaction) != 1 || byFaction[0].ID != npc.ID {
		t.Error("ListNPCsByFaction: unexpected result")
	}

	// ListAliveNPCsByLocation
	alive, err := q.ListAliveNPCsByLocation(ctx, statedb.ListAliveNPCsByLocationParams{
		CampaignID: camp.ID,
		LocationID: loc.ID,
	})
	if err != nil {
		t.Fatalf("ListAliveNPCsByLocation: %v", err)
	}
	if len(alive) != 1 {
		t.Error("ListAliveNPCsByLocation: expected 1 alive NPC")
	}

	// UpdateNPC
	upd, err := q.UpdateNPC(ctx, statedb.UpdateNPCParams{
		ID:          npc.ID,
		Name:        "Gandalf the White",
		Disposition: 75,
		LocationID:  loc.ID,
		FactionID:   faction.ID,
		Alive:       true,
		Hp:          int4(100),
	})
	if err != nil {
		t.Fatalf("UpdateNPC: %v", err)
	}
	if upd.Name != "Gandalf the White" || upd.Disposition != 75 {
		t.Errorf("UpdateNPC: unexpected result")
	}

	// UpdateNPCDisposition
	upd, err = q.UpdateNPCDisposition(ctx, statedb.UpdateNPCDispositionParams{ID: npc.ID, Disposition: -10})
	if err != nil {
		t.Fatalf("UpdateNPCDisposition: %v", err)
	}
	if upd.Disposition != -10 {
		t.Errorf("UpdateNPCDisposition: expected -10, got %d", upd.Disposition)
	}

	// UpdateNPCLocation – move to a new location
	loc2 := createLocation(t, q, camp.ID, "Castle")
	upd, err = q.UpdateNPCLocation(ctx, statedb.UpdateNPCLocationParams{ID: npc.ID, LocationID: loc2.ID})
	if err != nil {
		t.Fatalf("UpdateNPCLocation: %v", err)
	}
	if upd.LocationID != loc2.ID {
		t.Error("UpdateNPCLocation: location not updated")
	}

	// KillNPC
	killed, err := q.KillNPC(ctx, npc.ID)
	if err != nil {
		t.Fatalf("KillNPC: %v", err)
	}
	if killed.Alive {
		t.Error("KillNPC: NPC should be dead")
	}

	// ListAliveNPCsByLocation should now return 0 for the original location
	alive, err = q.ListAliveNPCsByLocation(ctx, statedb.ListAliveNPCsByLocationParams{
		CampaignID: camp.ID,
		LocationID: loc.ID,
	})
	if err != nil {
		t.Fatalf("ListAliveNPCsByLocation after kill: %v", err)
	}
	if len(alive) != 0 {
		t.Errorf("ListAliveNPCsByLocation: expected 0, got %d", len(alive))
	}
}

// TestIntegrationPlayerCharacters tests full CRUD and updates for player characters.
func TestIntegrationPlayerCharacters(t *testing.T) {
	ctx := context.Background()
	q := newTx(t)
	user := createUser(t, q, "pc-user")
	camp := createCampaign(t, q, user.ID)
	loc := createLocation(t, q, camp.ID, "Starting Village")

	// CreatePlayerCharacter
	pc, err := q.CreatePlayerCharacter(ctx, statedb.CreatePlayerCharacterParams{
		CampaignID:        camp.ID,
		UserID:            user.ID,
		Name:              "Frodo",
		Description:       txt("A brave hobbit"),
		Hp:                100,
		MaxHp:             100,
		Experience:        0,
		Level:             1,
		Status:            "active",
		CurrentLocationID: loc.ID,
	})
	if err != nil {
		t.Fatalf("CreatePlayerCharacter: %v", err)
	}
	if pc.Name != "Frodo" || pc.Level != 1 {
		t.Errorf("CreatePlayerCharacter: unexpected result")
	}

	// GetPlayerCharacterByID
	got, err := q.GetPlayerCharacterByID(ctx, pc.ID)
	if err != nil {
		t.Fatalf("GetPlayerCharacterByID: %v", err)
	}
	if got.ID != pc.ID {
		t.Error("GetPlayerCharacterByID: ID mismatch")
	}

	// GetPlayerCharacterByCampaign
	pcs, err := q.GetPlayerCharacterByCampaign(ctx, camp.ID)
	if err != nil {
		t.Fatalf("GetPlayerCharacterByCampaign: %v", err)
	}
	if len(pcs) != 1 || pcs[0].ID != pc.ID {
		t.Error("GetPlayerCharacterByCampaign: unexpected result")
	}

	// UpdatePlayerCharacter
	upd, err := q.UpdatePlayerCharacter(ctx, statedb.UpdatePlayerCharacterParams{
		ID:         pc.ID,
		Name:       "Frodo Baggins",
		Hp:         90,
		MaxHp:      100,
		Experience: 100,
		Level:      2,
		Status:     "active",
	})
	if err != nil {
		t.Fatalf("UpdatePlayerCharacter: %v", err)
	}
	if upd.Name != "Frodo Baggins" || upd.Level != 2 {
		t.Errorf("UpdatePlayerCharacter: unexpected result")
	}

	// UpdatePlayerStats
	stats := []byte(`{"strength":18,"dexterity":14}`)
	upd, err = q.UpdatePlayerStats(ctx, statedb.UpdatePlayerStatsParams{ID: pc.ID, Stats: stats})
	if err != nil {
		t.Fatalf("UpdatePlayerStats: %v", err)
	}

	// UpdatePlayerHP
	upd, err = q.UpdatePlayerHP(ctx, statedb.UpdatePlayerHPParams{ID: pc.ID, Hp: 50, MaxHp: 100})
	if err != nil {
		t.Fatalf("UpdatePlayerHP: %v", err)
	}
	if upd.Hp != 50 {
		t.Errorf("UpdatePlayerHP: expected 50, got %d", upd.Hp)
	}

	// UpdatePlayerExperience
	upd, err = q.UpdatePlayerExperience(ctx, statedb.UpdatePlayerExperienceParams{ID: pc.ID, Experience: 500, Level: 3})
	if err != nil {
		t.Fatalf("UpdatePlayerExperience: %v", err)
	}
	if upd.Level != 3 {
		t.Errorf("UpdatePlayerExperience: expected level 3, got %d", upd.Level)
	}

	// UpdatePlayerLocation
	loc2 := createLocation(t, q, camp.ID, "Dark Dungeon")
	upd, err = q.UpdatePlayerLocation(ctx, statedb.UpdatePlayerLocationParams{ID: pc.ID, CurrentLocationID: loc2.ID})
	if err != nil {
		t.Fatalf("UpdatePlayerLocation: %v", err)
	}
	if upd.CurrentLocationID != loc2.ID {
		t.Error("UpdatePlayerLocation: location not updated")
	}

	// UpdatePlayerStatus
	upd, err = q.UpdatePlayerStatus(ctx, statedb.UpdatePlayerStatusParams{ID: pc.ID, Status: "resting"})
	if err != nil {
		t.Fatalf("UpdatePlayerStatus: %v", err)
	}
	if upd.Status != "resting" {
		t.Errorf("UpdatePlayerStatus: expected resting, got %q", upd.Status)
	}
}

// TestIntegrationQuests tests full CRUD for quests, including subquests.
func TestIntegrationQuests(t *testing.T) {
	ctx := context.Background()
	q := newTx(t)
	user := createUser(t, q, "quest-user")
	camp := createCampaign(t, q, user.ID)

	// CreateQuest (main)
	main, err := q.CreateQuest(ctx, statedb.CreateQuestParams{
		CampaignID: camp.ID,
		Title:      "Destroy the Ring",
		QuestType:  "main",
		Status:     "active",
	})
	if err != nil {
		t.Fatalf("CreateQuest (main): %v", err)
	}

	// CreateQuest (subquest)
	sub, err := q.CreateQuest(ctx, statedb.CreateQuestParams{
		CampaignID:    camp.ID,
		ParentQuestID: main.ID,
		Title:         "Find the Fellowship",
		QuestType:     "side",
		Status:        "active",
	})
	if err != nil {
		t.Fatalf("CreateQuest (sub): %v", err)
	}
	if !sub.ParentQuestID.Valid || sub.ParentQuestID != main.ID {
		t.Error("CreateQuest (sub): parent_quest_id not set")
	}

	// GetQuestByID
	got, err := q.GetQuestByID(ctx, main.ID)
	if err != nil {
		t.Fatalf("GetQuestByID: %v", err)
	}
	if got.ID != main.ID {
		t.Error("GetQuestByID: ID mismatch")
	}

	// ListQuestsByCampaign
	quests, err := q.ListQuestsByCampaign(ctx, camp.ID)
	if err != nil {
		t.Fatalf("ListQuestsByCampaign: %v", err)
	}
	if len(quests) != 2 {
		t.Errorf("ListQuestsByCampaign: expected 2, got %d", len(quests))
	}

	// ListActiveQuests
	active, err := q.ListActiveQuests(ctx, camp.ID)
	if err != nil {
		t.Fatalf("ListActiveQuests: %v", err)
	}
	if len(active) != 2 {
		t.Errorf("ListActiveQuests: expected 2, got %d", len(active))
	}

	// ListQuestsByType
	mainType, err := q.ListQuestsByType(ctx, statedb.ListQuestsByTypeParams{CampaignID: camp.ID, QuestType: "main"})
	if err != nil {
		t.Fatalf("ListQuestsByType: %v", err)
	}
	if len(mainType) != 1 || mainType[0].ID != main.ID {
		t.Error("ListQuestsByType: unexpected result")
	}

	// ListSubquestsByParentQuest
	subs, err := q.ListSubquestsByParentQuest(ctx, main.ID)
	if err != nil {
		t.Fatalf("ListSubquestsByParentQuest: %v", err)
	}
	if len(subs) != 1 || subs[0].ID != sub.ID {
		t.Error("ListSubquestsByParentQuest: unexpected result")
	}

	// UpdateQuest
	upd, err := q.UpdateQuest(ctx, statedb.UpdateQuestParams{
		ID:        main.ID,
		Title:     "Destroy the One Ring",
		QuestType: "main",
		Status:    "active",
	})
	if err != nil {
		t.Fatalf("UpdateQuest: %v", err)
	}
	if upd.Title != "Destroy the One Ring" {
		t.Errorf("UpdateQuest: unexpected title %q", upd.Title)
	}

	// UpdateQuestStatus
	upd, err = q.UpdateQuestStatus(ctx, statedb.UpdateQuestStatusParams{ID: main.ID, Status: "completed"})
	if err != nil {
		t.Fatalf("UpdateQuestStatus: %v", err)
	}
	if upd.Status != "completed" {
		t.Errorf("UpdateQuestStatus: expected completed, got %q", upd.Status)
	}

	// Active quests should decrease (only 1 left: sub is still active)
	active, err = q.ListActiveQuests(ctx, camp.ID)
	if err != nil {
		t.Fatalf("ListActiveQuests after completion: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("ListActiveQuests: expected 1, got %d", len(active))
	}
}

// TestIntegrationQuestObjectives tests CRUD for quest objectives.
func TestIntegrationQuestObjectives(t *testing.T) {
	ctx := context.Background()
	q := newTx(t)
	user := createUser(t, q, "obj-user")
	camp := createCampaign(t, q, user.ID)
	quest := createQuest(t, q, camp.ID)

	// CreateObjective
	obj, err := q.CreateObjective(ctx, statedb.CreateObjectiveParams{
		QuestID:     quest.ID,
		Description: "Reach Rivendell",
		Completed:   false,
		OrderIndex:  1,
	})
	if err != nil {
		t.Fatalf("CreateObjective: %v", err)
	}
	if obj.Description != "Reach Rivendell" || obj.Completed {
		t.Errorf("CreateObjective: unexpected result")
	}

	obj2, err := q.CreateObjective(ctx, statedb.CreateObjectiveParams{
		QuestID:     quest.ID,
		Description: "Form the Fellowship",
		Completed:   false,
		OrderIndex:  2,
	})
	if err != nil {
		t.Fatalf("CreateObjective (obj2): %v", err)
	}

	// ListObjectivesByQuest
	objs, err := q.ListObjectivesByQuest(ctx, quest.ID)
	if err != nil {
		t.Fatalf("ListObjectivesByQuest: %v", err)
	}
	if len(objs) != 2 {
		t.Errorf("ListObjectivesByQuest: expected 2, got %d", len(objs))
	}

	// CompleteObjective
	done, err := q.CompleteObjective(ctx, obj.ID)
	if err != nil {
		t.Fatalf("CompleteObjective: %v", err)
	}
	if !done.Completed {
		t.Error("CompleteObjective: expected completed=true")
	}

	// UpdateObjective
	upd, err := q.UpdateObjective(ctx, statedb.UpdateObjectiveParams{
		ID:          obj2.ID,
		Description: "Form the Fellowship of the Ring",
		Completed:   false,
		OrderIndex:  2,
	})
	if err != nil {
		t.Fatalf("UpdateObjective: %v", err)
	}
	if upd.Description != "Form the Fellowship of the Ring" {
		t.Errorf("UpdateObjective: unexpected description %q", upd.Description)
	}
}

// TestIntegrationItems tests full CRUD for the items table.
func TestIntegrationItems(t *testing.T) {
	ctx := context.Background()
	q := newTx(t)
	user := createUser(t, q, "item-user")
	camp := createCampaign(t, q, user.ID)
	pc1 := createPC(t, q, camp.ID, user.ID)
	pc2 := createPC(t, q, camp.ID, user.ID)

	// CreateItem (owned by pc1)
	item, err := q.CreateItem(ctx, statedb.CreateItemParams{
		CampaignID:        camp.ID,
		PlayerCharacterID: pc1.ID,
		Name:              "The One Ring",
		Description:       txt("A dangerous golden ring"),
		ItemType:          "magical",
		Rarity:            "legendary",
		Equipped:          false,
		Quantity:          1,
	})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}
	if item.Name != "The One Ring" {
		t.Errorf("CreateItem: unexpected name %q", item.Name)
	}

	// GetItemByID
	got, err := q.GetItemByID(ctx, item.ID)
	if err != nil {
		t.Fatalf("GetItemByID: %v", err)
	}
	if got.ID != item.ID {
		t.Error("GetItemByID: ID mismatch")
	}

	// ListItemsByPlayer
	byPlayer, err := q.ListItemsByPlayer(ctx, statedb.ListItemsByPlayerParams{
		CampaignID:        camp.ID,
		PlayerCharacterID: pc1.ID,
	})
	if err != nil {
		t.Fatalf("ListItemsByPlayer: %v", err)
	}
	if len(byPlayer) != 1 || byPlayer[0].ID != item.ID {
		t.Error("ListItemsByPlayer: unexpected result")
	}

	// ListItemsByType
	byType, err := q.ListItemsByType(ctx, statedb.ListItemsByTypeParams{
		CampaignID: camp.ID,
		ItemType:   "magical",
	})
	if err != nil {
		t.Fatalf("ListItemsByType: %v", err)
	}
	if len(byType) != 1 || byType[0].ID != item.ID {
		t.Error("ListItemsByType: unexpected result")
	}

	// UpdateItem
	upd, err := q.UpdateItem(ctx, statedb.UpdateItemParams{
		ID:       item.ID,
		Name:     "The Ring of Power",
		ItemType: "magical",
		Rarity:   "legendary",
		Equipped: false,
		Quantity: 1,
	})
	if err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}
	if upd.Name != "The Ring of Power" {
		t.Errorf("UpdateItem: unexpected name %q", upd.Name)
	}

	// UpdateItemEquipped
	upd, err = q.UpdateItemEquipped(ctx, statedb.UpdateItemEquippedParams{ID: item.ID, Equipped: true})
	if err != nil {
		t.Fatalf("UpdateItemEquipped: %v", err)
	}
	if !upd.Equipped {
		t.Error("UpdateItemEquipped: expected equipped=true")
	}

	// UpdateItemQuantity
	upd, err = q.UpdateItemQuantity(ctx, statedb.UpdateItemQuantityParams{ID: item.ID, Quantity: 3})
	if err != nil {
		t.Fatalf("UpdateItemQuantity: %v", err)
	}
	if upd.Quantity != 3 {
		t.Errorf("UpdateItemQuantity: expected 3, got %d", upd.Quantity)
	}

	// TransferItem to pc2
	upd, err = q.TransferItem(ctx, statedb.TransferItemParams{ID: item.ID, PlayerCharacterID: pc2.ID})
	if err != nil {
		t.Fatalf("TransferItem: %v", err)
	}
	if upd.PlayerCharacterID != pc2.ID {
		t.Error("TransferItem: player_character_id not updated")
	}

	// DeleteItem
	if err := q.DeleteItem(ctx, item.ID); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}
	if _, err := q.GetItemByID(ctx, item.ID); err == nil {
		t.Error("expected error after delete, got nil")
	}
}

// TestIntegrationLocationConnections tests CRUD and directional queries for connections.
func TestIntegrationLocationConnections(t *testing.T) {
	ctx := context.Background()
	q := newTx(t)
	user := createUser(t, q, "conn-user")
	camp := createCampaign(t, q, user.ID)
	locA := createLocation(t, q, camp.ID, "Town")
	locB := createLocation(t, q, camp.ID, "Forest")

	// CreateConnection (bidirectional)
	conn, err := q.CreateConnection(ctx, statedb.CreateConnectionParams{
		CampaignID:     camp.ID,
		FromLocationID: locA.ID,
		ToLocationID:   locB.ID,
		Description:    txt("A winding path"),
		Bidirectional:  true,
		TravelTime:     txt("1 hour"),
	})
	if err != nil {
		t.Fatalf("CreateConnection: %v", err)
	}
	if conn.FromLocationID != locA.ID {
		t.Error("CreateConnection: from_location_id mismatch")
	}

	// GetConnectionsFromLocation (from A → B via forward edge)
	fromA, err := q.GetConnectionsFromLocation(ctx, statedb.GetConnectionsFromLocationParams{
		CampaignID: camp.ID,
		LocationID: locA.ID,
	})
	if err != nil {
		t.Fatalf("GetConnectionsFromLocation (A): %v", err)
	}
	if len(fromA) != 1 || fromA[0].ConnectedLocationID != locB.ID {
		t.Error("GetConnectionsFromLocation (A): unexpected result")
	}

	// GetConnectionsFromLocation (from B → A via bidirectional reverse)
	fromB, err := q.GetConnectionsFromLocation(ctx, statedb.GetConnectionsFromLocationParams{
		CampaignID: camp.ID,
		LocationID: locB.ID,
	})
	if err != nil {
		t.Fatalf("GetConnectionsFromLocation (B): %v", err)
	}
	if len(fromB) != 1 || fromB[0].ConnectedLocationID != locA.ID {
		t.Error("GetConnectionsFromLocation (B): expected reverse edge via bidirectional")
	}

	// DeleteConnection
	if err := q.DeleteConnection(ctx, statedb.DeleteConnectionParams{
		ID:         conn.ID,
		CampaignID: camp.ID,
	}); err != nil {
		t.Fatalf("DeleteConnection: %v", err)
	}
	fromA, err = q.GetConnectionsFromLocation(ctx, statedb.GetConnectionsFromLocationParams{
		CampaignID: camp.ID,
		LocationID: locA.ID,
	})
	if err != nil {
		t.Fatalf("GetConnectionsFromLocation after delete: %v", err)
	}
	if len(fromA) != 0 {
		t.Errorf("expected 0 connections after delete, got %d", len(fromA))
	}
}

// TestIntegrationEntityRelationships tests full CRUD for entity_relationships.
func TestIntegrationEntityRelationships(t *testing.T) {
	ctx := context.Background()
	q := newTx(t)
	user := createUser(t, q, "rel-user")
	camp := createCampaign(t, q, user.ID)
	npc1 := createNPC(t, q, camp.ID)
	npc2 := createNPC(t, q, camp.ID)

	// CreateRelationship
	rel, err := q.CreateRelationship(ctx, statedb.CreateRelationshipParams{
		CampaignID:       camp.ID,
		SourceEntityType: "npc",
		SourceEntityID:   npc1.ID,
		TargetEntityType: "npc",
		TargetEntityID:   npc2.ID,
		RelationshipType: "ally",
		Description:      txt("Companions in arms"),
		Strength:         int4(80),
	})
	if err != nil {
		t.Fatalf("CreateRelationship: %v", err)
	}
	if rel.RelationshipType != "ally" {
		t.Errorf("CreateRelationship: unexpected type %q", rel.RelationshipType)
	}

	// GetRelationshipsByEntity
	byEntity, err := q.GetRelationshipsByEntity(ctx, statedb.GetRelationshipsByEntityParams{
		CampaignID: camp.ID,
		EntityType: "npc",
		EntityID:   npc1.ID,
	})
	if err != nil {
		t.Fatalf("GetRelationshipsByEntity: %v", err)
	}
	if len(byEntity) != 1 || byEntity[0].ID != rel.ID {
		t.Error("GetRelationshipsByEntity: unexpected result")
	}

	// GetRelationshipsBetween
	between, err := q.GetRelationshipsBetween(ctx, statedb.GetRelationshipsBetweenParams{
		CampaignID:       camp.ID,
		FirstEntityType:  "npc",
		FirstEntityID:    npc1.ID,
		SecondEntityType: "npc",
		SecondEntityID:   npc2.ID,
	})
	if err != nil {
		t.Fatalf("GetRelationshipsBetween: %v", err)
	}
	if len(between) != 1 || between[0].ID != rel.ID {
		t.Error("GetRelationshipsBetween: unexpected result")
	}

	// ListRelationshipsByCampaign
	all, err := q.ListRelationshipsByCampaign(ctx, camp.ID)
	if err != nil {
		t.Fatalf("ListRelationshipsByCampaign: %v", err)
	}
	if len(all) != 1 || all[0].ID != rel.ID {
		t.Error("ListRelationshipsByCampaign: unexpected result")
	}

	// UpdateRelationship
	upd, err := q.UpdateRelationship(ctx, statedb.UpdateRelationshipParams{
		ID:               rel.ID,
		CampaignID:       camp.ID,
		RelationshipType: "enemy",
		Description:      txt("Fallen out"),
		Strength:         int4(-50),
	})
	if err != nil {
		t.Fatalf("UpdateRelationship: %v", err)
	}
	if upd.RelationshipType != "enemy" {
		t.Errorf("UpdateRelationship: expected enemy, got %q", upd.RelationshipType)
	}

	// DeleteRelationship
	if err := q.DeleteRelationship(ctx, statedb.DeleteRelationshipParams{
		ID:         rel.ID,
		CampaignID: camp.ID,
	}); err != nil {
		t.Fatalf("DeleteRelationship: %v", err)
	}
	all, err = q.ListRelationshipsByCampaign(ctx, camp.ID)
	if err != nil {
		t.Fatalf("ListRelationshipsByCampaign after delete: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 relationships after delete, got %d", len(all))
	}
}

// TestIntegrationWorldFacts tests CRUD and the supersede chain for world_facts.
func TestIntegrationWorldFacts(t *testing.T) {
	ctx := context.Background()
	q := newTx(t)
	user := createUser(t, q, "facts-user")
	camp := createCampaign(t, q, user.ID)

	// CreateFact
	f1, err := q.CreateFact(ctx, statedb.CreateFactParams{
		CampaignID: camp.ID,
		Fact:       "The king is alive",
		Category:   "politics",
		Source:     "herald",
	})
	if err != nil {
		t.Fatalf("CreateFact: %v", err)
	}

	f2, err := q.CreateFact(ctx, statedb.CreateFactParams{
		CampaignID: camp.ID,
		Fact:       "Dragons exist in the north",
		Category:   "lore",
		Source:     "scholar",
	})
	if err != nil {
		t.Fatalf("CreateFact (f2): %v", err)
	}

	// GetFactByID
	got, err := q.GetFactByID(ctx, f1.ID)
	if err != nil {
		t.Fatalf("GetFactByID: %v", err)
	}
	if got.ID != f1.ID {
		t.Error("GetFactByID: ID mismatch")
	}

	// ListFactsByCampaign
	facts, err := q.ListFactsByCampaign(ctx, camp.ID)
	if err != nil {
		t.Fatalf("ListFactsByCampaign: %v", err)
	}
	if len(facts) != 2 {
		t.Errorf("ListFactsByCampaign: expected 2, got %d", len(facts))
	}

	// ListFactsByCategory
	politics, err := q.ListFactsByCategory(ctx, statedb.ListFactsByCategoryParams{
		CampaignID: camp.ID,
		Category:   "politics",
	})
	if err != nil {
		t.Fatalf("ListFactsByCategory: %v", err)
	}
	if len(politics) != 1 || politics[0].ID != f1.ID {
		t.Error("ListFactsByCategory: unexpected result")
	}

	// ListActiveFactsByCampaign – both unsuperseded
	active, err := q.ListActiveFactsByCampaign(ctx, camp.ID)
	if err != nil {
		t.Fatalf("ListActiveFactsByCampaign: %v", err)
	}
	if len(active) != 2 {
		t.Errorf("ListActiveFactsByCampaign: expected 2, got %d", len(active))
	}

	// SupersedeFact – replace f1 with a corrected fact
	newFact, err := q.SupersedeFact(ctx, statedb.SupersedeFactParams{
		OldFactID: f1.ID,
		Fact:      "The king is dead, long live the king",
		Category:  "politics",
		Source:    "town crier",
	})
	if err != nil {
		t.Fatalf("SupersedeFact: %v", err)
	}
	if newFact.Fact != "The king is dead, long live the king" {
		t.Errorf("SupersedeFact: unexpected fact %q", newFact.Fact)
	}

	// ListActiveFactsByCampaign – f1 is now superseded; only f2 and newFact are active
	active, err = q.ListActiveFactsByCampaign(ctx, camp.ID)
	if err != nil {
		t.Fatalf("ListActiveFactsByCampaign after supersede: %v", err)
	}
	if len(active) != 2 {
		t.Errorf("ListActiveFactsByCampaign: expected 2 (f2 + newFact), got %d", len(active))
	}

	// Verify f1 is superseded
	f1Updated, err := q.GetFactByID(ctx, f1.ID)
	if err != nil {
		t.Fatalf("GetFactByID (old): %v", err)
	}
	if !f1Updated.SupersededBy.Valid {
		t.Error("expected f1 to be superseded")
	}
	if f1Updated.SupersededBy != newFact.ID {
		t.Error("f1.superseded_by should point to newFact.id")
	}

	// Unused but validated: f2 still active
	_ = f2
}

// TestIntegrationExpandedWorldTables tests CRUD and association queries for expanded world tables.
func TestIntegrationExpandedWorldTables(t *testing.T) {
	ctx := context.Background()
	q := newTx(t)
	user := createUser(t, q, "expanded-world-user")
	camp := createCampaign(t, q, user.ID)
	faction, err := q.CreateFaction(ctx, statedb.CreateFactionParams{
		CampaignID: camp.ID,
		Name:       "Expanded Faction",
	})
	if err != nil {
		t.Fatalf("CreateFaction: %v", err)
	}

	// Languages CRUD + ListLanguagesByFaction
	lang, err := q.CreateLanguage(ctx, statedb.CreateLanguageParams{
		CampaignID: camp.ID,
		Name:       "Eldertongue",
		Description: txt("Ancestral language"),
		Phonology:  []byte(`{"vowels":["a","e"]}`),
		Naming:     []byte(`{"pattern":"CV-CV"}`),
		Vocabulary: []byte(`{"sun":"sol"}`),
		SpokenByFactionIds: []pgtype.UUID{faction.ID},
	})
	if err != nil {
		t.Fatalf("CreateLanguage: %v", err)
	}
	gotLang, err := q.GetLanguageByID(ctx, lang.ID)
	if err != nil {
		t.Fatalf("GetLanguageByID: %v", err)
	}
	if gotLang.ID != lang.ID {
		t.Error("GetLanguageByID: ID mismatch")
	}
	langs, err := q.ListLanguagesByCampaign(ctx, camp.ID)
	if err != nil {
		t.Fatalf("ListLanguagesByCampaign: %v", err)
	}
	if len(langs) != 1 || langs[0].ID != lang.ID {
		t.Error("ListLanguagesByCampaign: unexpected result")
	}
	updatedLang, err := q.UpdateLanguage(ctx, statedb.UpdateLanguageParams{
		ID:         lang.ID,
		Name:       "Modern Eldertongue",
		Description: txt("Contemporary dialect"),
		Phonology:  []byte(`{"vowels":["a","e","i"]}`),
		Naming:     []byte(`{"pattern":"CVC"}`),
		Vocabulary: []byte(`{"sun":"sol","moon":"luna"}`),
		SpokenByFactionIds: []pgtype.UUID{faction.ID},
	})
	if err != nil {
		t.Fatalf("UpdateLanguage: %v", err)
	}
	if updatedLang.Name != "Modern Eldertongue" {
		t.Errorf("UpdateLanguage: expected Modern Eldertongue, got %q", updatedLang.Name)
	}
	if updatedLang.Description != "Contemporary dialect" {
		t.Errorf("UpdateLanguage: expected Contemporary dialect description, got %q", updatedLang.Description)
	}
	langsByFaction, err := q.ListLanguagesByFaction(ctx, faction.ID)
	if err != nil {
		t.Fatalf("ListLanguagesByFaction: %v", err)
	}
	if len(langsByFaction) != 1 || langsByFaction[0].ID != lang.ID {
		t.Error("ListLanguagesByFaction: unexpected result")
	}

	// Belief systems CRUD
	belief, err := q.CreateBeliefSystem(ctx, statedb.CreateBeliefSystemParams{
		CampaignID: camp.ID,
		Name:       "Solar Creed",
		Details:    []byte(`{"principle":"light"}`),
	})
	if err != nil {
		t.Fatalf("CreateBeliefSystem: %v", err)
	}
	gotBelief, err := q.GetBeliefSystemByID(ctx, belief.ID)
	if err != nil {
		t.Fatalf("GetBeliefSystemByID: %v", err)
	}
	if gotBelief.ID != belief.ID {
		t.Error("GetBeliefSystemByID: ID mismatch")
	}
	beliefs, err := q.ListBeliefSystemsByCampaign(ctx, camp.ID)
	if err != nil {
		t.Fatalf("ListBeliefSystemsByCampaign: %v", err)
	}
	if len(beliefs) != 1 || beliefs[0].ID != belief.ID {
		t.Error("ListBeliefSystemsByCampaign: unexpected result")
	}
	updatedBelief, err := q.UpdateBeliefSystem(ctx, statedb.UpdateBeliefSystemParams{
		ID:      belief.ID,
		Name:    "Reformed Solar Creed",
		Details: []byte(`{"principle":"balance"}`),
	})
	if err != nil {
		t.Fatalf("UpdateBeliefSystem: %v", err)
	}
	if updatedBelief.Name != "Reformed Solar Creed" {
		t.Errorf("UpdateBeliefSystem: expected Reformed Solar Creed, got %q", updatedBelief.Name)
	}

	// Economic systems CRUD
	economy, err := q.CreateEconomicSystem(ctx, statedb.CreateEconomicSystemParams{
		CampaignID: camp.ID,
		Name:       "Coin Guild",
		Details:    []byte(`{"currency":"mark"}`),
	})
	if err != nil {
		t.Fatalf("CreateEconomicSystem: %v", err)
	}
	gotEconomy, err := q.GetEconomicSystemByID(ctx, economy.ID)
	if err != nil {
		t.Fatalf("GetEconomicSystemByID: %v", err)
	}
	if gotEconomy.ID != economy.ID {
		t.Error("GetEconomicSystemByID: ID mismatch")
	}
	economicSystems, err := q.ListEconomicSystemsByCampaign(ctx, camp.ID)
	if err != nil {
		t.Fatalf("ListEconomicSystemsByCampaign: %v", err)
	}
	if len(economicSystems) != 1 || economicSystems[0].ID != economy.ID {
		t.Error("ListEconomicSystemsByCampaign: unexpected result")
	}
	updatedEconomy, err := q.UpdateEconomicSystem(ctx, statedb.UpdateEconomicSystemParams{
		ID:      economy.ID,
		Name:    "Merchant Guild",
		Details: []byte(`{"currency":"crown"}`),
	})
	if err != nil {
		t.Fatalf("UpdateEconomicSystem: %v", err)
	}
	if updatedEconomy.Name != "Merchant Guild" {
		t.Errorf("UpdateEconomicSystem: expected Merchant Guild, got %q", updatedEconomy.Name)
	}

	// Cultures CRUD + GetBeliefSystemByCulture + ListCulturesByLanguage
	culture, err := q.CreateCulture(ctx, statedb.CreateCultureParams{
		CampaignID:     camp.ID,
		LanguageID:     lang.ID,
		BeliefSystemID: belief.ID,
		Name:           "Highland Culture",
		Details:        []byte(`{"value":"honor"}`),
	})
	if err != nil {
		t.Fatalf("CreateCulture: %v", err)
	}
	gotCulture, err := q.GetCultureByID(ctx, culture.ID)
	if err != nil {
		t.Fatalf("GetCultureByID: %v", err)
	}
	if gotCulture.ID != culture.ID {
		t.Error("GetCultureByID: ID mismatch")
	}
	cultures, err := q.ListCulturesByCampaign(ctx, camp.ID)
	if err != nil {
		t.Fatalf("ListCulturesByCampaign: %v", err)
	}
	if len(cultures) != 1 || cultures[0].ID != culture.ID {
		t.Error("ListCulturesByCampaign: unexpected result")
	}
	updatedCulture, err := q.UpdateCulture(ctx, statedb.UpdateCultureParams{
		ID:             culture.ID,
		LanguageID:     lang.ID,
		BeliefSystemID: belief.ID,
		Name:           "Coastal Culture",
		Details:        []byte(`{"value":"trade"}`),
	})
	if err != nil {
		t.Fatalf("UpdateCulture: %v", err)
	}
	if updatedCulture.Name != "Coastal Culture" {
		t.Errorf("UpdateCulture: expected Coastal Culture, got %q", updatedCulture.Name)
	}

	beliefByCulture, err := q.GetBeliefSystemByCulture(ctx, culture.ID)
	if err != nil {
		t.Fatalf("GetBeliefSystemByCulture: %v", err)
	}
	if beliefByCulture.ID != belief.ID {
		t.Error("GetBeliefSystemByCulture: unexpected result")
	}

	culturesByLanguage, err := q.ListCulturesByLanguage(ctx, lang.ID)
	if err != nil {
		t.Fatalf("ListCulturesByLanguage: %v", err)
	}
	if len(culturesByLanguage) != 1 || culturesByLanguage[0].ID != culture.ID {
		t.Error("ListCulturesByLanguage: unexpected result")
	}

	if err := q.DeleteCulture(ctx, culture.ID); err != nil {
		t.Fatalf("DeleteCulture: %v", err)
	}
	if err := q.DeleteEconomicSystem(ctx, economy.ID); err != nil {
		t.Fatalf("DeleteEconomicSystem: %v", err)
	}
	if err := q.DeleteBeliefSystem(ctx, belief.ID); err != nil {
		t.Fatalf("DeleteBeliefSystem: %v", err)
	}
	if err := q.DeleteLanguage(ctx, lang.ID); err != nil {
		t.Fatalf("DeleteLanguage: %v", err)
	}
}

// TestIntegrationMemories tests vector storage, retrieval, and cosine similarity search.
func TestIntegrationMemories(t *testing.T) {
	ctx := context.Background()
	q := newTx(t)
	user := createUser(t, q, "mem-user")
	camp := createCampaign(t, q, user.ID)
	loc := createLocation(t, q, camp.ID, "Memory Palace")
	npc := createNPC(t, q, camp.ID)

	// CreateMemory – orthogonal unit vectors for predictable similarity
	m0, err := q.CreateMemory(ctx, statedb.CreateMemoryParams{
		CampaignID: camp.ID,
		Content:    "The hero entered the dungeon",
		Embedding:  makeVec(0),
		MemoryType: "scene",
		LocationID: loc.ID,
	})
	if err != nil {
		t.Fatalf("CreateMemory (m0): %v", err)
	}

	m1, err := q.CreateMemory(ctx, statedb.CreateMemoryParams{
		CampaignID:   camp.ID,
		Content:      "The wizard cast a spell",
		Embedding:    makeVec(1),
		MemoryType:   "npc_interaction",
		LocationID:   loc.ID,
		NpcsInvolved: []pgtype.UUID{npc.ID},
	})
	if err != nil {
		t.Fatalf("CreateMemory (m1): %v", err)
	}

	m2, err := q.CreateMemory(ctx, statedb.CreateMemoryParams{
		CampaignID: camp.ID,
		Content:    "Ancient lore was discovered",
		Embedding:  makeVec(2),
		MemoryType: "lore",
	})
	if err != nil {
		t.Fatalf("CreateMemory (m2): %v", err)
	}

	// GetMemoryByID
	got, err := q.GetMemoryByID(ctx, m0.ID)
	if err != nil {
		t.Fatalf("GetMemoryByID: %v", err)
	}
	if got.Content != m0.Content {
		t.Errorf("GetMemoryByID: unexpected content %q", got.Content)
	}

	// ListMemoriesByCampaign
	mems, err := q.ListMemoriesByCampaign(ctx, camp.ID)
	if err != nil {
		t.Fatalf("ListMemoriesByCampaign: %v", err)
	}
	if len(mems) != 3 {
		t.Errorf("ListMemoriesByCampaign: expected 3, got %d", len(mems))
	}

	// SearchMemoriesBySimilarity – query with vec(0) should rank m0 first (distance 0)
	results, err := q.SearchMemoriesBySimilarity(ctx, statedb.SearchMemoriesBySimilarityParams{
		CampaignID:     camp.ID,
		QueryEmbedding: makeVec(0),
		LimitCount:     3,
	})
	if err != nil {
		t.Fatalf("SearchMemoriesBySimilarity: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("SearchMemoriesBySimilarity: expected 3, got %d", len(results))
	}
	// The closest match to makeVec(0) should be m0 (distance 0)
	if results[0].ID != m0.ID {
		t.Errorf("SearchMemoriesBySimilarity: expected m0 first, got %v", results[0].ID)
	}
	if results[0].Distance > 1e-6 {
		t.Errorf("SearchMemoriesBySimilarity: expected distance ≈0, got %f", results[0].Distance)
	}

	// SearchMemoriesWithFilters – filter by memory_type=lore
	filtered, err := q.SearchMemoriesWithFilters(ctx, statedb.SearchMemoriesWithFiltersParams{
		CampaignID:     camp.ID,
		QueryEmbedding: makeVec(2),
		MemoryType:     txt("lore"),
		LimitCount:     10,
	})
	if err != nil {
		t.Fatalf("SearchMemoriesWithFilters: %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != m2.ID {
		t.Errorf("SearchMemoriesWithFilters: expected only m2 (lore), got %d results", len(filtered))
	}

	// SearchMemoriesWithFilters – filter by npc_id
	filteredByNPC, err := q.SearchMemoriesWithFilters(ctx, statedb.SearchMemoriesWithFiltersParams{
		CampaignID:     camp.ID,
		QueryEmbedding: makeVec(1),
		NpcID:          npc.ID,
		LimitCount:     10,
	})
	if err != nil {
		t.Fatalf("SearchMemoriesWithFilters (npc): %v", err)
	}
	if len(filteredByNPC) != 1 || filteredByNPC[0].ID != m1.ID {
		t.Errorf("SearchMemoriesWithFilters (npc): expected only m1, got %d results", len(filteredByNPC))
	}
}

// TestIntegrationSessionLogs tests CRUD and list operations for session logs.
func TestIntegrationSessionLogs(t *testing.T) {
	ctx := context.Background()
	q := newTx(t)
	user := createUser(t, q, "log-user")
	camp := createCampaign(t, q, user.ID)
	loc := createLocation(t, q, camp.ID, "Battle Field")
	npc := createNPC(t, q, camp.ID)

	// CreateSessionLog
	log1, err := q.CreateSessionLog(ctx, statedb.CreateSessionLogParams{
		CampaignID:   camp.ID,
		TurnNumber:   1,
		PlayerInput:  "I attack the goblin",
		InputType:    "action",
		LlmResponse:  "You swing your sword...",
		LocationID:   loc.ID,
		NpcsInvolved: []pgtype.UUID{npc.ID},
	})
	if err != nil {
		t.Fatalf("CreateSessionLog: %v", err)
	}
	if log1.TurnNumber != 1 {
		t.Errorf("CreateSessionLog: expected turn 1, got %d", log1.TurnNumber)
	}

	log2, err := q.CreateSessionLog(ctx, statedb.CreateSessionLogParams{
		CampaignID:  camp.ID,
		TurnNumber:  2,
		PlayerInput: "I look around",
		InputType:   "action",
		LlmResponse: "You see a dark cave...",
		LocationID:  loc.ID,
	})
	if err != nil {
		t.Fatalf("CreateSessionLog (log2): %v", err)
	}

	// GetSessionLogByID
	got, err := q.GetSessionLogByID(ctx, log1.ID)
	if err != nil {
		t.Fatalf("GetSessionLogByID: %v", err)
	}
	if got.ID != log1.ID {
		t.Error("GetSessionLogByID: ID mismatch")
	}

	// ListSessionLogsByCampaign
	logs, err := q.ListSessionLogsByCampaign(ctx, camp.ID)
	if err != nil {
		t.Fatalf("ListSessionLogsByCampaign: %v", err)
	}
	if len(logs) != 2 {
		t.Errorf("ListSessionLogsByCampaign: expected 2, got %d", len(logs))
	}

	// ListRecentSessionLogs
	recent, err := q.ListRecentSessionLogs(ctx, statedb.ListRecentSessionLogsParams{
		CampaignID: camp.ID,
		LimitCount: 1,
	})
	if err != nil {
		t.Fatalf("ListRecentSessionLogs: %v", err)
	}
	if len(recent) != 1 || recent[0].ID != log2.ID {
		t.Error("ListRecentSessionLogs: expected most recent log (turn 2)")
	}

	// ListSessionLogsByLocation
	byLoc, err := q.ListSessionLogsByLocation(ctx, statedb.ListSessionLogsByLocationParams{
		CampaignID: camp.ID,
		LocationID: loc.ID,
	})
	if err != nil {
		t.Fatalf("ListSessionLogsByLocation: %v", err)
	}
	if len(byLoc) != 2 {
		t.Errorf("ListSessionLogsByLocation: expected 2, got %d", len(byLoc))
	}
}

// TestIntegrationForeignKeyConstraints verifies that FK violations are correctly rejected.
func TestIntegrationForeignKeyConstraints(t *testing.T) {
	ctx := context.Background()

	// phantom UUID that does not exist in any table
	phantom := pgtype.UUID{Bytes: [16]byte{0xFF, 0xFF, 0xFF, 0xFF}, Valid: true}

	t.Run("campaign_requires_valid_user", func(t *testing.T) {
		q := newTx(t)
		_, err := q.CreateCampaign(ctx, statedb.CreateCampaignParams{
			Name:      "orphan",
			Status:    "active",
			CreatedBy: phantom,
		})
		if err == nil {
			t.Fatal("expected FK violation for non-existent created_by user, got nil")
		}
	})

	t.Run("location_requires_valid_campaign", func(t *testing.T) {
		q := newTx(t)
		_, err := q.CreateLocation(ctx, statedb.CreateLocationParams{
			CampaignID: phantom,
			Name:       "ghost town",
		})
		if err == nil {
			t.Fatal("expected FK violation for non-existent campaign_id, got nil")
		}
	})

	t.Run("npc_requires_valid_campaign", func(t *testing.T) {
		q := newTx(t)
		_, err := q.CreateNPC(ctx, statedb.CreateNPCParams{
			CampaignID:  phantom,
			Name:        "ghost npc",
			Disposition: 0,
			Alive:       true,
		})
		if err == nil {
			t.Fatal("expected FK violation for non-existent campaign_id, got nil")
		}
	})

	t.Run("quest_requires_valid_campaign", func(t *testing.T) {
		q := newTx(t)
		_, err := q.CreateQuest(ctx, statedb.CreateQuestParams{
			CampaignID: phantom,
			Title:      "ghost quest",
			QuestType:  "main",
			Status:     "active",
		})
		if err == nil {
			t.Fatal("expected FK violation for non-existent campaign_id, got nil")
		}
	})

	t.Run("delete_user_with_campaigns_blocked", func(t *testing.T) {
		q := newTx(t)
		user := createUser(t, q, "restricted-user")
		createCampaign(t, q, user.ID)
		if err := q.DeleteUser(ctx, user.ID); err == nil {
			t.Fatal("expected FK restriction when deleting user that owns campaigns, got nil")
		}
	})

	t.Run("delete_campaign_with_locations_blocked", func(t *testing.T) {
		q := newTx(t)
		user := createUser(t, q, "camp-with-locs")
		camp := createCampaign(t, q, user.ID)
		createLocation(t, q, camp.ID, "some place")
		if err := q.DeleteCampaign(ctx, camp.ID); err == nil {
			t.Fatal("expected FK restriction when deleting campaign that has locations, got nil")
		}
	})

	t.Run("memory_requires_valid_campaign", func(t *testing.T) {
		q := newTx(t)
		_, err := q.CreateMemory(ctx, statedb.CreateMemoryParams{
			CampaignID: phantom,
			Content:    "ghost memory",
			Embedding:  makeVec(0),
			MemoryType: "scene",
		})
		if err == nil {
			t.Fatal("expected FK violation for non-existent campaign_id, got nil")
		}
	})

	t.Run("item_requires_valid_campaign", func(t *testing.T) {
		q := newTx(t)
		_, err := q.CreateItem(ctx, statedb.CreateItemParams{
			CampaignID: phantom,
			Name:       "ghost item",
			ItemType:   "weapon",
			Rarity:     "common",
			Equipped:   false,
			Quantity:   1,
		})
		if err == nil {
			t.Fatal("expected FK violation for non-existent campaign_id, got nil")
		}
	})
}
