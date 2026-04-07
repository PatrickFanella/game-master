//go:build integration

package handlers_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	clog "github.com/charmbracelet/log"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/PatrickFanella/game-master/internal/auth"
	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/engine"
	"github.com/PatrickFanella/game-master/internal/handlers"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/pkg/api"
)

const (
	testDBName    = "game_master_test"
	testDBUser    = "game_master"
	testDBPass    = "game_master"
	migrationsDir = "../../migrations"
)

var (
	testPool   *pgxpool.Pool
	testUserID uuid.UUID
)

func TestMain(m *testing.M) {
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
		fmt.Fprintf(os.Stderr, "failed to start postgres container: %v\n", err)
		exitCode = 1
		return
	}
	defer func() {
		if err := container.Terminate(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "failed to terminate container: %v\n", err)
		}
	}()

	dsn, err := container.ConnectionString(setupCtx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get connection string: %v\n", err)
		exitCode = 1
		return
	}

	migrationsDB, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open database for migrations: %v\n", err)
		exitCode = 1
		return
	}
	defer migrationsDB.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, migrationsDB, os.DirFS(migrationsDir))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create goose provider: %v\n", err)
		exitCode = 1
		return
	}
	if _, err := provider.Up(setupCtx); err != nil {
		fmt.Fprintf(os.Stderr, "failed to run migrations: %v\n", err)
		exitCode = 1
		return
	}

	testPool, err = pgxpool.New(setupCtx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create connection pool: %v\n", err)
		exitCode = 1
		return
	}
	defer testPool.Close()

	// Create a shared test user. CreateUser auto-generates a UUID, so we
	// read it back and use it for the NoOpMiddleware across all tests.
	q := statedb.New(testPool)
	user, err := q.CreateUser(setupCtx, "integration-test-user")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create test user: %v\n", err)
		exitCode = 1
		return
	}
	testUserID = dbutil.FromPgtype(user.ID)

	exitCode = m.Run()
}

// ---------------------------------------------------------------------------
// Stub engine
// ---------------------------------------------------------------------------

// stubIntegrationEngine implements engine.GameEngine for action endpoint tests.
type stubIntegrationEngine struct {
	turnResult *engine.TurnResult
	turnErr    error
}

func (s *stubIntegrationEngine) ProcessTurn(_ context.Context, _ uuid.UUID, _ string) (*engine.TurnResult, error) {
	return s.turnResult, s.turnErr
}

func (s *stubIntegrationEngine) GetGameState(_ context.Context, _ uuid.UUID) (*engine.GameState, error) {
	return nil, nil
}

func (s *stubIntegrationEngine) NewCampaign(_ context.Context, _ uuid.UUID) (*domain.Campaign, error) {
	return nil, nil
}

func (s *stubIntegrationEngine) LoadCampaign(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (s *stubIntegrationEngine) ProcessTurnStream(ctx context.Context, campaignID uuid.UUID, input string) (<-chan engine.StreamEvent, error) {
	ch := make(chan engine.StreamEvent, 1)
	go func() {
		defer close(ch)
		result, err := s.ProcessTurn(ctx, campaignID, input)
		if err != nil {
			ch <- engine.StreamEvent{Type: "error", Err: err}
			return
		}
		ch <- engine.StreamEvent{Type: "result", Result: result}
	}()
	return ch, nil
}

// ---------------------------------------------------------------------------
// Router helper
// ---------------------------------------------------------------------------

func testRouter(t *testing.T, eng engine.GameEngine) (http.Handler, *statedb.Queries) {
	t.Helper()
	queries := statedb.New(testPool)
	logger := clog.New(os.Stderr)
	logger.SetLevel(clog.ErrorLevel)

	campaignH := &handlers.CampaignHandlers{Engine: eng, Queries: queries, Logger: logger}
	charH := &handlers.CharacterHandlers{Queries: queries, Logger: logger}
	worldH := &handlers.WorldHandlers{Queries: queries, Logger: logger}
	actionH := &handlers.ActionHandlers{Engine: eng, Queries: queries, Logger: logger}

	r := chi.NewRouter()
	authMW := auth.NewNoOpMiddleware(testUserID)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authMW.Authenticate)
		r.Route("/campaigns", func(r chi.Router) {
			r.Get("/", campaignH.ListCampaigns)
			r.Post("/", campaignH.CreateCampaign)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", campaignH.GetCampaign)
				r.Put("/", campaignH.UpdateCampaign)
				r.Delete("/", campaignH.DeleteCampaign)
				r.Get("/character", charH.GetCharacter)
				r.Get("/character/inventory", charH.GetCharacterInventory)
				r.Get("/character/abilities", charH.GetCharacterAbilities)
				r.Get("/locations", worldH.ListLocations)
				r.Get("/locations/{lid}", worldH.GetLocation)
				r.Get("/npcs", worldH.ListNPCs)
				r.Get("/npcs/{nid}", worldH.GetNPC)
				r.Get("/quests", worldH.ListQuests)
				r.Get("/quests/{qid}", worldH.GetQuest)
				r.Post("/action", actionH.ProcessAction)
			})
		})
	})
	return r, queries
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	return bytes.NewBuffer(b)
}

