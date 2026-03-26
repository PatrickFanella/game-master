package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/PatrickFanella/game-master/internal/llm"
	"github.com/PatrickFanella/game-master/internal/tools"
)

// ---------------------------------------------------------------------------
// Mock LLM provider
// ---------------------------------------------------------------------------

// mockProvider is a simple sequence-based mock: each call to Complete returns
// the next pre-configured response. If more calls are made than responses
// provided the test fails.
type mockProvider struct {
	t         *testing.T
	responses []*llm.Response
	errors    []error
	callCount int
}

func (m *mockProvider) Complete(_ context.Context, _ []llm.Message, _ []llm.Tool) (*llm.Response, error) {
	if m.callCount >= len(m.responses) {
		m.t.Fatalf("mockProvider: unexpected call #%d (only %d response(s) configured)", m.callCount+1, len(m.responses))
	}
	resp := m.responses[m.callCount]
	err := m.errors[m.callCount]
	m.callCount++
	return resp, err
}

func (m *mockProvider) Stream(_ context.Context, _ []llm.Message, _ []llm.Tool) (<-chan llm.StreamChunk, error) {
	panic("Stream not implemented in mockProvider")
}

// newMockProvider creates a mock provider whose Complete method returns the
// supplied (response, error) pairs in order.
func newMockProvider(t *testing.T, pairs ...struct {
	resp *llm.Response
	err  error
}) *mockProvider {
	t.Helper()
	mp := &mockProvider{
		t:         t,
		responses: make([]*llm.Response, len(pairs)),
		errors:    make([]error, len(pairs)),
	}
	for i, p := range pairs {
		mp.responses[i] = p.resp
		mp.errors[i] = p.err
	}
	return mp
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildTestToolRegistry registers a single "mock_tool" that accepts a string
// argument "name". The handler counts calls and can be made to fail on the
// first N calls.
func buildProcessorTestRegistry(t *testing.T, failFirstN int) (*tools.Registry, *int) {
	t.Helper()
	reg := tools.NewRegistry()
	callCount := 0

	handler := func(_ context.Context, args map[string]any) (*tools.ToolResult, error) {
		callCount++
		if callCount <= failFirstN {
			return nil, errors.New("handler: simulated execution error")
		}
		name, _ := args["name"].(string)
		return &tools.ToolResult{
			Success:   true,
			Data:      map[string]any{"greeted": name},
			Narrative: "Tool executed successfully.",
		}, nil
	}

	if err := reg.Register(llm.Tool{
		Name:        "mock_tool",
		Description: "A test tool",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []any{"name"},
		},
	}, handler); err != nil {
		t.Fatalf("buildProcessorTestRegistry: %v", err)
	}

	return reg, &callCount
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestTurnProcessor_RetrySucceeds verifies acceptance criterion:
// "Unit test: first attempt fails, retry succeeds"
//
// Scenario:
//   - Initial LLM call returns a tool call with a bad "name" argument value.
//   - The registry handler fails on the first call (simulating a runtime error).
//   - The retry LLM call returns a corrected tool call.
//   - The handler succeeds on the second call.
//   - The applied result comes from the retried tool call.
//   - The narrative from the initial response is preserved.
func TestTurnProcessor_RetrySucceeds(t *testing.T) {
	reg, callCount := buildProcessorTestRegistry(t, 1 /* fail first 1 call */)
	validator := tools.NewValidator(reg)

	initialTC := llm.ToolCall{
		ID:        "tc-1",
		Name:      "mock_tool",
		Arguments: map[string]any{"name": "bad_value"},
	}
	correctedTC := llm.ToolCall{
		ID:        "tc-1-retry",
		Name:      "mock_tool",
		Arguments: map[string]any{"name": "good_value"},
	}

	provider := newMockProvider(t,
		struct {
			resp *llm.Response
			err  error
		}{
			resp: &llm.Response{
				Content:   "Once upon a time...",
				ToolCalls: []llm.ToolCall{initialTC},
			},
			err: nil,
		},
		struct {
			resp *llm.Response
			err  error
		}{
			resp: &llm.Response{
				Content:   "",
				ToolCalls: []llm.ToolCall{correctedTC},
			},
			err: nil,
		},
	)

	tp := NewTurnProcessor(provider, reg, validator)
	messages := []llm.Message{{Role: llm.RoleUser, Content: "Do something"}}

	narrative, applied, err := tp.ProcessWithRecovery(context.Background(), messages, reg.List())

	if err != nil {
		t.Fatalf("ProcessWithRecovery: unexpected error: %v", err)
	}
	if narrative != "Once upon a time..." {
		t.Errorf("narrative = %q, want %q", narrative, "Once upon a time...")
	}
	if len(applied) != 1 {
		t.Fatalf("len(applied) = %d, want 1", len(applied))
	}
	if applied[0].Tool != "mock_tool" {
		t.Errorf("applied[0].Tool = %q, want %q", applied[0].Tool, "mock_tool")
	}
	if provider.callCount != 2 {
		t.Errorf("provider.callCount = %d, want 2 (initial + one retry)", provider.callCount)
	}
	if *callCount != 2 {
		t.Errorf("handler call count = %d, want 2 (initial failure + retry success)", *callCount)
	}
}

// TestTurnProcessor_BothAttemptsFailToolSkipped verifies acceptance criteria:
// "Unit test: both attempts fail, tool skipped, narrative preserved"
//
// Scenario:
//   - Initial LLM call returns a tool call.
//   - The registry handler fails on both the first and second calls.
//   - The retry LLM call returns a corrected tool call.
//   - After both executions fail the tool call is skipped.
//   - The narrative from the initial response is still returned.
//   - AppliedToolCalls is empty.
func TestTurnProcessor_BothAttemptsFailToolSkipped(t *testing.T) {
	reg, callCount := buildProcessorTestRegistry(t, 2 /* fail first 2 calls */)
	validator := tools.NewValidator(reg)

	initialTC := llm.ToolCall{
		ID:        "tc-2",
		Name:      "mock_tool",
		Arguments: map[string]any{"name": "bad_value"},
	}
	retryTC := llm.ToolCall{
		ID:        "tc-2-retry",
		Name:      "mock_tool",
		Arguments: map[string]any{"name": "still_bad"},
	}

	provider := newMockProvider(t,
		struct {
			resp *llm.Response
			err  error
		}{
			resp: &llm.Response{
				Content:   "The hero stands ready.",
				ToolCalls: []llm.ToolCall{initialTC},
			},
			err: nil,
		},
		struct {
			resp *llm.Response
			err  error
		}{
			resp: &llm.Response{
				Content:   "",
				ToolCalls: []llm.ToolCall{retryTC},
			},
			err: nil,
		},
	)

	tp := NewTurnProcessor(provider, reg, validator)
	messages := []llm.Message{{Role: llm.RoleUser, Content: "Attack the dragon"}}

	narrative, applied, err := tp.ProcessWithRecovery(context.Background(), messages, reg.List())

	if err != nil {
		t.Fatalf("ProcessWithRecovery: unexpected error: %v", err)
	}
	if narrative != "The hero stands ready." {
		t.Errorf("narrative = %q, want %q", narrative, "The hero stands ready.")
	}
	if len(applied) != 0 {
		t.Errorf("len(applied) = %d, want 0 (failed tool call should be skipped)", len(applied))
	}
	if provider.callCount != 2 {
		t.Errorf("provider.callCount = %d, want 2 (initial + one retry)", provider.callCount)
	}
	if *callCount != 2 {
		t.Errorf("handler call count = %d, want 2 (first failure + retry failure)", *callCount)
	}
}

// TestTurnProcessor_SuccessfulToolCallApplied verifies that a tool call that
// succeeds on the first attempt is included in applied without a retry.
func TestTurnProcessor_SuccessfulToolCallApplied(t *testing.T) {
	reg, callCount := buildProcessorTestRegistry(t, 0 /* never fail */)
	validator := tools.NewValidator(reg)

	tc := llm.ToolCall{
		ID:        "tc-3",
		Name:      "mock_tool",
		Arguments: map[string]any{"name": "hero"},
	}

	provider := newMockProvider(t,
		struct {
			resp *llm.Response
			err  error
		}{
			resp: &llm.Response{
				Content:   "You swing your sword.",
				ToolCalls: []llm.ToolCall{tc},
			},
			err: nil,
		},
	)

	tp := NewTurnProcessor(provider, reg, validator)
	messages := []llm.Message{{Role: llm.RoleUser, Content: "Attack"}}

	narrative, applied, err := tp.ProcessWithRecovery(context.Background(), messages, reg.List())

	if err != nil {
		t.Fatalf("ProcessWithRecovery: unexpected error: %v", err)
	}
	if narrative != "You swing your sword." {
		t.Errorf("narrative = %q, want %q", narrative, "You swing your sword.")
	}
	if len(applied) != 1 {
		t.Fatalf("len(applied) = %d, want 1", len(applied))
	}
	if provider.callCount != 1 {
		t.Errorf("provider.callCount = %d, want 1 (no retry needed)", provider.callCount)
	}
	if *callCount != 1 {
		t.Errorf("handler call count = %d, want 1", *callCount)
	}
}

// TestTurnProcessor_NarrativePreservedWithMixedToolCalls verifies that
// successful tool calls are still applied even when sibling tool calls fail.
//
// Scenario:
//   - Initial response contains two tool calls: one succeeds, one fails.
//   - The retry for the failed call also fails.
//   - The successful call's result is returned; the failed one is skipped.
//   - The narrative is preserved.
func TestTurnProcessor_NarrativePreservedWithMixedToolCalls(t *testing.T) {
	reg := tools.NewRegistry()
	callsByName := map[string]int{}

	makeHandler := func(name string, alwaysFail bool) tools.Handler {
		return func(_ context.Context, args map[string]any) (*tools.ToolResult, error) {
			callsByName[name]++
			if alwaysFail {
				return nil, fmt.Errorf("handler %q: simulated failure", name)
			}
			return &tools.ToolResult{Success: true, Data: map[string]any{"tool": name}}, nil
		}
	}

	mustRegister := func(toolName string, alwaysFail bool) {
		if err := reg.Register(llm.Tool{
			Name: toolName,
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{"x": map[string]any{"type": "string"}},
				"required":   []any{"x"},
			},
		}, makeHandler(toolName, alwaysFail)); err != nil {
			t.Fatalf("Register %q: %v", toolName, err)
		}
	}

	mustRegister("good_tool", false)
	mustRegister("bad_tool", true)

	validator := tools.NewValidator(reg)

	goodTC := llm.ToolCall{ID: "tc-good", Name: "good_tool", Arguments: map[string]any{"x": "v1"}}
	badTC := llm.ToolCall{ID: "tc-bad", Name: "bad_tool", Arguments: map[string]any{"x": "v2"}}
	badRetryTC := llm.ToolCall{ID: "tc-bad-retry", Name: "bad_tool", Arguments: map[string]any{"x": "v3"}}

	provider := newMockProvider(t,
		struct {
			resp *llm.Response
			err  error
		}{
			resp: &llm.Response{
				Content:   "Narrative with mixed results.",
				ToolCalls: []llm.ToolCall{goodTC, badTC},
			},
			err: nil,
		},
		// Retry response for bad_tool.
		struct {
			resp *llm.Response
			err  error
		}{
			resp: &llm.Response{
				Content:   "",
				ToolCalls: []llm.ToolCall{badRetryTC},
			},
			err: nil,
		},
	)

	tp := NewTurnProcessor(provider, reg, validator)
	messages := []llm.Message{{Role: llm.RoleUser, Content: "Do mixed things"}}

	narrative, applied, err := tp.ProcessWithRecovery(context.Background(), messages, reg.List())

	if err != nil {
		t.Fatalf("ProcessWithRecovery: unexpected error: %v", err)
	}
	if narrative != "Narrative with mixed results." {
		t.Errorf("narrative = %q, want %q", narrative, "Narrative with mixed results.")
	}
	if len(applied) != 1 {
		t.Fatalf("len(applied) = %d, want 1 (only good_tool applied)", len(applied))
	}
	if applied[0].Tool != "good_tool" {
		t.Errorf("applied[0].Tool = %q, want %q", applied[0].Tool, "good_tool")
	}
}

