package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/PatrickFanella/game-master/internal/llm"
)

// Handler executes a tool call and returns a JSON-serializable result.
type Handler func(ctx context.Context, args map[string]any) (map[string]any, error)

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

// Tools returns registered tool definitions in registration order.
func (r *Registry) Tools() []llm.Tool {
	if r == nil || len(r.tools) == 0 {
		return nil
	}
	out := make([]llm.Tool, len(r.tools))
	copy(out, r.tools)
	return out
}

// Invoke executes a registered tool by name.
func (r *Registry) Invoke(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
	if r == nil {
		return nil, errors.New("tool registry is nil")
	}
	h, ok := r.handlers[name]
	if !ok {
		return nil, fmt.Errorf("tool %q is not registered", name)
	}
	return h(ctx, args)
}
