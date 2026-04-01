// Package quest provides the quest log view for the TUI.
// It renders active and completed quests with expandable objectives using
// domain types and data delivered via UpdateMsg.
package quest

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/tui/styles"
)

const (
	containerHPad = 4
	containerVPad = 2
)

// Status group display order.
var statusOrder = []domain.QuestStatus{
	domain.QuestStatusActive,
	domain.QuestStatusCompleted,
	domain.QuestStatusFailed,
	domain.QuestStatusAbandoned,
}

// statusLabels maps quest status to display header text.
var statusLabels = map[domain.QuestStatus]string{
	domain.QuestStatusActive:    "Active",
	domain.QuestStatusCompleted: "Completed",
	domain.QuestStatusFailed:    "Failed",
	domain.QuestStatusAbandoned: "Abandoned",
}

// NavigateBackMsg is emitted when the user presses Escape to return to the
// narrative view.
type NavigateBackMsg struct{}

// UpdateMsg delivers fresh quest data to the view.
type UpdateMsg struct {
	Quests     []domain.Quest
	Objectives map[uuid.UUID][]domain.QuestObjective
}

// Model is the Bubble Tea model for the quest log view.
type Model struct {
	width, height int
	quests        []domain.Quest
	objectives    map[uuid.UUID][]domain.QuestObjective
	cursor        int
	expanded      map[uuid.UUID]bool
	loaded        bool
	viewport      viewport.Model
}

// New returns a freshly initialised quest Model.
func New() Model {
	vp := viewport.New(40, 1)
	return Model{
		viewport: vp,
		expanded: make(map[uuid.UUID]bool),
	}
}

// SetSize updates the viewport dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	vpWidth := width - containerHPad
	vpHeight := height - containerVPad - 2
	if vpWidth < 1 {
		vpWidth = 1
	}
	if vpHeight < 1 {
		vpHeight = 1
	}
	m.viewport.Width = vpWidth
	m.viewport.Height = vpHeight
	if m.loaded {
		m.refreshContent()
	}
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case UpdateMsg:
		m.quests = msg.Quests
		m.objectives = msg.Objectives
		if m.objectives == nil {
			m.objectives = make(map[uuid.UUID][]domain.QuestObjective)
		}
		m.loaded = true

		// Default-expand active quests.
		m.expanded = make(map[uuid.UUID]bool)
		for _, q := range m.quests {
			if q.Status == domain.QuestStatusActive {
				m.expanded[q.ID] = true
			}
		}

		ordered := m.orderedQuests()
		if m.cursor >= len(ordered) {
			m.cursor = max(0, len(ordered)-1)
		}
		m.refreshContent()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return NavigateBackMsg{} }
		case "j", "down":
			ordered := m.orderedQuests()
			if len(ordered) > 0 {
				m.cursor = (m.cursor + 1) % len(ordered)
				m.refreshContent()
			}
			return m, nil
		case "k", "up":
			ordered := m.orderedQuests()
			if len(ordered) > 0 {
				m.cursor = (m.cursor - 1 + len(ordered)) % len(ordered)
				m.refreshContent()
			}
			return m, nil
		case "enter":
			ordered := m.orderedQuests()
			if m.cursor >= 0 && m.cursor < len(ordered) {
				qid := ordered[m.cursor].ID
				m.expanded[qid] = !m.expanded[qid]
				m.refreshContent()
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View implements tea.Model and renders the quest log.
func (m Model) View() string {
	title := styles.Header.Render("📜 Quest Log")

	if !m.loaded || len(m.quests) == 0 {
		msg := "No quests yet. Your adventure awaits."
		if !m.loaded {
			msg = "No quest data available."
		}
		placeholder := styles.SystemMessage.Render(msg)
		hint := styles.Muted.Render("Press Esc to return to the narrative view.")
		content := styles.JoinVertical(placeholder, "", hint)
		return styles.Container.Width(m.width).Render(
			styles.JoinVertical(title, "", content),
		)
	}

	hint := styles.Muted.Render("↑/↓ navigate · Enter expand · Esc return")
	return styles.Container.Width(m.width).Render(
		styles.JoinVertical(title, "", m.viewport.View(), "", hint),
	)
}

// refreshContent rebuilds the viewport content.
func (m *Model) refreshContent() {
	ordered := m.orderedQuests()
	grouped := m.groupByStatus()

	var sections []string
	idx := 0

	for _, status := range statusOrder {
		quests := grouped[status]
		if len(quests) == 0 {
			continue
		}

		label, ok := statusLabels[status]
		if !ok {
			label = string(status)
		}
		sections = append(sections, styles.SubHeader.Render("◆ "+label))

		for _, q := range quests {
			_ = ordered // cursor indexes into the full ordered list

			cursor := "  "
			if idx == m.cursor {
				cursor = "> "
			}

			expand := "▸ "
			if m.expanded[q.ID] {
				expand = "▾ "
			}

			badge := questTypeBadge(q.QuestType)
			line := fmt.Sprintf("%s%s%s %s", cursor, expand, q.Title, badge)
			sections = append(sections, line)

			if m.expanded[q.ID] {
				objs := m.sortedObjectives(q.ID)
				for _, obj := range objs {
					sections = append(sections, renderObjective(obj))
				}
			}
			idx++
		}
		sections = append(sections, "")
	}

	m.viewport.SetContent(strings.Join(sections, "\n"))
}

// orderedQuests returns quests in display order: active → completed → failed → abandoned.
func (m *Model) orderedQuests() []domain.Quest {
	grouped := m.groupByStatus()
	var result []domain.Quest
	for _, status := range statusOrder {
		result = append(result, grouped[status]...)
	}
	return result
}

// groupByStatus groups quests by their status.
func (m *Model) groupByStatus() map[domain.QuestStatus][]domain.Quest {
	grouped := make(map[domain.QuestStatus][]domain.Quest)
	for _, q := range m.quests {
		grouped[q.Status] = append(grouped[q.Status], q)
	}
	return grouped
}

// sortedObjectives returns objectives for a quest sorted by OrderIndex.
func (m *Model) sortedObjectives(questID uuid.UUID) []domain.QuestObjective {
	objs := make([]domain.QuestObjective, len(m.objectives[questID]))
	copy(objs, m.objectives[questID])
	sort.Slice(objs, func(i, j int) bool {
		return objs[i].OrderIndex < objs[j].OrderIndex
	})
	return objs
}

func renderObjective(obj domain.QuestObjective) string {
	if obj.Completed {
		return styles.StatusSuccess.Render("  ✓ ") + styles.Muted.Render(obj.Description)
	}
	return styles.Muted.Render("  ○ ") + styles.Body.Render(obj.Description)
}

// questTypeBadge returns a colored type indicator.
func questTypeBadge(t domain.QuestType) string {
	var label string
	var color lipgloss.AdaptiveColor
	switch t {
	case domain.QuestTypeShortTerm:
		label, color = "[ST]", styles.ColorSuccess
	case domain.QuestTypeMediumTerm:
		label, color = "[MT]", styles.ColorWarning
	case domain.QuestTypeLongTerm:
		label, color = "[LT]", styles.ColorAccent
	default:
		label, color = "[??]", styles.ColorMuted
	}
	return lipgloss.NewStyle().Foreground(color).Render(label)
}