func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(rec.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v (body: %s)", err, rec.Body.String())
	}
	return v
}

// createCampaignViaAPI creates a campaign through the HTTP endpoint and returns
// the parsed response. This ensures campaigns are owned by testUserID.
func createCampaignViaAPI(t *testing.T, router http.Handler, name string) api.CampaignResponse {
	t.Helper()
	body := jsonBody(t, api.CampaignCreateRequest{
		Name:  name,
		Genre: "fantasy",
		Tone:  "dark",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/campaigns", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create campaign: want 201, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	return decodeJSON[api.CampaignResponse](t, rec)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestIntegrationCampaignCRUD(t *testing.T) {
	router, _ := testRouter(t, &stubIntegrationEngine{})

	// POST — create
	camp := createCampaignViaAPI(t, router, "CRUD Campaign")
	if camp.Name != "CRUD Campaign" {
		t.Errorf("create: name = %q, want %q", camp.Name, "CRUD Campaign")
	}
	if camp.Genre != "fantasy" {
		t.Errorf("create: genre = %q, want %q", camp.Genre, "fantasy")
	}
	if camp.Tone != "dark" {
		t.Errorf("create: tone = %q, want %q", camp.Tone, "dark")
	}
	if camp.Status != "active" {
		t.Errorf("create: status = %q, want %q", camp.Status, "active")
	}
	if camp.CreatedBy != testUserID.String() {
		t.Errorf("create: created_by = %q, want %q", camp.CreatedBy, testUserID.String())
	}

	// GET list — should contain the created campaign
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/campaigns", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d", rec.Code)
	}
	list := decodeJSON[api.CampaignListResponse](t, rec)
	found := false
	for _, c := range list.Campaigns {
		if c.ID == camp.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("list: created campaign %s not found in list of %d", camp.ID, len(list.Campaigns))
	}

	// GET single
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/campaigns/"+camp.ID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get: want 200, got %d", rec.Code)
	}
	got := decodeJSON[api.CampaignResponse](t, rec)
	if got.ID != camp.ID || got.Name != camp.Name {
		t.Errorf("get: mismatch: got ID=%s Name=%s", got.ID, got.Name)
	}

	// PUT — update name
	rec = httptest.NewRecorder()
	body := jsonBody(t, api.CampaignCreateRequest{Name: "Updated Campaign", Genre: "sci-fi"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/campaigns/"+camp.ID, body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: want 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	updated := decodeJSON[api.CampaignResponse](t, rec)
	if updated.Name != "Updated Campaign" {
		t.Errorf("update: name = %q, want %q", updated.Name, "Updated Campaign")
	}
	if updated.Genre != "sci-fi" {
		t.Errorf("update: genre = %q, want %q", updated.Genre, "sci-fi")
	}

	// DELETE
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/campaigns/"+camp.ID, nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: want 204, got %d", rec.Code)
	}

	// GET after delete → 404
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/campaigns/"+camp.ID, nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("get after delete: want 404, got %d", rec.Code)
	}
}

