// Package logpanel provides a Bubbletea model that renders structured log
// entries from a [logging.RingBuffer] inside a scrollable viewport panel.
package logpanel

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/PatrickFanella/game-master/internal/logging"
	"github.com/PatrickFanella/game-master/tui/styles"
)

// tickMsg drives periodic polling of the ring buffer.
type tickMsg struct{}

// tickCmd returns a command that fires a tickMsg every 250ms.
func tickCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

// Model displays structured log entries in a scrollable viewport with
// level and service filtering.
type Model struct {
	width, height int
	buf           *logging.RingBuffer
	lastSeq       uint64
	entries       []logging.LogEntry // current filtered snapshot
	viewport      viewport.Model
	minLevel      slog.Level
	services      map[string]bool // nil = all; non-nil = selected only
	knownServices []string
	autoScroll    bool
	focused       bool
}

// New creates a log panel backed by buf. If buf is nil the panel renders a
// placeholder message instead of log entries.
func New(buf *logging.RingBuffer) Model {
	return Model{
		buf:        buf,
		minLevel:   slog.LevelDebug,
		autoScroll: true,
		viewport:   viewport.New(0, 0),
	}
}

// SetSize updates the panel dimensions and rebuilds the viewport.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h

	vpW := max(w-4, 0) // border (2) + padding (2)
	vpH := max(h-2, 0) // top + bottom border
	m.viewport.Width = vpW
	m.viewport.Height = vpH

	m.rebuildContent()
}

// SetFocused controls whether the panel renders with focused styling.
func (m *Model) SetFocused(f bool) { m.focused = f }

// Init starts the tick loop.
func (m Model) Init() tea.Cmd { return tickCmd() }

// Update handles tick, keyboard, and viewport messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		if m.buf != nil {
			if seq := m.buf.Seq(); seq != m.lastSeq {
				m.lastSeq = seq
				m.rebuildContent()
			}
		}
		return m, tickCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case "d":
			m.minLevel = slog.LevelDebug
			m.rebuildContent()
		case "i":
			m.minLevel = slog.LevelInfo
			m.rebuildContent()
		case "w":
			m.minLevel = slog.LevelWarn
			m.rebuildContent()
		case "e":
			m.minLevel = slog.LevelError
			m.rebuildContent()
		case "s":
			m.cycleService()
			m.rebuildContent()
		case "S":
			m.services = nil
			m.rebuildContent()
		case "c":
			if m.buf != nil {
				m.buf.Clear()
			}
			m.rebuildContent()
		default:
			// Forward navigation keys to viewport.
			wasAtBottom := m.viewport.AtBottom()
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			// If the user scrolled away from bottom, disable auto-scroll.
			if wasAtBottom && !m.viewport.AtBottom() {
				m.autoScroll = false
			}
			// Re-enable when they reach the bottom.
			if m.viewport.AtBottom() {
				m.autoScroll = true
			}
			return m, cmd
		}
		return m, nil
	}

	return m, nil
}

// View renders the log panel with header, viewport, and footer.
func (m Model) View() string {
	container := styles.Container
	if m.focused {
		container = styles.FocusedContainer
	}

	header := fmt.Sprintf(" Logs  [%s] ", shortLevel(m.minLevel))
	if m.services != nil {
		var names []string
		for s := range m.services {
			names = append(names, s)
		}
		slices.Sort(names)
		header += strings.Join(names, ",") + " "
	} else {
		header += "all services "
	}

	footer := " d/i/w/e: level · s/S: service · c: clear "

	content := m.viewport.View()

	return container.
		Width(m.width).
		Height(m.height).
		BorderTop(true).
		BorderBottom(true).
		BorderLeft(true).
		BorderRight(true).
		Render(
			lipgloss.JoinVertical(lipgloss.Left,
				styles.Muted.Render(header),
				content,
				styles.Muted.Render(footer),
			),
		)
}

// --- internals ---

// rebuildContent filters entries and re-renders the viewport content.
func (m *Model) rebuildContent() {
	if m.buf == nil {
		m.viewport.SetContent("No log buffer available")
		return
	}

	all := m.buf.Snapshot()
	m.entries = m.filterEntries(all)

	var b strings.Builder
	for idx, entry := range m.entries {
		if idx > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(renderEntry(entry))
	}

	m.viewport.SetContent(b.String())
	if m.autoScroll {
		m.viewport.GotoBottom()
	}
}

// filterEntries returns entries matching the current level and service filters.
// It also refreshes knownServices as a side effect.
func (m *Model) filterEntries(all []logging.LogEntry) []logging.LogEntry {
	seen := make(map[string]struct{})
	var out []logging.LogEntry

	for _, e := range all {
		seen[e.Service] = struct{}{}
		if e.Level < m.minLevel {
			continue
		}
		if m.services != nil && !m.services[e.Service] {
			continue
		}
		out = append(out, e)
	}

	m.knownServices = m.knownServices[:0]
	for s := range seen {
		m.knownServices = append(m.knownServices, s)
	}
	slices.Sort(m.knownServices)

	return out
}

// cycleService advances the service filter to the next known service, or sets
// it to the first known service when currently showing all.
func (m *Model) cycleService() {
	if len(m.knownServices) == 0 {
		return
	}

	if m.services == nil {
		// First press: select only the first known service.
		m.services = map[string]bool{m.knownServices[0]: true}
		return
	}

	// Find current and advance.
	var current string
	for s := range m.services {
		current = s
		break
	}
	idx := 0
	for i, s := range m.knownServices {
		if s == current {
			idx = i
			break
		}
	}
	next := (idx + 1) % len(m.knownServices)
	m.services = map[string]bool{m.knownServices[next]: true}
}

// renderEntry formats a single log entry as a styled line.
func renderEntry(entry logging.LogEntry) string {
	var b strings.Builder

	ts := entry.Time.Local().Format("15:04:05")
	b.WriteString(styles.LogTimestamp.Render(ts))
	b.WriteByte(' ')

	lvl := shortLevel(entry.Level)
	b.WriteString(levelStyle(entry.Level).Render(lvl))
	b.WriteByte(' ')

	svc := fmt.Sprintf("%-8s", entry.Service)
	b.WriteString(styles.LogService.Render(svc))
	b.WriteByte(' ')

	b.WriteString(styles.Body.Render(entry.Message))

	for _, f := range entry.Fields {
		b.WriteByte(' ')
		b.WriteString(styles.LogFieldKey.Render(f.Key + "="))
		b.WriteString(styles.LogFieldValue.Render(f.Value))
	}

	return b.String()
}

// shortLevel returns a 3-character level abbreviation matching the JSONL handler.
func shortLevel(level slog.Level) string {
	switch {
	case level <= slog.LevelDebug:
		return "DBG"
	case level < slog.LevelWarn:
		return "INF"
	case level < slog.LevelError:
		return "WRN"
	default:
		return "ERR"
	}
}

// levelStyle returns the lipgloss style for the given slog level.
func levelStyle(level slog.Level) lipgloss.Style {
	switch {
	case level <= slog.LevelDebug:
		return styles.LogLevelDebug
	case level < slog.LevelWarn:
		return styles.LogLevelInfo
	case level < slog.LevelError:
		return styles.LogLevelWarn
	default:
		return styles.LogLevelError
	}
}
