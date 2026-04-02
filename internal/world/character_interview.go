package world

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PatrickFanella/game-master/internal/llm"
)

// extractCharacterToolName is the tool the LLM calls to signal that the
// character interview is complete and deliver the gathered profile.
const extractCharacterToolName = "extract_character_profile"

// CharacterProfile captures the player's character concept gathered during the
// character-creation interview.
type CharacterProfile struct {
	Name        string   `json:"name"`
	Concept     string   `json:"concept"`     // e.g. "elven ranger", "street samurai"
	Background  string   `json:"background"`
	Personality string   `json:"personality"`
	Motivations []string `json:"motivations"`
	Strengths   []string `json:"strengths"`
	Weaknesses  []string `json:"weaknesses"`
}

// Complete returns true when every field has been populated, indicating the
// interview has gathered sufficient information.
func (p *CharacterProfile) Complete() bool {
	return p.Name != "" &&
		p.Concept != "" &&
		p.Background != "" &&
		p.Personality != "" &&
		len(p.Motivations) > 0 &&
		len(p.Strengths) > 0 &&
		len(p.Weaknesses) > 0
}

// extractCharacterProfileTool defines the LLM tool schema for character
// profile extraction.
func extractCharacterProfileTool() llm.Tool {
	return llm.Tool{
		Name:        extractCharacterToolName,
		Description: "Extract the character profile from the interview conversation. Call this tool when you have gathered enough information on all seven topics.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "The character's name.",
				},
				"concept": map[string]any{
					"type":        "string",
					"description": "Short concept or archetype (e.g. \"elven ranger\", \"street samurai\").",
				},
				"background": map[string]any{
					"type":        "string",
					"description": "The character's backstory and history.",
				},
				"personality": map[string]any{
					"type":        "string",
					"description": "Key personality traits and demeanor.",
				},
				"motivations": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "What drives the character forward.",
				},
				"strengths": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "The character's notable strengths and abilities.",
				},
				"weaknesses": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "The character's flaws, vulnerabilities, or limitations.",
				},
			},
			"required": []string{
				"name", "concept", "background", "personality",
				"motivations", "strengths", "weaknesses",
			},
		},
	}
}

// CharacterInterviewer drives an LLM-powered character-creation interview. It
// accumulates conversation history and detects when the LLM signals completion
// by calling the extract_character_profile tool.
type CharacterInterviewer struct {
	provider        llm.Provider
	history         []llm.Message
	profile         *CharacterProfile
	done            bool
	campaignProfile *CampaignProfile
}

// NewCharacterInterviewer creates a CharacterInterviewer backed by the given
// LLM provider. If campaignProfile is non-nil the system prompt is tailored to
// the established world; otherwise generic instructions are used.
func NewCharacterInterviewer(provider llm.Provider, campaignProfile *CampaignProfile) *CharacterInterviewer {
	return &CharacterInterviewer{
		provider:        provider,
		campaignProfile: campaignProfile,
		history: []llm.Message{
			{Role: llm.RoleSystem, Content: buildCharacterSystemPrompt(campaignProfile)},
		},
	}
}

// CharacterInterviewResult holds the LLM's response from a single character
// interview step.
type CharacterInterviewResult struct {
	// Message is the text the LLM produced for the player.
	Message string
	// Profile is non-nil when the interview is complete and the LLM has
	// called extract_character_profile.
	Profile *CharacterProfile
	// Done is true when the interview is finished.
	Done bool
}

// Start sends an initial empty-input turn so the LLM generates its opening
// greeting and first question without requiring player input.
func (ci *CharacterInterviewer) Start(ctx context.Context) (*CharacterInterviewResult, error) {
	return ci.Step(ctx, "")
}

