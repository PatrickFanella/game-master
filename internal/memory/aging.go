package memory

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// DefaultWindowSize is used when AgingPolicy.WindowSize is not set or <= 0.
const DefaultWindowSize = 20

// AgingPolicy defines the sliding window parameters for turn aging.
type AgingPolicy struct {
	WindowSize int // number of recent turns to keep un-summarized
}

// TurnsToAge returns the number of oldest turns that should be aged
// (summarized and embedded) given the current total turn count.
func (p AgingPolicy) TurnsToAge(totalTurns int) int {
	window := p.WindowSize
	if window <= 0 {
		window = DefaultWindowSize
	}
	if totalTurns <= window {
		return 0
	}
	return totalTurns - window
}

// TurnSummarizer abstracts turn summarization so WindowAger can be tested
// without a real LLM provider.
type TurnSummarizer interface {
	SummarizeTurn(ctx context.Context, playerInput string, llmResponse string, toolCalls string) (*SummaryResult, error)
}

// AgingTurn represents a session log entry eligible for aging.
type AgingTurn struct {
	ID          uuid.UUID
	CampaignID  uuid.UUID
	PlayerInput string
	LLMResponse string
	ToolCalls   string
	TurnNumber  int
}

// JobSubmitter accepts embed jobs for asynchronous processing.
// *Pipeline implements this interface.
type JobSubmitter interface {
	Submit(EmbedJob) bool
}

// AgingStore provides the persistence operations needed by WindowAger.
type AgingStore interface {
	// ListUnsummarizedTurns returns session logs that haven't been summarized yet,
	// ordered by turn_number ascending, limited to limit entries.
	ListUnsummarizedTurns(ctx context.Context, campaignID uuid.UUID, limit int) ([]AgingTurn, error)
	// MarkTurnSummarized marks a session log as summarized.
	MarkTurnSummarized(ctx context.Context, sessionLogID uuid.UUID) error
}

// WindowAger manages the aging of old turns by summarizing and embedding them.
type WindowAger struct {
	policy     AgingPolicy
	summarizer TurnSummarizer
	pipeline   JobSubmitter
	store      AgingStore
	logger     *slog.Logger
}

// NewWindowAger returns a WindowAger wired to the given dependencies.
// A nil logger falls back to slog.Default().
func NewWindowAger(policy AgingPolicy, summarizer TurnSummarizer, pipeline JobSubmitter, store AgingStore, logger *slog.Logger) *WindowAger {
	if logger == nil {
		logger = slog.Default()
	}
	return &WindowAger{
		policy:     policy,
		summarizer: summarizer,
		pipeline:   pipeline,
		store:      store,
		logger:     logger,
	}
}

// CheckAndAge checks if any turns need aging and processes them.
// It is safe to call concurrently and from a goroutine.
// Errors from individual turns are logged but do not abort processing of
// remaining turns; the method always returns nil.
func (w *WindowAger) CheckAndAge(ctx context.Context, campaignID uuid.UUID, totalTurns int) error {
	toAge := w.policy.TurnsToAge(totalTurns)
	if toAge == 0 {
		return nil
	}

	turns, err := w.store.ListUnsummarizedTurns(ctx, campaignID, toAge)
	if err != nil {
		return fmt.Errorf("listing unsummarized turns: %w", err)
	}

	for _, turn := range turns {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled during aging: %w", err)
		}

		result, err := w.summarizer.SummarizeTurn(ctx, turn.PlayerInput, turn.LLMResponse, turn.ToolCalls)
		if err != nil {
			w.logger.Warn("aging: summarize failed",
				"turn_id", turn.ID,
				"turn_number", turn.TurnNumber,
				"error", err,
			)
			continue
		}

		if ok := w.pipeline.Submit(EmbedJob{
			Text:       result.Summary,
			CampaignID: turn.CampaignID,
			MemoryType: result.EventType,
		}); !ok {
			w.logger.Warn("aging: pipeline submit failed (buffer full)",
				"turn_id", turn.ID,
				"turn_number", turn.TurnNumber,
			)
		}

		if err := w.store.MarkTurnSummarized(ctx, turn.ID); err != nil {
			w.logger.Warn("aging: mark summarized failed",
				"turn_id", turn.ID,
				"turn_number", turn.TurnNumber,
				"error", err,
			)
			continue
		}
	}

	return nil
}
