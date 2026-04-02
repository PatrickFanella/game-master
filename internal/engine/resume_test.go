package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/game"
)

type stubResumeStore struct {
	state    *game.GameState
	stateErr error
	logs     []domain.SessionLog
	logsErr  error

	// recorded args
	gatherCampaignID uuid.UUID
	logsCampaignID   uuid.UUID
	logsLimit        int
}

func (s *stubResumeStore) GatherState(_ context.Context, campaignID uuid.UUID) (*game.GameState, error) {
	s.gatherCampaignID = campaignID
	if s.stateErr != nil {
		return nil, s.stateErr
	}
	return s.state, nil
}

func (s *stubResumeStore) ListRecentSessionLogs(_ context.Context, campaignID uuid.UUID, limit int) ([]domain.SessionLog, error) {
	s.logsCampaignID = campaignID
	s.logsLimit = limit
	if s.logsErr != nil {
		return nil, s.logsErr
	}
	return s.logs, nil
}

func TestResume_Success(t *testing.T) {
	cid := uuid.New()
	state := &game.GameState{
		Campaign: domain.Campaign{ID: cid, Name: "Test"},
	}
	logs := []domain.SessionLog{
		{PlayerInput: "go north", LLMResponse: "You head north."},
		{PlayerInput: "look around", LLMResponse: "A dark forest."},
	}
	store := &stubResumeStore{state: state, logs: logs}
	summarize := func(_ context.Context, l []domain.SessionLog) (string, error) {
		return "Previously, the hero explored the forest.", nil
	}

	r := NewResumer(store, summarize, nil)
	result, err := r.Resume(context.Background(), cid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.State != state {
		t.Fatal("state mismatch")
	}
	if result.PreviousSummary != "Previously, the hero explored the forest." {
		t.Fatalf("summary = %q", result.PreviousSummary)
	}
	if result.TurnCount != 2 {
		t.Fatalf("turn count = %d, want 2", result.TurnCount)
	}
	if store.gatherCampaignID != cid {
		t.Fatal("GatherState not called with correct campaign ID")
	}
}

func TestResume_NoLogs(t *testing.T) {
	cid := uuid.New()
	state := &game.GameState{Campaign: domain.Campaign{ID: cid}}
	store := &stubResumeStore{state: state, logs: nil}
	summarize := func(_ context.Context, _ []domain.SessionLog) (string, error) {
		t.Fatal("summarize should not be called with no logs")
		return "", nil
	}

	r := NewResumer(store, summarize, nil)
	result, err := r.Resume(context.Background(), cid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PreviousSummary != "" {
		t.Fatalf("expected empty summary, got %q", result.PreviousSummary)
	}
	if result.TurnCount != 0 {
		t.Fatalf("turn count = %d, want 0", result.TurnCount)
	}
}

func TestResume_GatherStateError(t *testing.T) {
	store := &stubResumeStore{stateErr: errors.New("db down")}
	r := NewResumer(store, nil, nil)
	_, err := r.Resume(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, store.stateErr) {
		t.Fatalf("expected wrapped db error, got: %v", err)
	}
}

func TestResume_SummarizeError(t *testing.T) {
	cid := uuid.New()
	state := &game.GameState{
		Campaign: domain.Campaign{ID: cid, Name: "Fallback Campaign"},
	}
	logs := []domain.SessionLog{{PlayerInput: "hello"}}
	store := &stubResumeStore{state: state, logs: logs}
	summarize := func(_ context.Context, _ []domain.SessionLog) (string, error) {
		return "", errors.New("LLM unavailable")
	}

	r := NewResumer(store, summarize, nil)
	result, err := r.Resume(context.Background(), cid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "Previously, in Fallback Campaign..."
	if result.PreviousSummary != want {
		t.Fatalf("summary = %q, want %q", result.PreviousSummary, want)
	}
}

func TestResume_LogsPassedToSummarizer(t *testing.T) {
	cid := uuid.New()
	state := &game.GameState{Campaign: domain.Campaign{ID: cid}}
	logs := []domain.SessionLog{
		{PlayerInput: "a"}, {PlayerInput: "b"}, {PlayerInput: "c"},
	}
	store := &stubResumeStore{state: state, logs: logs}

	var receivedLogs []domain.SessionLog
	summarize := func(_ context.Context, l []domain.SessionLog) (string, error) {
		receivedLogs = l
		return "ok", nil
	}

	r := NewResumer(store, summarize, nil)
	_, err := r.Resume(context.Background(), cid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(receivedLogs) != 3 {
		t.Fatalf("summarizer received %d logs, want 3", len(receivedLogs))
	}
	if receivedLogs[0].PlayerInput != "a" {
		t.Fatalf("first log input = %q, want 'a'", receivedLogs[0].PlayerInput)
	}
}
