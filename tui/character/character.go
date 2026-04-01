// Package character provides the character sheet view for the TUI.
// It renders player stats, HP/XP bars, abilities, and status using domain data.
package character

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/tui/styles"
)

const (
	barWidth       = 20
	xpPerLevel     = 1000
	containerHPad  = 4 // border (2) + padding (2)
	containerVPad  = 2 // header line + spacer
	filledBlock    = "█"
	emptyBlock     = "░"
)

// NavigateBackMsg is emitted when the user presses Escape to return to the
// narrative view.
type NavigateBackMsg struct{}

// UpdateMsg delivers fresh player data to the character view.
type UpdateMsg struct {
	Player domain.PlayerCharacter
}

// Model is the Bubble Tea model for the character sheet view.
type Model struct {
	width, height int
	player        domain.PlayerCharacter
	loaded        bool
	viewport      viewport.Model
}

// New returns a freshly initialised character Model.
func New() Model {
	vp := viewport.New(40, 1)
	return Model{viewport: vp}
}

// SetSize updates the viewport dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	vpWidth := width - containerHPad
	vpHeight := height - containerVPad - 2 // extra for title + hint
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
		m.player = msg.Player
		m.loaded = true
		m.refreshContent()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return NavigateBackMsg{} }
		case "j", "down":
			m.viewport.LineDown(1)
			return m, nil
		case "k", "up":
			m.viewport.LineUp(1)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View implements tea.Model and renders the character sheet.
func (m Model) View() string {
	title := styles.Header.Render("⚔️  Character Sheet")

	if !m.loaded {
		placeholder := styles.SystemMessage.Render("No character data available.")
		hint := styles.Muted.Render("Press Esc to return to the narrative view.")
		content := styles.JoinVertical(placeholder, "", hint)
		return styles.Container.Width(m.width).Render(
			styles.JoinVertical(title, "", content),
		)
	}

	hint := styles.Muted.Render("↑/↓ scroll · Esc return")
	return styles.Container.Width(m.width).Render(
		styles.JoinVertical(title, "", m.viewport.View(), "", hint),
	)
}

// refreshContent rebuilds the viewport content from current player data.
func (m *Model) refreshContent() {
	var sections []string

	// Name + Level
	nameLevel := styles.SubHeader.Render(fmt.Sprintf("%s    Level %d", m.player.Name, m.player.Level))
	sections = append(sections, nameLevel)
	sections = append(sections, styles.Muted.Render("━━━━━━━━━━━━━━━━━━━━━━━━━━"))

	// HP bar
	sections = append(sections, renderBar("HP", m.player.HP, m.player.MaxHP))

	// XP bar
	xpNeeded := m.player.Level * xpPerLevel
	if xpNeeded < 1 {
		xpNeeded = xpPerLevel
	}
	sections = append(sections, renderBar("XP", m.player.Experience, xpNeeded))

	// Status
	statusStyle := styles.StatusSuccess
	switch {
	case m.player.Status == "injured" || m.player.Status == "wounded":
		statusStyle = styles.StatusWarning
	case m.player.Status != "healthy" && m.player.Status != "alive" && m.player.Status != "":
		statusStyle = styles.StatusError
	}
	if m.player.Status != "" {
		sections = append(sections, fmt.Sprintf("Status: %s", statusStyle.Render(m.player.Status)))
	}

	sections = append(sections, "")

	// Stats
	sections = append(sections, m.renderStats()...)

	sections = append(sections, "")

	// Abilities
	sections = append(sections, m.renderAbilities()...)

	m.viewport.SetContent(strings.Join(sections, "\n"))
}

func (m *Model) renderStats() []string {
	header := styles.SubHeader.Render("◆ Stats")
	lines := []string{header}

	if len(m.player.Stats) == 0 {
		lines = append(lines, styles.Muted.Render("  No stats available."))
		return lines
	}

	var parsed map[string]any
	if err := json.Unmarshal(m.player.Stats, &parsed); err != nil {
		lines = append(lines, styles.Muted.Render("  Stats unavailable."))
		return lines
	}

	// Sort keys for deterministic output.
	keys := make([]string, 0, len(parsed))
	for k := range parsed {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Find longest key for alignment.
	maxLen := 0
	for _, k := range keys {
		if len(k) > maxLen {
			maxLen = len(k)
		}
	}

	for _, k := range keys {
		label := strings.Title(k) //nolint:staticcheck // simple capitalisation
		padded := fmt.Sprintf("%-*s", maxLen+2, label+":")
		lines = append(lines, fmt.Sprintf("  %s %v", styles.Body.Render(padded), parsed[k]))
	}
	return lines
}

func (m *Model) renderAbilities() []string {
	header := styles.SubHeader.Render("◆ Abilities")
	lines := []string{header}

	if len(m.player.Abilities) == 0 {
		lines = append(lines, styles.Muted.Render("  No abilities available."))
		return lines
	}

	var parsed []any
	if err := json.Unmarshal(m.player.Abilities, &parsed); err != nil {
		lines = append(lines, styles.Muted.Render("  Abilities unavailable."))
		return lines
	}

	for _, raw := range parsed {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _ := entry["name"].(string)
		if name == "" {
			continue
		}
		parts := []string{name}
		if typ, ok := entry["type"].(string); ok && typ != "" {
			parts = append(parts, typ)
		}
		if cd, ok := entry["cooldown"].(string); ok && cd != "" {
			parts = append(parts, cd)
		}
		detail := strings.Join(parts[1:], ", ")
		if detail != "" {
			lines = append(lines, fmt.Sprintf("  • %s (%s)", name, detail))
		} else {
			lines = append(lines, fmt.Sprintf("  • %s", name))
		}
	}

	if len(lines) == 1 {
		lines = append(lines, styles.Muted.Render("  No abilities available."))
	}
	return lines
}

// renderBar produces a text progress bar like "HP: ████████░░░░  18/25".
func renderBar(label string, current, max int) string {
	if max <= 0 {
		return fmt.Sprintf("%s: %s  %d/%d", label, styles.Muted.Render(strings.Repeat(emptyBlock, barWidth)), current, max)
	}

	ratio := float64(current) / float64(max)
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	filled := int(ratio * float64(barWidth))
	empty := barWidth - filled

	// Color based on percentage.
	var barColor lipgloss.AdaptiveColor
	switch {
	case ratio > 0.5:
		barColor = styles.ColorSuccess
	case ratio > 0.25:
		barColor = styles.ColorWarning
	default:
		barColor = styles.ColorError
	}

	filledStyle := lipgloss.NewStyle().Foreground(barColor)
	filledStr := filledStyle.Render(strings.Repeat(filledBlock, filled))
	emptyStr := styles.Muted.Render(strings.Repeat(emptyBlock, empty))

	return fmt.Sprintf("%s: %s%s  %d/%d", label, filledStr, emptyStr, current, max)
}
