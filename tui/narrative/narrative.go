// Package narrative provides the main story / conversation view for the TUI.
// It renders the scrollable dialogue history (NPC speech, player actions, and
// system messages) using the shared styles package.
package narrative

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
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

const (
	narrativeViewportWidthOffset  = 4 // border + horizontal padding
	narrativeViewportHeightOffset = 4 // border + title line + spacer line
	narrativeInputHeightOffset    = 2 // spacer line + input line
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
	viewport      viewport.Model
	input         textinput.Model
	autoScroll    bool
}

// New returns a freshly initialised narrative Model.
func New() Model {
	m := Model{autoScroll: true}
	m.viewport = viewport.New(40, 1)
	m.input = textinput.New()
	m.input.Placeholder = "What do you do?"
	m.input.Focus()
	return m
}

// SetSize updates the viewport dimensions so the view can word-wrap correctly.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.Width, m.viewport.Height = m.viewportSize()
	m.input.Width = m.viewport.Width
	m.refreshViewportContent()
	if m.autoScroll {
		m.viewport.GotoBottom()
	}
}

// AddEntry appends a new entry to the dialogue log.
func (m *Model) AddEntry(e Entry) {
	m.log = append(m.log, e)
	m.refreshViewportContent()
	if m.autoScroll {
		m.viewport.GotoBottom()
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			text := strings.TrimSpace(m.input.Value())
			if text == "" {
				return m, nil
			}
			m.AddEntry(Entry{Kind: KindPlayer, Text: text})
			m.input.Reset()
			return m, nil
		case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			m.autoScroll = m.viewport.AtBottom()
			return m, cmd
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	case tea.MouseMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		m.autoScroll = m.viewport.AtBottom()
		return m, cmd
	default:
		return m, nil
	}
}

// View implements tea.Model and renders the narrative log.
func (m Model) View() string {
	content := m.viewport.View()
	if len(m.log) == 0 {
		content = styles.SystemMessage.Render("The adventure begins…")
	}
	title := styles.Header.Render("📖 Narrative")
	input := m.input.View()

	box := styles.Container.
		Width(m.width).
		Height(m.height).
		Render(styles.JoinVertical(title, "", content, "", input))

	return box
}

func (m *Model) refreshViewportContent() {
	innerWidth, _ := m.viewportSize()

	var sb strings.Builder
	for _, e := range m.log {
		sb.WriteString(m.renderEntry(e, innerWidth))
		sb.WriteString("\n")
	}
	m.viewport.SetContent(sb.String())
}

func (m Model) viewportSize() (width, height int) {
	width = m.width - narrativeViewportWidthOffset
	if m.width == 0 {
		width = 40
	} else if width < 1 {
		width = 1
	}

	height = m.height - narrativeViewportHeightOffset - narrativeInputHeightOffset
	if m.height == 0 {
		height = 1
	} else if height < 1 {
		height = 1
	}

	return width, height
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
