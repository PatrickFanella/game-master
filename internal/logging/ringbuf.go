package logging

import "sync"

// RingBuffer stores the most recent log entries in a fixed-size circular buffer.
type RingBuffer struct {
	mu       sync.RWMutex
	entries  []LogEntry
	capacity int
	head     int
	count    int
	seq      uint64
}

// NewRingBuffer creates a ring buffer with at least one slot.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity < 1 {
		capacity = 1
	}

	return &RingBuffer{
		entries:  make([]LogEntry, capacity),
		capacity: capacity,
	}
}

// Write appends an entry, evicting the oldest entry when the buffer is full.
func (b *RingBuffer) Write(entry LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	entry = cloneEntry(entry)
	b.seq++

	if b.count < b.capacity {
		idx := (b.head + b.count) % b.capacity
		b.entries[idx] = entry
		b.count++
		return
	}

	b.entries[b.head] = entry
	b.head = (b.head + 1) % b.capacity
}

// Snapshot returns the current buffer contents in chronological order.
func (b *RingBuffer) Snapshot() []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	out := make([]LogEntry, b.count)
	for i := range b.count {
		idx := (b.head + i) % b.capacity
		out[i] = cloneEntry(b.entries[idx])
	}
	return out
}

// Seq reports a monotonically increasing sequence for every write and clear.
func (b *RingBuffer) Seq() uint64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.seq
}

// Clear removes all buffered entries and advances the sequence.
func (b *RingBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.entries = make([]LogEntry, b.capacity)
	b.head = 0
	b.count = 0
	b.seq++
}

func cloneEntry(entry LogEntry) LogEntry {
	if len(entry.Fields) == 0 {
		entry.Fields = nil
		return entry
	}

	fields := make([]Field, len(entry.Fields))
	copy(fields, entry.Fields)
	entry.Fields = fields
	return entry
}
