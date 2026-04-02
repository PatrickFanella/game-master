package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/PatrickFanella/game-master/internal/assembly"
	"github.com/PatrickFanella/game-master/internal/domain"
	"github.com/PatrickFanella/game-master/internal/game"
	"github.com/PatrickFanella/game-master/internal/llm"
	"github.com/PatrickFanella/game-master/internal/tools"
)

type scriptedResponse struct {
	resp *llm.Response
	err  error
}

type providerCall struct {
	messages []llm.Message
	tools    []llm.Tool
}

type scriptedProvider struct {
	t         *testing.T
	scripts   []scriptedResponse
	callCount int
	calls     []providerCall
}

func newScriptedProvider(t *testing.T, scripts ...scriptedResponse) *scriptedProvider {
	t.Helper()
	return &scriptedProvider{t: t, scripts: scripts}
}

func (p *scriptedProvider) Complete(_ context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Response, error) {
	if p.callCount >= len(p.scripts) {
		p.t.Fatalf("Complete called %d time(s), but only %d response(s) were configured", p.callCount+1, len(p.scripts))
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
	p.t.Fatal("unexpected Stream call in pipeline test")
	return nil, nil
}

type fakeStateManager struct {
	state             *game.GameState
	gatheredCampaigns []uuid.UUID
	savedLogs         []domain.SessionLog
	recentLogs        []domain.SessionLog
}

func (f *fakeStateManager) GetOrCreateDefaultUser(context.Context) (*domain.User, error) {
	return &domain.User{}, nil
}

func (f *fakeStateManager) CreateCampaign(context.Context, game.CreateCampaignParams) (*domain.Campaign, error) {
	return &domain.Campaign{}, nil
}

func (f *fakeStateManager) GatherState(_ context.Context, campaignID uuid.UUID) (*game.GameState, error) {
	f.gatheredCampaigns = append(f.gatheredCampaigns, campaignID)
	return f.state, nil
}

func (f *fakeStateManager) SaveSessionLog(_ context.Context, log domain.SessionLog) error {
	log.ToolCalls = append(json.RawMessage(nil), log.ToolCalls...)
	f.savedLogs = append(f.savedLogs, log)
	return nil
}

// ListRecentSessionLogs returns the fake recent logs.
func (f *fakeStateManager) ListRecentSessionLogs(_ context.Context, _ uuid.UUID, _ int) ([]domain.SessionLog, error) {
	return append([]domain.SessionLog(nil), f.recentLogs...), nil
}

// GetCampaignByID returns the campaign from the fake state.
func (f *fakeStateManager) GetCampaignByID(_ context.Context, _ uuid.UUID) (*domain.Campaign, error) {
	if f.state != nil {
		c := f.state.Campaign
		return &c, nil
	}
	return &domain.Campaign{}, nil
}

func makePipelineState(campaignID uuid.UUID) *game.GameState {
	locationID := uuid.New()
	return &game.GameState{
		Campaign: domain.Campaign{
			ID:          campaignID,
			Name:        "Campaign of Echoes",
			Description: "Track a fading signal through ancient ruins.",
			Genre:       "Fantasy",
			Tone:        "Tense",
		},
		Player: domain.PlayerCharacter{
			ID:                uuid.New(),
			Name:              "Mira",
			Level:             4,
			HP:                18,
			MaxHP:             20,
			Status:            "Alert",
			CurrentLocationID: &locationID,
		},
		CurrentLocation: domain.Location{
			ID:          locationID,
			Name:        "Signal Tower",
			Description: "A wind-scoured tower overlooking the valley.",
		},
		NearbyNPCs: []domain.NPC{
			{Name: "Caretaker Ivo", Description: "Watching the horizon.", Alive: true},
		},
	}
}

func makeRecentDomainLog(campaignID uuid.UUID, turn int, playerInput, response string) domain.SessionLog {
	return domain.SessionLog{
		ID:          uuid.New(),
		CampaignID:  campaignID,
		TurnNumber:  turn,
		PlayerInput: playerInput,
		InputType:   domain.Classify(playerInput),
		LLMResponse: response,
		ToolCalls:   json.RawMessage("[]"),
		CreatedAt:   time.Unix(int64(turn), 0).UTC(),
	}
}

func newPipelineTestEngine(state *fakeStateManager, provider llm.Provider, reg *tools.Registry) *Engine {
	return &Engine{
		state:     state,
		assembler: assembly.NewContextAssembler(reg),
		processor: NewTurnProcessor(provider, reg, tools.NewValidator(reg), nil),
	}
}

func registerStringTool(t *testing.T, reg *tools.Registry, name string, handler tools.Handler) {
	t.Helper()
	if err := reg.Register(llm.Tool{
		Name:        name,
		Description: "test tool " + name,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"value": map[string]any{"type": "string"},
			},
			"required": []any{"value"},
		},
	}, handler); err != nil {
		t.Fatalf("register %s: %v", name, err)
	}
}

