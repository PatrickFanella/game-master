package logging

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestFormatColorLine_BasicEntry(t *testing.T) {
	entry := LogEntry{
		Time:    time.Date(2024, 3, 15, 14, 30, 45, 0, time.UTC),
		Level:   slog.LevelInfo,
		Service: "engine",
		Message: "player moved",
		Fields:  []Field{{Key: "room", Value: "tavern"}},
	}

	got := FormatColorLine(entry)

	for _, want := range []string{"14:30:45", "INF", "engine", "player moved", "room", "tavern"} {
		if !strings.Contains(got, want) {
			t.Errorf("FormatColorLine missing %q in %q", want, got)
		}
	}

	if !strings.HasSuffix(got, "\n") {
		t.Error("FormatColorLine output should end with newline")
	}
}

func TestFormatColorLine_EmptyService(t *testing.T) {
	entry := LogEntry{
		Time:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Level:   slog.LevelWarn,
		Service: "",
		Message: "fallback test",
	}

	got := FormatColorLine(entry)

	// Should still render with fallback service ("app").
	if !strings.Contains(got, "fallback test") {
		t.Errorf("FormatColorLine missing message in %q", got)
	}
	if !strings.Contains(got, "app") {
		t.Errorf("FormatColorLine missing fallback service in %q", got)
	}
}

func TestParseJSONLEntry_Valid(t *testing.T) {
	line := []byte(`{"time":"2024-01-01T12:00:00Z","level":"INFO","service":"engine","message":"test msg"}`)

	entry, err := ParseJSONLEntry(line)
	if err != nil {
		t.Fatalf("ParseJSONLEntry returned error: %v", err)
	}

	if entry.Level != slog.LevelInfo {
		t.Errorf("Level = %v, want %v", entry.Level, slog.LevelInfo)
	}
	if entry.Service != "engine" {
		t.Errorf("Service = %q, want %q", entry.Service, "engine")
	}
	if entry.Message != "test msg" {
		t.Errorf("Message = %q, want %q", entry.Message, "test msg")
	}
	wantTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	if !entry.Time.Equal(wantTime) {
		t.Errorf("Time = %v, want %v", entry.Time, wantTime)
	}
}

func TestParseJSONLEntry_WithFields(t *testing.T) {
	line := []byte(`{"time":"2024-01-01T12:00:00Z","level":"INFO","service":"engine","message":"test","fields":[{"key":"room","value":"tavern"},{"key":"hp","value":"42"}]}`)

	entry, err := ParseJSONLEntry(line)
	if err != nil {
		t.Fatalf("ParseJSONLEntry returned error: %v", err)
	}

	if len(entry.Fields) != 2 {
		t.Fatalf("Fields len = %d, want 2", len(entry.Fields))
	}
	if entry.Fields[0].Key != "room" || entry.Fields[0].Value != "tavern" {
		t.Errorf("Fields[0] = %+v, want {room tavern}", entry.Fields[0])
	}
	if entry.Fields[1].Key != "hp" || entry.Fields[1].Value != "42" {
		t.Errorf("Fields[1] = %+v, want {hp 42}", entry.Fields[1])
	}
}

func TestParseJSONLEntry_InvalidJSON(t *testing.T) {
	_, err := ParseJSONLEntry([]byte(`{not json`))
	if err == nil {
		t.Fatal("ParseJSONLEntry should return error for invalid JSON")
	}
}

func TestParseJSONLEntry_AllLevels(t *testing.T) {
	tests := []struct {
		levelStr string
		want     slog.Level
	}{
		{"DEBUG", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{"ERROR", slog.LevelError},
	}

	for _, tc := range tests {
		t.Run(tc.levelStr, func(t *testing.T) {
			line := []byte(`{"time":"2024-01-01T12:00:00Z","level":"` + tc.levelStr + `","service":"x","message":"m"}`)
			entry, err := ParseJSONLEntry(line)
			if err != nil {
				t.Fatalf("ParseJSONLEntry(%q) error: %v", tc.levelStr, err)
			}
			if entry.Level != tc.want {
				t.Errorf("Level = %v, want %v", entry.Level, tc.want)
			}
		})
	}
}