// TestTurnProcessor_InitialLLMFailureReturnsError verifies that when the
// initial LLM call fails the error is propagated to the caller.
func TestTurnProcessor_InitialLLMFailureReturnsError(t *testing.T) {
	reg, _ := buildProcessorTestRegistry(t, 0)
	validator := tools.NewValidator(reg)

	provider := newMockProvider(t,
		struct {
			resp *llm.Response
			err  error
		}{
			resp: nil,
			err:  errors.New("network failure"),
		},
	)

	tp := NewTurnProcessor(provider, reg, validator)
	messages := []llm.Message{{Role: llm.RoleUser, Content: "Hello"}}

	_, _, err := tp.ProcessWithRecovery(context.Background(), messages, reg.List())

	if err == nil {
		t.Fatal("ProcessWithRecovery: expected error, got nil")
	}
}

// TestTurnProcessor_ValidationFailureTriggersRetry verifies that a
// pre-execution validation failure (missing required argument) also
// triggers the retry path.
func TestTurnProcessor_ValidationFailureTriggersRetry(t *testing.T) {
	reg, callCount := buildProcessorTestRegistry(t, 0 /* handler always succeeds */)
	validator := tools.NewValidator(reg)

	// Tool call missing required "name" argument → validation will fail.
	invalidTC := llm.ToolCall{
		ID:        "tc-val",
		Name:      "mock_tool",
		Arguments: map[string]any{}, // "name" is required but absent
	}
	correctedTC := llm.ToolCall{
		ID:        "tc-val-retry",
		Name:      "mock_tool",
		Arguments: map[string]any{"name": "corrected"},
	}

	provider := newMockProvider(t,
		struct {
			resp *llm.Response
			err  error
		}{
			resp: &llm.Response{
				Content:   "Validation retry narrative.",
				ToolCalls: []llm.ToolCall{invalidTC},
			},
			err: nil,
		},
		struct {
			resp *llm.Response
			err  error
		}{
			resp: &llm.Response{
				Content:   "",
				ToolCalls: []llm.ToolCall{correctedTC},
			},
			err: nil,
		},
	)

	tp := NewTurnProcessor(provider, reg, validator)
	messages := []llm.Message{{Role: llm.RoleUser, Content: "Do the thing"}}

	narrative, applied, err := tp.ProcessWithRecovery(context.Background(), messages, reg.List())

	if err != nil {
		t.Fatalf("ProcessWithRecovery: unexpected error: %v", err)
	}
	if narrative != "Validation retry narrative." {
		t.Errorf("narrative = %q, want %q", narrative, "Validation retry narrative.")
	}
	if len(applied) != 1 {
		t.Fatalf("len(applied) = %d, want 1", len(applied))
	}
	if provider.callCount != 2 {
		t.Errorf("provider.callCount = %d, want 2 (initial + one retry)", provider.callCount)
	}
	if *callCount != 1 {
		t.Errorf("handler call count = %d, want 1 (only retry succeeds)", *callCount)
	}
}

