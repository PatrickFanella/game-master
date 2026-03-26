// Package narrative provides the main story / conversation view for the TUI.
// It renders the scrollable dialogue history (NPC speech, player actions, and
// system messages) using the shared styles package.
package narrative

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/PatrickFanella/game-master/internal/engine"
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
	defaultChoiceListWidth        = 40
	maxVisibleChoices             = 3
)

// Entry is a single line (or paragraph) in the narrative log.
type Entry struct {
	Kind    EntryKind
	Speaker string // NPC name, player handle, or "" for system messages
	Text    string
}

// SubmitMsg is emitted when the player submits typed input or selects a choice.
type SubmitMsg struct {
	Input    string
	ChoiceID string
}

type choiceItem struct {
	choice engine.Choice
}

func (i choiceItem) Title() string       { return i.choice.Text }
func (i choiceItem) Description() string { return "" }
func (i choiceItem) FilterValue() string { return i.choice.Text }

// Model is the Bubble Tea model for the narrative view.
type Model struct {
	width, height int
	log           []Entry
	viewport      viewport.Model
	input         textinput.Model
	choices       list.Model
	spinner       spinner.Model
	autoScroll    bool
	loading       bool
}

// New returns a freshly initialised narrative Model.
func New() Model {
	m := Model{autoScroll: true}
	m.viewport = viewport.New(40, 1)

	m.input = textinput.New()
	m.input.Placeholder = "What do you do?"
	m.input.Focus()

	m.choices = list.New(nil, newChoiceDelegate(), defaultChoiceListWidth, 0)
	m.choices.SetShowTitle(false)
	m.choices.SetShowStatusBar(false)
	m.choices.SetShowHelp(false)
	m.choices.SetFilteringEnabled(false)

	m.spinner = spinner.New()
	m.spinner.Spinner = spinner.Dot
	m.spinner.Style = lipgloss.NewStyle().Foreground(styles.ColorAccent)
	return m
}

// SetSize updates the viewport dimensions so the view can word-wrap correctly.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.refreshLayout()
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

// BeginStreamingNPCEntry appends a new NPC / GM entry ready for incremental text.
func (m *Model) BeginStreamingNPCEntry() {
	m.AddEntry(Entry{Kind: KindNPC, Speaker: "Game Master"})
}

// AppendToLastEntry appends text to the most recent narrative entry.
func (m *Model) AppendToLastEntry(text string) {
	if len(m.log) == 0 {
		m.AddEntry(Entry{Kind: KindSystem, Text: text})
		return
	}
	m.log[len(m.log)-1].Text += text
	m.refreshViewportContent()
	if m.autoScroll {
		m.viewport.GotoBottom()
	}
}

// SetChoices replaces the suggested choice list.
func (m *Model) SetChoices(choices []engine.Choice) {
	items := make([]list.Item, 0, len(choices))
	for _, c := range choices {
		items = append(items, choiceItem{choice: c})
	}
	m.choices.SetItems(items)
	m.choices.Select(0)
	m.refreshLayout()
}

// ClearChoices removes any suggested choices.
func (m *Model) ClearChoices() {
	m.SetChoices(nil)
}

// SetLoading toggles the loading indicator.
func (m *Model) SetLoading(loading bool) tea.Cmd {
	m.loading = loading
	m.refreshLayout()
	if loading {
		return m.spinner.Tick
	}
	return nil
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		if m.loading {
			switch msg.Type {
			case tea.KeyPgUp, tea.KeyPgDown:
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				m.autoScroll = m.viewport.AtBottom()
				return m, cmd
			default:
				return m, nil
			}
		}

		switch msg.Type {
		case tea.KeyEnter:
			text := strings.TrimSpace(m.input.Value())
			if text != "" {
				m.input.Reset()
				return m, func() tea.Msg {
					return SubmitMsg{Input: text}
				}
			}
			if item, ok := m.choices.SelectedItem().(choiceItem); ok {
				return m, func() tea.Msg {
					return SubmitMsg{Input: item.choice.Text, ChoiceID: item.choice.ID}
				}
			}
			return m, nil
		case tea.KeyUp, tea.KeyDown:
			if m.hasChoices() && strings.TrimSpace(m.input.Value()) == "" {
				var cmd tea.Cmd
				m.choices, cmd = m.choices.Update(msg)
				return m, cmd
			}
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			m.autoScroll = m.viewport.AtBottom()
			return m, cmd
		case tea.KeyPgUp, tea.KeyPgDown:
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

	blocks := []string{title, "", content}
	if m.loading {
		blocks = append(blocks, "", m.spinner.View()+"  "+styles.Body.Render("Thinking…"))
	}
	if m.hasChoices() {
		blocks = append(blocks, "", styles.SubHeader.Render("Suggested choices"), m.choices.View())
	}
	blocks = append(blocks, "", m.input.View())

	box := styles.Container.
		Width(m.width).
		Height(m.height).
		Render(styles.JoinVertical(blocks...))

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

func (m *Model) refreshLayout() {
	m.viewport.Width, m.viewport.Height = m.viewportSize()
	m.input.Width = m.viewport.Width
	m.choices.SetSize(m.viewport.Width, m.visibleChoiceCount())
}

func (m Model) viewportSize() (width, height int) {
	width = m.width - narrativeViewportWidthOffset
	if m.width == 0 {
		width = 40
	} else if width < 1 {
		width = 1
	}

	height = m.height - narrativeViewportHeightOffset - narrativeInputHeightOffset - m.loadingHeightOffset() - m.choiceHeightOffset()
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

func (m Model) hasChoices() bool {
	return len(m.choices.Items()) > 0
}

func (m Model) visibleChoiceCount() int {
	count := len(m.choices.Items())
	if count > maxVisibleChoices {
		return maxVisibleChoices
	}
	return count
}

func (m Model) loadingHeightOffset() int {
	if m.loading {
		return 2
	}
	return 0
}

func (m Model) choiceHeightOffset() int {
	if !m.hasChoices() {
		return 0
	}
	return 2 + m.visibleChoiceCount()
}

func newChoiceDelegate() list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(styles.ColorAccent).
		BorderForeground(styles.ColorAccent)
	return delegate
}
