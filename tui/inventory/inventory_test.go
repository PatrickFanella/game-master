package inventory

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Compile-time checks: *Model must satisfy tea.Model and the sub-model
// interface (tea.Model + SetSize) expected by the Router. Importing the parent
// tui package here would create a cycle, so the interface is inlined.
var _ tea.Model = (*Model)(nil)
var _ interface {
	tea.Model
	SetSize(width, height int)
} = (*Model)(nil)

func TestNewReturnsModel(t *testing.T) {
	m := New()
	if m.width != 0 || m.height != 0 {
		t.Fatal("expected zero dimensions on a freshly created model")
	}
}

func TestSetSizeUpdatesDimensions(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	if m.width != 80 || m.height != 24 {
		t.Fatalf("expected 80x24, got %dx%d", m.width, m.height)
	}
}

func TestInitReturnsNil(t *testing.T) {
	m := New()
	if m.Init() != nil {
		t.Fatal("Init() should return nil")
	}
}

func TestEscEmitsNavigateBackMsg(t *testing.T) {
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

func TestOtherKeyReturnsNoCmd(t *testing.T) {
	m := New()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected no command for non-Esc keys")
	}
}

func TestViewContainsInventoryHeader(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	got := m.View()
	if !strings.Contains(got, "Inventory") {
		t.Fatalf("expected View to contain %q, got:\n%s", "Inventory", got)
	}
}

func TestViewContainsPlaceholderText(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	got := m.View()
	if !strings.Contains(got, "future update") {
		t.Fatalf("expected View to contain placeholder text about a future update, got:\n%s", got)
	}
}

func TestViewContainsEscHint(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	got := m.View()
	if !strings.Contains(got, "Esc") {
		t.Fatalf("expected View to contain Esc key hint, got:\n%s", got)
	}
}