// TestTurnProcessor_NoToolCalls verifies that when the LLM returns no tool
// calls, the narrative is returned and applied is nil.
func TestTurnProcessor_NoToolCalls(t *testing.T) {
	reg, _ := buildProcessorTestRegistry(t, 0)
	validator := tools.NewValidator(reg)

	provider := newMockProvider(t,
		struct {
			resp *llm.Response
			err  error
		}{
			resp: &llm.Response{
				Content:   "Pure narrative, no tools.",
				ToolCalls: nil,
			},
			err: nil,
		},
	)

	tp := NewTurnProcessor(provider, reg, validator)
	messages := []llm.Message{{Role: llm.RoleUser, Content: "Describe the scene"}}

	narrative, applied, err := tp.ProcessWithRecovery(context.Background(), messages, reg.List())

	if err != nil {
		t.Fatalf("ProcessWithRecovery: unexpected error: %v", err)
	}
	if narrative != "Pure narrative, no tools." {
		t.Errorf("narrative = %q, want %q", narrative, "Pure narrative, no tools.")
	}
	if len(applied) != 0 {
		t.Errorf("len(applied) = %d, want 0", len(applied))
	}
	if provider.callCount != 1 {
		t.Errorf("provider.callCount = %d, want 1", provider.callCount)
	}
}

