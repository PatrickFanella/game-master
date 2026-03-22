// Package character provides the character sheet view for the TUI.
// It renders a player's name, class, level, and core statistics using the
// shared styles package.
package character

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PatrickFanella/game-master/tui/styles"
)

// Stat is a named numeric attribute on the character sheet.
type Stat struct {
	Name  string
	Value int
	Max   int // 0 means no cap
}

// Model is the Bubble Tea model for the character view.
type Model struct {
	width, height int

	Name  string
	Class string
	Level int
	HP    Stat
	Stats []Stat
}

// New returns a freshly initialised character Model with placeholder data.
func New() Model {
	return Model{
		Name:  "Adventurer",
		Class: "Unknown",
		Level: 1,
		HP:    Stat{Name: "HP", Value: 10, Max: 10},
		Stats: []Stat{
			{Name: "STR", Value: 10},
			{Name: "DEX", Value: 10},
			{Name: "CON", Value: 10},
			{Name: "INT", Value: 10},
			{Name: "WIS", Value: 10},
			{Name: "CHA", Value: 10},
		},
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

// View implements tea.Model and renders the character sheet.
func (m Model) View() string {
	title := styles.Header.Render("⚔️  Character")

	nameLine := styles.SubHeader.Render(m.Name) +
		styles.Muted.Render(fmt.Sprintf("  %s · Level %d", m.Class, m.Level))

	hpLabel := styles.StatusSuccess.Render("HP")
	hpValue := styles.Body.Render(fmt.Sprintf("%d / %d", m.HP.Value, m.HP.Max))
	hpLine := hpLabel + styles.Muted.Render(" ") + hpValue

	var statParts []string
	for _, s := range m.Stats {
		label := styles.Muted.Render(s.Name)
		val := styles.Body.Render(fmt.Sprintf("%2d", s.Value))
		statParts = append(statParts, label+" "+val)
	}

	statsLine := strings.Join(statParts, "  ")

	content := styles.JoinVertical(nameLine, "", hpLine, "", statsLine)
	return styles.Container.Width(m.width).Render(
		styles.JoinVertical(title, "", content),
	)
}
