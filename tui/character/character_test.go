package character

import (
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
)

// Compile-time checks: *Model must satisfy tea.Model and the sub-model
// interface (tea.Model + SetSize) expected by the Router.
var _ tea.Model = (*Model)(nil)
var _ interface {
	tea.Model
	SetSize(width, height int)
} = (*Model)(nil)

func testPlayer() domain.PlayerCharacter {
	return domain.PlayerCharacter{
		ID:         uuid.New(),
		CampaignID: uuid.New(),
		UserID:     uuid.New(),
		Name:       "Aric",
		Level:      5,
		HP:         18,
		MaxHP:      25,
		Experience: 1200,
		Status:     "healthy",
		Stats:      json.RawMessage(`{"strength":14,"dexterity":12}`),
		Abilities:  json.RawMessage(`[{"name":"Fireball","type":"spell"},{"name":"Shield Bash","type":"melee"}]`),
	}
}

func modelWithPlayer(t *testing.T) Model {
	t.Helper()
	m := New()
	m.SetSize(80, 24)
	updated, _ := m.Update(UpdateMsg{Player: testPlayer()})
	return *(updated.(*Model))
}

func TestNew_ReturnsModel(t *testing.T) {
	m := New()
	if m.width != 0 || m.height != 0 {
		t.Fatal("expected zero dimensions on a freshly created model")
	}
	if m.loaded {
		t.Fatal("model should not be loaded initially")
	}
}

func TestSetSize_UpdatesDimensions(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	if m.width != 80 || m.height != 24 {
		t.Fatalf("expected 80x24, got %dx%d", m.width, m.height)
	}
}

func TestInit_ReturnsNil(t *testing.T) {
	m := New()
	if m.Init() != nil {
		t.Fatal("Init() should return nil")
	}
}

func TestUpdate_EscEmitsNavigateBackMsg(t *testing.T) {
	m := New()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected a command on Esc, got nil")
	}
	msg := cmd()
	if _, ok := msg.(NavigateBackMsg); !ok {
		t.Fatalf("expected NavigateBackMsg, got %T", msg)
	}
}

func TestView_EmptyState(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	got := m.View()
	if !strings.Contains(got, "No character data available") {
		t.Fatalf("expected empty state message, got:\n%s", got)
	}
}

func TestView_WithPlayer(t *testing.T) {
	m := modelWithPlayer(t)
	got := m.View()

	for _, want := range []string{"Aric", "Level 5", "18", "25"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected View to contain %q, got:\n%s", want, got)
		}
	}
}

func TestView_StatsRendered(t *testing.T) {
	m := modelWithPlayer(t)
	got := m.View()

	for _, want := range []string{"Strength", "14", "Dexterity", "12"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected View to contain stat %q, got:\n%s", want, got)
		}
	}
}

func TestView_AbilitiesRendered(t *testing.T) {
	m := modelWithPlayer(t)
	got := m.View()

	for _, want := range []string{"Fireball", "Shield Bash"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected View to contain ability %q, got:\n%s", want, got)
		}
	}
}

func TestView_MalformedStats(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	player := testPlayer()
	player.Stats = json.RawMessage(`invalid`)
	updated, _ := m.Update(UpdateMsg{Player: player})
	got := updated.(*Model).View()
	if !strings.Contains(got, "Stats unavailable") {
		t.Fatalf("expected 'Stats unavailable' for malformed JSON, got:\n%s", got)
	}
}

func TestView_MalformedAbilities(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	player := testPlayer()
	player.Abilities = json.RawMessage(`invalid`)
	updated, _ := m.Update(UpdateMsg{Player: player})
	got := updated.(*Model).View()
	if !strings.Contains(got, "Abilities unavailable") {
		t.Fatalf("expected 'Abilities unavailable' for malformed JSON, got:\n%s", got)
	}
}

func TestView_NilStats(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	player := testPlayer()
	player.Stats = nil
	updated, _ := m.Update(UpdateMsg{Player: player})
	got := updated.(*Model).View()
	if !strings.Contains(got, "No stats available") {
		t.Fatalf("expected 'No stats available' for nil Stats, got:\n%s", got)
	}
}

func TestView_ZeroMaxHP(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	player := testPlayer()
	player.HP = 0
	player.MaxHP = 0
	updated, _ := m.Update(UpdateMsg{Player: player})
	// Should not panic.
	got := updated.(*Model).View()
	if !strings.Contains(got, "HP:") {
		t.Fatalf("expected HP bar even with zero max, got:\n%s", got)
	}
}

func TestView_ContainsCharacterSheetHeader(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	got := m.View()
	if !strings.Contains(got, "Character Sheet") {
		t.Fatalf("expected View to contain 'Character Sheet', got:\n%s", got)
	}
}

func TestView_ContainsEscHint(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	got := m.View()
	if !strings.Contains(got, "Esc") {
		t.Fatalf("expected View to contain Esc hint, got:\n%s", got)
	}
}
