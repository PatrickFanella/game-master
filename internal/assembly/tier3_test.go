package assembly

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/game"
	"github.com/PatrickFanella/game-master/internal/memory"
)

// stubRetriever implements MemoryRetriever for tests.
type stubRetriever struct {
	results  []memory.MemoryResult
	err      error
	captured struct {
		campaignID uuid.UUID
		query      string
		limit      int
	}
}

func (s *stubRetriever) SearchSimilar(_ context.Context, campaignID uuid.UUID, query string, limit int) ([]memory.MemoryResult, error) {
	s.captured.campaignID = campaignID
	s.captured.query = query
	s.captured.limit = limit
	return s.results, s.err
}

func TestTier3Retriever_BasicRetrieval(t *testing.T) {
	stub := &stubRetriever{
		results: []memory.MemoryResult{
			{Content: "The dragon attacked", MemoryType: "narrative", Distance: 0.2},
			{Content: "Sword found in cave", MemoryType: "item", Distance: 0.35},
		},
	}
	r := NewTier3Retriever(stub, 5, nil)

	got, err := r.Retrieve(context.Background(), uuid.New(), "look around", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	if got[0] != "The dragon attacked (narrative, relevance: 0.80)" {
		t.Errorf("unexpected formatted result[0]: %s", got[0])
	}
	if got[1] != "Sword found in cave (item, relevance: 0.65)" {
		t.Errorf("unexpected formatted result[1]: %s", got[1])
	}
}

func TestTier3Retriever_EmptyResults(t *testing.T) {
	stub := &stubRetriever{results: nil}
	r := NewTier3Retriever(stub, 5, nil)

	got, err := r.Retrieve(context.Background(), uuid.New(), "hello", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %d results", len(got))
	}
}

func TestTier3Retriever_RetrieverError(t *testing.T) {
	stub := &stubRetriever{err: errors.New("database down")}
	r := NewTier3Retriever(stub, 5, nil)

	got, err := r.Retrieve(context.Background(), uuid.New(), "test", nil)
	if err != nil {
		t.Fatalf("error should not propagate, got: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice on error, got %d results", len(got))
	}
}

func TestTier3Retriever_CompositeQuery(t *testing.T) {
	stub := &stubRetriever{}
	r := NewTier3Retriever(stub, 3, nil)

	state := &game.GameState{
		CurrentLocation: domain.Location{Name: "Dark Forest"},
		ActiveQuests: []domain.Quest{
			{Title: "Find the Lost Amulet"},
			{Title: "Should be ignored"},
		},
	}

	_, _ = r.Retrieve(context.Background(), uuid.New(), "search for clues", state)

	want := "search for clues Dark Forest Find the Lost Amulet"
	if stub.captured.query != want {
		t.Errorf("composite query mismatch\n got: %q\nwant: %q", stub.captured.query, want)
	}
}

func TestTier3Retriever_NilState(t *testing.T) {
	stub := &stubRetriever{
		results: []memory.MemoryResult{
			{Content: "old memory", MemoryType: "event", Distance: 0.1},
		},
	}
	r := NewTier3Retriever(stub, 5, nil)

	got, err := r.Retrieve(context.Background(), uuid.New(), "just the input", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if stub.captured.query != "just the input" {
		t.Errorf("query should be bare input, got: %q", stub.captured.query)
	}
}

func TestTier3Retriever_DefaultLimit(t *testing.T) {
	stub := &stubRetriever{}

	r := NewTier3Retriever(stub, 0, nil)
	_, _ = r.Retrieve(context.Background(), uuid.New(), "test", nil)

	if stub.captured.limit != 5 {
		t.Errorf("expected default limit 5, got %d", stub.captured.limit)
	}

	r2 := NewTier3Retriever(stub, -3, nil)
	_, _ = r2.Retrieve(context.Background(), uuid.New(), "test", nil)

	if stub.captured.limit != 5 {
		t.Errorf("expected default limit 5 for negative input, got %d", stub.captured.limit)
	}
}
