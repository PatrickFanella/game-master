package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// --- stubs ---

type stubSummarizer struct {
	results map[uuid.UUID]*SummaryResult
	errs    map[uuid.UUID]error
	calls   []uuid.UUID // turn IDs in order called
}

func (s *stubSummarizer) SummarizeTurn(_ context.Context, playerInput, llmResponse, toolCalls string) (*SummaryResult, error) {
	// We identify calls by matching playerInput which we set to turn ID string in tests.
	id := uuid.MustParse(playerInput)
	s.calls = append(s.calls, id)
	if err, ok := s.errs[id]; ok {
		return nil, err
	}
	if r, ok := s.results[id]; ok {
		return r, nil
	}
	return &SummaryResult{Summary: "summary of " + playerInput, EventType: "exploration"}, nil
}

type submittedJob struct {
	job EmbedJob
}

type stubSubmitter struct {
	jobs     []submittedJob
	fullAt   int // return false after this many submits; -1 = never full
}

func (s *stubSubmitter) Submit(job EmbedJob) bool {
	if s.fullAt >= 0 && len(s.jobs) >= s.fullAt {
		return false
	}
	s.jobs = append(s.jobs, submittedJob{job: job})
	return true
}

type stubAgingStore struct {
	turns       []AgingTurn
	listErr     error
	markErrs    map[uuid.UUID]error
	markedIDs   []uuid.UUID
	listCalls   int
}

func (s *stubAgingStore) ListUnsummarizedTurns(_ context.Context, _ uuid.UUID, _ int) ([]AgingTurn, error) {
	s.listCalls++
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.turns, nil
}

func (s *stubAgingStore) MarkTurnSummarized(_ context.Context, id uuid.UUID) error {
	if err, ok := s.markErrs[id]; ok {
		return err
	}
	s.markedIDs = append(s.markedIDs, id)
	return nil
}

// --- AgingPolicy tests ---

func TestAgingPolicy_TurnsToAge(t *testing.T) {
	cases := []struct {
		name       string
		window     int
		totalTurns int
		want       int
	}{
		{"zero turns", 10, 0, 0},
		{"under window", 10, 5, 0},
		{"at window", 10, 10, 0},
		{"one over", 10, 11, 1},
		{"many over", 10, 25, 15},
		{"window of 1", 1, 3, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := AgingPolicy{WindowSize: tc.window}
			got := p.TurnsToAge(tc.totalTurns)
			if got != tc.want {
				t.Errorf("TurnsToAge(%d) with window %d = %d, want %d", tc.totalTurns, tc.window, got, tc.want)
			}
		})
	}
}

func TestAgingPolicy_DefaultWindow(t *testing.T) {
	cases := []struct {
		name       string
		window     int
		totalTurns int
		want       int
	}{
		{"zero window uses default", 0, 25, 5},
		{"negative window uses default", -5, 25, 5},
		{"at default window", 0, 20, 0},
		{"under default window", 0, 15, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := AgingPolicy{WindowSize: tc.window}
			got := p.TurnsToAge(tc.totalTurns)
			if got != tc.want {
				t.Errorf("TurnsToAge(%d) with window %d = %d, want %d", tc.totalTurns, tc.window, got, tc.want)
			}
		})
	}
}

// --- WindowAger tests ---

func makeTestTurns(n int) []AgingTurn {
	turns := make([]AgingTurn, n)
	for i := range turns {
		id := uuid.New()
		turns[i] = AgingTurn{
			ID:          id,
			CampaignID:  uuid.New(),
			PlayerInput: id.String(), // used by stubSummarizer to identify the call
			LLMResponse: "response",
			ToolCalls:   "",
			TurnNumber:  i + 1,
		}
	}
	return turns
}