// TestTurnProcessor_RetryLLMCallFails verifies that when the retry LLM call
// itself returns an error (e.g. network failure during the retry), the tool
// call is still skipped and the turn completes without a top-level error.
// The narrative from the initial response is preserved.
func TestTurnProcessor_RetryLLMCallFails(t *testing.T) {
	reg, callCount := buildProcessorTestRegistry(t, 999 /* always fail */)
	validator := tools.NewValidator(reg)

	initialTC := llm.ToolCall{
		ID:        "tc-rllm",
		Name:      "mock_tool",
		Arguments: map[string]any{"name": "bad"},
	}

	provider := newMockProvider(t,
		// Initial call succeeds but tool call will fail execution.
		struct {
			resp *llm.Response
			err  error
		}{
			resp: &llm.Response{
				Content:   "Retry LLM failure narrative.",
				ToolCalls: []llm.ToolCall{initialTC},
			},
			err: nil,
		},
		// Retry LLM call itself fails.
		struct {
			resp *llm.Response
			err  error
		}{
			resp: nil,
			err:  errors.New("network failure during retry"),
		},
	)

	tp := NewTurnProcessor(provider, reg, validator)
	messages := []llm.Message{{Role: llm.RoleUser, Content: "Do something"}}

	narrative, applied, err := tp.ProcessWithRecovery(context.Background(), messages, reg.List())

	// The top-level call must not fail – the retry LLM error is swallowed and logged.
	if err != nil {
		t.Fatalf("ProcessWithRecovery: unexpected error: %v", err)
	}
	if narrative != "Retry LLM failure narrative." {
		t.Errorf("narrative = %q, want %q", narrative, "Retry LLM failure narrative.")
	}
	if len(applied) != 0 {
		t.Errorf("len(applied) = %d, want 0 (tool call should be skipped)", len(applied))
	}
	if provider.callCount != 2 {
		t.Errorf("provider.callCount = %d, want 2 (initial + failed retry attempt)", provider.callCount)
	}
	// The handler must be called exactly once (initial attempt), not during retry
	// since the retry LLM call never returned a corrected tool call.
	if *callCount != 1 {
		t.Errorf("handler call count = %d, want 1", *callCount)
	}
}