func TestIntegrationCharacterEndpoints(t *testing.T) {
	router, queries := testRouter(t, &stubIntegrationEngine{})
	ctx := context.Background()

	camp := createCampaignViaAPI(t, router, "Char Campaign")
	campUUID, _ := uuid.Parse(camp.ID)
	pgCampID := dbutil.ToPgtype(campUUID)
	pgUserID := dbutil.ToPgtype(testUserID)

	// Seed player character with abilities stored as JSON.
	abilities, _ := json.Marshal([]api.CharacterAbility{
		{Name: "Fireball", Description: "A ball of fire"},
		{Name: "Shield", Description: "A protective barrier"},
	})
	pc, err := queries.CreatePlayerCharacter(ctx, statedb.CreatePlayerCharacterParams{
		CampaignID:  pgCampID,
		UserID:      pgUserID,
		Name:        "Gandalf",
		Description: pgtype.Text{String: "A wizard", Valid: true},
		Hp:          100,
		MaxHp:       100,
		Experience:  0,
		Level:       1,
		Status:      "alive",
		Abilities:   abilities,
	})
	if err != nil {
		t.Fatalf("seed PC: %v", err)
	}

	// Seed items.
	for _, item := range []struct{ name, itype, rarity string }{
		{"Sword", "weapon", "common"},
		{"Potion", "consumable", "uncommon"},
	} {
		_, err := queries.CreateItem(ctx, statedb.CreateItemParams{
			CampaignID:        pgCampID,
			PlayerCharacterID: pc.ID,
			Name:              item.name,
			ItemType:          item.itype,
			Rarity:            item.rarity,
			Quantity:          1,
		})
		if err != nil {
			t.Fatalf("seed item %s: %v", item.name, err)
		}
	}

	// GET /character
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/campaigns/"+camp.ID+"/character", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get character: want 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	charResp := decodeJSON[api.CharacterResponse](t, rec)
	if charResp.Name != "Gandalf" {
		t.Errorf("character name = %q, want %q", charResp.Name, "Gandalf")
	}
	if charResp.HP != 100 || charResp.MaxHP != 100 {
		t.Errorf("character HP=%d MaxHP=%d, want 100/100", charResp.HP, charResp.MaxHP)
	}

	// GET /character/inventory
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/campaigns/"+camp.ID+"/character/inventory", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get inventory: want 200, got %d", rec.Code)
	}
	items := decodeJSON[[]api.ItemResponse](t, rec)
	if len(items) != 2 {
		t.Errorf("inventory count = %d, want 2", len(items))
	}

	// GET /character/abilities
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/campaigns/"+camp.ID+"/character/abilities", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get abilities: want 200, got %d", rec.Code)
	}
	abils := decodeJSON[[]api.CharacterAbility](t, rec)
	if len(abils) != 2 {
		t.Errorf("abilities count = %d, want 2", len(abils))
	}
	if abils[0].Name != "Fireball" {
		t.Errorf("first ability = %q, want %q", abils[0].Name, "Fireball")
	}
}

func TestIntegrationLocationEndpoints(t *testing.T) {
	router, queries := testRouter(t, &stubIntegrationEngine{})
	ctx := context.Background()

	camp := createCampaignViaAPI(t, router, "Location Campaign")
	campUUID, _ := uuid.Parse(camp.ID)
	pgCampID := dbutil.ToPgtype(campUUID)

	loc1, err := queries.CreateLocation(ctx, statedb.CreateLocationParams{
		CampaignID:   pgCampID,
		Name:         "Tavern",
		Description:  pgtype.Text{String: "A cozy tavern", Valid: true},
		Region:       pgtype.Text{String: "Village", Valid: true},
		LocationType: pgtype.Text{String: "building", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed location 1: %v", err)
	}
	loc2, err := queries.CreateLocation(ctx, statedb.CreateLocationParams{
		CampaignID:   pgCampID,
		Name:         "Forest",
		Description:  pgtype.Text{String: "A dark forest", Valid: true},
		Region:       pgtype.Text{String: "Wilderness", Valid: true},
		LocationType: pgtype.Text{String: "outdoor", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed location 2: %v", err)
	}

	// Connection: tavern → forest.
	_, err = queries.CreateConnection(ctx, statedb.CreateConnectionParams{
		CampaignID:     pgCampID,
		FromLocationID: loc1.ID,
		ToLocationID:   loc2.ID,
		Description:    pgtype.Text{String: "A dirt path", Valid: true},
		Bidirectional:  true,
		TravelTime:     pgtype.Text{String: "30 minutes", Valid: true},
	})
	if err != nil {
		t.Fatalf("seed connection: %v", err)
	}

	// GET /locations
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/campaigns/"+camp.ID+"/locations", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list locations: want 200, got %d", rec.Code)
	}
	locs := decodeJSON[[]api.LocationResponse](t, rec)
	if len(locs) != 2 {
		t.Errorf("location count = %d, want 2", len(locs))
	}

	// GET /locations/{lid} — tavern with connection
	loc1UUID := dbutil.FromPgtype(loc1.ID)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/campaigns/"+camp.ID+"/locations/"+loc1UUID.String(), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get location: want 200, got %d", rec.Code)
	}
	locResp := decodeJSON[api.LocationResponse](t, rec)
	if locResp.Name != "Tavern" {
		t.Errorf("location name = %q, want %q", locResp.Name, "Tavern")
	}
	if len(locResp.Connections) != 1 {
		t.Errorf("connections count = %d, want 1", len(locResp.Connections))
	} else {
		loc2UUID := dbutil.FromPgtype(loc2.ID)
		if locResp.Connections[0].ToLocationID != loc2UUID.String() {
			t.Errorf("connection target = %q, want %q", locResp.Connections[0].ToLocationID, loc2UUID.String())
		}
	}
}

