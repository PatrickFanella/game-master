package logging

import (
	"io"
	"log/slog"
	"testing"
)

func TestBridgeWriterMapsLevelsAndTrimsNewline(t *testing.T) {
	t.Parallel()

	buf := NewRingBuffer(8)
	logger := slog.New(NewHandler(HandlerOptions{
		MinLevel:   slog.LevelDebug,
		RingBuffer: buf,
		Stderr:     io.Discard,
		File:       io.Discard,
	}))
	bridge := NewBridgeWriter(logger)

	lines := []string{
		"DEBU debug line\n",
		"INFO info line\n",
		"WARN warn line\n",
		"ERRO error line\n",
		"\n",
	}
	for _, line := range lines {
		if n, err := bridge.Write([]byte(line)); err != nil {
			t.Fatalf("write %q: %v", line, err)
		} else if n != len(line) {
			t.Fatalf("write len for %q: got %d, want %d", line, n, len(line))
		}
	}

	entries := buf.Snapshot()
	if len(entries) != 4 {
		t.Fatalf("snapshot len: got %d, want 4", len(entries))
	}

	want := []struct {
		level slog.Level
		msg   string
	}{
		{level: slog.LevelDebug, msg: "debug line"},
		{level: slog.LevelInfo, msg: "info line"},
		{level: slog.LevelWarn, msg: "warn line"},
		{level: slog.LevelError, msg: "error line"},
	}
	for i := range want {
		if entries[i].Level != want[i].level {
			t.Fatalf("entry[%d] level: got %v, want %v", i, entries[i].Level, want[i].level)
		}
		if entries[i].Message != want[i].msg {
			t.Fatalf("entry[%d] message: got %q, want %q", i, entries[i].Message, want[i].msg)
		}
	}
}
