package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
)

// compile-time interface check
var _ ResolveCombatStore = (*stubResolveCombatStore)(nil)

type stubResolveCombatStore struct {
	player              *domain.PlayerCharacter
	npcs                map[uuid.UUID]*domain.NPC
	getPlayerErr        error
	updateHPErr         error
	updateStatusErr     error
	updateLocationErr   error
	addXPErr            error
	createItemErr       error
	markDeadErr         error
	getNPCErr           error
	updateDisposErr     error
	updatedHP           int
	updatedMaxHP        int
	updatedStatus       string
	updatedLocationID   uuid.UUID
	addedXP             int
	createdItems        []createdItemArgs
	killedNPCIDs        []uuid.UUID
	updatedDispositions map[uuid.UUID]int
}

type createdItemArgs struct {
	name        string
	description string
	itemType    string
	rarity      string
	quantity    int
}

func (s *stubResolveCombatStore) GetPlayerCharacterByID(_ context.Context, _ uuid.UUID) (*domain.PlayerCharacter, error) {
	return s.player, s.getPlayerErr
}

func (s *stubResolveCombatStore) UpdatePlayerHP(_ context.Context, _ uuid.UUID, hp, maxHP int) error {
	s.updatedHP = hp
	s.updatedMaxHP = maxHP
	return s.updateHPErr
}

func (s *stubResolveCombatStore) UpdatePlayerStatus(_ context.Context, _ uuid.UUID, status string) error {
	s.updatedStatus = status
	return s.updateStatusErr
}

func (s *stubResolveCombatStore) UpdatePlayerLocation(_ context.Context, _ uuid.UUID, locationID uuid.UUID) error {
	s.updatedLocationID = locationID
	return s.updateLocationErr
}

func (s *stubResolveCombatStore) AddPlayerExperience(_ context.Context, _ uuid.UUID, xpAmount int) error {
	s.addedXP += xpAmount
	return s.addXPErr
}

func (s *stubResolveCombatStore) CreatePlayerItem(_ context.Context, _ uuid.UUID, name, description, itemType, rarity string, quantity int) (uuid.UUID, error) {
	if s.createItemErr != nil {
		return uuid.Nil, s.createItemErr
	}
	s.createdItems = append(s.createdItems, createdItemArgs{name, description, itemType, rarity, quantity})
	return uuid.New(), nil
}

func (s *stubResolveCombatStore) MarkNPCDead(_ context.Context, npcID uuid.UUID) error {
	if s.markDeadErr != nil {
		return s.markDeadErr
	}
	s.killedNPCIDs = append(s.killedNPCIDs, npcID)
	return nil
}

func (s *stubResolveCombatStore) GetNPCByID(_ context.Context, npcID uuid.UUID) (*domain.NPC, error) {
	if s.getNPCErr != nil {
		return nil, s.getNPCErr
	}
	if npc, ok := s.npcs[npcID]; ok {
		return npc, nil
	}
	return nil, nil
}

func (s *stubResolveCombatStore) UpdateNPCDisposition(_ context.Context, npcID uuid.UUID, newDisposition int) error {
	if s.updateDisposErr != nil {
		return s.updateDisposErr
	}
	if s.updatedDispositions == nil {
		s.updatedDispositions = make(map[uuid.UUID]int)
	}
	s.updatedDispositions[npcID] = newDisposition
	return nil
}

// baseCombatStateArgsForResolve builds a combat state with the player still alive
// and the enemy dead, suitable for resolve_combat tests.
func baseCombatStateArgsForResolve(playerID, enemyID uuid.UUID) map[string]any {
	return map[string]any{
		"id":               uuid.New().String(),
		"campaign_id":      uuid.New().String(),
		"round_number":     1,
		"status":           "active",
		"narrative":        "",
		"initiative_order": []any{playerID.String(), enemyID.String()},
		"environment": map[string]any{
			"description": "forest",
		},
		"combatants": []any{
			map[string]any{
				"entity_id":   playerID.String(),
				"entity_type": "player",
				"name":        "Aria",
				"hp":          18,
				"max_hp":      30,
				"stats":       map[string]any{"dexterity": 14},
				"conditions":  []any{},
				"initiative":  12,
				"status":      "alive",
				"surprised":   false,
			},
			map[string]any{
				"entity_id":   enemyID.String(),
				"entity_type": "npc",
				"name":        "Goblin",
				"hp":          0,
				"max_hp":      9,
				"stats":       map[string]any{"dexterity": 10},
				"conditions":  []any{},
				"initiative":  8,
				"status":      "dead",
				"surprised":   false,
			},
		},
	}
}

