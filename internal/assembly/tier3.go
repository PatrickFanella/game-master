package assembly

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/game"
	"github.com/PatrickFanella/game-master/internal/memory"
)

// MemoryRetriever abstracts semantic memory search for the assembly package.
type MemoryRetriever interface {
	SearchSimilar(ctx context.Context, campaignID uuid.UUID, query string, limit int) ([]memory.MemoryResult, error)
}
// Tier3Retriever fetches semantically relevant memories for a player turn and
// formats them for inclusion in the LLM context window.
type Tier3Retriever struct {
	retriever MemoryRetriever
	limit     int
	logger   *slog.Logger
}

// NewTier3Retriever creates a Tier3Retriever. A non-positive limit defaults to 5.
// A nil logger falls back to slog.Default().
func NewTier3Retriever(retriever MemoryRetriever, limit int, logger *slog.Logger) *Tier3Retriever {
	if limit <= 0 {
		limit = 5
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Tier3Retriever{retriever: retriever, limit: limit, logger: logger}
}

// Retrieve builds a composite query from the player input and current game
// state, searches for similar memories, and returns them as pre-formatted
// strings ready for the assembler. Errors are logged but never propagated —
// a failed memory lookup must not block a turn.
func (t *Tier3Retriever) Retrieve(ctx context.Context, campaignID uuid.UUID, playerInput string, state *game.GameState) ([]string, error) {
	query := buildCompositeQuery(playerInput, state)

	results, err := t.retriever.SearchSimilar(ctx, campaignID, query, t.limit)
	if err != nil {
		t.logger.Warn("tier3 memory retrieval failed", "campaign_id", campaignID, "error", err)
		return nil, nil
	}

	if len(results) == 0 {
		return nil, nil
	}

	formatted := make([]string, len(results))
	for i, r := range results {
		formatted[i] = fmt.Sprintf("%s (%s, relevance: %.2f)", r.Content, r.MemoryType, 1-r.Distance)
	}
	return formatted, nil
}

// buildCompositeQuery enriches the raw player input with location and quest
// context so the vector search returns more situationally relevant results.
func buildCompositeQuery(playerInput string, state *game.GameState) string {
	parts := []string{playerInput}

	if state != nil {
		if state.CurrentLocation.Name != "" {
			parts = append(parts, state.CurrentLocation.Name)
		}
		if len(state.ActiveQuests) > 0 {
			parts = append(parts, state.ActiveQuests[0].Title)
		}
	}

	return strings.Join(parts, " ")
}
