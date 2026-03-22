// Package quest provides the quest log view for the TUI.
// It renders active and completed objectives using the shared styles package.
package quest

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PatrickFanella/game-master/tui/styles"
)

// ObjectiveStatus tracks whether an objective is pending, completed, or failed.
type ObjectiveStatus int

const (
	StatusPending ObjectiveStatus = iota
	StatusComplete
	StatusFailed
)

// Objective is a single task within a quest.
type Objective struct {
	Description string
	Status      ObjectiveStatus
}

// Quest groups a name with its list of objectives.
type Quest struct {
	Name       string
	Objectives []Objective
}

// Model is the Bubble Tea model for the quest log view.
type Model struct {
	width, height int
	Quests        []Quest
}

// New returns a freshly initialised quest Model with a placeholder quest.
func New() Model {
	return Model{
		Quests: []Quest{
			{
				Name: "The Missing Merchant",
				Objectives: []Objective{
					{Description: "Speak to the innkeeper", Status: StatusComplete},
					{Description: "Investigate the east road", Status: StatusPending},
					{Description: "Find the missing cart", Status: StatusPending},
				},
			},
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

// View implements tea.Model and renders the quest log.
func (m Model) View() string {
	title := styles.Header.Render("📜 Quest Log")

	var sections []string
	for _, q := range m.Quests {
		questName := styles.SubHeader.Render("◆ " + q.Name)
		var objLines []string
		for _, obj := range q.Objectives {
			objLines = append(objLines, renderObjective(obj))
		}
		sections = append(sections, styles.JoinVertical(
			append([]string{questName}, objLines...)...,
		))
	}

	if len(sections) == 0 {
		sections = []string{styles.Muted.Render("  No active quests.")}
	}

	content := strings.Join(sections, "\n\n")
	return styles.Container.Width(m.width).Render(
		styles.JoinVertical(title, "", content),
	)
}

func renderObjective(obj Objective) string {
	switch obj.Status {
	case StatusComplete:
		return styles.StatusSuccess.Render("  ✓ ") + styles.Muted.Render(obj.Description)
	case StatusFailed:
		return styles.StatusError.Render("  ✗ ") + styles.Muted.Render(obj.Description)
	default:
		return styles.Muted.Render("  ○ ") + styles.Body.Render(obj.Description)
	}
}
