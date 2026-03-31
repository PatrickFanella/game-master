package world

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/PatrickFanella/game-master/internal/llm"
)

// extractToolName is the tool the LLM calls to signal that the interview is
// complete and deliver the gathered profile.
const extractToolName = "extract_campaign_profile"

// extractProfileTool defines the LLM tool schema for profile extraction.
func extractProfileTool() llm.Tool {
	return llm.Tool{
		Name:        extractToolName,
		Description: "Extract the campaign profile from the interview conversation. Call this tool when you have gathered enough information on all six topics.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"genre": map[string]any{
					"type":        "string",
					"description": "Short genre label (e.g. \"dark fantasy\", \"sci-fi\").",
				},
				"tone": map[string]any{
					"type":        "string",
					"description": "Short tone description (e.g. \"gritty and tense\").",
				},
				"themes": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Narrative themes the player wants to explore.",
				},
				"world_type": map[string]any{
					"type":        "string",
					"description": "Kind of world (e.g. \"war-torn kingdom\").",
				},
				"danger_level": map[string]any{
					"type":        "string",
					"description": "How lethal the world is: low, moderate, high, or brutal.",
				},
				"political_complexity": map[string]any{
					"type":        "string",
					"description": "Degree of political intrigue: simple, moderate, or complex.",
				},
			},
			"required": []string{
				"genre", "tone", "themes", "world_type",
				"danger_level", "political_complexity",
			},
		},
	}
}

// Interviewer drives an LLM-powered campaign-creation interview. It
// accumulates conversation history and detects when the LLM signals
// completion by calling the extract_campaign_profile tool.
type Interviewer struct {
	provider llm.Provider
	history  []llm.Message
	profile  *CampaignProfile
	done     bool
}

// NewInterviewer creates an Interviewer backed by the given LLM provider and
// seeds the conversation history with the system prompt.
func NewInterviewer(provider llm.Provider) *Interviewer {
	return &Interviewer{
		provider: provider,
		history: []llm.Message{
			{Role: llm.RoleSystem, Content: interviewPrompt},
		},
	}
}

// InterviewResult holds the LLM's response from a single interview step.
type InterviewResult struct {
	// Message is the text the LLM produced for the player.
	Message string
	// Profile is non-nil when the interview is complete and the LLM has
	// called extract_campaign_profile.
	Profile *CampaignProfile
	// Done is true when the interview is finished.
	Done bool
}

// Start sends an initial empty-input turn so the LLM generates its opening
// greeting and first question without requiring player input.
func (iv *Interviewer) Start(ctx context.Context) (*InterviewResult, error) {
	return iv.Step(ctx, "")
}

// Step processes one round of the interview. It appends playerInput to the
// conversation history, calls the LLM, and returns the result.
//
// When playerInput is empty (e.g. from [Interviewer.Start]) no user message
// is appended; the LLM responds to the system prompt alone.
//
// If the LLM calls extract_campaign_profile, the profile is parsed and
// returned in [InterviewResult.Profile] along with Done set to true.
func (iv *Interviewer) Step(ctx context.Context, playerInput string) (*InterviewResult, error) {
	if iv.done {
		return &InterviewResult{
			Message: "The interview is already complete.",
			Profile: iv.profile,
			Done:    true,
		}, nil
	}

	if playerInput != "" {
		iv.history = append(iv.history, llm.Message{
			Role:    llm.RoleUser,
			Content: playerInput,
		})
	}

	tools := []llm.Tool{extractProfileTool()}
	resp, err := iv.provider.Complete(ctx, iv.history, tools)
	if err != nil {
		return nil, fmt.Errorf("interview LLM call: %w", err)
	}

	// Check for the extract_campaign_profile tool call.
	for _, tc := range resp.ToolCalls {
		if tc.Name != extractToolName {
			continue
		}
		profile, parseErr := parseProfile(tc.Arguments)
		if parseErr != nil {
			return nil, fmt.Errorf("parse campaign profile from tool call: %w", parseErr)
		}
		// Ensure the extracted profile is actually complete before
		// marking the interview as done.
		if !profile.Complete() {
			return nil, fmt.Errorf("incomplete campaign profile extracted from tool call")
		}
		iv.profile = profile
		iv.done = true

		// Append the assistant message so the conversation history is
		// well-formed if the caller wants to inspect it.
		iv.history = append(iv.history, llm.Message{
			Role:      llm.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		return &InterviewResult{
			Message: resp.Content,
			Profile: profile,
			Done:    true,
		}, nil
	}

	// No profile-extraction tool call — ordinary interview turn.
	iv.history = append(iv.history, llm.Message{
		Role:      llm.RoleAssistant,
		Content:   resp.Content,
		ToolCalls: resp.ToolCalls,
	})

	return &InterviewResult{
		Message: resp.Content,
		Done:    false,
	}, nil
}

// History returns a deep copy of the accumulated conversation history.
func (iv *Interviewer) History() []llm.Message {
	out := make([]llm.Message, len(iv.history))
	for i, msg := range iv.history {
		msgCopy := msg

		if msg.ToolCalls != nil {
			tcCopy := make([]llm.ToolCall, len(msg.ToolCalls))
			copy(tcCopy, msg.ToolCalls)
			msgCopy.ToolCalls = tcCopy
		}

		out[i] = msgCopy
	}
	return out
}

// Done reports whether the interview has concluded.
func (iv *Interviewer) Done() bool {
	return iv.done
}

// Profile returns the extracted campaign profile, or nil if the interview has
// not yet concluded.
func (iv *Interviewer) Profile() *CampaignProfile {
	return iv.profile
}

// parseProfile converts the LLM tool-call arguments into a CampaignProfile.
func parseProfile(args map[string]any) (*CampaignProfile, error) {
	data, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("marshal tool args: %w", err)
	}
	var p CampaignProfile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("unmarshal profile: %w", err)
	}
	return &p, nil
}
