package quest

import (
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

func testQuestData() ([]domain.Quest, map[uuid.UUID][]domain.QuestObjective) {
	questID1 := uuid.New()
	questID2 := uuid.New()
	quests := []domain.Quest{
		{ID: questID1, Title: "The Missing Merchant", QuestType: domain.QuestTypeShortTerm, Status: domain.QuestStatusActive},
		{ID: questID2, Title: "Welcome to Town", QuestType: domain.QuestTypeMediumTerm, Status: domain.QuestStatusCompleted},
	}
	objectives := map[uuid.UUID][]domain.QuestObjective{
		questID1: {
			{ID: uuid.New(), QuestID: questID1, Description: "Speak to the innkeeper", Completed: true, OrderIndex: 0},
			{ID: uuid.New(), QuestID: questID1, Description: "Investigate east road", Completed: false, OrderIndex: 1},
		},
	}
	return quests, objectives
}

func modelWithQuests(t *testing.T) Model {
	t.Helper()
	m := New()
	m.SetSize(80, 24)
	quests, objectives := testQuestData()
	updated, _ := m.Update(UpdateMsg{Quests: quests, Objectives: objectives})
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
	if len(m.quests) != 0 {
		t.Fatal("expected no quests on a freshly created model")
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
	updated, _ := m.Update(UpdateMsg{Quests: nil})
	got := updated.(*Model).View()
	if !strings.Contains(got, "No quests yet") {
		t.Fatalf("expected empty quest message, got:\n%s", got)
	}
}

func TestView_WithQuests(t *testing.T) {
	m := modelWithQuests(t)
	got := m.View()

	for _, want := range []string{"The Missing Merchant", "Welcome to Town"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected View to contain %q, got:\n%s", want, got)
		}
	}

	// Verify status grouping.
	if !strings.Contains(got, "Active") {
		t.Fatal("expected Active section header")
	}
	if !strings.Contains(got, "Completed") {
		t.Fatal("expected Completed section header")
	}
}

func TestView_ObjectivesRendered(t *testing.T) {
	m := modelWithQuests(t)
	got := m.View()

	// Active quest should be auto-expanded.
	if !strings.Contains(got, "Speak to the innkeeper") {
		t.Fatalf("expected completed objective text, got:\n%s", got)
	}
	if !strings.Contains(got, "Investigate east road") {
		t.Fatalf("expected pending objective text, got:\n%s", got)
	}
	// Check markers.
	if !strings.Contains(got, "✓") {
		t.Fatal("expected ✓ marker for completed objective")
	}
	if !strings.Contains(got, "○") {
		t.Fatal("expected ○ marker for pending objective")
	}
}

func TestView_ExpandCollapse(t *testing.T) {
	m := modelWithQuests(t)

	// Active quest at cursor 0 should be expanded. Press Enter to collapse.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(*Model)
	got := m2.View()

	// After collapsing, objectives should not appear.
	if strings.Contains(got, "Speak to the innkeeper") {
		t.Fatal("expected objectives to be hidden after collapse")
	}

	// Press Enter again to re-expand.
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m3 := updated2.(*Model)
	got2 := m3.View()
	if !strings.Contains(got2, "Speak to the innkeeper") {
		t.Fatal("expected objectives to reappear after re-expanding")
	}
}

func TestView_QuestTypeBadges(t *testing.T) {
	m := modelWithQuests(t)
	got := m.View()

	if !strings.Contains(got, "[ST]") {
		t.Fatal("expected [ST] badge for short term quest")
	}
	if !strings.Contains(got, "[MT]") {
		t.Fatal("expected [MT] badge for medium term quest")
	}
}

func TestView_CursorNavigation(t *testing.T) {
	m := modelWithQuests(t)
	// Move cursor down.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m2 := updated.(*Model)
	if m2.cursor != 1 {
		t.Fatalf("expected cursor at 1 after j, got %d", m2.cursor)
	}

	// Wrap around.
	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m3 := updated2.(*Model)
	if m3.cursor != 0 {
		t.Fatalf("expected cursor to wrap to 0, got %d", m3.cursor)
	}
}

func TestView_ContainsQuestLogHeader(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	got := m.View()
	if !strings.Contains(got, "Quest Log") {
		t.Fatalf("expected View to contain 'Quest Log', got:\n%s", got)
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

func TestView_NoObjectivesQuest(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	quests := []domain.Quest{
		{ID: uuid.New(), Title: "Solo Quest", QuestType: domain.QuestTypeLongTerm, Status: domain.QuestStatusActive},
	}
	updated, _ := m.Update(UpdateMsg{Quests: quests, Objectives: nil})
	got := updated.(*Model).View()
	if !strings.Contains(got, "Solo Quest") {
		t.Fatalf("expected quest title, got:\n%s", got)
	}
	if !strings.Contains(got, "[LT]") {
		t.Fatal("expected [LT] badge")
	}
}
