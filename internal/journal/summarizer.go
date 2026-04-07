package journal

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/llm"
)

// Summarizer generates narrative summaries of session turns using an LLM.
type Summarizer struct {
	llm   llm.Provider
	store *Store
}

// NewSummarizer creates a Summarizer backed by the given LLM provider and store.
func NewSummarizer(provider llm.Provider, store *Store) *Summarizer {
	return &Summarizer{llm: provider, store: store}
}

// Summarize fetches session logs for the given turn range and generates a
// chapter-style narrative summary, storing the result in session_summaries.
func (s *Summarizer) Summarize(ctx context.Context, campaignID uuid.UUID, fromTurn, toTurn int) (*Summary, error) {
	logs, err := s.store.ListSessionLogsByRange(ctx, campaignID, fromTurn, toTurn)
	if err != nil {
		return nil, fmt.Errorf("fetch session logs: %w", err)
	}
	if len(logs) == 0 {
		return nil, fmt.Errorf("no session logs found for turns %d-%d", fromTurn, toTurn)
	}

	prompt := buildSummarizationPrompt(logs, fromTurn, toTurn)

	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: journalSummarizerPrompt},
		{Role: llm.RoleUser, Content: prompt},
	}

	resp, err := s.llm.Complete(ctx, messages, nil)
	if err != nil {
		return nil, fmt.Errorf("LLM summarization call: %w", err)
	}

	summary, err := s.store.CreateSummary(ctx, campaignID, fromTurn, toTurn, resp.Content)
	if err != nil {
		return nil, fmt.Errorf("save summary: %w", err)
	}

	return &summary, nil
}

// SummarizeUnsummarized generates a summary for all turns that have not yet
// been summarized. Returns nil if there are no unsummarized turns.
func (s *Summarizer) SummarizeUnsummarized(ctx context.Context, campaignID uuid.UUID) (*Summary, error) {
	maxTurn, err := s.store.MaxSummarizedTurn(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("get max summarized turn: %w", err)
	}

	fromTurn := maxTurn + 1
	// Use a high upper bound; ListSessionLogsByRange will only return what exists.
	toTurn := fromTurn + 10000

	logs, err := s.store.ListSessionLogsByRange(ctx, campaignID, fromTurn, toTurn)
	if err != nil {
		return nil, fmt.Errorf("fetch unsummarized logs: %w", err)
	}
	if len(logs) == 0 {
		return nil, nil
	}

	actualToTurn := logs[len(logs)-1].TurnNumber
	return s.Summarize(ctx, campaignID, fromTurn, actualToTurn)
}

const journalSummarizerPrompt = `You are a chronicle writer for a tabletop RPG campaign. Given a sequence of player actions and game master responses, write a chapter-style narrative summary.

Guidelines:
- Write in third person past tense
- Capture key events, discoveries, and character interactions
- Preserve important names, places, and quest developments
- Be concise but evocative — aim for 2-4 paragraphs
- Do not add information that is not present in the source material`

func buildSummarizationPrompt(logs []SessionLogRow, fromTurn, toTurn int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Summarize the following game session (turns %d through %d):\n\n", fromTurn, toTurn)

	for _, log := range logs {
		fmt.Fprintf(&b, "--- Turn %d ---\n", log.TurnNumber)
		if log.PlayerInput != "" {
			fmt.Fprintf(&b, "Player: %s\n", log.PlayerInput)
		}
		if log.LLMResponse != "" {
			fmt.Fprintf(&b, "Game Master: %s\n", log.LLMResponse)
		}
		b.WriteString("\n")
	}

	return b.String()
}
