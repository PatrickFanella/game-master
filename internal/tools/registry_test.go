package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/PatrickFanella/game-master/internal/llm"
)

func TestNewRegistry(t *testing.T) {
	reg := NewRegistry()
	if reg == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if tools := reg.List(); tools != nil {
		t.Fatalf("new registry List = %v, want nil", tools)
	}
}

func TestRegisterAndList(t *testing.T) {
	reg := NewRegistry()
	tool := llm.Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters:  map[string]any{"type": "object"},
	}
	handler := func(_ context.Context, _ map[string]any) (*ToolResult, error) {
		return &ToolResult{Success: true, Data: map[string]any{"ok": true}}, nil
	}

	if err := reg.Register(tool, handler); err != nil {
		t.Fatalf("Register: %v", err)
	}

	tools := reg.List()
	if len(tools) != 1 {
		t.Fatalf("List returned %d tools, want 1", len(tools))
	}
	if tools[0].Name != "test_tool" {
		t.Fatalf("List[0].Name = %q, want %q", tools[0].Name, "test_tool")
	}
}

func TestListReturnsCopy(t *testing.T) {
	reg := NewRegistry()
	tool := llm.Tool{
		Name: "copy_tool",
		Parameters: map[string]any{
			"type": "object",
			"nested": map[string]any{
				"key": "original",
			},
		},
	}
	handler := func(_ context.Context, _ map[string]any) (*ToolResult, error) {
		return &ToolResult{Success: true}, nil
	}
	if err := reg.Register(tool, handler); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Mutating the returned slice element's Name must not affect the registry.
	list1 := reg.List()
	list1[0].Name = "mutated"
	list2 := reg.List()
	if list2[0].Name != "copy_tool" {
		t.Fatalf("List[0].Name = %q after Name mutation, want %q", list2[0].Name, "copy_tool")
	}

	// Mutating a top-level Parameters entry must not affect the registry.
	list3 := reg.List()
	list3[0].Parameters["type"] = "mutated-type"
	list4 := reg.List()
	if list4[0].Parameters["type"] != "object" {
		t.Fatalf("Parameters[\"type\"] = %v after top-level mutation, want %q", list4[0].Parameters["type"], "object")
	}

	// Mutating a nested Parameters map must not affect the registry.
	list5 := reg.List()
	nested, _ := list5[0].Parameters["nested"].(map[string]any)
	nested["key"] = "mutated-nested"
	list6 := reg.List()
	nested2, _ := list6[0].Parameters["nested"].(map[string]any)
	if nested2["key"] != "original" {
		t.Fatalf("Parameters[\"nested\"][\"key\"] = %v after nested mutation, want %q", nested2["key"], "original")
	}
}

func TestListRegistrationOrder(t *testing.T) {
	reg := NewRegistry()
	names := []string{"alpha", "beta", "gamma"}
	for _, name := range names {
		n := name
		err := reg.Register(
			llm.Tool{Name: n, Parameters: map[string]any{}},
			func(_ context.Context, _ map[string]any) (*ToolResult, error) {
				return &ToolResult{Success: true}, nil
			},
		)
		if err != nil {
			t.Fatalf("Register %q: %v", n, err)
		}
	}

	got := reg.List()
	if len(got) != len(names) {
		t.Fatalf("List returned %d tools, want %d", len(got), len(names))
	}
	for i, want := range names {
		if got[i].Name != want {
			t.Fatalf("List[%d].Name = %q, want %q", i, got[i].Name, want)
		}
	}
}

func TestExecute(t *testing.T) {
	reg := NewRegistry()
	tool := llm.Tool{Name: "greet", Parameters: map[string]any{}}
	handler := func(_ context.Context, args map[string]any) (*ToolResult, error) {
		name, _ := args["name"].(string)
		return &ToolResult{
			Success:   true,
			Data:      map[string]any{"greeting": "Hello, " + name},
			Narrative: "Greeted " + name + ".",
		}, nil
	}
	if err := reg.Register(tool, handler); err != nil {
		t.Fatalf("Register: %v", err)
	}

	result, err := reg.Execute(context.Background(), "greet", map[string]any{"name": "World"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Success {
		t.Fatal("Execute: result.Success = false, want true")
	}
	if result.Data["greeting"] != "Hello, World" {
		t.Fatalf("Execute: greeting = %v, want Hello, World", result.Data["greeting"])
	}
	if result.Narrative != "Greeted World." {
		t.Fatalf("Execute: Narrative = %q, want %q", result.Narrative, "Greeted World.")
	}
}

func TestExecuteUnknownToolReturnsDescriptiveError(t *testing.T) {
	reg := NewRegistry()

	_, err := reg.Execute(context.Background(), "nonexistent_tool", nil)
	if err == nil {
		t.Fatal("Execute with unknown tool name: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent_tool") {
		t.Fatalf("Execute error = %q, want tool name in message", err.Error())
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("Execute error = %q, want 'not registered' in message", err.Error())
	}
}

func TestRegisterDuplicateNameReturnsError(t *testing.T) {
	reg := NewRegistry()
	tool := llm.Tool{Name: "dup", Parameters: map[string]any{}}
	handler := func(_ context.Context, _ map[string]any) (*ToolResult, error) {
		return &ToolResult{Success: true}, nil
	}

	if err := reg.Register(tool, handler); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	err := reg.Register(tool, handler)
	if err == nil {
		t.Fatal("second Register with same name: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "dup") {
		t.Fatalf("duplicate Register error = %q, want tool name in message", err.Error())
	}
}

func TestRegisterEmptyNameReturnsError(t *testing.T) {
	reg := NewRegistry()
	err := reg.Register(llm.Tool{Name: ""}, func(_ context.Context, _ map[string]any) (*ToolResult, error) {
		return nil, nil
	})
	if err == nil {
		t.Fatal("Register with empty name: expected error, got nil")
	}
}

func TestRegisterNilHandlerReturnsError(t *testing.T) {
	reg := NewRegistry()
	err := reg.Register(llm.Tool{Name: "some_tool"}, nil)
	if err == nil {
		t.Fatal("Register with nil handler: expected error, got nil")
	}
}

func TestToolResultFields(t *testing.T) {
	result := &ToolResult{
		Success:   true,
		Data:      map[string]any{"key": "value"},
		Narrative: "Something happened.",
	}
	if !result.Success {
		t.Error("Success field not set correctly")
	}
	if result.Data["key"] != "value" {
		t.Error("Data field not set correctly")
	}
	if result.Narrative != "Something happened." {
		t.Error("Narrative field not set correctly")
	}
}
