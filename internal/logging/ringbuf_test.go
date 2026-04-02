package logging

import (
	"log/slog"
	"sync"
	"testing"
	"time"
)

func testEntry(msg string) LogEntry {
	return LogEntry{
		Time:    time.Unix(1, 0).UTC(),
		Level:   slog.LevelInfo,
		Service: "engine",
		Message: msg,
		Fields: []Field{{
			Key:   "message",
			Value: msg,
		}},
	}
}

func snapshotMessages(entries []LogEntry) []string {
	msgs := make([]string, len(entries))
	for i, entry := range entries {
		msgs[i] = entry.Message
	}
	return msgs
}

func TestRingBufferPreservesInsertionOrderUntilCapacity(t *testing.T) {
	buf := NewRingBuffer(3)

	buf.Write(testEntry("one"))
	buf.Write(testEntry("two"))

	snapshot := buf.Snapshot()
	if got, want := snapshotMessages(snapshot), []string{"one", "two"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("messages: got %v, want %v", got, want)
	}
	if got, want := buf.Seq(), uint64(2); got != want {
		t.Fatalf("seq: got %d, want %d", got, want)
	}
}

func TestRingBufferOverwritesOldestEntries(t *testing.T) {
	buf := NewRingBuffer(3)

	for _, msg := range []string{"one", "two", "three", "four", "five"} {
		buf.Write(testEntry(msg))
	}

	snapshot := buf.Snapshot()
	if got, want := snapshotMessages(snapshot), []string{"three", "four", "five"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("messages: got %v, want %v", got, want)
	}
	if got, want := buf.Seq(), uint64(5); got != want {
		t.Fatalf("seq: got %d, want %d", got, want)
	}
}

func TestRingBufferSnapshotReturnsDeepCopy(t *testing.T) {
	buf := NewRingBuffer(2)
	buf.Write(testEntry("one"))

	snapshot := buf.Snapshot()
	snapshot[0].Message = "mutated"
	snapshot[0].Fields[0] = Field{Key: "changed", Value: "changed"}
	snapshot = append(snapshot, testEntry("extra"))

	fresh := buf.Snapshot()
	if got, want := fresh[0].Message, "one"; got != want {
		t.Fatalf("message: got %q, want %q", got, want)
	}
	if got, want := fresh[0].Fields[0].Key, "message"; got != want {
		t.Fatalf("field key: got %q, want %q", got, want)
	}
	if got, want := fresh[0].Fields[0].Value, "one"; got != want {
		t.Fatalf("field value: got %q, want %q", got, want)
	}
}

func TestRingBufferClearResetsContentsAndAdvancesSeq(t *testing.T) {
	buf := NewRingBuffer(2)
	buf.Write(testEntry("one"))
	buf.Write(testEntry("two"))

	buf.Clear()

	if got := buf.Snapshot(); len(got) != 0 {
		t.Fatalf("snapshot len after clear: got %d, want 0", len(got))
	}
	if got, want := buf.Seq(), uint64(3); got != want {
		t.Fatalf("seq after clear: got %d, want %d", got, want)
	}

	buf.Write(testEntry("three"))
	if got, want := snapshotMessages(buf.Snapshot()), []string{"three"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("messages after clear: got %v, want %v", got, want)
	}
	if got, want := buf.Seq(), uint64(4); got != want {
		t.Fatalf("seq after write: got %d, want %d", got, want)
	}
}

func TestRingBufferCapacityLessThanOneUsesSafeMinimum(t *testing.T) {
	buf := NewRingBuffer(0)
	buf.Write(testEntry("one"))
	buf.Write(testEntry("two"))

	snapshot := buf.Snapshot()
	if got, want := snapshotMessages(snapshot), []string{"two"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("messages: got %v, want %v", got, want)
	}
}

func TestRingBufferConcurrentWritesStayBounded(t *testing.T) {
	const (
		capacity = 16
		writes   = 256
	)

	buf := NewRingBuffer(capacity)
	var wg sync.WaitGroup
	wg.Add(writes)

	for i := range writes {
		go func(i int) {
			defer wg.Done()
			buf.Write(testEntry(time.Unix(int64(i), 0).UTC().Format(time.RFC3339)))
		}(i)
	}

	wg.Wait()

	if got := len(buf.Snapshot()); got > capacity {
		t.Fatalf("snapshot len: got %d, want <= %d", got, capacity)
	}
	if got, want := buf.Seq(), uint64(writes); got != want {
		t.Fatalf("seq: got %d, want %d", got, want)
	}
}
