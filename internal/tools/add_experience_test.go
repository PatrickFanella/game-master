package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
)

type stubAddExperienceStore struct {
	player             *domain.PlayerCharacter
	getErr             error
	updateErr          error
	lastPlayerID       uuid.UUID
	lastExperience     int
	lastLevel          int
	updateCallCount    int
}

func (s *stubAddExperienceStore) GetPlayerCharacterByID(_ context.Context, _ uuid.UUID) (*domain.PlayerCharacter, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.player, nil
}

func (s *stubAddExperienceStore) UpdatePlayerExperience(_ context.Context, playerCharacterID uuid.UUID, experience, level int) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.lastPlayerID = playerCharacterID
	s.lastExperience = experience
	s.lastLevel = level
	s.updateCallCount++
	return nil
}

func TestRegisterAddExperience(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterAddExperience(reg, &stubAddExperienceStore{}); err != nil {
		t.Fatalf("register add_experience: %v", err)
	}

	registered := reg.List()
	if len(registered) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(registered))
	}
	if registered[0].Name != addExperienceToolName {
		t.Fatalf("tool name = %q, want %q", registered[0].Name, addExperienceToolName)
	}
}

func TestAddExperienceHandleAccumulatesExperienceAndFlagsLevelUp(t *testing.T) {
	playerID := uuid.New()
	store := &stubAddExperienceStore{
		player: &domain.PlayerCharacter{
			ID:         playerID,
			Experience: 980,
			Level:      1,
		},
	}
	h := NewAddExperienceHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	got, err := h.Handle(ctx, map[string]any{
		"amount": 50,
		"reason": "defeating the bandit",
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if store.lastExperience != 1030 {
		t.Fatalf("updated experience = %d, want 1030", store.lastExperience)
	}
	if store.lastLevel != 1 {
		t.Fatalf("updated level = %d, want 1", store.lastLevel)
	}
	if got.Data["level_up_available"] != true {
		t.Fatalf("level_up_available = %v, want true", got.Data["level_up_available"])
	}
	if got.Narrative != "You gained 50 XP for defeating the bandit." {
		t.Fatalf("narrative = %q", got.Narrative)
	}
}

func TestAddExperienceHandleUsesConfigurableThreshold(t *testing.T) {
	playerID := uuid.New()
	store := &stubAddExperienceStore{
		player: &domain.PlayerCharacter{
			ID:         playerID,
			Experience: 95,
			Level:      1,
		},
	}
	h := NewAddExperienceHandlerWithThreshold(store, func(_ int) int { return 100 })
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	got, err := h.Handle(ctx, map[string]any{
		"amount": 5,
		"reason": "a quest milestone",
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if got.Data["level_up_available"] != true {
		t.Fatalf("level_up_available = %v, want true", got.Data["level_up_available"])
	}
}

func TestAddExperienceHandleUsesCumulativeDefaultThreshold(t *testing.T) {
	playerID := uuid.New()
	store := &stubAddExperienceStore{
		player: &domain.PlayerCharacter{
			ID:         playerID,
			Experience: 2999,
			Level:      2,
		},
	}
	h := NewAddExperienceHandler(store)
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	got, err := h.Handle(ctx, map[string]any{
		"amount": 1,
		"reason": "finishing a battle",
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if got.Data["level_up_available"] != true {
		t.Fatalf("level_up_available = %v, want true", got.Data["level_up_available"])
	}
}

func TestAddExperienceHandleValidationAndStoreErrors(t *testing.T) {
	playerID := uuid.New()
	ctx := WithCurrentPlayerCharacterID(context.Background(), playerID)

	t.Run("requires player context", func(t *testing.T) {
		h := NewAddExperienceHandler(&stubAddExperienceStore{})
		_, err := h.Handle(context.Background(), map[string]any{
			"amount": 1,
			"reason": "test",
		})
		if err == nil || !strings.Contains(err.Error(), "requires current player character id in context") {
			t.Fatalf("error = %v, want missing context", err)
		}
	})

	t.Run("rejects non-positive amount", func(t *testing.T) {
		h := NewAddExperienceHandler(&stubAddExperienceStore{
			player: &domain.PlayerCharacter{ID: playerID, Level: 1},
		})
		_, err := h.Handle(ctx, map[string]any{
			"amount": 0,
			"reason": "test",
		})
		if err == nil || !strings.Contains(err.Error(), "amount must be greater than 0") {
			t.Fatalf("error = %v, want amount validation", err)
		}
	})

	t.Run("get player wrapped error", func(t *testing.T) {
		h := NewAddExperienceHandler(&stubAddExperienceStore{
			getErr: errors.New("db read failed"),
		})
		_, err := h.Handle(ctx, map[string]any{
			"amount": 5,
			"reason": "test",
		})
		if err == nil || !strings.Contains(err.Error(), "get player character") {
			t.Fatalf("error = %v, want get player character wrapper", err)
		}
	})

	t.Run("update experience wrapped error", func(t *testing.T) {
		h := NewAddExperienceHandler(&stubAddExperienceStore{
			player: &domain.PlayerCharacter{ID: playerID, Experience: 10, Level: 1},
			updateErr: errors.New("db write failed"),
		})
		_, err := h.Handle(ctx, map[string]any{
			"amount": 5,
			"reason": "test",
		})
		if err == nil || !strings.Contains(err.Error(), "update player experience") {
			t.Fatalf("error = %v, want update player experience wrapper", err)
		}
	})
}

var _ AddExperienceStore = (*stubAddExperienceStore)(nil)
