package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/config"
	"github.com/PatrickFanella/game-master/internal/dbutil"
	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/engine"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
	"github.com/PatrickFanella/game-master/tui/narrative"
)

// Compile-time check: App must implement tea.Model.
var _ tea.Model = App{}

// testCfg is a minimal config suitable for unit tests.
var testCfg = config.Config{
	LLM: config.LLMConfig{Provider: "ollama"},
}

// testCampaign is a zero-value campaign used in unit tests.
var testCampaign = statedb.Campaign{}

type mockGameEngine struct {
	processTurnFn     func(context.Context, uuid.UUID, string) (*engine.TurnResult, error)
	loadCampaignFn    func(context.Context, uuid.UUID) error
	inputs            []string
	campaignIDs       []uuid.UUID
	loadedCampaignIDs []uuid.UUID
}

func (m *mockGameEngine) ProcessTurn(ctx context.Context, campaignID uuid.UUID, input string) (*engine.TurnResult, error) {
	m.inputs = append(m.inputs, input)
	m.campaignIDs = append(m.campaignIDs, campaignID)
	if m.processTurnFn != nil {
		return m.processTurnFn(ctx, campaignID, input)
	}
	return &engine.TurnResult{}, nil
}

func (m *mockGameEngine) GetGameState(context.Context, uuid.UUID) (*engine.GameState, error) {
	return &engine.GameState{}, nil
}

func (m *mockGameEngine) NewCampaign(context.Context, uuid.UUID) (*domain.Campaign, error) {
	return nil, errors.New("not implemented")
}

func (m *mockGameEngine) LoadCampaign(ctx context.Context, campaignID uuid.UUID) error {
	m.loadedCampaignIDs = append(m.loadedCampaignIDs, campaignID)
	if m.loadCampaignFn != nil {
		return m.loadCampaignFn(ctx, campaignID)
	}
	return nil
}

func keyForView(view ViewState) rune {
	// ViewState is zero-based; user-facing key bindings are 1-based.
	return rune('1' + view)
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
	app := NewApp(testCfg, testCampaign)
	if app.router.TabCount() != 4 {
		t.Fatalf("expected 4 registered views, got %d", app.router.TabCount())
	}
}

func TestNewAppStartsOnNarrative(t *testing.T) {
	app := NewApp(testCfg, testCampaign)
	if app.ActiveViewState() != ViewNarrative {
		t.Fatalf("expected initial ViewState %d (ViewNarrative), got %d",
			ViewNarrative, app.ActiveViewState())
	}
}

func TestAppTabNamesMatchViewStates(t *testing.T) {
	app := NewApp(testCfg, testCampaign)
	tabs := app.router.Tabs()
	expected := []string{"Narrative", "Character", "Inventory", "Quests"}
	for i, name := range expected {
		if tabs[i].Name != name {
			t.Errorf("tab[%d]: expected %q, got %q", i, name, tabs[i].Name)
		}
	}
}

func TestAppInitReturnsNil(t *testing.T) {
	app := NewApp(testCfg, testCampaign)
	if app.Init() != nil {
		t.Fatal("Init() should return nil")
	}
}

func TestAppUpdateQuitCtrlC(t *testing.T) {
	app := NewApp(testCfg, testCampaign)
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit command for ctrl+c, got nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected tea.QuitMsg for ctrl+c")
	}
}

func TestAppUpdateQuitQ(t *testing.T) {
	app := NewApp(testCfg, testCampaign)
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected quit command for q, got nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected tea.QuitMsg for q")
	}
}

func TestAppUpdateTabNextView(t *testing.T) {
	app := NewApp(testCfg, testCampaign)
	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated := m.(App)
	if updated.ActiveViewState() != ViewCharacterSheet {
		t.Fatalf("expected ViewCharacterSheet after tab, got %d", updated.ActiveViewState())
	}
}

