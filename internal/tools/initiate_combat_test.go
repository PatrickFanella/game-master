package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
)

var _ InitiateCombatStore = (*stubInitiateCombatStore)(nil)

type stubInitiateCombatStore struct {
	player                *domain.PlayerCharacter
	npcsByName            map[string]*domain.NPC
	createErr             error
	listErr               error
	getPlayerErr          error
	statusErr             error
	logErr                error
	createdNPCs           []InitiateCombatNPCParams
	updatedStatusPlayerID uuid.UUID
	updatedStatus         string
	loggedEntry           *InitiateCombatLogEntry
}

func (s *stubInitiateCombatStore) GetPlayerCharacterByID(_ context.Context, _ uuid.UUID) (*domain.PlayerCharacter, error) {
	if s.getPlayerErr != nil {
		return nil, s.getPlayerErr
	}
	return s.player, nil
}

func (s *stubInitiateCombatStore) ListNPCsByCampaign(_ context.Context, _ uuid.UUID) ([]domain.NPC, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	if len(s.npcsByName) == 0 {
		return nil, nil
	}
	out := make([]domain.NPC, 0, len(s.npcsByName))
	for _, npc := range s.npcsByName {
		out = append(out, *npc)
	}
	return out, nil
}

func (s *stubInitiateCombatStore) CreateNPC(_ context.Context, params InitiateCombatNPCParams) (*domain.NPC, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	s.createdNPCs = append(s.createdNPCs, params)
	npc := &domain.NPC{ID: uuid.New(), CampaignID: params.CampaignID, Name: params.Name}
	if s.npcsByName == nil {
		s.npcsByName = map[string]*domain.NPC{}
	}
	s.npcsByName[params.Name] = npc
	return npc, nil
}

func (s *stubInitiateCombatStore) UpdatePlayerStatus(_ context.Context, playerCharacterID uuid.UUID, status string) error {
	if s.statusErr != nil {
		return s.statusErr
	}
	s.updatedStatusPlayerID = playerCharacterID
	s.updatedStatus = status
	return nil
}

func (s *stubInitiateCombatStore) LogCombatStart(_ context.Context, entry InitiateCombatLogEntry) error {
	if s.logErr != nil {
		return s.logErr
	}
	e := entry
	s.loggedEntry = &e
	return nil
}

func TestRegisterInitiateCombat(t *testing.T) {
	reg := NewRegistry()
	store := &stubInitiateCombatStore{}
	if err := RegisterInitiateCombat(reg, store); err != nil {
		t.Fatalf("register initiate_combat: %v", err)
	}

	registered := reg.List()
	if len(registered) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(registered))
	}
	if registered[0].Name != initiateCombatToolName {
		t.Fatalf("tool name = %q, want %q", registered[0].Name, initiateCombatToolName)
	}
	required, ok := registered[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", registered[0].Parameters["required"])
	}
	if len(required) != 2 || required[0] != "enemies" || required[1] != "environment" {
		t.Fatalf("required schema = %#v, want [enemies environment]", required)
	}
}

