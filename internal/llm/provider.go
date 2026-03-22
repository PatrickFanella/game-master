// Package llm defines the provider-agnostic interface and types for
// interacting with large-language-model backends.
package llm

import "context"

// Role identifies the sender of a chat message.
type Role string

const (
	// RoleSystem is used for system-level instructions.
	RoleSystem Role = "system"
	// RoleUser is used for user-supplied prompts.
	RoleUser Role = "user"
	// RoleAssistant is used for model-generated replies.
	RoleAssistant Role = "assistant"
	// RoleTool is used for tool/function result messages.
	RoleTool Role = "tool"
)

// Message represents a single message in a chat conversation.
type Message struct {
	// Role indicates who sent the message.
	Role Role
	// Content is the textual body of the message.
	Content string
	// ToolCalls contains any tool invocations requested by the model.
	ToolCalls []ToolCall
	// ToolCallID identifies the tool call this message is responding to.
	// It is set when Role is RoleTool.
	ToolCallID string
}

// Tool describes a function that the model may invoke.
type Tool struct {
	// Name is the unique identifier of the tool.
	Name string
	// Description explains what the tool does.
	Description string
	// Parameters is a JSON-Schema object describing the tool's parameters.
	Parameters map[string]any
}

// ToolCall represents a single tool invocation requested by the model.
type ToolCall struct {
	// ID is a unique identifier for this invocation.
	ID string
	// Name is the tool to invoke.
	Name string
	// Arguments contains the key-value arguments for the tool.
	Arguments map[string]any
}

// Usage reports token consumption for a completion request.
type Usage struct {
	// PromptTokens is the number of tokens in the prompt.
	PromptTokens int
	// CompletionTokens is the number of tokens in the completion.
	CompletionTokens int
	// TotalTokens is the sum of prompt and completion tokens.
	TotalTokens int
}

// Response is the result of a synchronous completion request.
type Response struct {
	// Content is the textual reply from the model.
	Content string
	// ToolCalls contains any tool invocations the model requested.
	ToolCalls []ToolCall
	// FinishReason indicates why the model stopped generating.
	FinishReason string
	// Usage reports token consumption for the request.
	Usage Usage
}

// StreamChunk is a single incremental piece of a streamed response.
//
// Provider implementations must send a final chunk with Done set to true and
// then close the channel. Callers should range over the channel; the loop
// will naturally end when the channel is closed. The Done flag on the last
// chunk lets callers distinguish a clean finish from an unexpected close.
type StreamChunk struct {
	// ContentDelta is the new text fragment in this chunk.
	ContentDelta string
	// ToolCallDelta contains incremental tool-call data, if any.
	ToolCallDelta *ToolCall
	// Done indicates the stream is complete. Implementations must send
	// exactly one chunk with Done set to true as the last item before
	// closing the channel.
	Done bool
}

// Provider defines the interface that every LLM backend must implement.
type Provider interface {
	// Complete sends a chat completion request and blocks until the full
	// response is available.
	Complete(ctx context.Context, messages []Message, tools []Tool) (*Response, error)
	// Stream sends a chat completion request and returns a channel that
	// delivers incremental response chunks. Implementations must send a
	// final StreamChunk with Done set to true and then close the channel.
	// Callers should range over the returned channel to consume all chunks.
	Stream(ctx context.Context, messages []Message, tools []Tool) (<-chan StreamChunk, error)
}
