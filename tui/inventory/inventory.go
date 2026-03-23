// Package inventory provides the inventory view for the TUI.
// It renders a placeholder inventory using the shared styles package.
// Real inventory data will be populated in the inventory management epic.
package inventory

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/PatrickFanella/game-master/tui/styles"
)

// NavigateBackMsg is emitted when the user presses Escape to return to the
// narrative view.
type NavigateBackMsg struct{}

// Model is the Bubble Tea model for the inventory view.
type Model struct {
	width, height int
}

// New returns a freshly initialised inventory Model.
func New() Model {
	return Model{}
}

// SetSize updates the viewport dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return nil }

// Update implements tea.Model. Pressing Escape emits NavigateBackMsg so the
// parent App can return focus to the narrative view.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEsc {
		return m, func() tea.Msg { return NavigateBackMsg{} }
	}
	return m, nil
}

// View implements tea.Model and renders the inventory placeholder.
func (m Model) View() string {
	title := styles.Header.Render("🎒 Inventory")

	placeholder := styles.SystemMessage.Render(
		"Inventory data will be available in a future update.",
	)
	hint := styles.Muted.Render("Press Esc to return to the narrative view.")

	content := styles.JoinVertical(placeholder, "", hint)
	return styles.Container.Width(m.width).Render(
		styles.JoinVertical(title, "", content),
	)
}
