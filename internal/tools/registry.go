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
// Each returned Tool has a deep copy of its Parameters map so callers
// cannot mutate the registry's internal schema.
func (r *Registry) List() []llm.Tool {
	if r == nil || len(r.tools) == 0 {
		return nil
	}
	out := make([]llm.Tool, len(r.tools))
	for i, t := range r.tools {
		out[i] = llm.Tool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  deepCopyMap(t.Parameters),
		}
	}
	return out
}

// deepCopyMap recursively copies a map[string]any, including nested maps and
// slices of maps, so that mutations to the copy do not affect the original.
func deepCopyMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		switch val := v.(type) {
		case map[string]any:
			dst[k] = deepCopyMap(val)
		case []any:
			dst[k] = deepCopySlice(val)
		default:
			dst[k] = v
		}
	}
	return dst
}

// deepCopySlice recursively copies a []any, deep-copying any nested
// map[string]any or []any elements.
func deepCopySlice(src []any) []any {
	if src == nil {
		return nil
	}
	dst := make([]any, len(src))
	for i, v := range src {
		switch val := v.(type) {
		case map[string]any:
			dst[i] = deepCopyMap(val)
		case []any:
			dst[i] = deepCopySlice(val)
		default:
			dst[i] = v
		}
	}
	return dst
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