func defaultPlayer(playerID uuid.UUID) *domain.PlayerCharacter {
	return &domain.PlayerCharacter{
		ID:         playerID,
		CampaignID: uuid.New(),
		Name:       "Aria",
		HP:         18,
		MaxHP:      30,
		Experience: 500,
		Level:      1,
		Stats:      []byte(`{"dexterity":14}`),
	}
}

// --- Registration test ---

func TestRegisterResolveCombat(t *testing.T) {
	reg := NewRegistry()
	store := &stubResolveCombatStore{}
	if err := RegisterResolveCombat(reg, store); err != nil {
		t.Fatalf("RegisterResolveCombat: %v", err)
	}

	registered := reg.List()
	if len(registered) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(registered))
	}
	if registered[0].Name != resolveCombatToolName {
		t.Fatalf("tool name = %q, want %q", registered[0].Name, resolveCombatToolName)
	}
	required, ok := registered[0].Parameters["required"].([]string)
	if !ok {
		t.Fatalf("required schema has unexpected type %T", registered[0].Parameters["required"])
	}
	if len(required) != 2 || required[0] != "combat_state" || required[1] != "outcome_type" {
		t.Fatalf("required schema = %#v, want [combat_state outcome_type]", required)
	}
}

func TestRegisterResolveCombatNilStoreReturnsError(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterResolveCombat(reg, nil); err == nil {
		t.Fatal("expected error registering with nil store")
	}
}

// --- Victory outcome ---

