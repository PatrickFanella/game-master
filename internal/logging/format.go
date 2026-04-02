package logging

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Adaptive colors for colored terminal log output.
var (
	colorMuted  = lipgloss.AdaptiveColor{Dark: "#6b7280", Light: "#868e96"}
	colorBlue   = lipgloss.AdaptiveColor{Dark: "#60a5fa", Light: "#2563eb"}
	colorAmber  = lipgloss.AdaptiveColor{Dark: "#facc15", Light: "#d97706"}
	colorRed    = lipgloss.AdaptiveColor{Dark: "#f87171", Light: "#dc2626"}
	colorPurple = lipgloss.AdaptiveColor{Dark: "#7c3aed", Light: "#a78bfa"}
)

// Precomputed styles — lipgloss styles are value types, safe to reuse.
var (
	timestampStyle = lipgloss.NewStyle().Foreground(colorMuted)
	serviceStyle   = lipgloss.NewStyle().Foreground(colorPurple)
	fieldKeyStyle  = lipgloss.NewStyle().Foreground(colorMuted)

	levelStyles = map[string]lipgloss.Style{
		"DBG": lipgloss.NewStyle().Bold(true).Foreground(colorMuted),
		"INF": lipgloss.NewStyle().Bold(true).Foreground(colorBlue),
		"WRN": lipgloss.NewStyle().Bold(true).Foreground(colorAmber),
		"ERR": lipgloss.NewStyle().Bold(true).Foreground(colorRed),
	}
)

// FormatColorLine renders a LogEntry as a styled terminal line with lipgloss colors.
func FormatColorLine(entry LogEntry) string {
	var b strings.Builder
	b.Grow(80 + len(entry.Fields)*20)

	b.WriteString(timestampStyle.Render(entry.Time.Format("15:04:05")))
	b.WriteByte(' ')

	badge := shortLevel(entry.Level)
	if style, ok := levelStyles[badge]; ok {
		b.WriteString(style.Render(badge))
	} else {
		b.WriteString(badge)
	}
	b.WriteByte(' ')

	svc := entry.Service
	if svc == "" {
		svc = fallbackService
	}
	b.WriteString(serviceStyle.Render(fmt.Sprintf("%-8s", svc)))
	b.WriteByte(' ')

	b.WriteString(entry.Message)

	for _, f := range entry.Fields {
		b.WriteByte(' ')
		b.WriteString(fieldKeyStyle.Render(f.Key))
		b.WriteByte('=')
		b.WriteString(f.Value)
	}

	b.WriteByte('\n')
	return b.String()
}

// jsonlEntry mirrors the JSONL shape produced by writeJSONL.
type jsonlEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Service string    `json:"service"`
	Message string    `json:"message"`
	Fields  []Field   `json:"fields,omitempty"`
}

// ParseJSONLEntry deserializes a single JSONL line (as produced by writeJSONL)
// back into a LogEntry.
func ParseJSONLEntry(line []byte) (LogEntry, error) {
	var raw jsonlEntry
	if err := json.Unmarshal(line, &raw); err != nil {
		return LogEntry{}, fmt.Errorf("logging: parse jsonl entry: %w", err)
	}

	level, err := parseLevelString(raw.Level)
	if err != nil {
		return LogEntry{}, fmt.Errorf("logging: parse level %q: %w", raw.Level, err)
	}

	return LogEntry{
		Time:    raw.Time,
		Level:   level,
		Service: raw.Service,
		Message: raw.Message,
		Fields:  raw.Fields,
	}, nil
}

// parseLevelString handles standard names (DEBUG, INFO, WARN, ERROR) and slog's
// offset format (e.g. "INFO+4", "DEBUG-2").
func parseLevelString(s string) (slog.Level, error) {
	s = strings.TrimSpace(strings.ToUpper(s))

	// Try the built-in slog parser first — it handles "DEBUG", "INFO", "WARN",
	// "ERROR" and their +N/-N variants.
	var level slog.Level
	if err := level.UnmarshalText([]byte(s)); err == nil {
		return level, nil
	}

	// Fallback for bare numeric levels.
	if n, err := strconv.Atoi(s); err == nil {
		return slog.Level(n), nil
	}

	return 0, fmt.Errorf("unrecognized level %q", s)
}
