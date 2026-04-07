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
	metas    map[string]ToolMeta
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Handler),
		metas:    make(map[string]ToolMeta),
	}
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

// RegisterWithMeta adds a tool definition, its handler, and associated metadata.
func (r *Registry) RegisterWithMeta(meta ToolMeta, handler Handler) error {
	if err := r.Register(meta.Definition, handler); err != nil {
		return err
	}
	r.metas[meta.Definition.Name] = meta
	return nil
}

// SetMeta associates metadata with an already-registered tool. This is useful
// when a tool is registered via a legacy RegisterX helper and metadata must be
// attached afterward. Returns an error if the tool name is not registered.
func (r *Registry) SetMeta(meta ToolMeta) error {
	if r == nil {
		return errors.New("tool registry is nil")
	}
	if _, ok := r.handlers[meta.Name]; !ok {
		return fmt.Errorf("cannot set meta for unregistered tool %q", meta.Name)
	}
	r.metas[meta.Name] = meta
	return nil
}

// GetMeta returns the metadata for a tool by name.
func (r *Registry) GetMeta(name string) (ToolMeta, bool) {
	if r == nil {
		return ToolMeta{}, false
	}
	m, ok := r.metas[name]
	return m, ok
}

// ListMeta returns metadata for all registered tools that have metadata.
func (r *Registry) ListMeta() []ToolMeta {
	if r == nil || len(r.metas) == 0 {
		return nil
	}
	out := make([]ToolMeta, 0, len(r.metas))
	// Return in registration order by iterating over the tools slice.
	for _, t := range r.tools {
		if m, ok := r.metas[t.Name]; ok {
			out = append(out, m)
		}
	}
	return out
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