func TestResolveCombatVictoryDistributesXPCreatesLootAndMarksNPCsDead(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()
	store := &stubResolveCombatStore{player: defaultPlayer(playerID)}
	h := NewResolveCombatHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	result, err := h.Handle(ctx, map[string]any{
		"combat_state": baseCombatStateArgsForResolve(playerID, enemyID),
		"outcome_type": "victory",
		"xp_earned":    150,
		"dead_npc_ids": []any{enemyID.String()},
		"loot": []any{
			map[string]any{"name": "Rusty Dagger", "description": "A chipped blade.", "item_type": "weapon"},
		},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if !result.Success {
		t.Fatal("expected success=true")
	}
	// XP awarded.
	if store.addedXP != 150 {
		t.Fatalf("added XP = %d, want 150", store.addedXP)
	}
	// NPC marked dead.
	if len(store.killedNPCIDs) != 1 || store.killedNPCIDs[0] != enemyID {
		t.Fatalf("killed NPCs = %v, want [%s]", store.killedNPCIDs, enemyID)
	}
	// Item created.
	if len(store.createdItems) != 1 || store.createdItems[0].name != "Rusty Dagger" {
		t.Fatalf("created items = %v", store.createdItems)
	}
	// Player HP persisted.
	if store.updatedHP != 18 || store.updatedMaxHP != 30 {
		t.Fatalf("persisted HP = %d/%d, want 18/30", store.updatedHP, store.updatedMaxHP)
	}
	// Player exited combat.
	if store.updatedStatus != playerStatusActive {
		t.Fatalf("player status = %q, want %q", store.updatedStatus, playerStatusActive)
	}
	// Result data.
	if xp, _ := result.Data["xp_earned"].(int); xp != 150 {
		t.Fatalf("data xp_earned = %v, want 150", result.Data["xp_earned"])
	}
	loot, _ := result.Data["loot"].([]map[string]any)
	if len(loot) != 1 {
		t.Fatalf("loot count = %d, want 1", len(loot))
	}
	if !strings.Contains(result.Narrative, "Victory") {
		t.Fatalf("narrative = %q, want Victory", result.Narrative)
	}
	// Combat state marked completed.
	combatState, _ := result.Data["combat_state"].(map[string]any)
	if status, _ := combatState["status"].(string); status != "completed" {
		t.Fatalf("combat_state.status = %q, want completed", status)
	}
}

func TestResolveCombatVictoryWithNoXPOrLoot(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()
	store := &stubResolveCombatStore{player: defaultPlayer(playerID)}
	h := NewResolveCombatHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	result, err := h.Handle(ctx, map[string]any{
		"combat_state": baseCombatStateArgsForResolve(playerID, enemyID),
		"outcome_type": "victory",
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success=true")
	}
	if store.addedXP != 0 {
		t.Fatalf("expected no XP awarded, got %d", store.addedXP)
	}
	if len(store.createdItems) != 0 {
		t.Fatalf("expected no loot created, got %d items", len(store.createdItems))
	}
}

// --- Defeat outcome ---

func TestResolveCombatDefeatSetsDefeatedStatus(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()
	store := &stubResolveCombatStore{player: defaultPlayer(playerID)}
	h := NewResolveCombatHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	result, err := h.Handle(ctx, map[string]any{
		"combat_state": baseCombatStateArgsForResolve(playerID, enemyID),
		"outcome_type": "defeat",
		"consequences": "The player is captured by the bandits.",
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success=true")
	}
	if store.updatedStatus != playerStatusDefeated {
		t.Fatalf("player status = %q, want %q", store.updatedStatus, playerStatusDefeated)
	}
	if status, _ := result.Data["player_status"].(string); status != playerStatusDefeated {
		t.Fatalf("data player_status = %q, want %q", status, playerStatusDefeated)
	}
	if cons, _ := result.Data["consequences"].(string); cons != "The player is captured by the bandits." {
		t.Fatalf("data consequences = %q", cons)
	}
	if !strings.Contains(result.Narrative, "defeated") {
		t.Fatalf("narrative = %q, want defeated", result.Narrative)
	}
	// Combat state marked completed on defeat.
	combatState, _ := result.Data["combat_state"].(map[string]any)
	if status, _ := combatState["status"].(string); status != "completed" {
		t.Fatalf("combat_state.status = %q, want completed", status)
	}
}

// --- Flee outcome ---

func TestResolveCombatFleeMovesPlayerAndExitsCombat(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()
	fleeLocationID := uuid.New()
	store := &stubResolveCombatStore{player: defaultPlayer(playerID)}
	h := NewResolveCombatHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	result, err := h.Handle(ctx, map[string]any{
		"combat_state":     baseCombatStateArgsForResolve(playerID, enemyID),
		"outcome_type":     "flee",
		"flee_location_id": fleeLocationID.String(),
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success=true")
	}
	if store.updatedLocationID != fleeLocationID {
		t.Fatalf("updated location = %s, want %s", store.updatedLocationID, fleeLocationID)
	}
	if store.updatedStatus != playerStatusActive {
		t.Fatalf("player status = %q, want %q", store.updatedStatus, playerStatusActive)
	}
	if loc, _ := result.Data["flee_location_id"].(string); loc != fleeLocationID.String() {
		t.Fatalf("data flee_location_id = %q, want %q", loc, fleeLocationID.String())
	}
	// Combat state marked fled.
	combatState, _ := result.Data["combat_state"].(map[string]any)
	if status, _ := combatState["status"].(string); status != "fled" {
		t.Fatalf("combat_state.status = %q, want fled", status)
	}
	if !strings.Contains(result.Narrative, "fled") {
		t.Fatalf("narrative = %q, want fled", result.Narrative)
	}
}

func TestResolveCombatFleeWithoutLocationExitsCombat(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()
	store := &stubResolveCombatStore{player: defaultPlayer(playerID)}
	h := NewResolveCombatHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	result, err := h.Handle(ctx, map[string]any{
		"combat_state": baseCombatStateArgsForResolve(playerID, enemyID),
		"outcome_type": "flee",
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success=true")
	}
	if store.updatedLocationID != uuid.Nil {
		t.Fatalf("expected no location update, got %s", store.updatedLocationID)
	}
	if store.updatedStatus != playerStatusActive {
		t.Fatalf("player status = %q, want %q", store.updatedStatus, playerStatusActive)
	}
}

// --- Surrender outcome ---

func TestResolveCombatSurrenderUpdatesNPCDispositions(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()
	// Use enemyID as the surrender NPC – it must be an NPC combatant in the combat state.
	npc := &domain.NPC{ID: enemyID, CampaignID: uuid.New(), Name: "Bandit Chief", Disposition: -20, Alive: true}
	store := &stubResolveCombatStore{
		player: defaultPlayer(playerID),
		npcs:   map[uuid.UUID]*domain.NPC{enemyID: npc},
	}
	h := NewResolveCombatHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	result, err := h.Handle(ctx, map[string]any{
		"combat_state":       baseCombatStateArgsForResolve(playerID, enemyID),
		"outcome_type":       "surrender",
		"surrender_npc_ids":  []any{enemyID.String()},
		"disposition_change": 30,
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success=true")
	}
	// Disposition updated: -20 + 30 = 10.
	if store.updatedDispositions[enemyID] != 10 {
		t.Fatalf("updated disposition = %d, want 10", store.updatedDispositions[enemyID])
	}
	if store.updatedStatus != playerStatusActive {
		t.Fatalf("player status = %q, want %q", store.updatedStatus, playerStatusActive)
	}
	updatedNPCs, _ := result.Data["updated_npcs"].([]map[string]any)
	if len(updatedNPCs) != 1 {
		t.Fatalf("updated_npcs count = %d, want 1", len(updatedNPCs))
	}
	if nd, _ := updatedNPCs[0]["new_disposition"].(int); nd != 10 {
		t.Fatalf("new_disposition = %d, want 10", nd)
	}
	if result.Narrative != "The combat has ended in surrender. Terms have been negotiated." {
		t.Fatalf("narrative = %q, want surrender narrative", result.Narrative)
	}
}

func TestResolveCombatSurrenderClampsDispositionAt100(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()
	// Use enemyID as the surrender NPC – it must be an NPC combatant in the combat state.
	npc := &domain.NPC{ID: enemyID, CampaignID: uuid.New(), Name: "Villager", Disposition: 80, Alive: true}
	store := &stubResolveCombatStore{
		player: defaultPlayer(playerID),
		npcs:   map[uuid.UUID]*domain.NPC{enemyID: npc},
	}
	h := NewResolveCombatHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	_, err := h.Handle(ctx, map[string]any{
		"combat_state":       baseCombatStateArgsForResolve(playerID, enemyID),
		"outcome_type":       "surrender",
		"surrender_npc_ids":  []any{enemyID.String()},
		"disposition_change": 50,
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	// 80 + 50 = 130, clamped to 100.
	if store.updatedDispositions[enemyID] != 100 {
		t.Fatalf("disposition = %d, want 100", store.updatedDispositions[enemyID])
	}
}

// --- Validation tests ---

func TestResolveCombatInvalidOutcomeTypeReturnsError(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()
	store := &stubResolveCombatStore{player: defaultPlayer(playerID)}
	h := NewResolveCombatHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	_, err := h.Handle(ctx, map[string]any{
		"combat_state": baseCombatStateArgsForResolve(playerID, enemyID),
		"outcome_type": "annihilate",
	})
	if err == nil || !strings.Contains(err.Error(), "outcome_type must be one of") {
		t.Fatalf("expected outcome_type error, got %v", err)
	}
}

func TestResolveCombatMissingPlayerCharacterIDInContextReturnsError(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()
	store := &stubResolveCombatStore{player: defaultPlayer(playerID)}
	h := NewResolveCombatHandler(store)

	_, err := h.Handle(context.Background(), map[string]any{
		"combat_state": baseCombatStateArgsForResolve(playerID, enemyID),
		"outcome_type": "victory",
	})
	if err == nil || !strings.Contains(err.Error(), "requires current player character id") {
		t.Fatalf("expected context error, got %v", err)
	}
}

func TestResolveCombatNilHandlerReturnsError(t *testing.T) {
	var h *ResolveCombatHandler
	_, err := h.Handle(context.Background(), map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "handler is nil") {
		t.Fatalf("expected nil handler error, got %v", err)
	}
}

func TestResolveCombatPlayerNotFoundReturnsError(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()
	store := &stubResolveCombatStore{player: nil}
	h := NewResolveCombatHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	_, err := h.Handle(ctx, map[string]any{
		"combat_state": baseCombatStateArgsForResolve(playerID, enemyID),
		"outcome_type": "victory",
	})
	if err == nil || !strings.Contains(err.Error(), "player character not found") {
		t.Fatalf("expected player not found error, got %v", err)
	}
}

func TestResolveCombatStoreErrorPropagates(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()
	store := &stubResolveCombatStore{
		player:      defaultPlayer(playerID),
		updateHPErr: errors.New("db write failed"),
	}
	h := NewResolveCombatHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	_, err := h.Handle(ctx, map[string]any{
		"combat_state": baseCombatStateArgsForResolve(playerID, enemyID),
		"outcome_type": "victory",
	})
	if err == nil || !strings.Contains(err.Error(), "persist player hp") {
		t.Fatalf("expected HP persist error, got %v", err)
	}
}

func TestResolveCombatVictoryNegativeXPReturnsError(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()
	store := &stubResolveCombatStore{player: defaultPlayer(playerID)}
	h := NewResolveCombatHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	_, err := h.Handle(ctx, map[string]any{
		"combat_state": baseCombatStateArgsForResolve(playerID, enemyID),
		"outcome_type": "victory",
		"xp_earned":    -10,
	})
	if err == nil || !strings.Contains(err.Error(), "xp_earned must be greater than or equal to 0") {
		t.Fatalf("expected negative XP error, got %v", err)
	}
}

func TestResolveCombatVictoryNonCombatantNPCIDReturnsError(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()
	unknownNPCID := uuid.New()
	store := &stubResolveCombatStore{player: defaultPlayer(playerID)}
	h := NewResolveCombatHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	_, err := h.Handle(ctx, map[string]any{
		"combat_state": baseCombatStateArgsForResolve(playerID, enemyID),
		"outcome_type": "victory",
		"dead_npc_ids": []any{unknownNPCID.String()},
	})
	if err == nil || !strings.Contains(err.Error(), "is not an NPC combatant in this combat") {
		t.Fatalf("expected non-combatant NPC error, got %v", err)
	}
}

func TestResolveCombatSurrenderNonCombatantNPCIDReturnsError(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()
	unknownNPCID := uuid.New()
	store := &stubResolveCombatStore{player: defaultPlayer(playerID)}
	h := NewResolveCombatHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	_, err := h.Handle(ctx, map[string]any{
		"combat_state":      baseCombatStateArgsForResolve(playerID, enemyID),
		"outcome_type":      "surrender",
		"surrender_npc_ids": []any{unknownNPCID.String()},
	})
	if err == nil || !strings.Contains(err.Error(), "is not an NPC combatant in this combat") {
		t.Fatalf("expected non-combatant NPC error, got %v", err)
	}
}

func TestResolveCombatVictoryInvalidLootItemTypeReturnsError(t *testing.T) {
	playerID := uuid.New()
	enemyID := uuid.New()
	store := &stubResolveCombatStore{player: defaultPlayer(playerID)}
	h := NewResolveCombatHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	_, err := h.Handle(ctx, map[string]any{
		"combat_state": baseCombatStateArgsForResolve(playerID, enemyID),
		"outcome_type": "victory",
		"loot": []any{
			map[string]any{"name": "Mystery Blob", "description": "Unknown.", "item_type": "unknown_type"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "item_type must be one of") {
		t.Fatalf("expected invalid item_type error, got %v", err)
	}
}
