package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

func TestHandlerEnabledRespectsMinLevel(t *testing.T) {
	t.Parallel()

	h := NewHandler(HandlerOptions{MinLevel: slog.LevelWarn})

	if h.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("info level unexpectedly enabled")
	}
	if !h.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("error level unexpectedly disabled")
	}
}

func TestHandlerServicePrecedence(t *testing.T) {
	t.Parallel()

	timestamp := time.Unix(1_700_000_000, 0).UTC()
	cases := []struct {
		name    string
		handler slog.Handler
		record  []slog.Attr
		want    string
	}{
		{
			name:    "record service overrides group",
			handler: NewHandler(HandlerOptions{RingBuffer: NewRingBuffer(1)}).WithGroup("engine"),
			record:  []slog.Attr{slog.String("service", "record")},
			want:    "record",
		},
		{
			name:    "group overrides fallback",
			handler: NewHandler(HandlerOptions{RingBuffer: NewRingBuffer(1)}).WithGroup("engine"),
			want:    "engine",
		},
		{
			name:    "fallback app used when unset",
			handler: NewHandler(HandlerOptions{RingBuffer: NewRingBuffer(1)}),
			want:    "app",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := slog.NewRecord(timestamp, slog.LevelInfo, "hello", 0)
			rec.AddAttrs(tc.record...)

			if err := tc.handler.Handle(context.Background(), rec); err != nil {
				t.Fatalf("handle: %v", err)
			}

			buf := tc.handler.(*Handler).opts.RingBuffer
			entries := buf.Snapshot()
			if len(entries) != 1 {
				t.Fatalf("snapshot len: got %d, want 1", len(entries))
			}
			if got := entries[0].Service; got != tc.want {
				t.Fatalf("service: got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHandlerWritesRingBufferOrderedAttrs(t *testing.T) {
	t.Parallel()

	buf := NewRingBuffer(1)
	h := NewHandler(HandlerOptions{RingBuffer: buf}).
		WithAttrs([]slog.Attr{slog.String("first", "one")}).
		WithAttrs([]slog.Attr{slog.Int("count", 2)})

	rec := slog.NewRecord(time.Unix(1_700_000_100, 0).UTC(), slog.LevelInfo, "hello", 0)
	rec.AddAttrs(slog.Bool("ok", true))

	if err := h.Handle(context.Background(), rec); err != nil {
		t.Fatalf("handle: %v", err)
	}

	entries := buf.Snapshot()
	if len(entries) != 1 {
		t.Fatalf("snapshot len: got %d, want 1", len(entries))
	}
	got := entries[0].Fields
	want := []Field{{Key: "first", Value: "one"}, {Key: "count", Value: "2"}, {Key: "ok", Value: "true"}}
	if len(got) != len(want) {
		t.Fatalf("fields len: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("field[%d]: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestHandlerFormatsStderrLine(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	h := NewHandler(HandlerOptions{Stderr: &stderr}).WithGroup("engine")

	rec := slog.NewRecord(time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC), slog.LevelInfo, "hello", 0)
	rec.AddAttrs(slog.String("foo", "bar"), slog.Int("n", 7))

	if err := h.Handle(context.Background(), rec); err != nil {
		t.Fatalf("handle: %v", err)
	}

	const want = "2024-01-02T03:04:05Z INF engine   hello foo=bar n=7\n"
	if got := stderr.String(); got != want {
		t.Fatalf("stderr:\n got %q\nwant %q", got, want)
	}
}

func TestHandlerWritesJSONL(t *testing.T) {
	t.Parallel()

	var file bytes.Buffer
	h := NewHandler(HandlerOptions{File: &file}).WithGroup("engine")

	rec := slog.NewRecord(time.Date(2024, time.January, 2, 3, 4, 5, 6, time.UTC), slog.LevelError, "boom", 0)
	rec.AddAttrs(slog.String("foo", "bar"), slog.Int("n", 7))

	if err := h.Handle(context.Background(), rec); err != nil {
		t.Fatalf("handle: %v", err)
	}

	var got struct {
		Time    time.Time `json:"time"`
		Level   string    `json:"level"`
		Service string    `json:"service"`
		Message string    `json:"message"`
		Fields  []Field   `json:"fields"`
	}
	if err := json.NewDecoder(&file).Decode(&got); err != nil {
		t.Fatalf("decode jsonl: %v", err)
	}

	if !got.Time.Equal(rec.Time) {
		t.Fatalf("time: got %s, want %s", got.Time, rec.Time)
	}
	if got.Level != "ERROR" {
		t.Fatalf("level: got %q, want %q", got.Level, "ERROR")
	}
	if got.Service != "engine" {
		t.Fatalf("service: got %q, want %q", got.Service, "engine")
	}
	if got.Message != "boom" {
		t.Fatalf("message: got %q, want %q", got.Message, "boom")
	}
	wantFields := []Field{{Key: "foo", Value: "bar"}, {Key: "n", Value: "7"}}
	if len(got.Fields) != len(wantFields) {
		t.Fatalf("fields len: got %d, want %d", len(got.Fields), len(wantFields))
	}
	for i := range wantFields {
		if got.Fields[i] != wantFields[i] {
			t.Fatalf("field[%d]: got %+v, want %+v", i, got.Fields[i], wantFields[i])
		}
	}
}
