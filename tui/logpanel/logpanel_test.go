package logpanel

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PatrickFanella/game-master/internal/logging"
)

// Compile-time check: *Model must satisfy tea.Model.
var _ tea.Model = (*Model)(nil)

func writeEntry(buf *logging.RingBuffer, level slog.Level, service, msg string) {
	buf.Write(logging.LogEntry{
		Time:    time.Now(),
		Level:   level,
		Service: service,
		Message: msg,
	})
}

func TestNew_NilBuffer(t *testing.T) {
	m := New(nil)
	m.SetSize(80, 24)
	v := m.View()
	if !strings.Contains(v, "No log buffer available") {
		t.Fatalf("expected nil-buffer placeholder in view, got:\n%s", v)
	}
}

func TestSetSize(t *testing.T) {
	buf := logging.NewRingBuffer(16)
	m := New(buf)
	m.SetSize(80, 24)
	if m.width != 80 {
		t.Fatalf("expected width 80, got %d", m.width)
	}
	if m.height != 24 {
		t.Fatalf("expected height 24, got %d", m.height)
	}
}

func TestLevelFilter(t *testing.T) {
	buf := logging.NewRingBuffer(64)
	writeEntry(buf, slog.LevelDebug, "engine", "debug msg")
	writeEntry(buf, slog.LevelInfo, "engine", "info msg")
	writeEntry(buf, slog.LevelWarn, "engine", "warn msg")
	writeEntry(buf, slog.LevelError, "engine", "error msg")

	mp := New(buf)
	m := &mp
	m.SetSize(120, 40)

	// Default level is Debug — all 4 entries visible.
	v := m.View()
	for _, msg := range []string{"debug msg", "info msg", "warn msg", "error msg"} {
		if !strings.Contains(v, msg) {
			t.Fatalf("expected %q visible at Debug level, view:\n%s", msg, v)
		}
	}

	// Press 'w' to set min level to Warn.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	m = updated.(*Model)

	v = m.View()
	if strings.Contains(v, "debug msg") {
		t.Fatal("debug msg should be hidden at Warn level")
	}
	if strings.Contains(v, "info msg") {
		t.Fatal("info msg should be hidden at Warn level")
	}
	if !strings.Contains(v, "warn msg") {
		t.Fatal("warn msg should be visible at Warn level")
	}
	if !strings.Contains(v, "error msg") {
		t.Fatal("error msg should be visible at Warn level")
	}
}

func TestServiceFilter(t *testing.T) {
	buf := logging.NewRingBuffer(64)
	writeEntry(buf, slog.LevelInfo, "engine", "engine msg")
	writeEntry(buf, slog.LevelInfo, "memory", "memory msg")
	writeEntry(buf, slog.LevelInfo, "tools", "tools msg")

	mp := New(buf)
	m := &mp
	m.SetSize(120, 40)

	// All services visible initially.
	v := m.View()
	for _, msg := range []string{"engine msg", "memory msg", "tools msg"} {
		if !strings.Contains(v, msg) {
			t.Fatalf("expected %q visible with no service filter, view:\n%s", msg, v)
		}
	}

	// Press 's' to cycle to first service (alphabetically: "engine").
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(*Model)

	v = m.View()
	if !strings.Contains(v, "engine msg") {
		t.Fatal("expected engine msg visible after first service cycle")
	}
	if strings.Contains(v, "memory msg") {
		t.Fatal("expected memory msg hidden after first service cycle")
	}
	if strings.Contains(v, "tools msg") {
		t.Fatal("expected tools msg hidden after first service cycle")
	}

	// Press 's' again to cycle to next service ("memory").
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(*Model)

	v = m.View()
	if strings.Contains(v, "engine msg") {
		t.Fatal("expected engine msg hidden after second service cycle")
	}
	if !strings.Contains(v, "memory msg") {
		t.Fatal("expected memory msg visible after second service cycle")
	}
}

func TestClear(t *testing.T) {
	buf := logging.NewRingBuffer(16)
	writeEntry(buf, slog.LevelInfo, "engine", "hello")
	seqBefore := buf.Seq()

	mp := New(buf)
	m := &mp
	m.SetSize(80, 24)

	// Press 'c' to clear.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	_ = updated.(*Model)

	snap := buf.Snapshot()
	if len(snap) != 0 {
		t.Fatalf("expected empty snapshot after clear, got %d entries", len(snap))
	}
	if buf.Seq() <= seqBefore {
		t.Fatal("expected Seq to increase after Clear")
	}
}

func TestAutoScroll(t *testing.T) {
	m := New(nil)
	if !m.autoScroll {
		t.Fatal("expected autoScroll true by default")
	}
}
