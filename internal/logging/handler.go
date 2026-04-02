package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const fallbackService = "app"

type storedAttr struct {
	attr slog.Attr
}

// HandlerOptions configures a slog Handler that can fan out to the in-memory ring buffer,
// a human-readable stream, and a JSONL stream.
type HandlerOptions struct {
	MinLevel   slog.Leveler
	RingBuffer *RingBuffer
	Stderr     io.Writer
	File       io.Writer
}

// Handler adapts slog records into the logging package's LogEntry shape and optional outputs.
type Handler struct {
	opts   HandlerOptions
	groups []string
	attrs  []storedAttr
	mu     *sync.Mutex
}

// NewHandler creates a Handler with immutable options and empty attrs/groups.
func NewHandler(opts HandlerOptions) *Handler {
	return &Handler{opts: opts, mu: &sync.Mutex{}}
}

// Enabled reports whether level meets the configured minimum level.
func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.minLevel()
}

// Handle converts the record into a LogEntry and writes it to any configured destinations.
func (h *Handler) Handle(ctx context.Context, record slog.Record) error {
	if !h.Enabled(ctx, record.Level) {
		return nil
	}

	service := fallbackService
	if len(h.groups) > 0 {
		service = h.groups[len(h.groups)-1]
	}

	fields := make([]Field, 0, len(h.attrs)+record.NumAttrs())
	for _, attr := range h.attrs {
		service = appendField(&fields, nil, attr.attr, service)
	}
	record.Attrs(func(attr slog.Attr) bool {
		service = appendField(&fields, nil, attr, service)
		return true
	})

	entry := LogEntry{
		Time:    record.Time.UTC(),
		Level:   record.Level,
		Service: service,
		Message: record.Message,
		Fields:  fields,
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.opts.RingBuffer != nil {
		h.opts.RingBuffer.Write(entry)
	}
	if h.opts.Stderr != nil {
		if _, err := io.WriteString(h.opts.Stderr, formatHumanLine(entry)); err != nil {
			return fmt.Errorf("write stderr log line: %w", err)
		}
	}
	if h.opts.File != nil {
		if err := writeJSONL(h.opts.File, entry); err != nil {
			return fmt.Errorf("write jsonl log line: %w", err)
		}
	}

	return nil
}

// WithAttrs returns a new Handler with attrs appended in order.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := h.clone()
	for _, attr := range attrs {
		clone.attrs = append(clone.attrs, storedAttr{attr: attr})
	}
	return clone
}

// WithGroup returns a new Handler with group appended for future service fallback resolution.
func (h *Handler) WithGroup(name string) slog.Handler {
	clone := h.clone()
	if name != "" {
		clone.groups = append(clone.groups, name)
	}
	return clone
}

func (h *Handler) clone() *Handler {
	clone := &Handler{
		opts:   h.opts,
		groups: append([]string(nil), h.groups...),
		attrs:  append([]storedAttr(nil), h.attrs...),
		mu:     h.mu,
	}
	return clone
}

func (h *Handler) minLevel() slog.Level {
	if h.opts.MinLevel == nil {
		return slog.LevelInfo
	}
	return h.opts.MinLevel.Level()
}

func appendField(fields *[]Field, groups []string, attr slog.Attr, service string) string {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return service
	}
	if attr.Value.Kind() == slog.KindGroup {
		nextGroups := append(append([]string(nil), groups...), attr.Key)
		for _, child := range attr.Value.Group() {
			service = appendField(fields, nextGroups, child, service)
		}
		return service
	}

	key := attr.Key
	if len(groups) > 0 {
		key = strings.Join(append(append([]string(nil), groups...), attr.Key), ".")
	}
	if key == "service" {
		service = stringifyValue(attr.Value)
	}
	*fields = append(*fields, Field{Key: key, Value: stringifyValue(attr.Value)})
	return service
}

func stringifyValue(value slog.Value) string {
	value = value.Resolve()
	return fmt.Sprint(value.Any())
}

func formatHumanLine(entry LogEntry) string {
	var b strings.Builder
	b.Grow(64 + len(entry.Fields)*16)
	b.WriteString(entry.Time.UTC().Format(time.RFC3339))
	b.WriteByte(' ')
	b.WriteString(shortLevel(entry.Level))
	b.WriteByte(' ')
	fmt.Fprintf(&b, "%-8s %s", entry.Service, entry.Message)
	for _, field := range entry.Fields {
		b.WriteByte(' ')
		b.WriteString(field.Key)
		b.WriteByte('=')
		b.WriteString(field.Value)
	}
	b.WriteByte('\n')
	return b.String()
}

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

func writeJSONL(dst io.Writer, entry LogEntry) error {
	payload := struct {
		Time    time.Time `json:"time"`
		Level   string    `json:"level"`
		Service string    `json:"service"`
		Message string    `json:"message"`
		Fields  []Field   `json:"fields,omitempty"`
	}{
		Time:    entry.Time.UTC(),
		Level:   strings.ToUpper(entry.Level.String()),
		Service: entry.Service,
		Message: entry.Message,
		Fields:  entry.Fields,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := dst.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}
