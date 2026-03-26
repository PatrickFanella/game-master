package tools

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestRegisterPresentChoices(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterPresentChoices(reg); err != nil {
		t.Fatalf("register present_choices: %v", err)
	}

	registered := reg.List()
	if len(registered) != 1 {
		t.Fatalf("registered tool count = %d, want 1", len(registered))
	}
	if registered[0].Name != presentChoicesToolName {
		t.Fatalf("tool name = %q, want %q", registered[0].Name, presentChoicesToolName)
	}
}

func TestPresentChoicesHandleValidChoices(t *testing.T) {
	h := NewPresentChoicesHandler()
	got, err := h.Handle(context.Background(), map[string]any{
		"choices": []any{
			map[string]any{
				"id":   "investigate-alcove",
				"text": "Investigate the carved alcove.",
				"type": "action",
			},
			map[string]any{
				"id":   "ask-merchant",
				"text": "Ask the merchant about the sigil.",
				"type": "dialogue",
			},
		},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !got.Success {
		t.Fatalf("result success = %v, want true", got.Success)
	}
	rawChoices, ok := got.Data["choices"].([]map[string]any)
	if !ok {
		t.Fatalf("result choices type = %T, want []map[string]any", got.Data["choices"])
	}
	if len(rawChoices) != 2 {
		t.Fatalf("result choices count = %d, want 2", len(rawChoices))
	}
	if rawChoices[1]["type"] != "dialogue" {
		t.Fatalf("second choice type = %v, want dialogue", rawChoices[1]["type"])
	}
}

func TestPresentChoicesHandleEmptyChoicesArray(t *testing.T) {
	h := NewPresentChoicesHandler()
	_, err := h.Handle(context.Background(), map[string]any{
		"choices": []any{},
	})
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
	if !strings.Contains(err.Error(), "at least one entry") {
		t.Fatalf("error = %v, want empty choices validation message", err)
	}
}

func TestPresentChoicesHandleTooManyChoices(t *testing.T) {
	choices := make([]any, 0, maxPresentChoices+1)
	for i := 0; i < maxPresentChoices+1; i++ {
		choices = append(choices, map[string]any{
			"id":   fmt.Sprintf("choice-%d", i+1),
			"text": "choice text",
			"type": "action",
		})
	}

	h := NewPresentChoicesHandler()
	_, err := h.Handle(context.Background(), map[string]any{
		"choices": choices,
	})
	if err == nil {
		t.Fatal("expected error for too many choices")
	}
	if !strings.Contains(err.Error(), "at most") {
		t.Fatalf("error = %v, want max choices validation message", err)
	}
}
