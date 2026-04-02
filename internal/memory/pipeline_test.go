package memory_test

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"log/slog"
	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/memory"
	statedb "github.com/PatrickFanella/game-master/internal/state/sqlc"
)

// --- spy stubs -----------------------------------------------------------

// spyEmbedder records calls and can be configured to fail for specific texts.
type spyEmbedder struct {
	mu       sync.Mutex
	calls    []string
	failFor  map[string]error
	vecDim   int
}

func newSpyEmbedder(dim int) *spyEmbedder {
	return &spyEmbedder{vecDim: dim, failFor: make(map[string]error)}
}

func (s *spyEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, text)
	if err, ok := s.failFor[text]; ok {
		return nil, err
	}
	return make([]float32, s.vecDim), nil
}

func (s *spyEmbedder) BatchEmbed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := s.Embed(context.Background(), t)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func (s *spyEmbedder) embedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

// spyStore records CreateMemory calls.
type spyStore struct {
	mu     sync.Mutex
	params []statedb.CreateMemoryParams
	err    error // if set, returned on every call
}

func (s *spyStore) CreateMemory(_ context.Context, arg statedb.CreateMemoryParams) (statedb.Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.params = append(s.params, arg)
	return statedb.Memory{}, s.err
}

func (s *spyStore) createCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.params)
}

// silentLogger returns a logger that discards output, keeping test output clean.
func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- tests ---------------------------------------------------------------

func TestPipeline_ProcessesJobs(t *testing.T) {
	emb := newSpyEmbedder(768)
	store := &spyStore{}
	p := memory.NewPipeline(emb, store, 16, silentLogger())

	for i := 0; i < 3; i++ {
		if !p.Submit(memory.EmbedJob{
			Text:       "test text",
			CampaignID: uuid.New(),
			MemoryType: "narrative",
		}) {
			t.Fatalf("submit %d returned false", i)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	if got := store.createCount(); got != 3 {
		t.Errorf("store.CreateMemory called %d times, want 3", got)
	}
}

func TestPipeline_EmbedErrorLogsAndContinues(t *testing.T) {
	emb := newSpyEmbedder(768)
	emb.failFor["fail-text"] = errors.New("boom")
	store := &spyStore{}
	p := memory.NewPipeline(emb, store, 16, silentLogger())

	p.Submit(memory.EmbedJob{Text: "fail-text", CampaignID: uuid.New(), MemoryType: "a"})
	p.Submit(memory.EmbedJob{Text: "ok-text", CampaignID: uuid.New(), MemoryType: "b"})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	if got := emb.embedCount(); got != 2 {
		t.Errorf("embed called %d times, want 2", got)
	}
	if got := store.createCount(); got != 1 {
		t.Errorf("store.CreateMemory called %d times, want 1", got)
	}
}

func TestPipeline_ShutdownDrainsRemaining(t *testing.T) {
	emb := newSpyEmbedder(768)
	store := &spyStore{}
	// Use a large buffer so all submits succeed before worker processes any.
	p := memory.NewPipeline(emb, store, 64, silentLogger())

	const n = 10
	for i := 0; i < n; i++ {
		p.Submit(memory.EmbedJob{Text: "drain", CampaignID: uuid.New(), MemoryType: "x"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	if got := store.createCount(); got != n {
		t.Errorf("store.CreateMemory called %d times, want %d", got, n)
	}
}

func TestPipeline_SubmitAfterShutdown(t *testing.T) {
	emb := newSpyEmbedder(768)
	store := &spyStore{}
	p := memory.NewPipeline(emb, store, 16, silentLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	// Must not panic; Submit uses recover internally.
	ok := p.Submit(memory.EmbedJob{Text: "late", CampaignID: uuid.New(), MemoryType: "x"})
	if ok {
		t.Error("Submit after Shutdown should return false")
	}
}

func TestPipeline_SubmitFullBuffer(t *testing.T) {
	emb := newSpyEmbedder(768)
	// slowStore blocks until released, ensuring the worker doesn't drain.
	blocker := make(chan struct{})
	store := &blockingStore{release: blocker}
	p := memory.NewPipeline(emb, store, 1, silentLogger())

	// First submit fills the buffer.
	if !p.Submit(memory.EmbedJob{Text: "a", CampaignID: uuid.New(), MemoryType: "x"}) {
		t.Fatal("first submit should succeed")
	}
	// Give the worker a moment to pick up the first job and block in store.
	time.Sleep(50 * time.Millisecond)

	// Second submit fills the now-empty single-slot buffer.
	if !p.Submit(memory.EmbedJob{Text: "b", CampaignID: uuid.New(), MemoryType: "x"}) {
		t.Fatal("second submit should succeed (worker blocked, slot free)")
	}

	// Third submit should fail — buffer full, worker blocked.
	if p.Submit(memory.EmbedJob{Text: "c", CampaignID: uuid.New(), MemoryType: "x"}) {
		t.Error("submit on full buffer should return false")
	}

	// Unblock and shut down cleanly.
	close(blocker)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestPipeline_ShutdownTimeout(t *testing.T) {
	emb := newSpyEmbedder(768)
	blocker := make(chan struct{})
	store := &blockingStore{release: blocker}
	p := memory.NewPipeline(emb, store, 16, silentLogger())

	// Submit a job so the worker blocks on the store.
	p.Submit(memory.EmbedJob{Text: "stuck", CampaignID: uuid.New(), MemoryType: "x"})
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already expired

	err := p.Shutdown(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	// Release the blocked worker so the goroutine exits (no leak).
	close(blocker)
}

// blockingStore blocks in CreateMemory until release is closed.
type blockingStore struct {
	release <-chan struct{}
}

func (s *blockingStore) CreateMemory(_ context.Context, _ statedb.CreateMemoryParams) (statedb.Memory, error) {
	<-s.release
	return statedb.Memory{}, nil
}
