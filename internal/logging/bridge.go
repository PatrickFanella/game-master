package logging

import (
	"context"
	"io"
	"log/slog"
	"strings"
)

// BridgeWriter adapts charmbracelet/log text output into slog records.
type BridgeWriter struct {
	logger *slog.Logger
}

// NewBridgeWriter creates a writer that forwards parsed log lines into logger.
func NewBridgeWriter(logger *slog.Logger) io.Writer {
	return &BridgeWriter{logger: logger}
}

// Write parses a single text log line, infers its level, and forwards it to slog.
func (w *BridgeWriter) Write(p []byte) (int, error) {
	line := strings.TrimSuffix(string(p), "\n")
	line = strings.TrimSuffix(line, "\r")
	if strings.TrimSpace(line) == "" || w.logger == nil {
		return len(p), nil
	}

	level, msg := parseBridgeLine(line)
	w.logger.Log(context.Background(), level, msg)
	return len(p), nil
}

func parseBridgeLine(line string) (slog.Level, string) {
	trimmed := strings.TrimSpace(line)
	upper := strings.ToUpper(trimmed)

	switch {
	case strings.HasPrefix(upper, "DEBUG"):
		return slog.LevelDebug, strings.TrimSpace(trimmed[5:])
	case strings.HasPrefix(upper, "DEBU"):
		return slog.LevelDebug, strings.TrimSpace(trimmed[4:])
	case strings.HasPrefix(upper, "INFO"):
		return slog.LevelInfo, strings.TrimSpace(trimmed[4:])
	case strings.HasPrefix(upper, "WARN"):
		return slog.LevelWarn, strings.TrimSpace(trimmed[4:])
	case strings.HasPrefix(upper, "ERROR"):
		return slog.LevelError, strings.TrimSpace(trimmed[5:])
	case strings.HasPrefix(upper, "ERRO"):
		return slog.LevelError, strings.TrimSpace(trimmed[4:])
	case strings.HasPrefix(upper, "ERR"):
		return slog.LevelError, strings.TrimSpace(trimmed[3:])
	default:
		return slog.LevelInfo, trimmed
	}
}
