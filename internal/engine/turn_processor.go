package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/PatrickFanella/game-master/internal/llm"
	"github.com/PatrickFanella/game-master/internal/tools"
	"github.com/PatrickFanella/game-master/pkg/api"
)

// TurnProcessor handles the tool-call portion of the turn pipeline with
// built-in error recovery. When a tool call fails validation or execution
// it sends the error back to the LLM and retries once. If the retry also
// fails the tool call is skipped; narrative text and all successful tool
// calls from the same response are still returned.
// StatusCallback is an optional function called when the turn processor
// transitions between processing stages. It is safe to set to nil.
type StatusCallback func(api.StatusPayload)

// TurnProcessor handles the tool-call portion of the turn pipeline with
// built-in error recovery and optional status callbacks.
type TurnProcessor struct {
	logger         *slog.Logger
	provider       llm.Provider
	registry       *tools.Registry
	validator      *tools.Validator
	StatusCallback StatusCallback
}

// NewTurnProcessor creates a TurnProcessor backed by the given LLM provider,
// tool registry, and validator.
func NewTurnProcessor(
	provider llm.Provider,
	registry *tools.Registry,
	validator *tools.Validator,
	logger *slog.Logger,
) *TurnProcessor {
	if logger == nil {
		logger = slog.Default()
	}
	return &TurnProcessor{
		logger:    logger,
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
	started := time.Now()
	tp.logger.Debug("turn processor started", "messages", len(messages), "tools", len(availableTools))
	tp.emitStatus(api.StatusPayload{Stage: "thinking", Description: "Generating response..."})
	resp, err := tp.provider.Complete(ctx, messages, availableTools)
	if err != nil {
		tp.logger.Error("turn processor initial llm call failed", "duration_ms", time.Since(started).Milliseconds(), "error", err)
		return "", nil, fmt.Errorf("initial LLM call failed: %w", err)
	}

	narrative = resp.Content
	tp.logger.Debug("turn processor initial llm response", "tool_calls", len(resp.ToolCalls), "narrative_len", len(narrative))
	if len(resp.ToolCalls) == 0 {
		tp.logger.Debug("turn processor completed without tool calls", "duration_ms", time.Since(started).Milliseconds())
		return narrative, nil, nil
	}

	allowed := make(map[string]struct{}, len(availableTools))
	for _, t := range availableTools {
		allowed[t.Name] = struct{}{}
	}

	tp.emitStatus(api.StatusPayload{Stage: "tools", Description: "Executing tool calls..."})
	assistantContent := resp.Content
	for _, tc := range resp.ToolCalls {
		tp.emitStatus(api.StatusPayload{Stage: "tool_execution", Tool: tc.Name, Description: fmt.Sprintf("Executing %s...", tc.Name)})
		tp.logger.Debug("attempting tool call", "tool", tc.Name, "tool_call_id", tc.ID)
		result, execErr := tp.attemptToolCall(ctx, tc, allowed)
		if execErr == nil {
			if atc, encErr := buildAppliedToolCall(tc, result); encErr != nil {
				tp.logger.Error("failed to encode applied tool call; skipping",
					"tool", tc.Name,
					"tool_call_id", tc.ID,
					"error", encErr.Error(),
				)
			} else {
				applied = append(applied, atc)
				tp.logger.Debug("tool call applied", "tool", tc.Name, "tool_call_id", tc.ID)
			}
			continue
		}

		tp.logger.Warn("tool call failed; requesting retry", "tool", tc.Name, "tool_call_id", tc.ID, "error", execErr.Error())
		retryTC, retryLLMErr := tp.requestRetry(ctx, tc, execErr, messages, assistantContent, availableTools)
		if retryLLMErr != nil {
			tp.logger.Error("tool call failed and retry LLM call also failed; skipping",
				"tool", tc.Name,
				"tool_call_id", tc.ID,
				"initial_error", execErr.Error(),
				"retry_llm_error", retryLLMErr.Error(),
			)
			continue
		}

		tp.logger.Debug("retry tool call received", "tool", retryTC.Name, "tool_call_id", retryTC.ID)
		retryResult, retryExecErr := tp.attemptToolCall(ctx, retryTC, allowed)
		if retryExecErr != nil {
			tp.logger.Error("tool call failed after retry; skipping",
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
			tp.logger.Error("failed to encode applied tool call after retry; skipping",
				"tool", retryTC.Name,
				"tool_call_id", retryTC.ID,
				"error", encErr.Error(),
			)
		} else {
			applied = append(applied, atc)
			tp.logger.Debug("tool call applied after retry", "tool", retryTC.Name, "tool_call_id", retryTC.ID)
		}
	}

	// If the model returned tool calls but no narrative text, send the tool
	// results back so it can generate the narrative based on outcomes.
	if narrative == "" && len(applied) > 0 {
		tp.logger.Debug("no narrative after tool calls; issuing continuation call", "applied_tool_calls", len(applied))
		narrative, err = tp.requestContinuation(ctx, messages, resp, applied, availableTools)
		if err != nil {
			tp.logger.Error("continuation call failed", "error", err)
			// Non-fatal: return what we have (empty narrative + applied tools).
		}
	}

	tp.logger.Debug("turn processor completed", "duration_ms", time.Since(started).Milliseconds(), "applied_tool_calls", len(applied), "narrative_len", len(narrative))
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
	tp.logger.Debug("tool execution succeeded", "tool", tc.Name, "tool_call_id", tc.ID)
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
	retryStarted := time.Now()
	retryMessages := make([]llm.Message, len(originalMessages), len(originalMessages)+2)
	copy(retryMessages, originalMessages)

	retryMessages = append(retryMessages, llm.Message{
		Role:      llm.RoleAssistant,
		Content:   assistantContent,
		ToolCalls: []llm.ToolCall{failedTC},
	})
	retryMessages = append(retryMessages, llm.Message{
		Role:       llm.RoleTool,
		Content:    fmt.Sprintf("Error: %s. Please retry with corrected arguments.", execErr.Error()),
		ToolCallID: failedTC.ID,
	})

	tp.logger.Debug("issuing retry llm call", "tool", failedTC.Name, "tool_call_id", failedTC.ID, "messages", len(retryMessages))
	retryResp, err := tp.provider.Complete(ctx, retryMessages, availableTools)
	if err != nil {
		tp.logger.Error("retry llm call failed", "tool", failedTC.Name, "tool_call_id", failedTC.ID, "duration_ms", time.Since(retryStarted).Milliseconds(), "error", err)
		return llm.ToolCall{}, fmt.Errorf("retry LLM call: %w", err)
	}

	for _, tc := range retryResp.ToolCalls {
		if tc.Name == failedTC.Name {
			tp.logger.Debug("retry llm call completed", "tool", failedTC.Name, "tool_call_id", failedTC.ID, "duration_ms", time.Since(retryStarted).Milliseconds(), "returned_tool_calls", len(retryResp.ToolCalls))
			return tc, nil
		}
	}
	if len(retryResp.ToolCalls) > 0 {
		tp.logger.Debug("retry llm call completed with fallback tool", "tool", failedTC.Name, "tool_call_id", failedTC.ID, "duration_ms", time.Since(retryStarted).Milliseconds(), "returned_tool_calls", len(retryResp.ToolCalls))
		return retryResp.ToolCalls[0], nil
	}

	return llm.ToolCall{}, fmt.Errorf(
		"LLM returned no tool calls in retry response for tool %q (tool_call_id=%s)",
		failedTC.Name,
		failedTC.ID,
	)
}

// requestContinuation sends tool results back to the LLM so it can produce
// narrative text based on the outcomes. This handles models (like Gemma) that
// emit tool calls without accompanying text.
func (tp *TurnProcessor) requestContinuation(
	ctx context.Context,
	originalMessages []llm.Message,
	initialResp *llm.Response,
	applied []AppliedToolCall,
	availableTools []llm.Tool,
) (string, error) {
	contMessages := make([]llm.Message, len(originalMessages), len(originalMessages)+1+len(applied))
	copy(contMessages, originalMessages)

	// Add the assistant message with tool calls.
	contMessages = append(contMessages, llm.Message{
		Role:      llm.RoleAssistant,
		Content:   initialResp.Content,
		ToolCalls: initialResp.ToolCalls,
	})

	// Add a tool-result message for each applied tool call.
	for _, atc := range applied {
		contMessages = append(contMessages, llm.Message{
			Role:       llm.RoleTool,
			Content:    string(atc.Result),
			ToolCallID: atc.Tool, // use tool name as fallback ID when ID is empty
		})
	}

	resp, err := tp.provider.Complete(ctx, contMessages, availableTools)
	if err != nil {
		return "", fmt.Errorf("continuation LLM call: %w", err)
	}
	tp.logger.Debug("continuation call completed", "narrative_len", len(resp.Content), "tool_calls", len(resp.ToolCalls))
	return resp.Content, nil
}

// emitStatus calls the status callback if set.
func (tp *TurnProcessor) emitStatus(s api.StatusPayload) {
	if tp.StatusCallback != nil {
		tp.StatusCallback(s)
	}
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
