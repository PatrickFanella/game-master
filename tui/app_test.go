package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PatrickFanella/game-master/internal/config"
)

// testCfg is a minimal config suitable for unit tests.
var testCfg = config.Config{
	LLM: config.LLMConfig{Provider: "ollama"},
}

func TestViewStateConstants(t *testing.T) {
	if ViewNarrative != 0 {
		t.Fatalf("ViewNarrative should be 0, got %d", ViewNarrative)
	}
	if ViewCharacterSheet != 1 {
		t.Fatalf("ViewCharacterSheet should be 1, got %d", ViewCharacterSheet)
	}
	if ViewInventory != 2 {
		t.Fatalf("ViewInventory should be 2, got %d", ViewInventory)
	}
	if ViewQuestLog != 3 {
		t.Fatalf("ViewQuestLog should be 3, got %d", ViewQuestLog)
	}
}

func TestNewAppRegistersAllViews(t *testing.T) {
	app := NewApp(testCfg)
	if app.router.TabCount() != 4 {
		t.Fatalf("expected 4 registered views, got %d", app.router.TabCount())
	}
}

func TestNewAppStartsOnNarrative(t *testing.T) {
	app := NewApp(testCfg)
	if app.ActiveViewState() != ViewNarrative {
		t.Fatalf("expected initial ViewState %d (ViewNarrative), got %d",
			ViewNarrative, app.ActiveViewState())
	}
}

func TestAppInitReturnsNil(t *testing.T) {
	app := NewApp(testCfg)
	if app.Init() != nil {
		t.Fatal("Init() should return nil")
	}
}

func TestAppUpdateQuitCtrlC(t *testing.T) {
	app := NewApp(testCfg)
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit command for ctrl+c, got nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected tea.QuitMsg for ctrl+c")
	}
}

func TestAppUpdateQuitQ(t *testing.T) {
	app := NewApp(testCfg)
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected quit command for q, got nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected tea.QuitMsg for q")
	}
}

func TestAppUpdateTabNextView(t *testing.T) {
	app := NewApp(testCfg)
	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated := m.(App)
	if updated.ActiveViewState() != ViewCharacterSheet {
		t.Fatalf("expected ViewCharacterSheet after tab, got %d", updated.ActiveViewState())
	}
}

func TestAppUpdateTabWrapsAround(t *testing.T) {
	app := NewApp(testCfg)
	// Advance to the last view (QuestLog = index 3)
	app.router.GoToTab(3)
	app.viewState = ViewQuestLog
	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated := m.(App)
	if updated.ActiveViewState() != ViewNarrative {
		t.Fatalf("expected ViewNarrative after wrapping tab, got %d", updated.ActiveViewState())
	}
}

func TestAppUpdateShiftTabPrevView(t *testing.T) {
	app := NewApp(testCfg)
	app.router.GoToTab(2)
	app.viewState = ViewInventory
	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	updated := m.(App)
	if updated.ActiveViewState() != ViewCharacterSheet {
		t.Fatalf("expected ViewCharacterSheet after shift+tab, got %d", updated.ActiveViewState())
	}
}

func TestAppUpdateNumberKeys(t *testing.T) {
	tests := []struct {
		key      rune
		expected ViewState
	}{
		{'1', ViewNarrative},
		{'2', ViewCharacterSheet},
		{'3', ViewInventory},
		{'4', ViewQuestLog},
	}

	for _, tt := range tests {
		app := NewApp(testCfg)
		m, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.key}})
		updated := m.(App)
		if updated.ActiveViewState() != tt.expected {
			t.Errorf("key %q: expected ViewState %d, got %d", tt.key, tt.expected, updated.ActiveViewState())
		}
	}
}

func TestAppUpdateViewSwitchingPreservesState(t *testing.T) {
	// Sub-model state should not be reset when switching between views.
	app := NewApp(testCfg)

	// The narrative view was seeded with entries in NewApp; verify the router
	// still holds those entries after switching away and back.
	narrativeViewBefore := app.router.Tabs()[0].View

	// Switch away to character sheet.
	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	updated := m.(App)
	if updated.ActiveViewState() != ViewCharacterSheet {
		t.Fatalf("expected ViewCharacterSheet, got %d", updated.ActiveViewState())
	}

	// Switch back to narrative.
	m2, _ := updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	backToNarrative := m2.(App)

	narrativeViewAfter := backToNarrative.router.Tabs()[0].View
	if narrativeViewBefore != narrativeViewAfter {
		t.Fatal("narrative sub-model was replaced when switching views (state not preserved)")
	}
}

func TestAppViewReturnsNonEmpty(t *testing.T) {
	app := NewApp(testCfg)
	v := app.View()
	if v == "" {
		t.Fatal("View() should return non-empty string")
	}
}

func TestAppWindowSizeUpdatesState(t *testing.T) {
	app := NewApp(testCfg)
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated := m.(App)
	if updated.width != 120 || updated.height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", updated.width, updated.height)
	}
}

func TestAppTabNamesMatchViewStates(t *testing.T) {
	app := NewApp(testCfg)
	tabs := app.router.Tabs()
	expected := []string{"Narrative", "Character", "Inventory", "Quests"}
	for i, name := range expected {
		if tabs[i].Name != name {
			t.Errorf("tab[%d]: expected %q, got %q", i, name, tabs[i].Name)
		}
	}
}