func TestAppUpdateTabWrapsAround(t *testing.T) {
	app := NewApp(testCfg, testCampaign)
	// Advance to the last view (QuestLog = index 3).
	app.router.GoToTab(3)
	app.viewState = ViewQuestLog
	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated := m.(App)
	if updated.ActiveViewState() != ViewNarrative {
		t.Fatalf("expected ViewNarrative after wrapping tab, got %d", updated.ActiveViewState())
	}
}

func TestAppUpdateShiftTabCyclesBackward(t *testing.T) {
	app := NewApp(testCfg, testCampaign)
	// shift+tab from Narrative wraps to QuestLog.
	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	updated := m.(App)
	if updated.ActiveViewState() != ViewQuestLog {
		t.Fatalf("expected ViewQuestLog after shift+tab wrap, got %d", updated.ActiveViewState())
	}
}

func TestAppUpdateShiftTabPrevView(t *testing.T) {
	app := NewApp(testCfg, testCampaign)
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
		app := NewApp(testCfg, testCampaign)
		m, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.key}})
		updated := m.(App)
		if updated.ActiveViewState() != tt.expected {
			t.Errorf("key %q: expected ViewState %d, got %d", tt.key, tt.expected, updated.ActiveViewState())
		}
	}
}

func TestAppUpdateViewSwitchingPreservesState(t *testing.T) {
	// Sub-model state should not be reset when switching between views.
	app := NewApp(testCfg, testCampaign)

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

func TestAppWindowSizeUpdatesState(t *testing.T) {
	app := NewApp(testCfg, testCampaign)
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated := m.(App)
	if updated.width != 120 || updated.height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", updated.width, updated.height)
	}
}

func TestAppViewReturnsNonEmpty(t *testing.T) {
	app := NewApp(testCfg, testCampaign)
	v := app.View()
	if v == "" {
		t.Fatal("View() should return non-empty string")
	}
}

func TestStatusBarShowsViewsHintsAndActiveView(t *testing.T) {
	app := NewApp(testCfg, testCampaign)
	_, statusBar := app.chrome()

	for _, label := range []string{"Narrative", "Character", "Inventory", "Quests"} {
		if !strings.Contains(statusBar, label) {
			t.Fatalf("expected status bar to include %q", label)
		}
	}
	if !strings.Contains(statusBar, "[Narrative]") {
		t.Fatal("expected status bar to highlight the active narrative view")
	}
	if !strings.Contains(statusBar, statusBarHints) {
		t.Fatal("expected status bar to include view switching key hints")
	}
}

func TestStatusBarUpdatesImmediatelyOnViewSwitch(t *testing.T) {
	app := NewApp(testCfg, testCampaign)
	targetInventoryKey := keyForView(ViewInventory)
	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{targetInventoryKey}})
	updated := m.(App)
	_, statusBar := updated.chrome()

	if !strings.Contains(statusBar, "[Inventory]") {
		t.Fatalf("expected status bar to highlight inventory after pressing %q", targetInventoryKey)
	}
	if strings.Contains(statusBar, "[Narrative]") {
		t.Fatalf("expected narrative to no longer be active after pressing %q", targetInventoryKey)
	}

	m2, _ := updated.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated2 := m2.(App)
	_, statusBar2 := updated2.chrome()
	if !strings.Contains(statusBar2, "[Quests]") {
		t.Fatal("expected status bar to highlight quests after tab cycling")
	}
}

func TestAppOtherKeyDelegatedToSubView(t *testing.T) {
	// Non-global keys should be forwarded to the active sub-view.
	// The active view is a real narrative.Model; pressing Enter doesn't crash.
	app := NewApp(testCfg, testCampaign)
	_, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// No panic = pass; the sub-view received the message.
}