func TestEngineProcessTurn_SimpleNarrativeAssemblesContext(t *testing.T) {
	campaignID := uuid.New()
	reg := tools.NewRegistry()
	registerStringTool(t, reg, "unused_tool", func(_ context.Context, _ map[string]any) (*tools.ToolResult, error) {
		return &tools.ToolResult{Success: true}, nil
	})

	provider := newScriptedProvider(t, scriptedResponse{
		resp: &llm.Response{Content: "The tower groans as the wind rolls through the stones."},
	})
	state := &fakeStateManager{
		state: makePipelineState(campaignID),
		recentLogs: []domain.SessionLog{
			makeRecentDomainLog(campaignID, 1, "Climb the tower stairs.", "You reach the tower summit."),
			makeRecentDomainLog(campaignID, 2, "Study the crackling beacon.", "The beacon spits blue sparks."),
		},
	}
	engine := newPipelineTestEngine(state, provider, reg)

	result, err := engine.ProcessTurn(context.Background(), campaignID, "Ask Ivo what he saw last night.")
	if err != nil {
		t.Fatalf("ProcessTurn() error = %v", err)
	}

	if result.Narrative != "The tower groans as the wind rolls through the stones." {
		t.Fatalf("Narrative = %q", result.Narrative)
	}
	if len(result.AppliedToolCalls) != 0 {
		t.Fatalf("AppliedToolCalls = %d, want 0", len(result.AppliedToolCalls))
	}
	if result.Choices != nil {
		t.Fatalf("Choices = %+v, want nil", result.Choices)
	}

	if len(state.gatheredCampaigns) != 1 || state.gatheredCampaigns[0] != campaignID {
		t.Fatalf("GatherState campaign IDs = %+v", state.gatheredCampaigns)
	}
	if len(provider.calls) != 1 {
		t.Fatalf("provider call count = %d, want 1", len(provider.calls))
	}

	call := provider.calls[0]
	if len(call.tools) != 1 || call.tools[0].Name != "unused_tool" {
		t.Fatalf("tools sent to provider = %+v", call.tools)
	}
	if len(call.messages) < 4 {
		t.Fatalf("message count = %d, want at least 4 (system + history + player input)", len(call.messages))
	}
	if call.messages[0].Role != llm.RoleSystem {
		t.Fatalf("first role = %q, want system", call.messages[0].Role)
	}
	if !strings.Contains(call.messages[0].Content, "Campaign of Echoes") ||
		!strings.Contains(call.messages[0].Content, "Mira") ||
		!strings.Contains(call.messages[0].Content, "Signal Tower") {
		t.Fatalf("system content missing assembled state: %q", call.messages[0].Content)
	}
	// Verify history appears in chronological order by scanning, not fixed indices.
	assertHistoryPairInOrder(t, call.messages,
		"Climb the tower stairs.", "You reach the tower summit.",
		"Study the crackling beacon.", "The beacon spits blue sparks.",
	)
	if last := call.messages[len(call.messages)-1]; last.Role != llm.RoleUser || last.Content != "Ask Ivo what he saw last night." {
		t.Fatalf("last message = %+v", last)
	}

	if len(state.savedLogs) != 1 {
		t.Fatalf("saved logs = %d, want 1", len(state.savedLogs))
	}
	if state.savedLogs[0].TurnNumber != 3 {
		t.Fatalf("saved turn number = %d, want 3", state.savedLogs[0].TurnNumber)
	}
	if string(state.savedLogs[0].ToolCalls) != "[]" {
		t.Fatalf("saved tool calls = %s, want []", state.savedLogs[0].ToolCalls)
	}
}

