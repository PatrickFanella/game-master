package logging

import (
	"log/slog"
	"time"
)

// Field is a structured logging key/value pair prepared for display and JSONL output.
type Field struct {
	Key   string
	Value string
}

// LogEntry is a single structured log event captured for UI display and streaming.
type LogEntry struct {
	Time    time.Time
	Level   slog.Level
	Service string
	Message string
	Fields  []Field
}