func TestInitiateCombatHandleSingleEnemyCreatesNPCAndEntersCombatMode(t *testing.T) {
	playerID := uuid.New()
	campaignID := uuid.New()
	locationID := uuid.New()
	store := &stubInitiateCombatStore{
		player: &domain.PlayerCharacter{
			ID:                playerID,
			CampaignID:        campaignID,
			Name:              "Aria",
			HP:                24,
			MaxHP:             30,
			Stats:             []byte(`{"dexterity":14}`),
			CurrentLocationID: &locationID,
		},
	}
	h := NewInitiateCombatHandler(store)
	ctx := WithCurrentPlayerCharacterID(WithCurrentLocationID(context.Background(), locationID), playerID)

	result, err := h.Handle(ctx, map[string]any{
		"enemies": []any{
			map[string]any{
				"name":        "Goblin Scout",
				"description": "A wiry goblin with a chipped spear.",
				"hp":          9,
				"stats":       map[string]any{"dexterity": 12},
				"abilities":   []any{"Nimble Escape"},
			},
		},
		"environment": "Narrow forest trail with dense undergrowth",
		"surprise":    "players",
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if !result.Success {
		t.Fatal("expected success=true")
	}
	if store.updatedStatusPlayerID != playerID || store.updatedStatus != combatPlayerStatus {
		t.Fatalf("player status update = (%s, %q), want (%s, %q)", store.updatedStatusPlayerID, store.updatedStatus, playerID, combatPlayerStatus)
	}
	if len(store.createdNPCs) != 1 {
		t.Fatalf("created NPC count = %d, want 1", len(store.createdNPCs))
	}
	if store.createdNPCs[0].Name != "Goblin Scout" {
		t.Fatalf("created NPC name = %q, want Goblin Scout", store.createdNPCs[0].Name)
	}
	if store.loggedEntry == nil {
		t.Fatal("expected combat start log entry")
	}
	if store.loggedEntry.EnvironmentDescription != "Narrow forest trail with dense undergrowth" {
		t.Fatalf("logged environment = %q", store.loggedEntry.EnvironmentDescription)
	}
	if gotMode, _ := result.Data["mode"].(string); gotMode != "combat" {
		t.Fatalf("result mode = %q, want combat", gotMode)
	}
	if !strings.Contains(result.Narrative, "Combat begins!") {
		t.Fatalf("narrative = %q, want combat opening", result.Narrative)
	}

	order, ok := result.Data["initiative_order"].([]map[string]any)
	if !ok {
		t.Fatalf("initiative_order type = %T, want []map[string]any", result.Data["initiative_order"])
	}
	if len(order) != 2 {
		t.Fatalf("initiative order len = %d, want 2", len(order))
	}
}

func TestInitiateCombatHandleMultipleEnemiesReusesExistingNPCs(t *testing.T) {
	playerID := uuid.New()
	campaignID := uuid.New()
	existingNPC := &domain.NPC{ID: uuid.New(), CampaignID: campaignID, Name: "Orc Brute"}
	store := &stubInitiateCombatStore{
		player: &domain.PlayerCharacter{
			ID:         playerID,
			CampaignID: campaignID,
			Name:       "Aria",
			HP:         24,
			MaxHP:      30,
			Stats:      []byte(`{"dexterity":14}`),
		},
		npcsByName: map[string]*domain.NPC{
			"Orc Brute": existingNPC,
		},
	}
	h := NewInitiateCombatHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	result, err := h.Handle(ctx, map[string]any{
		"enemies": []any{
			map[string]any{"name": "Orc Brute", "description": "Heavy infantry.", "hp": 16, "stats": map[string]any{"dexterity": 9}, "abilities": []any{"Cleave"}},
			map[string]any{"name": "Bandit Archer", "description": "Fast ranged attacker.", "hp": 11, "stats": map[string]any{"dexterity": 15}, "abilities": []any{"Volley"}},
			map[string]any{"name": "Bandit Captain", "description": "Commands the raiders.", "hp": 18, "stats": map[string]any{"dexterity": 13}, "abilities": []any{"Commanding Shout"}},
		},
		"environment": "Ruined watchtower courtyard",
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if len(store.createdNPCs) != 2 {
		t.Fatalf("created NPC count = %d, want 2", len(store.createdNPCs))
	}
	if store.loggedEntry == nil {
		t.Fatal("expected combat start log entry")
	}
	if len(store.loggedEntry.EnemyNPCIDs) != 3 {
		t.Fatalf("logged enemy IDs len = %d, want 3", len(store.loggedEntry.EnemyNPCIDs))
	}
	order, ok := result.Data["initiative_order"].([]map[string]any)
	if !ok {
		t.Fatalf("initiative_order type = %T", result.Data["initiative_order"])
	}
	if len(order) != 4 {
		t.Fatalf("initiative order len = %d, want 4", len(order))
	}
}

func TestInitiateCombatHandleValidationAndStoreErrors(t *testing.T) {
	playerID := uuid.New()
	store := &stubInitiateCombatStore{
		player: &domain.PlayerCharacter{ID: playerID, CampaignID: uuid.New(), Name: "Aria", HP: 24, MaxHP: 30, Stats: []byte(`{"dexterity":14}`)},
	}
	h := NewInitiateCombatHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	_, err := h.Handle(ctx, map[string]any{
		"enemies":     []any{},
		"environment": "Marsh",
	})
	if err == nil || !strings.Contains(err.Error(), "must contain at least one enemy") {
		t.Fatalf("expected empty enemies validation error, got %v", err)
	}

	_, err = h.Handle(ctx, map[string]any{
		"enemies":     []any{map[string]any{"name": "Ghoul", "description": "Undead", "hp": 7, "stats": map[string]any{"dexterity": 10}, "abilities": []any{"Claw"}}},
		"environment": "Crypt",
		"surprise":    "invalid-side",
	})
	if err == nil || !strings.Contains(err.Error(), "must be one of: players, enemies") {
		t.Fatalf("expected surprise validation error, got %v", err)
	}

	store.listErr = errors.New("query failed")
	_, err = h.Handle(ctx, map[string]any{
		"enemies":     []any{map[string]any{"name": "Ghoul", "description": "Undead", "hp": 7, "stats": map[string]any{"dexterity": 10}, "abilities": []any{"Claw"}}},
		"environment": "Crypt",
	})
	if err == nil || !strings.Contains(err.Error(), "list campaign npcs: query failed") {
		t.Fatalf("expected campaign npc list error, got %v", err)
	}

	store.listErr = nil
	store.statusErr = errors.New("write failed")
	_, err = h.Handle(ctx, map[string]any{
		"enemies":     []any{map[string]any{"name": "Ghoul", "description": "Undead", "hp": 7, "stats": map[string]any{"dexterity": 10}, "abilities": []any{"Claw"}}},
		"environment": "Crypt",
	})
	if err == nil || !strings.Contains(err.Error(), "set combat mode: write failed") {
		t.Fatalf("expected status update error, got %v", err)
	}
}