func TestIntegrationNPCEndpoints(t *testing.T) {
	router, queries := testRouter(t, &stubIntegrationEngine{})
	ctx := context.Background()

	camp := createCampaignViaAPI(t, router, "NPC Campaign")
	campUUID, _ := uuid.Parse(camp.ID)
	pgCampID := dbutil.ToPgtype(campUUID)

	npc1, err := queries.CreateNPC(ctx, statedb.CreateNPCParams{
		CampaignID:  pgCampID,
		Name:        "Barkeep",
		Description: pgtype.Text{String: "A friendly barkeep", Valid: true},
		Personality: pgtype.Text{String: "jovial", Valid: true},
		Disposition: 50,
		Alive:       true,
		Hp:          pgtype.Int4{Int32: 30, Valid: true},
	})
	if err != nil {
		t.Fatalf("seed NPC 1: %v", err)
	}
	_, err = queries.CreateNPC(ctx, statedb.CreateNPCParams{
		CampaignID:  pgCampID,
		Name:        "Goblin",
		Description: pgtype.Text{String: "A sneaky goblin", Valid: true},
		Disposition: -20,
		Alive:       true,
		Hp:          pgtype.Int4{Int32: 10, Valid: true},
	})
	if err != nil {
		t.Fatalf("seed NPC 2: %v", err)
	}

	// GET /npcs
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/campaigns/"+camp.ID+"/npcs", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list npcs: want 200, got %d", rec.Code)
	}
	npcs := decodeJSON[[]api.NPCResponse](t, rec)
	if len(npcs) != 2 {
		t.Errorf("npc count = %d, want 2", len(npcs))
	}

	// GET /npcs/{nid}
	npc1UUID := dbutil.FromPgtype(npc1.ID)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/campaigns/"+camp.ID+"/npcs/"+npc1UUID.String(), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get npc: want 200, got %d", rec.Code)
	}
	npcResp := decodeJSON[api.NPCResponse](t, rec)
	if npcResp.Name != "Barkeep" {
		t.Errorf("npc name = %q, want %q", npcResp.Name, "Barkeep")
	}
	if npcResp.Disposition != 50 {
		t.Errorf("npc disposition = %d, want 50", npcResp.Disposition)
	}
	if !npcResp.Alive {
		t.Errorf("npc alive = false, want true")
	}
}

