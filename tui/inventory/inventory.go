// Package inventory provides the inventory view for the TUI.
// It renders a list of carried items using the shared styles package.
package inventory

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PatrickFanella/game-master/tui/styles"
)

// Item represents a single entry in the player's inventory.
type Item struct {
	Name     string
	Quantity int
	Weight   float64 // kg
}

// Model is the Bubble Tea model for the inventory view.
type Model struct {
	width, height int
	Items         []Item
	Gold          int
}

// New returns a freshly initialised inventory Model with placeholder data.
func New() Model {
	return Model{
		Items: []Item{
			{Name: "Torch", Quantity: 3, Weight: 0.5},
			{Name: "Rope (10m)", Quantity: 1, Weight: 2.0},
			{Name: "Rations", Quantity: 5, Weight: 0.4},
		},
		Gold: 15,
	}
}

// SetSize updates the viewport dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }

// View implements tea.Model and renders the inventory list.
func (m Model) View() string {
	title := styles.Header.Render("🎒 Inventory")
	goldLine := styles.StatusWarning.Render("Gold:") + " " + styles.Body.Render(fmt.Sprintf("%d gp", m.Gold))

	var lines []string
	for _, item := range m.Items {
		qty := styles.Muted.Render(fmt.Sprintf("x%d", item.Quantity))
		name := styles.Body.Render(item.Name)
		wt := styles.Muted.Render(fmt.Sprintf("(%.1f kg)", item.Weight*float64(item.Quantity)))
		lines = append(lines, strings.Join([]string{"  · ", qty, " ", name, "  ", wt}, ""))
	}
	if len(lines) == 0 {
		lines = []string{styles.Muted.Render("  (empty)")}
	}

	itemList := strings.Join(lines, "\n")
	content := styles.JoinVertical(goldLine, "", itemList)
	return styles.Container.Width(m.width).Render(
		styles.JoinVertical(title, "", content),
	)
}