// TestTurnProcessor_HallucinatedToolCallSkipped verifies that a tool call
// whose name is not in the advertised availableTools list is rejected as a
// hallucination and follows the retry-then-skip path even if the name happens
// to exist in the registry.
func TestTurnProcessor_HallucinatedToolCallSkipped(t *testing.T) {
	reg := tools.NewRegistry()

	if err := reg.Register(llm.Tool{
		Name: "real_tool",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"x": map[string]any{"type": "string"}},
			"required":   []any{"x"},
		},
	}, func(_ context.Context, _ map[string]any) (*tools.ToolResult, error) {
		return &tools.ToolResult{Success: true}, nil
	}); err != nil {
		t.Fatalf("Register real_tool: %v", err)
	}

	if err := reg.Register(llm.Tool{
		Name: "secret_tool",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"x": map[string]any{"type": "string"}},
			"required":   []any{"x"},
		},
	}, func(_ context.Context, _ map[string]any) (*tools.ToolResult, error) {
		t.Error("secret_tool handler must never be called")
		return nil, errors.New("should not be called")
	}); err != nil {
		t.Fatalf("Register secret_tool: %v", err)
	}

	validator := tools.NewValidator(reg)

	// Advertise only real_tool to the LLM; secret_tool is in the registry
	// but not in the available set.
	advertised := []llm.Tool{reg.List()[0]} // real_tool only

	hallucinatedTC := llm.ToolCall{
		ID:        "tc-halluc",
		Name:      "secret_tool",
		Arguments: map[string]any{"x": "v"},
	}
	// Retry response also returns no corrected tool call for real_tool.
	retryTC := llm.ToolCall{
		ID:        "tc-halluc-retry",
		Name:      "secret_tool",
		Arguments: map[string]any{"x": "v2"},
	}

	provider := newMockProvider(t,
		struct {
			resp *llm.Response
			err  error
		}{
			resp: &llm.Response{
				Content:   "Hallucination narrative.",
				ToolCalls: []llm.ToolCall{hallucinatedTC},
			},
			err: nil,
		},
		struct {
			resp *llm.Response
			err  error
		}{
			resp: &llm.Response{
				Content:   "",
				ToolCalls: []llm.ToolCall{retryTC},
			},
			err: nil,
		},
	)

	tp := NewTurnProcessor(provider, reg, validator)
	messages := []llm.Message{{Role: llm.RoleUser, Content: "Do the thing"}}

	narrative, applied, err := tp.ProcessWithRecovery(context.Background(), messages, advertised)

	if err != nil {
		t.Fatalf("ProcessWithRecovery: unexpected error: %v", err)
	}
	if narrative != "Hallucination narrative." {
		t.Errorf("narrative = %q, want %q", narrative, "Hallucination narrative.")
	}
	if len(applied) != 0 {
		t.Errorf("len(applied) = %d, want 0 (hallucinated tool must be skipped)", len(applied))
	}
}
