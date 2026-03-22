// Package narrative provides the main story / conversation view for the TUI.
// It renders the scrollable dialogue history (NPC speech, player actions, and
// system messages) using the shared styles package.
package narrative

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/PatrickFanella/game-master/tui/styles"
)

// EntryKind classifies a log entry for styling purposes.
type EntryKind int

const (
	KindSystem EntryKind = iota
	KindNPC
	KindPlayer
)

// Entry is a single line (or paragraph) in the narrative log.
type Entry struct {
	Kind    EntryKind
	Speaker string // NPC name, player handle, or "" for system messages
	Text    string
}

// Model is the Bubble Tea model for the narrative view.
type Model struct {
	width, height int
	log           []Entry
}

// New returns a freshly initialised narrative Model.
func New() Model {
	return Model{}
}

// SetSize updates the viewport dimensions so the view can word-wrap correctly.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// AddEntry appends a new entry to the dialogue log.
func (m *Model) AddEntry(e Entry) {
	m.log = append(m.log, e)
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }

// View implements tea.Model and renders the narrative log.
func (m Model) View() string {
	innerWidth := m.width - 4 // subtract border + padding
	if m.width == 0 {
		// Unknown width: use a reasonable default.
		innerWidth = 40
	} else if innerWidth < 1 {
		// Known but very small width: clamp to a minimum of 1.
		innerWidth = 1
	}

	var sb strings.Builder
	for _, e := range m.log {
		sb.WriteString(m.renderEntry(e, innerWidth))
		sb.WriteString("\n")
	}

	content := sb.String()
	if content == "" {
		content = styles.SystemMessage.Render("The adventure begins…")
	}

	title := styles.Header.Render("📖 Narrative")

	box := styles.Container.
		Width(m.width).
		Render(styles.JoinVertical(title, "", content))

	return box
}

func (m Model) renderEntry(e Entry, maxWidth int) string {
	wrapStyle := lipgloss.NewStyle().Width(maxWidth)

	switch e.Kind {
	case KindNPC:
		speaker := styles.NPCName.Render(e.Speaker + ":")
		dialogue := styles.NPCDialogue.Inherit(wrapStyle).Render(e.Text)
		return styles.JoinVertical(speaker, dialogue)
	case KindPlayer:
		prefix := styles.PlayerInputPrefix.Render()
		inputWidth := maxWidth - 2
		if inputWidth < 1 {
			inputWidth = 1
		}
		input := styles.PlayerInput.Width(inputWidth).Render(e.Text)
		return prefix + input
	default:
		return styles.SystemMessage.Inherit(wrapStyle).Render(e.Text)
	}
}