func TestIntegrationQuestEndpoints(t *testing.T) {
	router, queries := testRouter(t, &stubIntegrationEngine{})
	ctx := context.Background()

	camp := createCampaignViaAPI(t, router, "Quest Campaign")
	campUUID, _ := uuid.Parse(camp.ID)
	pgCampID := dbutil.ToPgtype(campUUID)

	quest, err := queries.CreateQuest(ctx, statedb.CreateQuestParams{
		CampaignID:  pgCampID,
		Title:       "Slay the Dragon",
		Description: pgtype.Text{String: "Defeat the ancient dragon", Valid: true},
		QuestType:   "short_term",
		Status:      "active",
	})
	if err != nil {
		t.Fatalf("seed quest: %v", err)
	}

	// Create objectives.
	_, err = queries.CreateObjective(ctx, statedb.CreateObjectiveParams{
		QuestID:     quest.ID,
		Description: "Find the dragon's lair",
		OrderIndex:  1,
	})
	if err != nil {
		t.Fatalf("seed objective 1: %v", err)
	}
	_, err = queries.CreateObjective(ctx, statedb.CreateObjectiveParams{
		QuestID:     quest.ID,
		Description: "Defeat the dragon",
		OrderIndex:  2,
	})
	if err != nil {
		t.Fatalf("seed objective 2: %v", err)
	}

	// Also create a side quest for type filtering.
	_, err = queries.CreateQuest(ctx, statedb.CreateQuestParams{
		CampaignID:  pgCampID,
		Title:       "Fetch Herbs",
		Description: pgtype.Text{String: "Gather herbs for the healer", Valid: true},
		QuestType:   "long_term",
		Status:      "active",
	})
	if err != nil {
		t.Fatalf("seed side quest: %v", err)
	}

	// GET /quests — all
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/campaigns/"+camp.ID+"/quests", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list quests: want 200, got %d", rec.Code)
	}
	quests := decodeJSON[[]api.QuestResponse](t, rec)
	if len(quests) != 2 {
		t.Errorf("quest count = %d, want 2", len(quests))
	}

	// GET /quests?type=short_term — filtered
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/campaigns/"+camp.ID+"/quests?type=short_term", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list quests by type: want 200, got %d", rec.Code)
	}
	mainQuests := decodeJSON[[]api.QuestResponse](t, rec)
	if len(mainQuests) != 1 {
		t.Errorf("main quest count = %d, want 1", len(mainQuests))
	}
	if len(mainQuests) > 0 && mainQuests[0].Title != "Slay the Dragon" {
		t.Errorf("main quest title = %q, want %q", mainQuests[0].Title, "Slay the Dragon")
	}

	// GET /quests/{qid} — with objectives
	questUUID := dbutil.FromPgtype(quest.ID)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/campaigns/"+camp.ID+"/quests/"+questUUID.String(), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get quest: want 200, got %d", rec.Code)
	}
	questResp := decodeJSON[api.QuestResponse](t, rec)
	if questResp.Title != "Slay the Dragon" {
		t.Errorf("quest title = %q, want %q", questResp.Title, "Slay the Dragon")
	}
	if len(questResp.Objectives) != 2 {
		t.Errorf("objectives count = %d, want 2", len(questResp.Objectives))
	}
	if len(questResp.Objectives) > 0 && questResp.Objectives[0].Description != "Find the dragon's lair" {
		t.Errorf("first objective = %q, want %q", questResp.Objectives[0].Description, "Find the dragon's lair")
	}
}

func TestIntegrationProcessAction(t *testing.T) {
	eng := &stubIntegrationEngine{
		turnResult: &engine.TurnResult{
			Narrative: "You swing your sword at the goblin!",
		},
	}
	router, _ := testRouter(t, eng)

	// Need a campaign for the URL (action handler validates campaign UUID).
	camp := createCampaignViaAPI(t, router, "Action Campaign")

	body := jsonBody(t, api.ActionRequest{Input: "attack the goblin"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/campaigns/"+camp.ID+"/action", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("process action: want 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	result := decodeJSON[api.TurnResult](t, rec)
	if result.Narrative != "You swing your sword at the goblin!" {
		t.Errorf("narrative = %q, want %q", result.Narrative, "You swing your sword at the goblin!")
	}
}

func TestIntegrationErrorCases(t *testing.T) {
	router, _ := testRouter(t, &stubIntegrationEngine{})

	// Create a campaign for sub-resource error tests.
	camp := createCampaignViaAPI(t, router, "Error Campaign")

	tests := []struct {
		name       string
		method     string
		url        string
		body       any
		wantStatus int
	}{
		{
			name:       "invalid UUID in campaign path",
			method:     http.MethodGet,
			url:        "/api/v1/campaigns/not-a-uuid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid JSON body on create",
			method:     http.MethodPost,
			url:        "/api/v1/campaigns",
			body:       "not json{{{",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "nonexistent campaign returns 404",
			method:     http.MethodGet,
			url:        "/api/v1/campaigns/" + uuid.New().String(),
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "action with empty body",
			method:     http.MethodPost,
			url:        "/api/v1/campaigns/" + camp.ID + "/action",
			body:       "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "action with empty input",
			method:     http.MethodPost,
			url:        "/api/v1/campaigns/" + camp.ID + "/action",
			body:       api.ActionRequest{Input: ""},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reqBody *bytes.Buffer
			switch v := tt.body.(type) {
			case string:
				reqBody = bytes.NewBufferString(v)
			case nil:
				reqBody = &bytes.Buffer{}
			default:
				reqBody = jsonBody(t, v)
			}
			req := httptest.NewRequest(tt.method, tt.url, reqBody)
			if tt.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("want status %d, got %d (body: %s)", tt.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}
