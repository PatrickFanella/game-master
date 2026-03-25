package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

// testApp returns a minimal App suitable for teatest integration tests.
func testApp() App {
	return NewApp(testCfg)
}

// finalTimeout is the maximum time to wait for the program to finish.
var finalTimeout = teatest.WithFinalTimeout(3 * time.Second)

// waitDuration is the maximum time for WaitFor checks.
var waitDuration = teatest.WithDuration(3 * time.Second)

func TestTeatest_AppBootsShowsNarrativeView(t *testing.T) {
	tm := teatest.NewTestModel(
		t,
		testApp(),
		teatest.WithInitialTermSize(100, 30),
	)

	// Wait until the UI output contains "Narrative", indicating that the
	// narrative view has been rendered.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Narrative"))
	}, waitDuration)

	// Quit the program so we can inspect the final model.
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})

	fm := tm.FinalModel(t, finalTimeout)
	app, ok := fm.(App)
	if !ok {
		t.Fatalf("expected App model, got %T", fm)
	}
	if app.ActiveViewState() != ViewNarrative {
		t.Fatalf("expected ViewNarrative on boot, got %d", app.ActiveViewState())
	}
}

func TestTeatest_TextInputAppearsInViewport(t *testing.T) {
	tm := teatest.NewTestModel(
		t,
		testApp(),
		teatest.WithInitialTermSize(100, 30),
	)

	// Ensure the program has started and rendered the initial view before
	// sending key messages.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Narrative"))
	}, waitDuration)

	// Send each character individually via Send so everything goes through
	// the message channel. Avoid characters that are global key bindings:
	// h (prev-tab), l (next-tab), q (quit), 1-4 (view switch).
	for _, r := range "see a cart" {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Quit and use FinalModel to verify the entry was added.
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	fm := tm.FinalModel(t, finalTimeout)
	app, ok := fm.(App)
	if !ok {
		t.Fatalf("expected App model, got %T", fm)
	}

	// Verify the submitted text is in the final rendered view.
	finalView := app.View()
	if !strings.Contains(finalView, "see a cart") {
		t.Fatal("expected submitted text 'see a cart' to appear in the final rendered view")
	}
}

func TestTeatest_NumberKeysSwitchToCorrectView(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected ViewState
		contains string
	}{
		{"press 2 → Character Sheet", "2", ViewCharacterSheet, "Character Sheet"},
		{"press 3 → Inventory", "3", ViewInventory, "Inventory"},
		{"press 4 → Quest Log", "4", ViewQuestLog, "Quest Log"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := teatest.NewTestModel(
				t,
				testApp(),
				teatest.WithInitialTermSize(100, 30),
			)

			// Press the number key to switch views.
			tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})

			// Wait for the expected view content.
			teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
				return bytes.Contains(bts, []byte(tt.contains))
			}, waitDuration)

			// Quit and verify final model state.
			tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
			fm := tm.FinalModel(t, finalTimeout)
			app, ok := fm.(App)
			if !ok {
				t.Fatalf("expected App model, got %T", fm)
			}
			if app.ActiveViewState() != tt.expected {
				t.Fatalf("expected ViewState %d, got %d", tt.expected, app.ActiveViewState())
			}
		})
	}

	// Test press 1 returns to Narrative from another view.
	t.Run("press 1 → Narrative from Character Sheet", func(t *testing.T) {
		tm := teatest.NewTestModel(
			t,
			testApp(),
			teatest.WithInitialTermSize(100, 30),
		)

		// First switch to Character Sheet.
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Character Sheet"))
		}, waitDuration)

		// Press 1 to go back to Narrative.
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte("[Narrative]"))
		}, waitDuration)

		tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
		fm := tm.FinalModel(t, finalTimeout)
		app, ok := fm.(App)
		if !ok {
			t.Fatalf("expected App model, got %T", fm)
		}
		if app.ActiveViewState() != ViewNarrative {
			t.Fatalf("expected ViewNarrative, got %d", app.ActiveViewState())
		}
	})
}

func TestTeatest_TabCyclesThroughViewsInOrder(t *testing.T) {
	tm := teatest.NewTestModel(
		t,
		testApp(),
		teatest.WithInitialTermSize(100, 30),
	)

	// Tab should cycle: Narrative → Character → Inventory → Quests → Narrative.
	// The status bar highlights the active view with brackets: [ViewName].
	expectedHighlights := []string{
		"[Character]",
		"[Inventory]",
		"[Quests]",
		"[Narrative]",
	}

	for _, highlight := range expectedHighlights {
		tm.Send(tea.KeyMsg{Type: tea.KeyTab})
		teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
			return bytes.Contains(bts, []byte(highlight))
		}, waitDuration)
	}

	// Quit and verify we're back on narrative after a full cycle.
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	fm := tm.FinalModel(t, finalTimeout)
	app, ok := fm.(App)
	if !ok {
		t.Fatalf("expected App model, got %T", fm)
	}
	if app.ActiveViewState() != ViewNarrative {
		t.Fatalf("expected ViewNarrative after full tab cycle, got %d", app.ActiveViewState())
	}
}

func TestTeatest_ViewSwitchingPreservesState(t *testing.T) {
	tm := teatest.NewTestModel(
		t,
		testApp(),
		teatest.WithInitialTermSize(100, 30),
	)

	// Ensure the program has started and rendered the initial view.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Narrative"))
	}, waitDuration)

	// Send each character individually via Send so everything goes through
	// the message channel. Avoid characters that are global key bindings:
	// h (prev-tab), l (next-tab), q (quit), 1-4 (view switch).
	for _, r := range "see a cart" {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	// Switch to character sheet (2) then back to narrative (1).
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})

	// Quit and use FinalModel to verify state was preserved.
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	fm := tm.FinalModel(t, finalTimeout)
	app, ok := fm.(App)
	if !ok {
		t.Fatalf("expected App model, got %T", fm)
	}

	if app.ActiveViewState() != ViewNarrative {
		t.Fatalf("expected ViewNarrative after switching back, got %d", app.ActiveViewState())
	}
	// The previously submitted text should still be in the rendered view.
	finalView := app.View()
	if !strings.Contains(finalView, "see a cart") {
		t.Fatal("expected submitted text 'see a cart' to remain visible after view round-trip")
	}
}

func TestTeatest_CtrlCTriggersQuit(t *testing.T) {
	tm := teatest.NewTestModel(
		t,
		testApp(),
		teatest.WithInitialTermSize(100, 30),
	)

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Narrative"))
	}, waitDuration)

	// Send ctrl+c and verify the program terminates.
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})

	fm := tm.FinalModel(t, finalTimeout)
	if fm == nil {
		t.Fatal("expected non-nil final model after ctrl+c quit")
	}
}