// Step processes one round of the character interview. It appends playerInput
// to the conversation history, calls the LLM, and returns the result.
//
// When playerInput is empty (e.g. from [CharacterInterviewer.Start]) no user
// message is appended; the LLM responds to the system prompt alone.
//
// If the LLM calls extract_character_profile, the profile is parsed and
// returned in [CharacterInterviewResult.Profile] along with Done set to true.
func (ci *CharacterInterviewer) Step(ctx context.Context, playerInput string) (*CharacterInterviewResult, error) {
	if ci.done {
		return &CharacterInterviewResult{
			Message: "The character interview is already complete.",
			Profile: ci.profile,
			Done:    true,
		}, nil
	}

	if playerInput != "" {
		ci.history = append(ci.history, llm.Message{
			Role:    llm.RoleUser,
			Content: playerInput,
		})
	}

	tools := []llm.Tool{extractCharacterProfileTool()}
	resp, err := ci.provider.Complete(ctx, ci.history, tools)
	if err != nil {
		return nil, fmt.Errorf("character interview LLM call: %w", err)
	}

	// Check for the extract_character_profile tool call.
	for _, tc := range resp.ToolCalls {
		if tc.Name != extractCharacterToolName {
			continue
		}
		profile, parseErr := parseCharacterProfile(tc.Arguments)
		if parseErr != nil {
			return nil, fmt.Errorf("parse character profile from tool call: %w", parseErr)
		}
		// Ensure the extracted profile is actually complete before
		// marking the interview as done.
		if !profile.Complete() {
			return nil, fmt.Errorf("incomplete character profile extracted from tool call")
		}
		ci.profile = profile
		ci.done = true

		// Append the assistant message so the conversation history is
		// well-formed if the caller wants to inspect it.
		ci.history = append(ci.history, llm.Message{
			Role:      llm.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		return &CharacterInterviewResult{
			Message: resp.Content,
			Profile: profile,
			Done:    true,
		}, nil
	}

	// No profile-extraction tool call — ordinary interview turn.
	ci.history = append(ci.history, llm.Message{
		Role:      llm.RoleAssistant,
		Content:   resp.Content,
		ToolCalls: resp.ToolCalls,
	})

	return &CharacterInterviewResult{
		Message: resp.Content,
		Done:    false,
	}, nil
}

// History returns a deep copy of the accumulated conversation history.
func (ci *CharacterInterviewer) History() []llm.Message {
	out := make([]llm.Message, len(ci.history))
	for i, msg := range ci.history {
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

// Done reports whether the character interview has concluded.
func (ci *CharacterInterviewer) Done() bool {
	return ci.done
}

// Profile returns the extracted character profile, or nil if the interview has
// not yet concluded.
func (ci *CharacterInterviewer) Profile() *CharacterProfile {
	return ci.profile
}

// parseCharacterProfile converts the LLM tool-call arguments into a
// CharacterProfile.
func parseCharacterProfile(args map[string]any) (*CharacterProfile, error) {
	data, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("marshal tool args: %w", err)
	}
	var p CharacterProfile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("unmarshal character profile: %w", err)
	}
	return &p, nil
}

// buildCharacterSystemPrompt constructs the system prompt for the character
// interview. When cp is non-nil the prompt is tailored to the campaign's
// genre, tone, and world type; otherwise generic instructions are used.
func buildCharacterSystemPrompt(cp *CampaignProfile) string {
	var b strings.Builder

	b.WriteString("You are a friendly and creative Character Designer helping a player create a new character for their tabletop RPG campaign. Your goal is to learn about the player's character through a natural, engaging conversation.\n\n")

	if cp != nil {
		b.WriteString("=== CAMPAIGN CONTEXT ===\n\n")
		fmt.Fprintf(&b, "The campaign is set in a %s world with a %s tone.", cp.WorldType, cp.Tone)
		if cp.Genre != "" {
			fmt.Fprintf(&b, " The genre is %s.", cp.Genre)
		}
		if len(cp.Themes) > 0 {
			fmt.Fprintf(&b, " Key themes include %s.", strings.Join(cp.Themes, ", "))
		}
		b.WriteString(" Tailor your questions and suggestions to fit this setting.\n\n")
	}

	b.WriteString(`=== INTERVIEW TOPICS ===

You must gather information on all of the following topics. Ask about them one or two at a time in a natural conversational flow. Do NOT present them as a checklist.

1. **Name** — What is the character's name?
2. **Concept** — What is their class, archetype, or role? (e.g. elven ranger, street samurai, scholarly mage, grizzled detective)
3. **Background** — What is their backstory? Where did they come from and what shaped them?
4. **Personality** — What are their key personality traits and demeanor?
5. **Motivations** — What drives them? What do they want or need?
6. **Strengths** — What are they good at? What abilities or qualities set them apart?
7. **Weaknesses** — What are their flaws, fears, or vulnerabilities?

=== CONVERSATION STYLE ===

- Be warm, enthusiastic, and encouraging.
- Start by greeting the player and asking what kind of character they envision.
- When the player answers, acknowledge their choice with genuine interest, briefly build on it, and then naturally transition to the next topic.
- If the player gives a vague answer, ask a gentle follow-up to clarify.
- If the player covers multiple topics at once, acknowledge all of them and move on to the remaining topics.
- Keep each of your messages concise — no more than a few short paragraphs.
- Do NOT number your questions or present them as a formal list during the conversation.

=== COMPLETION ===

Once you have gathered enough information on ALL seven topics, call the extract_character_profile tool with the gathered details. The tool call signals that the interview is complete.

When calling extract_character_profile:
- name: the character's name
- concept: a short archetype label (e.g. "elven ranger")
- background: a summary of their backstory
- personality: key personality traits
- motivations: a JSON array of motivation strings
- strengths: a JSON array of strength strings
- weaknesses: a JSON array of weakness strings

After calling the tool, write a brief, enthusiastic summary of the character you have built together, confirming the choices back to the player.
`)

	return b.String()
}
