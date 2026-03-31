package world

import (
	"context"
	"testing"

	"github.com/PatrickFanella/game-master/internal/llm"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// scriptedResponse is a pre-configured LLM response for test scenarios.
type scriptedResponse struct {
	resp *llm.Response
	err  error
}

// scriptedProvider replays a fixed sequence of responses, recording calls.
type scriptedProvider struct {
	t         *testing.T
	scripts   []scriptedResponse
	callCount int
	calls     []providerCall
}

type providerCall struct {
	messages []llm.Message
	tools    []llm.Tool
}

func newScriptedProvider(t *testing.T, scripts ...scriptedResponse) *scriptedProvider {
	t.Helper()
	return &scriptedProvider{t: t, scripts: scripts}
}

func (p *scriptedProvider) Complete(_ context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Response, error) {
	if p.callCount >= len(p.scripts) {
		p.t.Fatalf("Complete called %d time(s), but only %d response(s) were configured",
			p.callCount+1, len(p.scripts))
	}
	p.calls = append(p.calls, providerCall{
		messages: append([]llm.Message(nil), messages...),
		tools:    append([]llm.Tool(nil), tools...),
	})
	script := p.scripts[p.callCount]
	p.callCount++
	return script.resp, script.err
}

func (p *scriptedProvider) Stream(_ context.Context, _ []llm.Message, _ []llm.Tool) (<-chan llm.StreamChunk, error) {
	p.t.Fatal("unexpected Stream call in interview test")
	return nil, nil
}

// ---------------------------------------------------------------------------
// Profile tests
// ---------------------------------------------------------------------------

func TestCampaignProfile_Complete(t *testing.T) {
	tests := []struct {
		name    string
		profile CampaignProfile
		want    bool
	}{
		{
			name:    "empty profile",
			profile: CampaignProfile{},
			want:    false,
		},
		{
			name: "missing themes",
			profile: CampaignProfile{
				Genre: "fantasy", Tone: "dark",
				WorldType: "wilderness", DangerLevel: "high",
				PoliticalComplexity: "complex",
			},
			want: false,
		},
		{
			name: "all fields populated",
			profile: CampaignProfile{
				Genre: "dark fantasy", Tone: "gritty",
				Themes:              []string{"survival", "betrayal"},
				WorldType:           "war-torn kingdom",
				DangerLevel:         "high",
				PoliticalComplexity: "complex",
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
// Interviewer tests
// ---------------------------------------------------------------------------

func TestInterviewer_Start_SendsSystemPromptAndReturnsGreeting(t *testing.T) {
	provider := newScriptedProvider(t, scriptedResponse{
		resp: &llm.Response{Content: "Welcome! What genre of campaign excites you?"},
	})

	iv := NewInterviewer(provider)
	result, err := iv.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if result.Message != "Welcome! What genre of campaign excites you?" {
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
	if msgs[0].Content != interviewPrompt {
		t.Error("system message should contain the interview prompt")
	}

	// Verify extract_campaign_profile tool was advertised.
	tools := provider.calls[0].tools
	if len(tools) != 1 || tools[0].Name != extractToolName {
		t.Errorf("expected extract_campaign_profile tool, got %v", tools)
	}
}

func TestInterviewer_Step_AccumulatesHistory(t *testing.T) {
	provider := newScriptedProvider(t,
		// Start response
		scriptedResponse{resp: &llm.Response{Content: "What genre?"}},
		// Step 1 response
		scriptedResponse{resp: &llm.Response{Content: "Great choice! What tone?"}},
		// Step 2 response
		scriptedResponse{resp: &llm.Response{Content: "Awesome! What themes?"}},
	)

	iv := NewInterviewer(provider)

	// Start
	_, err := iv.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Step 1: player answers genre
	_, err = iv.Step(context.Background(), "I love dark fantasy")
	if err != nil {
		t.Fatalf("Step(1) error = %v", err)
	}

	// Step 2: player answers tone
	result, err := iv.Step(context.Background(), "Gritty and tense")
	if err != nil {
		t.Fatalf("Step(2) error = %v", err)
	}
	if result.Message != "Awesome! What themes?" {
		t.Errorf("Message = %q", result.Message)
	}
	if result.Done {
		t.Error("interview should not be done yet")
	}

	// Verify history accumulation.
	history := iv.History()
	// system + assistant("What genre?") + user("I love dark fantasy") +
	// assistant("Great choice! What tone?") + user("Gritty and tense") +
	// assistant("Awesome! What themes?")
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

func TestInterviewer_Step_ExtractsProfileOnToolCall(t *testing.T) {
	provider := newScriptedProvider(t,
		// Start response
		scriptedResponse{resp: &llm.Response{Content: "What genre?"}},
		// Final response with tool call
		scriptedResponse{resp: &llm.Response{
			Content: "Wonderful! Here is your campaign profile.",
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_123",
					Name: extractToolName,
					Arguments: map[string]any{
						"genre":                "dark fantasy",
						"tone":                 "gritty and tense",
						"themes":               []any{"survival", "betrayal"},
						"world_type":            "war-torn kingdom",
						"danger_level":          "high",
						"political_complexity":  "complex",
					},
				},
			},
		}},
	)

	iv := NewInterviewer(provider)
	_, err := iv.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	result, err := iv.Step(context.Background(), "I think that covers everything")
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
	if p.Genre != "dark fantasy" {
		t.Errorf("Genre = %q", p.Genre)
	}
	if p.Tone != "gritty and tense" {
		t.Errorf("Tone = %q", p.Tone)
	}
	if len(p.Themes) != 2 || p.Themes[0] != "survival" || p.Themes[1] != "betrayal" {
		t.Errorf("Themes = %v", p.Themes)
	}
	if p.WorldType != "war-torn kingdom" {
		t.Errorf("WorldType = %q", p.WorldType)
	}
	if p.DangerLevel != "high" {
		t.Errorf("DangerLevel = %q", p.DangerLevel)
	}
	if p.PoliticalComplexity != "complex" {
		t.Errorf("PoliticalComplexity = %q", p.PoliticalComplexity)
	}

	// Verify accessor methods.
	if !iv.Done() {
		t.Error("iv.Done() should be true")
	}
	if iv.Profile() == nil {
		t.Error("iv.Profile() should be non-nil")
	}
}

func TestInterviewer_Step_AfterDoneReturnsEarly(t *testing.T) {
	provider := newScriptedProvider(t,
		scriptedResponse{resp: &llm.Response{
			Content: "All set!",
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_1",
					Name: extractToolName,
					Arguments: map[string]any{
						"genre": "sci-fi", "tone": "epic",
						"themes":               []any{"exploration"},
						"world_type":            "space station",
						"danger_level":          "moderate",
						"political_complexity":  "simple",
					},
				},
			},
		}},
	)

	iv := NewInterviewer(provider)
	_, err := iv.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Second call after completion should return early without calling LLM.
	result, err := iv.Step(context.Background(), "anything else?")
	if err != nil {
		t.Fatalf("Step() error = %v", err)
	}
	if !result.Done {
		t.Error("expected Done = true")
	}
	if result.Profile == nil {
		t.Error("expected Profile to be returned")
	}
	// Provider should only have been called once (from Start).
	if provider.callCount != 1 {
		t.Errorf("provider called %d time(s), want 1", provider.callCount)
	}
}

func TestInterviewer_Step_LLMErrorPropagated(t *testing.T) {
	provider := newScriptedProvider(t,
		scriptedResponse{resp: &llm.Response{Content: "What genre?"}},
		scriptedResponse{err: context.DeadlineExceeded},
	)

	iv := NewInterviewer(provider)
	_, err := iv.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	_, err = iv.Step(context.Background(), "dark fantasy")
	if err == nil {
		t.Fatal("expected error from Step")
	}
}

func TestInterviewer_Step_IgnoresUnknownToolCalls(t *testing.T) {
	provider := newScriptedProvider(t,
		scriptedResponse{resp: &llm.Response{
			Content: "Interesting! Tell me more.",
			ToolCalls: []llm.ToolCall{
				{ID: "call_x", Name: "unknown_tool", Arguments: map[string]any{"foo": "bar"}},
			},
		}},
	)

	iv := NewInterviewer(provider)
	result, err := iv.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if result.Done {
		t.Error("should not be done when unknown tool is called")
	}
	if result.Profile != nil {
		t.Error("profile should be nil when unknown tool is called")
	}
	if result.Message != "Interesting! Tell me more." {
		t.Errorf("Message = %q", result.Message)
	}
}

func TestInterviewer_History_ReturnsCopy(t *testing.T) {
	provider := newScriptedProvider(t,
		scriptedResponse{resp: &llm.Response{
			Content: "Hello!",
			ToolCalls: []llm.ToolCall{
				{ID: "tc_1", Name: "unknown_tool", Arguments: map[string]any{"k": "v"}},
			},
		}},
	)

	iv := NewInterviewer(provider)
	_, _ = iv.Start(context.Background())

	h1 := iv.History()
	h2 := iv.History()

	// Mutate Content and verify h2 is unaffected.
	h1[1].Content = "MUTATED"
	if h2[1].Content == "MUTATED" {
		t.Error("History() should return independent copies (Content)")
	}

	// Mutate ToolCalls and verify h2 is unaffected.
	h1[1].ToolCalls[0].Name = "MUTATED"
	if h2[1].ToolCalls[0].Name == "MUTATED" {
		t.Error("History() should return independent copies (ToolCalls)")
	}
}

func TestInterviewPromptIsLoaded(t *testing.T) {
	if interviewPrompt == "" {
		t.Fatal("interviewPrompt embed should not be empty")
	}
}

func TestInterviewer_Step_RejectsIncompleteProfile(t *testing.T) {
	provider := newScriptedProvider(t,
		scriptedResponse{resp: &llm.Response{
			Content: "Let me summarize...",
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_1",
					Name: extractToolName,
					Arguments: map[string]any{
						"genre": "fantasy",
						"tone":  "dark",
						// missing themes, world_type, danger_level, political_complexity
					},
				},
			},
		}},
	)

	iv := NewInterviewer(provider)
	_, err := iv.Start(context.Background())
	if err == nil {
		t.Fatal("expected error for incomplete profile")
	}
	if iv.Done() {
		t.Error("interview should not be marked done for incomplete profile")
	}
}
