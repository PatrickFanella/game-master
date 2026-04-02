package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetup_CreatesDirectory(t *testing.T) {
	original := slog.Default()
	t.Cleanup(func() { slog.SetDefault(original) })

	dir := filepath.Join(t.TempDir(), "sub", "deep")
	logPath := filepath.Join(dir, "game.jsonl")

	result, err := Setup(logPath, slog.LevelDebug)
	if err != nil {
		t.Fatalf("Setup error: %v", err)
	}
	t.Cleanup(result.Cleanup)

	if result.RingBuffer == nil {
		t.Fatal("RingBuffer is nil")
	}

	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("JSONL file not created: %v", err)
	}
}

func TestSetup_EmptyPath(t *testing.T) {
	_, err := Setup("", slog.LevelInfo)
	if err == nil {
		t.Fatal("Setup with empty path should return error")
	}
}

func TestSetup_WritesJSONL(t *testing.T) {
	original := slog.Default()
	t.Cleanup(func() { slog.SetDefault(original) })

	logPath := filepath.Join(t.TempDir(), "game.jsonl")

	result, err := Setup(logPath, slog.LevelDebug)
	if err != nil {
		t.Fatalf("Setup error: %v", err)
	}
	t.Cleanup(result.Cleanup)

	slog.Info("hello from test", "key1", "val1")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "hello from test") {
		t.Errorf("JSONL file missing logged message, got: %s", content)
	}
}

func TestSetup_Cleanup(t *testing.T) {
	original := slog.Default()
	t.Cleanup(func() { slog.SetDefault(original) })

	logPath := filepath.Join(t.TempDir(), "game.jsonl")

	result, err := Setup(logPath, slog.LevelInfo)
	if err != nil {
		t.Fatalf("Setup error: %v", err)
	}

	// Should not panic.
	result.Cleanup()
}
