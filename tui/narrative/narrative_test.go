package narrative

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Compile-time check: Model must implement tea.Model.
var _ tea.Model = (*Model)(nil)

func TestAddEntryAutoScrollsToBottom(t *testing.T) {
	m := New()
	m.SetSize(50, 8)

	for i := 0; i < 8; i++ {
		m.AddEntry(Entry{Kind: KindSystem, Text: strings.Repeat("line", 3)})
	}

	if !m.viewport.AtBottom() {
		t.Fatal("expected viewport to auto-scroll to bottom after appending entries")
	}
	if !m.autoScroll {
		t.Fatal("expected auto-scroll to remain enabled at bottom")
	}
}

func TestManualScrollUpDisablesAutoScroll(t *testing.T) {
	model := New()
	m := &model
	m.SetSize(50, 8)

	for i := 0; i < 10; i++ {
		m.AddEntry(Entry{Kind: KindSystem, Text: "entry"})
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(*Model)

	if m.viewport.AtBottom() {
		t.Fatal("expected viewport to move away from bottom after key up")
	}
	if m.autoScroll {
		t.Fatal("expected auto-scroll to disable after manual upward scroll")
	}
}

func TestManualScrollBackToBottomEnablesAutoScroll(t *testing.T) {
	model := New()
	m := &model
	m.SetSize(50, 8)

	for i := 0; i < 10; i++ {
		m.AddEntry(Entry{Kind: KindSystem, Text: "entry"})
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(*Model)
	if m.autoScroll {
		t.Fatal("expected auto-scroll disabled after scrolling up")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m = updated.(*Model)

	if !m.viewport.AtBottom() {
		t.Fatal("expected viewport at bottom after page down")
	}
	if !m.autoScroll {
		t.Fatal("expected auto-scroll re-enabled at bottom")
	}
}

func TestDoesNotAutoScrollWhenUserScrolledUp(t *testing.T) {
	model := New()
	m := &model
	m.SetSize(50, 8)

	for i := 0; i < 10; i++ {
		m.AddEntry(Entry{Kind: KindSystem, Text: "entry"})
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(*Model)
	offsetBefore := m.viewport.YOffset

	m.AddEntry(Entry{Kind: KindSystem, Text: "new entry"})

	if m.viewport.YOffset != offsetBefore {
		t.Fatalf("expected y-offset to remain %d when auto-scroll is off, got %d", offsetBefore, m.viewport.YOffset)
	}
	if m.autoScroll {
		t.Fatal("expected auto-scroll to remain disabled while user is not at bottom")
	}
}

func TestPageAndArrowKeysScroll(t *testing.T) {
	model := New()
	m := &model
	m.SetSize(50, 8)

	for i := 0; i < 12; i++ {
		m.AddEntry(Entry{Kind: KindSystem, Text: "entry"})
	}

	start := m.viewport.YOffset
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = updated.(*Model)
	if m.viewport.YOffset >= start {
		t.Fatal("expected page up to reduce y-offset")
	}

	afterPgUp := m.viewport.YOffset
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(*Model)
	if m.viewport.YOffset <= afterPgUp {
		t.Fatal("expected down arrow to increase y-offset")
	}
}

func TestMouseWheelScroll(t *testing.T) {
	model := New()
	m := &model
	m.SetSize(50, 8)

	for i := 0; i < 12; i++ {
		m.AddEntry(Entry{Kind: KindSystem, Text: "entry"})
	}

	start := m.viewport.YOffset
	updated, _ := m.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelUp,
	})
	m = updated.(*Model)

	if m.viewport.YOffset >= start {
		t.Fatal("expected mouse wheel up to scroll viewport up")
	}
}

func TestResizeUpdatesViewportDimensions(t *testing.T) {
	m := New()
	m.SetSize(80, 20)

	wantWidth := 80 - narrativeViewportWidthOffset
	wantHeight := 20 - narrativeViewportHeightOffset - narrativeInputHeightOffset
	if m.viewport.Width != wantWidth {
		t.Fatalf("expected viewport width %d, got %d", wantWidth, m.viewport.Width)
	}
	if m.viewport.Height != wantHeight {
		t.Fatalf("expected viewport height %d, got %d", wantHeight, m.viewport.Height)
	}
	if m.input.Width != wantWidth {
		t.Fatalf("expected input width %d, got %d", wantWidth, m.input.Width)
	}

	m.SetSize(20, 6)
	wantWidth = 20 - narrativeViewportWidthOffset
	wantHeight = 6 - narrativeViewportHeightOffset - narrativeInputHeightOffset
	if wantHeight < 1 {
		wantHeight = 1
	}
	if m.viewport.Width != wantWidth {
		t.Fatalf("expected viewport width %d after resize, got %d", wantWidth, m.viewport.Width)
	}
	if m.viewport.Height != wantHeight {
		t.Fatalf("expected viewport height %d after resize, got %d", wantHeight, m.viewport.Height)
	}
	if m.input.Width != wantWidth {
		t.Fatalf("expected input width %d after resize, got %d", wantWidth, m.input.Width)
	}
}

func TestInputFocusedByDefault(t *testing.T) {
	m := New()
	if !m.input.Focused() {
		t.Fatal("expected input to be focused by default")
	}
}

func TestInputPlaceholder(t *testing.T) {
	m := New()
	if m.input.Placeholder != "What do you do?" {
		t.Fatalf("expected placeholder %q, got %q", "What do you do?", m.input.Placeholder)
	}
}

func TestEnterSubmitsInputAndClears(t *testing.T) {
	model := New()
	m := &model
	m.SetSize(50, 8)
	m.input.SetValue("look around")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*Model)

	if m.input.Value() != "" {
		t.Fatalf("expected input to clear after submit, got %q", m.input.Value())
	}
	if len(m.log) == 0 {
		t.Fatal("expected submitted input to be added to narrative log")
	}
	last := m.log[len(m.log)-1]
	if last.Kind != KindPlayer {
		t.Fatalf("expected last entry kind %v, got %v", KindPlayer, last.Kind)
	}
	if last.Text != "look around" {
		t.Fatalf("expected last entry text %q, got %q", "look around", last.Text)
	}
}

func TestEnterIgnoresEmptySubmission(t *testing.T) {
	model := New()
	m := &model
	m.SetSize(50, 8)
	m.input.SetValue("   ")
	before := len(m.log)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(*Model)

	if len(m.log) != before {
		t.Fatalf("expected empty submission to be ignored; before=%d after=%d", before, len(m.log))
	}
	if m.input.Value() != "   " {
		t.Fatalf("expected input to remain unchanged for empty submission, got %q", m.input.Value())
	}
}
