package world

import (
	"context"
	"fmt"
	"testing"

	"github.com/PatrickFanella/game-master/internal/llm"
)

// ---------------------------------------------------------------------------
// CharacterProfile tests
// ---------------------------------------------------------------------------

func TestCharacterProfile_Complete(t *testing.T) {
	tests := []struct {
		name    string
		profile CharacterProfile
		want    bool
	}{
		{
			name:    "empty profile",
			profile: CharacterProfile{},
			want:    false,
		},
		{
			name: "missing motivations",
			profile: CharacterProfile{
				Name: "Kael", Concept: "elven ranger",
				Background: "forest-born", Personality: "stoic",
				Strengths: []string{"archery"}, Weaknesses: []string{"pride"},
			},
			want: false,
		},
		{
			name: "missing name",
			profile: CharacterProfile{
				Concept: "elven ranger", Background: "forest-born",
				Personality: "stoic", Motivations: []string{"revenge"},
				Strengths: []string{"archery"}, Weaknesses: []string{"pride"},
			},
			want: false,
		},
		{
			name: "missing weaknesses",
			profile: CharacterProfile{
				Name: "Kael", Concept: "elven ranger",
				Background: "forest-born", Personality: "stoic",
				Motivations: []string{"revenge"}, Strengths: []string{"archery"},
			},
			want: false,
		},
		{
			name: "all fields populated",
			profile: CharacterProfile{
				Name: "Kael", Concept: "elven ranger",
				Background: "raised in the ancient forest of Thal'rin",
				Personality: "stoic and observant",
				Motivations: []string{"protect the forest", "avenge fallen kin"},
				Strengths:   []string{"archery", "tracking"},
				Weaknesses:  []string{"distrustful of outsiders", "haunted by loss"},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.profile.Complete(); got != tt.want {
				t.Errorf("Complete() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CharacterInterviewer tests
// ---------------------------------------------------------------------------

func TestCharacterInterviewer_Start(t *testing.T) {
	provider := newScriptedProvider(t, scriptedResponse{
		resp: &llm.Response{Content: "Welcome! What kind of character do you envision?"},
	})

	ci := NewCharacterInterviewer(provider, &CampaignProfile{
		Genre: "dark fantasy", Tone: "gritty",
		Themes: []string{"survival"}, WorldType: "war-torn kingdom",
		DangerLevel: "high", PoliticalComplexity: "complex",
	})
	result, err := ci.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if result.Message != "Welcome! What kind of character do you envision?" {
		t.Errorf("Message = %q", result.Message)
	}
	if result.Done {
		t.Error("Done should be false after Start")
	}
	if result.Profile != nil {
		t.Error("Profile should be nil after Start")
	}

	// Verify the system prompt was sent.
	if len(provider.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(provider.calls))
	}
	msgs := provider.calls[0].messages
	if len(msgs) < 1 || msgs[0].Role != llm.RoleSystem {
		t.Fatal("first message should be system prompt")
	}
	// System prompt should mention the campaign context.
	if got := msgs[0].Content; got == "" {
		t.Error("system message should not be empty")
	}

	// Verify extract_character_profile tool was advertised.
	tools := provider.calls[0].tools
	if len(tools) != 1 || tools[0].Name != extractCharacterToolName {
		t.Errorf("expected extract_character_profile tool, got %v", tools)
	}
}

func TestCharacterInterviewer_StepConversation(t *testing.T) {
	provider := newScriptedProvider(t,
		// Start response
		scriptedResponse{resp: &llm.Response{Content: "What kind of character?"}},
		// Step 1 response
		scriptedResponse{resp: &llm.Response{Content: "An elven ranger, nice! What's their name?"}},
		// Step 2 response
		scriptedResponse{resp: &llm.Response{Content: "Kael — great name! Tell me about their background."}},
	)

	ci := NewCharacterInterviewer(provider, nil)

	// Start
	_, err := ci.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Step 1
	_, err = ci.Step(context.Background(), "I want an elven ranger")
	if err != nil {
		t.Fatalf("Step(1) error = %v", err)
	}

	// Step 2
	result, err := ci.Step(context.Background(), "Their name is Kael")
	if err != nil {
		t.Fatalf("Step(2) error = %v", err)
	}
	if result.Message != "Kael — great name! Tell me about their background." {
		t.Errorf("Message = %q", result.Message)
	}
	if result.Done {
		t.Error("interview should not be done yet")
	}

	// Verify history accumulation.
	history := ci.History()
	// system + assistant + user + assistant + user + assistant = 6
	wantLen := 6
	if len(history) != wantLen {
		t.Fatalf("history length = %d, want %d", len(history), wantLen)
	}
	// Check that the LLM received the full conversation on call 3.
	call3Msgs := provider.calls[2].messages
	// system + assistant + user + assistant + user = 5
	if len(call3Msgs) != 5 {
		t.Fatalf("call 3 messages = %d, want 5", len(call3Msgs))
	}
}

func TestCharacterInterviewer_ExtractProfile(t *testing.T) {
	provider := newScriptedProvider(t,
		// Start response
		scriptedResponse{resp: &llm.Response{Content: "What kind of character?"}},
		// Final response with tool call
		scriptedResponse{resp: &llm.Response{
			Content: "Great character! Here's your profile.",
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_char_1",
					Name: extractCharacterToolName,
					Arguments: map[string]any{
						"name":        "Kael",
						"concept":     "elven ranger",
						"background":  "raised in the ancient forest of Thal'rin",
						"personality": "stoic and observant",
						"motivations": []any{"protect the forest", "avenge fallen kin"},
						"strengths":   []any{"archery", "tracking"},
						"weaknesses":  []any{"distrustful of outsiders", "haunted by loss"},
					},
				},
			},
		}},
	)

	ci := NewCharacterInterviewer(provider, nil)
	_, err := ci.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	result, err := ci.Step(context.Background(), "I think that covers everything")
	if err != nil {
		t.Fatalf("Step() error = %v", err)
	}

	if !result.Done {
		t.Fatal("Done should be true after profile extraction")
	}
	if result.Profile == nil {
		t.Fatal("Profile should not be nil")
	}

	p := result.Profile
	if p.Name != "Kael" {
		t.Errorf("Name = %q", p.Name)
	}
	if p.Concept != "elven ranger" {
		t.Errorf("Concept = %q", p.Concept)
	}
	if p.Background != "raised in the ancient forest of Thal'rin" {
		t.Errorf("Background = %q", p.Background)
	}
	if p.Personality != "stoic and observant" {
		t.Errorf("Personality = %q", p.Personality)
	}
	if len(p.Motivations) != 2 || p.Motivations[0] != "protect the forest" {
		t.Errorf("Motivations = %v", p.Motivations)
	}
	if len(p.Strengths) != 2 || p.Strengths[0] != "archery" {
		t.Errorf("Strengths = %v", p.Strengths)
	}
	if len(p.Weaknesses) != 2 || p.Weaknesses[0] != "distrustful of outsiders" {
		t.Errorf("Weaknesses = %v", p.Weaknesses)
	}

	// Verify accessor methods.
	if !ci.Done() {
		t.Error("ci.Done() should be true")
	}
	if ci.Profile() == nil {
		t.Error("ci.Profile() should be non-nil")
	}
}

func TestCharacterInterviewer_IncompleteProfile(t *testing.T) {
	provider := newScriptedProvider(t,
		// Start response
		scriptedResponse{resp: &llm.Response{Content: "What kind of character?"}},
		// Tool call with missing fields (no weaknesses)
		scriptedResponse{resp: &llm.Response{
			Content: "Let me try to summarize...",
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_partial",
					Name: extractCharacterToolName,
					Arguments: map[string]any{
						"name":        "Kael",
						"concept":     "elven ranger",
						"background":  "forest-born",
						"personality": "stoic",
						"motivations": []any{"revenge"},
						"strengths":   []any{"archery"},
						// weaknesses deliberately omitted
					},
				},
			},
		}},
	)

	ci := NewCharacterInterviewer(provider, nil)
	_, err := ci.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	_, err = ci.Step(context.Background(), "that should be enough")
	if err == nil {
		t.Fatal("expected error for incomplete profile, got nil")
	}

	// Should not be marked done.
	if ci.Done() {
		t.Error("ci.Done() should be false after incomplete extraction")
	}
	if ci.Profile() != nil {
		t.Error("ci.Profile() should be nil after incomplete extraction")
	}
}

func TestCharacterInterviewer_AlreadyDone(t *testing.T) {
	provider := newScriptedProvider(t,
		scriptedResponse{resp: &llm.Response{
			Content: "All set!",
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_1",
					Name: extractCharacterToolName,
					Arguments: map[string]any{
						"name": "Kael", "concept": "ranger",
						"background": "forest", "personality": "stoic",
						"motivations": []any{"revenge"},
						"strengths":   []any{"archery"},
						"weaknesses":  []any{"pride"},
					},
				},
			},
		}},
	)

	ci := NewCharacterInterviewer(provider, nil)
	_, err := ci.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Second call after completion should return early without calling LLM.
	result, err := ci.Step(context.Background(), "anything else?")
	if err != nil {
		t.Fatalf("Step() error = %v", err)
	}
	if !result.Done {
		t.Error("Done should be true")
	}
	if result.Profile == nil {
		t.Error("Profile should be non-nil")
	}
	if result.Message != "The character interview is already complete." {
		t.Errorf("Message = %q", result.Message)
	}

	// Provider should have only been called once (the Start).
	if provider.callCount != 1 {
		t.Errorf("provider called %d times, want 1", provider.callCount)
	}
}