func TestWindowAger_NoAgingNeeded(t *testing.T) {
	store := &stubAgingStore{}
	summarizer := &stubSummarizer{}
	submitter := &stubSubmitter{fullAt: -1}

	ager := NewWindowAger(AgingPolicy{WindowSize: 10}, summarizer, submitter, store, nil)
	err := ager.CheckAndAge(context.Background(), uuid.New(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.listCalls != 0 {
		t.Errorf("store.ListUnsummarizedTurns called %d times, want 0", store.listCalls)
	}
	if len(summarizer.calls) != 0 {
		t.Errorf("summarizer called %d times, want 0", len(summarizer.calls))
	}
}

func TestWindowAger_AgesTurns(t *testing.T) {
	turns := makeTestTurns(3)
	store := &stubAgingStore{turns: turns, markErrs: map[uuid.UUID]error{}}
	summarizer := &stubSummarizer{
		results: map[uuid.UUID]*SummaryResult{},
		errs:    map[uuid.UUID]error{},
	}
	for _, turn := range turns {
		summarizer.results[turn.ID] = &SummaryResult{
			Summary:   "aged: " + turn.ID.String(),
			EventType: "combat",
		}
	}
	submitter := &stubSubmitter{fullAt: -1}

	ager := NewWindowAger(AgingPolicy{WindowSize: 10}, summarizer, submitter, store, nil)
	// totalTurns=13 with window=10 → 3 to age
	err := ager.CheckAndAge(context.Background(), uuid.New(), 13)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all 3 turns summarized
	if len(summarizer.calls) != 3 {
		t.Fatalf("summarizer called %d times, want 3", len(summarizer.calls))
	}
	for i, id := range summarizer.calls {
		if id != turns[i].ID {
			t.Errorf("summarizer call %d: got %s, want %s", i, id, turns[i].ID)
		}
	}

	// Verify all 3 jobs submitted
	if len(submitter.jobs) != 3 {
		t.Fatalf("submitted %d jobs, want 3", len(submitter.jobs))
	}
	for i, sj := range submitter.jobs {
		if sj.job.Text != "aged: "+turns[i].ID.String() {
			t.Errorf("job %d text = %q, want %q", i, sj.job.Text, "aged: "+turns[i].ID.String())
		}
		if sj.job.MemoryType != "combat" {
			t.Errorf("job %d memory type = %q, want %q", i, sj.job.MemoryType, "combat")
		}
	}

	// Verify all 3 turns marked
	if len(store.markedIDs) != 3 {
		t.Fatalf("marked %d turns, want 3", len(store.markedIDs))
	}
	for i, id := range store.markedIDs {
		if id != turns[i].ID {
			t.Errorf("marked turn %d: got %s, want %s", i, id, turns[i].ID)
		}
	}
}

func TestWindowAger_SummarizerError(t *testing.T) {
	turns := makeTestTurns(3)
	failID := turns[1].ID // middle turn fails

	store := &stubAgingStore{turns: turns, markErrs: map[uuid.UUID]error{}}
	summarizer := &stubSummarizer{
		results: map[uuid.UUID]*SummaryResult{},
		errs:    map[uuid.UUID]error{failID: errors.New("llm unavailable")},
	}

	submitter := &stubSubmitter{fullAt: -1}
	ager := NewWindowAger(AgingPolicy{WindowSize: 10}, summarizer, submitter, store, nil)

	err := ager.CheckAndAge(context.Background(), uuid.New(), 13)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All 3 summarize attempts made
	if len(summarizer.calls) != 3 {
		t.Fatalf("summarizer called %d times, want 3", len(summarizer.calls))
	}

	// Only 2 jobs submitted (middle one failed)
	if len(submitter.jobs) != 2 {
		t.Fatalf("submitted %d jobs, want 2", len(submitter.jobs))
	}

	// Only 2 turns marked
	if len(store.markedIDs) != 2 {
		t.Fatalf("marked %d turns, want 2", len(store.markedIDs))
	}
	// Verify the failed turn was not marked
	for _, id := range store.markedIDs {
		if id == failID {
			t.Errorf("failed turn %s should not be marked summarized", failID)
		}
	}
}

func TestWindowAger_MarkError(t *testing.T) {
	turns := makeTestTurns(3)
	failID := turns[0].ID

	store := &stubAgingStore{
		turns:    turns,
		markErrs: map[uuid.UUID]error{failID: errors.New("db write error")},
	}
	summarizer := &stubSummarizer{
		results: map[uuid.UUID]*SummaryResult{},
		errs:    map[uuid.UUID]error{},
	}
	submitter := &stubSubmitter{fullAt: -1}

	ager := NewWindowAger(AgingPolicy{WindowSize: 10}, summarizer, submitter, store, nil)
	err := ager.CheckAndAge(context.Background(), uuid.New(), 13)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All 3 summarized
	if len(summarizer.calls) != 3 {
		t.Fatalf("summarizer called %d times, want 3", len(summarizer.calls))
	}

	// All 3 jobs submitted (submit happens before mark)
	if len(submitter.jobs) != 3 {
		t.Fatalf("submitted %d jobs, want 3", len(submitter.jobs))
	}

	// Only 2 marked (first one failed)
	if len(store.markedIDs) != 2 {
		t.Fatalf("marked %d turns, want 2", len(store.markedIDs))
	}
}

func TestWindowAger_EmptyTurns(t *testing.T) {
	store := &stubAgingStore{turns: []AgingTurn{}, markErrs: map[uuid.UUID]error{}}
	summarizer := &stubSummarizer{}
	submitter := &stubSubmitter{fullAt: -1}

	ager := NewWindowAger(AgingPolicy{WindowSize: 5}, summarizer, submitter, store, nil)
	// totalTurns=10 with window=5 → 5 to age, but store returns empty
	err := ager.CheckAndAge(context.Background(), uuid.New(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.listCalls != 1 {
		t.Errorf("store.ListUnsummarizedTurns called %d times, want 1", store.listCalls)
	}
	if len(summarizer.calls) != 0 {
		t.Errorf("summarizer called %d times, want 0", len(summarizer.calls))
	}
}

func TestWindowAger_ContextCancelled(t *testing.T) {
	turns := makeTestTurns(3)
	store := &stubAgingStore{turns: turns, markErrs: map[uuid.UUID]error{}}
	summarizer := &stubSummarizer{
		results: map[uuid.UUID]*SummaryResult{},
		errs:    map[uuid.UUID]error{},
	}
	submitter := &stubSubmitter{fullAt: -1}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	ager := NewWindowAger(AgingPolicy{WindowSize: 10}, summarizer, submitter, store, nil)
	err := ager.CheckAndAge(ctx, uuid.New(), 13)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
	// No turns should have been processed
	if len(summarizer.calls) != 0 {
		t.Errorf("summarizer called %d times, want 0", len(summarizer.calls))
	}
}

func TestWindowAger_PipelineFull(t *testing.T) {
	turns := makeTestTurns(2)
	store := &stubAgingStore{turns: turns, markErrs: map[uuid.UUID]error{}}
	summarizer := &stubSummarizer{
		results: map[uuid.UUID]*SummaryResult{},
		errs:    map[uuid.UUID]error{},
	}
	// Pipeline is "full" after 0 submits
	submitter := &stubSubmitter{fullAt: 0}

	ager := NewWindowAger(AgingPolicy{WindowSize: 5}, summarizer, submitter, store, nil)
	err := ager.CheckAndAge(context.Background(), uuid.New(), 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both turns still summarized
	if len(summarizer.calls) != 2 {
		t.Fatalf("summarizer called %d times, want 2", len(summarizer.calls))
	}

	// No jobs accepted
	if len(submitter.jobs) != 0 {
		t.Fatalf("submitted %d jobs, want 0", len(submitter.jobs))
	}

	// Both turns still marked (submit failure doesn't block marking)
	if len(store.markedIDs) != 2 {
		t.Fatalf("marked %d turns, want 2", len(store.markedIDs))
	}
}
