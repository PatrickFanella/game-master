package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
)

type stubLevelUpStore struct {
	player                *domain.PlayerCharacter
	getErr                error
	updateLevelErr        error
	updateStatsErr        error
	updateAbilitiesErr    error
	lastPlayerID          uuid.UUID
	lastLevel             int
	lastStats             json.RawMessage
	lastAbilities         json.RawMessage
	updateLevelCallCount  int
	updateStatsCallCount  int
	updateAbilityCallCount int
}

func (s *stubLevelUpStore) GetPlayerCharacterByID(_ context.Context, _ uuid.UUID) (*domain.PlayerCharacter, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.player, nil
}

func (s *stubLevelUpStore) UpdatePlayerLevel(_ context.Context, playerCharacterID uuid.UUID, level int) error {
	if s.updateLevelErr != nil {
		return s.updateLevelErr
	}
	s.lastPlayerID = playerCharacterID
	s.lastLevel = level
	s.updateLevelCallCount++
	return nil
}

func (s *stubLevelUpStore) UpdatePlayerStats(_ context.Context, _ uuid.UUID, stats json.RawMessage) error {
	if s.updateStatsErr != nil {
		return s.updateStatsErr
	}
	s.lastStats = append([]byte(nil), stats...)
	s.updateStatsCallCount++
	return nil
}

func (s *stubLevelUpStore) UpdatePlayerAbilities(_ context.Context, _ uuid.UUID, abilities json.RawMessage) error {
	if s.updateAbilitiesErr != nil {
		return s.updateAbilitiesErr
	}
	s.lastAbilities = append([]byte(nil), abilities...)
	s.updateAbilityCallCount++
	return nil
}

func TestRegisterLevelUp(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterLevelUp(reg, &stubLevelUpStore{}); err != nil {
		t.Fatalf("register level_up: %v", err)
	}

	registered := reg.List()
	if len(registered) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(registered))
	}
	if registered[0].Name != levelUpToolName {
		t.Fatalf("tool name = %q, want %q", registered[0].Name, levelUpToolName)
	}
}

func TestLevelUpHandleAppliesLevelStatsAndAbilities(t *testing.T) {
	playerID := uuid.New()
	store := &stubLevelUpStore{
		player: &domain.PlayerCharacter{
			ID:         playerID,
			Experience: 1000,
			Level:      1,
			Stats:      []byte(`{"strength":10,"dexterity":12}`),
			Abilities:  []byte(`["Parry"]`),
		},
	}
	h := NewLevelUpHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	got, err := h.Handle(ctx, map[string]any{
		"stat_boosts": map[string]any{
			"strength": 2,
		},
		"new_abilities": []any{"Power Strike", "Parry"},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if store.lastLevel != 2 {
		t.Fatalf("updated level = %d, want 2", store.lastLevel)
	}
	if store.updateLevelCallCount != 1 {
		t.Fatalf("update level call count = %d, want 1", store.updateLevelCallCount)
	}
	if store.updateStatsCallCount != 1 {
		t.Fatalf("update stats call count = %d, want 1", store.updateStatsCallCount)
	}
	if store.updateAbilityCallCount != 1 {
		t.Fatalf("update abilities call count = %d, want 1", store.updateAbilityCallCount)
	}

	var stats map[string]any
	if err := json.Unmarshal(store.lastStats, &stats); err != nil {
		t.Fatalf("unmarshal stats: %v", err)
	}
	if stats["strength"] != float64(12) {
		t.Fatalf("strength = %v, want 12", stats["strength"])
	}

	var abilities []string
	if err := json.Unmarshal(store.lastAbilities, &abilities); err != nil {
		t.Fatalf("unmarshal abilities: %v", err)
	}
	if len(abilities) != 2 {
		t.Fatalf("abilities length = %d, want 2", len(abilities))
	}
	if got.Data["new_level"] != 2 {
		t.Fatalf("new_level = %v, want 2", got.Data["new_level"])
	}
	if _, ok := got.Data["stat_boosts_applied"]; ok {
		t.Fatalf("unexpected stat_boosts_applied field in response")
	}
	updatedStats, ok := got.Data["updated_stats"].(map[string]int)
	if !ok {
		t.Fatalf("updated_stats has unexpected type %T", got.Data["updated_stats"])
	}
	if updatedStats["strength"] != 12 {
		t.Fatalf("updated_stats[strength] = %d, want 12", updatedStats["strength"])
	}
	if got.Narrative != "You reached level 2." {
		t.Fatalf("narrative = %q", got.Narrative)
	}
}

func TestLevelUpHandleNormalizesStatBoostKeys(t *testing.T) {
	playerID := uuid.New()
	store := &stubLevelUpStore{
		player: &domain.PlayerCharacter{
			ID:         playerID,
			Experience: 1000,
			Level:      1,
			Stats:      []byte(`{"strength":10}`),
		},
	}
	h := NewLevelUpHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	_, err := h.Handle(ctx, map[string]any{
		"stat_boosts": map[string]any{
			" strength ": 2,
			"STRENGTH":   1,
		},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var stats map[string]any
	if err := json.Unmarshal(store.lastStats, &stats); err != nil {
		t.Fatalf("unmarshal stats: %v", err)
	}
	if stats["strength"] != float64(13) {
		t.Fatalf("strength = %v, want 13", stats["strength"])
	}
}

func TestLevelUpHandleUsesConfigurableThreshold(t *testing.T) {
	playerID := uuid.New()
	store := &stubLevelUpStore{
		player: &domain.PlayerCharacter{
			ID:         playerID,
			Experience: 9,
			Level:      1,
		},
	}
	h := NewLevelUpHandlerWithThreshold(store, func(_ int) int { return 10 })
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	_, err := h.Handle(ctx, map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "insufficient experience") {
		t.Fatalf("error = %v, want insufficient experience", err)
	}

	store.player.Experience = 10
	got, err := h.Handle(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if got.Data["new_level"] != 2 {
		t.Fatalf("new_level = %v, want 2", got.Data["new_level"])
	}
}

func TestLevelUpHandleValidationAndStoreErrors(t *testing.T) {
	playerID := uuid.New()
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	t.Run("requires player context", func(t *testing.T) {
		h := NewLevelUpHandler(&stubLevelUpStore{})
		_, err := h.Handle(context.Background(), map[string]any{})
		if err == nil || !strings.Contains(err.Error(), "requires current player character id in context") {
			t.Fatalf("error = %v, want missing context", err)
		}
	})

	t.Run("get player wrapped error", func(t *testing.T) {
		h := NewLevelUpHandler(&stubLevelUpStore{
			getErr: errors.New("db read failed"),
		})
		_, err := h.Handle(ctx, map[string]any{})
		if err == nil || !strings.Contains(err.Error(), "get player character") {
			t.Fatalf("error = %v, want get player character wrapper", err)
		}
	})

	t.Run("update level wrapped error", func(t *testing.T) {
		h := NewLevelUpHandler(&stubLevelUpStore{
			player: &domain.PlayerCharacter{
				ID:         playerID,
				Experience: 1000,
				Level:      1,
			},
			updateLevelErr: errors.New("db write failed"),
		})
		_, err := h.Handle(ctx, map[string]any{})
		if err == nil || !strings.Contains(err.Error(), "update player level") {
			t.Fatalf("error = %v, want update player level wrapper", err)
		}
	})
}

var _ LevelUpStore = (*stubLevelUpStore)(nil)
