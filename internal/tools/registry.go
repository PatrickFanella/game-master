package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/PatrickFanella/game-master/internal/llm"
)

// ToolResult holds the outcome of a tool invocation.
type ToolResult struct {
	// Success indicates whether the tool call completed successfully.
	Success bool
	// Data contains the structured result returned by the tool.
	Data map[string]any
	// Narrative is an optional human-readable description of the result,
	// suitable for inclusion in the LLM response.
	Narrative string
}

// Handler executes a tool call and returns a ToolResult.
type Handler func(ctx context.Context, args map[string]any) (*ToolResult, error)

// Registry stores tool definitions and their handlers.
type Registry struct {
	tools    []llm.Tool
	handlers map[string]Handler
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

// Register adds a tool definition and its handler.
func (r *Registry) Register(tool llm.Tool, handler Handler) error {
	if r == nil {
		return errors.New("tool registry is nil")
	}
	if tool.Name == "" {
		return errors.New("tool name is required")
	}
	if handler == nil {
		return errors.New("tool handler is required")
	}
	if _, exists := r.handlers[tool.Name]; exists {
		return fmt.Errorf("tool %q is already registered", tool.Name)
	}

	r.tools = append(r.tools, tool)
	r.handlers[tool.Name] = handler
	return nil
}

// List returns registered tool definitions in registration order,
// in the llm.Tool format suitable for passing to an LLM provider call.
func (r *Registry) List() []llm.Tool {
	if r == nil || len(r.tools) == 0 {
		return nil
	}
	out := make([]llm.Tool, len(r.tools))
	copy(out, r.tools)
	return out
}

// Execute looks up a handler by tool name and invokes it with the given
// arguments. Returns a descriptive error if the tool name is not registered.
func (r *Registry) Execute(ctx context.Context, name string, args map[string]any) (*ToolResult, error) {
	if r == nil {
		return nil, errors.New("tool registry is nil")
	}
	h, ok := r.handlers[name]
	if !ok {
		return nil, fmt.Errorf("tool %q is not registered", name)
	}
	return h(ctx, args)
}
