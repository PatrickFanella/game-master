package engine

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/game"
)

// ResumeStore is the narrow read interface needed to reload a campaign.
type ResumeStore interface {
	GatherState(ctx context.Context, campaignID uuid.UUID) (*game.GameState, error)
	ListRecentSessionLogs(ctx context.Context, campaignID uuid.UUID, limit int) ([]domain.SessionLog, error)
}

// ResumeResult holds the loaded campaign state together with a narrative
// recap of recent turns.
type ResumeResult struct {
	State           *game.GameState
	PreviousSummary string // "Previously..." narrative
	TurnCount       int    // total turns in the returned window
}

// Resumer loads campaign state and produces a recap summary.
type Resumer struct {
	logger    *slog.Logger
	store     ResumeStore
	summarize func(ctx context.Context, logs []domain.SessionLog) (string, error)
}

// NewResumer constructs a Resumer.  The summarize func is injected so
// callers can supply an LLM-backed implementation in production and a
// deterministic stub in tests.
func NewResumer(
	store ResumeStore,
	summarize func(ctx context.Context, logs []domain.SessionLog) (string, error),
	logger *slog.Logger,
) *Resumer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Resumer{logger: logger, store: store, summarize: summarize}
}

// Resume loads full campaign state and generates a "Previously..." summary
// from the most recent session logs.
func (r *Resumer) Resume(ctx context.Context, campaignID uuid.UUID) (*ResumeResult, error) {
	state, err := r.store.GatherState(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("resume: gather state: %w", err)
	}

	const recentLogLimit = 5
	logs, err := r.store.ListRecentSessionLogs(ctx, campaignID, recentLogLimit)
	if err != nil {
		return nil, fmt.Errorf("resume: list recent logs: %w", err)
	}

	if len(logs) == 0 {
		return &ResumeResult{State: state}, nil
	}

	summary, err := r.summarize(ctx, logs)
	if err != nil {
		r.logger.Warn("resume: summarize failed, using fallback",
			"campaign_id", campaignID,
			"error", err,
		)
		summary = fmt.Sprintf("Previously, in %s...", state.Campaign.Name)
	}

	return &ResumeResult{
		State:           state,
		PreviousSummary: summary,
		TurnCount:       len(logs),
	}, nil
}