func TestEngineProcessTurn_ReturnsChoicesInTurnResult(t *testing.T) {
	campaignID := uuid.New()
	provider := newScriptedProvider(t, scriptedResponse{
		resp: &llm.Response{Content: `The signal pulses faster as night falls.

1. Inspect the cracked lens.
2. Question Ivo about the old keep.
3. Leave the tower and head for the valley.

Or describe what you'd like to do—you're never limited to the options above.`},
	})
	state := &fakeStateManager{state: makePipelineState(campaignID)}
	engine := newPipelineTestEngine(state, provider, tools.NewRegistry())

	result, err := engine.ProcessTurn(context.Background(), campaignID, "What are my options?")
	if err != nil {
		t.Fatalf("ProcessTurn() error = %v", err)
	}

	if result.Narrative != "The signal pulses faster as night falls." {
		t.Fatalf("Narrative = %q", result.Narrative)
	}
	if len(result.Choices) != 3 {
		t.Fatalf("Choices = %d, want 3", len(result.Choices))
	}
	if result.Choices[1].ID != "2" || result.Choices[1].Text != "Question Ivo about the old keep." {
		t.Fatalf("choice[1] = %+v", result.Choices[1])
	}
	if len(state.savedLogs) == 0 {
		t.Fatalf("expected at least one session log to be saved, got %d", len(state.savedLogs))
	}
	if state.savedLogs[0].LLMResponse != "The signal pulses faster as night falls." {
		t.Fatalf("saved log narrative = %q", state.savedLogs[0].LLMResponse)
	}
}

func TestEngineProcessTurn_ExecutesMultipleToolCallsAndAppliesStateChanges(t *testing.T) {
	campaignID := uuid.New()
	reg := tools.NewRegistry()
	fakeDB := struct {
		events []string
		flags  []string
	}{}

	registerStringTool(t, reg, "record_event", func(_ context.Context, args map[string]any) (*tools.ToolResult, error) {
		value := args["value"].(string)
		fakeDB.events = append(fakeDB.events, value)
		return &tools.ToolResult{Success: true, Data: map[string]any{"event": value}}, nil
	})
	registerStringTool(t, reg, "set_flag", func(_ context.Context, args map[string]any) (*tools.ToolResult, error) {
		value := args["value"].(string)
		fakeDB.flags = append(fakeDB.flags, value)
		return &tools.ToolResult{Success: true, Data: map[string]any{"flag": value}}, nil
	})

	provider := newScriptedProvider(t, scriptedResponse{
		resp: &llm.Response{
			Content: "You chalk a route onto the stones and mark the beacon as secured.",
			ToolCalls: []llm.ToolCall{
				{ID: "tc-1", Name: "record_event", Arguments: map[string]any{"value": "mapped secret route"}},
				{ID: "tc-2", Name: "set_flag", Arguments: map[string]any{"value": "beacon_secured"}},
			},
		},
	})
	state := &fakeStateManager{state: makePipelineState(campaignID)}
	engine := newPipelineTestEngine(state, provider, reg)

	result, err := engine.ProcessTurn(context.Background(), campaignID, "Secure the beacon and note the route.")
	if err != nil {
		t.Fatalf("ProcessTurn() error = %v", err)
	}

	if result.Narrative != "You chalk a route onto the stones and mark the beacon as secured." {
		t.Fatalf("Narrative = %q", result.Narrative)
	}
	if len(result.AppliedToolCalls) != 2 {
		t.Fatalf("AppliedToolCalls = %d, want 2", len(result.AppliedToolCalls))
	}
	if len(fakeDB.events) != 1 || fakeDB.events[0] != "mapped secret route" {
		t.Fatalf("event changes = %+v", fakeDB.events)
	}
	if len(fakeDB.flags) != 1 || fakeDB.flags[0] != "beacon_secured" {
		t.Fatalf("flag changes = %+v", fakeDB.flags)
	}

	if len(state.savedLogs) == 0 {
		t.Fatalf("no session logs were saved")
	}

	var savedCalls []AppliedToolCall
	if err := json.Unmarshal(state.savedLogs[0].ToolCalls, &savedCalls); err != nil {
		t.Fatalf("unmarshal saved tool calls: %v", err)
	}
	if len(savedCalls) != 2 {
		t.Fatalf("saved tool calls = %d, want 2", len(savedCalls))
	}
	if savedCalls[0].Tool != "record_event" || savedCalls[1].Tool != "set_flag" {
		t.Fatalf("saved tool call order = %+v", savedCalls)
	}
}

