package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/PatrickFanella/game-master/internal/llm"
	"github.com/PatrickFanella/game-master/internal/tools"
)

// TurnProcessor handles the tool-call portion of the turn pipeline with
// built-in error recovery. When a tool call fails validation or execution
// it sends the error back to the LLM and retries once. If the retry also
// fails the tool call is skipped; narrative text and all successful tool
// calls from the same response are still returned.
type TurnProcessor struct {
	provider  llm.Provider
	registry  *tools.Registry
	validator *tools.Validator
}

// NewTurnProcessor creates a TurnProcessor backed by the given LLM provider,
// tool registry, and validator.
func NewTurnProcessor(
	provider llm.Provider,
	registry *tools.Registry,
	validator *tools.Validator,
) *TurnProcessor {
	return &TurnProcessor{
		provider:  provider,
		registry:  registry,
		validator: validator,
	}
}

// ProcessWithRecovery sends messages to the LLM provider, then executes
// every tool call in the response. For each tool call that fails validation
// or execution it:
//  1. Sends the error back to the LLM (together with the original context)
//     and requests exactly one retry.
//  2. If the retry also fails, skips the tool call and logs the failure at
//     ERROR level with full context.
//
// Only tool calls whose names are present in availableTools are executed;
// tool calls for tools not in that set are treated as validation failures and
// follow the same retry-then-skip path. This prevents hallucinated tool calls
// from being dispatched even if they happen to exist in the registry.
//
// Narrative text from the initial response is always returned regardless of
// tool call outcomes. Successful tool calls – including successful retries –
// are collected in the returned slice.
//
// The function only returns a non-nil error when the initial LLM call itself
// fails; individual tool call failures are handled via retry-then-skip.
func (tp *TurnProcessor) ProcessWithRecovery(
	ctx context.Context,
	messages []llm.Message,
	availableTools []llm.Tool,
) (narrative string, applied []AppliedToolCall, err error) {
	// Initial LLM call.
	resp, err := tp.provider.Complete(ctx, messages, availableTools)
	if err != nil {
		return "", nil, fmt.Errorf("initial LLM call failed: %w", err)
	}

	narrative = resp.Content

	if len(resp.ToolCalls) == 0 {
		return narrative, nil, nil
	}

	// Build an allowlist from the tools actually advertised to the LLM so
	// that hallucinated tool names are rejected before execution.
	allowed := make(map[string]struct{}, len(availableTools))
	for _, t := range availableTools {
		allowed[t.Name] = struct{}{}
	}

	// Build the assistant message for retry context. When retrying a specific
	// failed tool call we include only that call so the LLM can focus on
	// correcting it without being confused by sibling calls.
	assistantContent := resp.Content

	for _, tc := range resp.ToolCalls {
		result, execErr := tp.attemptToolCall(ctx, tc, allowed)
		if execErr == nil {
			if atc, encErr := buildAppliedToolCall(tc, result); encErr != nil {
				slog.Error("failed to encode applied tool call; skipping",
					"tool", tc.Name,
					"tool_call_id", tc.ID,
					"error", encErr.Error(),
				)
			} else {
				applied = append(applied, atc)
			}
			continue
		}

		// First attempt failed – request one retry from the LLM.
		retryTC, retryLLMErr := tp.requestRetry(
			ctx, tc, execErr, messages, assistantContent, availableTools,
		)
		if retryLLMErr != nil {
			slog.Error("tool call failed and retry LLM call also failed; skipping",
				"tool", tc.Name,
				"tool_call_id", tc.ID,
				"initial_error", execErr.Error(),
				"retry_llm_error", retryLLMErr.Error(),
			)
			continue
		}

		retryResult, retryExecErr := tp.attemptToolCall(ctx, retryTC, allowed)
		if retryExecErr != nil {
			slog.Error("tool call failed after retry; skipping",
				"tool", tc.Name,
				"tool_call_id", tc.ID,
				"retry_tool_call_id", retryTC.ID,
				"initial_error", execErr.Error(),
				"retry_error", retryExecErr.Error(),
				"retry_arguments", retryTC.Arguments,
			)
			continue
		}

		if atc, encErr := buildAppliedToolCall(retryTC, retryResult); encErr != nil {
			slog.Error("failed to encode applied tool call after retry; skipping",
				"tool", retryTC.Name,
				"tool_call_id", retryTC.ID,
				"error", encErr.Error(),
			)
		} else {
			applied = append(applied, atc)
		}
	}

	return narrative, applied, nil
}