func TestAppUnknownMsgForwardedToSubView(t *testing.T) {
	// Non-key, non-window messages (e.g. custom command results) must be
	// forwarded to the active sub-view rather than silently dropped.
	type customMsg struct{ value string }
	app := NewApp(testCfg, testCampaign)
	// Sending an unrecognised message type must not panic and must return a
	// well-formed model (not nil).
	m, _ := app.Update(customMsg{"hello"})
	if m == nil {
		t.Fatal("Update should return a non-nil model for unknown message types")
	}
	if _, ok := m.(App); !ok {
		t.Fatal("Update should return an App model for unknown message types")
	}
}

func TestAppSubmitCallsEngineAndStreamsNarrativeWithChoices(t *testing.T) {
	campaignID := uuid.New()
	mockEngine := &mockGameEngine{
		processTurnFn: func(_ context.Context, gotCampaignID uuid.UUID, gotInput string) (*engine.TurnResult, error) {
			if gotCampaignID != campaignID {
				t.Fatalf("expected campaign id %s, got %s", campaignID, gotCampaignID)
			}
			if gotInput != "open the door" {
				t.Fatalf("expected input %q, got %q", "open the door", gotInput)
			}
			return &engine.TurnResult{
				Narrative: "The heavy oak door swings inward.",
				Choices: []engine.Choice{
					{ID: "enter", Text: "Step inside"},
					{ID: "listen", Text: "Listen at the threshold"},
				},
			}, nil
		},
	}

	app := NewAppWithEngine(testCfg, statedb.Campaign{ID: dbutil.ToPgtype(campaignID)}, context.Background(), mockEngine)
	sized, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app = sized.(App)
	model, cmd := app.Update(narrative.SubmitMsg{Input: "open the door"})
	updated := model.(App)

	if !updated.turnBusy {
		t.Fatal("expected app to remain busy while the turn is in flight")
	}
	if !strings.Contains(updated.View(), "Thinking…") {
		t.Fatal("expected loading indicator while processing")
	}

	msg := updated.processTurn("open the door")()
	model, cmd = updated.Update(msg)
	updated = model.(App)
	if cmd == nil {
		t.Fatal("expected narrative streaming command")
	}

	for updated.turnBusy {
		msg = cmd()
		model, cmd = updated.Update(msg)
		updated = model.(App)
	}

	if len(mockEngine.inputs) != 1 || mockEngine.inputs[0] != "open the door" {
		t.Fatalf("expected engine to be called once with player input, got %#v", mockEngine.inputs)
	}
	view := updated.View()
	if !strings.Contains(view, "The heavy oak door swings inward.") {
		t.Fatal("expected streamed narrative to appear in the view")
	}
	if !strings.Contains(view, "Suggested choices") || !strings.Contains(view, "Step inside") {
		t.Fatal("expected suggested choices to render below the narrative")
	}
}

func TestAppTurnErrorAddsSystemMessage(t *testing.T) {
	mockEngine := &mockGameEngine{
		processTurnFn: func(context.Context, uuid.UUID, string) (*engine.TurnResult, error) {
			return nil, errors.New("connection failed")
		},
	}

	app := NewAppWithEngine(testCfg, testCampaign, context.Background(), mockEngine)
	sized, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app = sized.(App)
	model, _ := app.Update(narrative.SubmitMsg{Input: "talk to innkeeper"})
	updated := model.(App)

	model, _ = updated.Update(updated.processTurn("talk to innkeeper")())
	updated = model.(App)

	if updated.turnBusy {
		t.Fatal("expected busy state to clear after error")
	}
	if !strings.Contains(updated.View(), "Error: connection failed") {
		t.Fatal("expected error state to be shown in the narrative viewport")
	}
}

func TestNextNarrativeChunkPreservesUTF8Runes(t *testing.T) {
	text := "🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂"

	chunk, remaining := nextNarrativeChunk(text)

	if chunk == "" || remaining == "" {
		t.Fatal("expected text to be split into two non-empty chunks")
	}
	if chunk != "🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂" {
		t.Fatalf("unexpected first chunk: %q", chunk)
	}
	if remaining != "🙂" {
		t.Fatalf("unexpected remaining chunk: %q", remaining)
	}
}