func TestEngineProcessTurn_RetriesBadToolCallAndAppliesCorrectedArguments(t *testing.T) {
	campaignID := uuid.New()
	reg := tools.NewRegistry()
	var appliedValues []string
	registerStringTool(t, reg, "set_flag", func(_ context.Context, args map[string]any) (*tools.ToolResult, error) {
		value := args["value"].(string)
		appliedValues = append(appliedValues, value)
		return &tools.ToolResult{Success: true, Data: map[string]any{"flag": value}}, nil
	})

	provider := newScriptedProvider(t,
		scriptedResponse{
			resp: &llm.Response{
				Content: "The mechanism grinds but does not fully lock.",
				ToolCalls: []llm.ToolCall{
					{ID: "bad-tool", Name: "set_flag", Arguments: map[string]any{}},
				},
			},
		},
		scriptedResponse{
			resp: &llm.Response{
				ToolCalls: []llm.ToolCall{
					{ID: "retry-tool", Name: "set_flag", Arguments: map[string]any{"value": "mechanism_locked"}},
				},
			},
		},
	)
	state := &fakeStateManager{state: makePipelineState(campaignID)}
	engine := newPipelineTestEngine(state, provider, reg)

	result, err := engine.ProcessTurn(context.Background(), campaignID, "Lock the mechanism.")
	if err != nil {
		t.Fatalf("ProcessTurn() error = %v", err)
	}

	if result.Narrative != "The mechanism grinds but does not fully lock." {
		t.Fatalf("Narrative = %q", result.Narrative)
	}
	if len(result.AppliedToolCalls) != 1 {
		t.Fatalf("AppliedToolCalls = %d, want 1", len(result.AppliedToolCalls))
	}
	if len(appliedValues) != 1 || appliedValues[0] != "mechanism_locked" {
		t.Fatalf("applied values = %+v", appliedValues)
	}
	if provider.callCount != 2 {
		t.Fatalf("provider call count = %d, want 2", provider.callCount)
	}
	if len(provider.calls) != 2 {
		t.Fatalf("provider calls recorded = %d, want 2", len(provider.calls))
	}

	retryCall := provider.calls[1]
	if got := retryCall.messages[len(retryCall.messages)-2]; got.Role != llm.RoleAssistant || len(got.ToolCalls) != 1 || got.ToolCalls[0].ID != "bad-tool" {
		t.Fatalf("retry assistant message = %+v", got)
	}
	if got := retryCall.messages[len(retryCall.messages)-1]; got.Role != llm.RoleTool || !strings.Contains(got.Content, "Please retry with corrected arguments.") {
		t.Fatalf("retry tool message = %+v", got)
	}
}

// assertHistoryPairInOrder verifies that user/assistant content pairs appear in
// order within messages, without assuming fixed indices. Pairs are provided as
// alternating user, assistant, user, assistant, ... strings.
func assertHistoryPairInOrder(t *testing.T, messages []llm.Message, pairs ...string) {
	t.Helper()
	if len(pairs)%2 != 0 {
		t.Fatal("pairs must be even (user, assistant, ...)")
	}

	cursor := 0
	for i := 0; i < len(pairs); i += 2 {
		userContent := pairs[i]
		assistantContent := pairs[i+1]
		found := false
		for j := cursor; j < len(messages)-1; j++ {
			if messages[j].Content == userContent && messages[j].Role == llm.RoleUser {
				if messages[j+1].Content == assistantContent && messages[j+1].Role == llm.RoleAssistant {
					cursor = j + 2
					found = true
					break
				}
			}
		}
		if !found {
			t.Fatalf("history pair not found in order: user=%q, assistant=%q", userContent, assistantContent)
		}
	}
}