// attemptToolCall validates and executes a single tool call. The allowed set
// is derived from the tools advertised to the LLM; tool calls whose names are
// not in the set are rejected as hallucinations before registry lookup.
// Both validation and execution errors are returned as-is so callers can
// include them in log messages or retry prompts.
func (tp *TurnProcessor) attemptToolCall(ctx context.Context, tc llm.ToolCall, allowed map[string]struct{}) (*tools.ToolResult, error) {
	if _, ok := allowed[tc.Name]; !ok {
		return nil, fmt.Errorf("validation: tool %q was not in the advertised tool list", tc.Name)
	}
	if err := tp.validator.ValidatePreExecution(tc); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}
	result, err := tp.registry.Execute(ctx, tc.Name, tc.Arguments)
	if err != nil {
		return nil, fmt.Errorf("execution: %w", err)
	}
	return result, nil
}

// requestRetry builds a retry conversation and calls the LLM once. The
// returned ToolCall is the corrected invocation suggested by the model.
//
// The retry context is:
//  1. The original conversation messages.
//  2. An assistant turn containing only the failed tool call.
//  3. A tool-result turn carrying the error message.
func (tp *TurnProcessor) requestRetry(
	ctx context.Context,
	failedTC llm.ToolCall,
	execErr error,
	originalMessages []llm.Message,
	assistantContent string,
	availableTools []llm.Tool,
) (llm.ToolCall, error) {
	retryMessages := make([]llm.Message, len(originalMessages), len(originalMessages)+2)
	copy(retryMessages, originalMessages)

	// Assistant message with only the single failed tool call.
	retryMessages = append(retryMessages, llm.Message{
		Role:      llm.RoleAssistant,
		Content:   assistantContent,
		ToolCalls: []llm.ToolCall{failedTC},
	})

	// Tool-result message carrying the error so the LLM can correct it.
	retryMessages = append(retryMessages, llm.Message{
		Role:       llm.RoleTool,
		Content:    fmt.Sprintf("Error: %s. Please retry with corrected arguments.", execErr.Error()),
		ToolCallID: failedTC.ID,
	})

	retryResp, err := tp.provider.Complete(ctx, retryMessages, availableTools)
	if err != nil {
		return llm.ToolCall{}, fmt.Errorf("retry LLM call: %w", err)
	}

	// Prefer a tool call with the same name; fall back to the first available.
	for _, tc := range retryResp.ToolCalls {
		if tc.Name == failedTC.Name {
			return tc, nil
		}
	}
	if len(retryResp.ToolCalls) > 0 {
		return retryResp.ToolCalls[0], nil
	}

	return llm.ToolCall{}, fmt.Errorf(
		"LLM returned no tool calls in retry response for tool %q (tool_call_id=%s)",
		failedTC.Name,
		failedTC.ID,
	)
}

// buildAppliedToolCall converts a raw tool call and its result into the
// engine's AppliedToolCall type, JSON-encoding the arguments and result data.
func buildAppliedToolCall(tc llm.ToolCall, result *tools.ToolResult) (AppliedToolCall, error) {
	argsJSON, err := json.Marshal(tc.Arguments)
	if err != nil {
		return AppliedToolCall{}, fmt.Errorf("marshal tool call arguments: %w", err)
	}

	var resultData map[string]any
	if result != nil {
		resultData = result.Data
	}
	resultJSON, err := json.Marshal(resultData)
	if err != nil {
		return AppliedToolCall{}, fmt.Errorf("marshal tool call result: %w", err)
	}

	return AppliedToolCall{
		Tool:      tc.Name,
		Arguments: json.RawMessage(argsJSON),
		Result:    json.RawMessage(resultJSON),
	}, nil
}