func TestCharacterInterviewer_LLMError(t *testing.T) {
	llmErr := fmt.Errorf("network timeout")
	provider := newScriptedProvider(t,
		scriptedResponse{resp: nil, err: llmErr},
	)

	ci := NewCharacterInterviewer(provider, nil)
	_, err := ci.Start(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if ci.Profile() != nil {
		t.Error("Profile should be nil after error")
	}
}

func TestCharacterInterviewer_WrongToolNameIgnored(t *testing.T) {
	provider := newScriptedProvider(t,
		// LLM returns a tool call with the wrong name — should be treated
		// as a normal conversation turn.
		scriptedResponse{resp: &llm.Response{
			Content: "Let me think about that...",
			ToolCalls: []llm.ToolCall{
				{
					ID:        "call_wrong",
					Name:      "wrong_tool",
					Arguments: map[string]any{"foo": "bar"},
				},
			},
		}},
	)

	ci := NewCharacterInterviewer(provider, nil)
	result, err := ci.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if result.Done {
		t.Error("Done should be false when wrong tool name is returned")
	}
	if result.Message != "Let me think about that..." {
		t.Errorf("Message = %q", result.Message)
	}
}

func TestCharacterInterviewer_NilCampaignProfile(t *testing.T) {
	provider := newScriptedProvider(t,
		scriptedResponse{resp: &llm.Response{Content: "Welcome! What character do you have in mind?"}},
	)

	ci := NewCharacterInterviewer(provider, nil)
	result, err := ci.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if result.Message != "Welcome! What character do you have in mind?" {
		t.Errorf("Message = %q", result.Message)
	}

	// System prompt should still be present but without campaign context section.
	msgs := provider.calls[0].messages
	if len(msgs) < 1 || msgs[0].Role != llm.RoleSystem {
		t.Fatal("first message should be system prompt")
	}
}
