package engine

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/PatrickFanella/game-master/internal/config"
	"github.com/PatrickFanella/game-master/internal/llm"
	"github.com/google/uuid"
)

func TestMarshalAppliedToolCallsNilEncodesArray(t *testing.T) {
	got, err := marshalAppliedToolCalls(nil)
	if err != nil {
		t.Fatalf("marshalAppliedToolCalls(nil) error = %v", err)
	}
	if string(got) != "[]" {
		t.Fatalf("expected empty JSON array, got %s", got)
	}
}

func TestMarshalAppliedToolCallsPreservesEntries(t *testing.T) {
	got, err := marshalAppliedToolCalls([]AppliedToolCall{{
		Tool:      "skill_check",
		Arguments: json.RawMessage(`{"skill":"stealth"}`),
		Result:    json.RawMessage(`{"success":true}`),
	}})
	if err != nil {
		t.Fatalf("marshalAppliedToolCalls() error = %v", err)
	}
	if string(got) == "[]" {
		t.Fatal("expected non-empty marshaled tool calls")
	}
}


type testProvider struct{}

func (p *testProvider) Complete(_ context.Context, _ []llm.Message, tools []llm.Tool) (*llm.Response, error) {
	var foundUpdateNPC bool
	var foundUpdatePlayerStats bool
	var foundAddExperience bool
	var foundLevelUp bool
	var foundAddAbility bool
	var foundRemoveAbility bool
	var foundUpdateQuest bool
	for _, tool := range tools {
		if tool.Name == "update_npc" {
			foundUpdateNPC = true
		}
		if tool.Name == "update_player_stats" {
			foundUpdatePlayerStats = true
		}
		if tool.Name == "add_experience" {
			foundAddExperience = true
		}
		if tool.Name == "level_up" {
			foundLevelUp = true
		}
		if tool.Name == "add_ability" {
			foundAddAbility = true
		}
		if tool.Name == "remove_ability" {
			foundRemoveAbility = true
		}
		if tool.Name == "update_quest" {
			foundUpdateQuest = true
		}
	}
	if !foundUpdateNPC {
		return nil, errors.New("update_npc tool not registered")
	}
	if !foundUpdatePlayerStats {
		return nil, errors.New("update_player_stats tool not registered")
	}
	if !foundAddExperience {
		return nil, errors.New("add_experience tool not registered")
	}
	if !foundLevelUp {
		return nil, errors.New("level_up tool not registered")
	}
	if !foundAddAbility {
		return nil, errors.New("add_ability tool not registered")
	}
	if !foundRemoveAbility {
		return nil, errors.New("remove_ability tool not registered")
	}
	if !foundUpdateQuest {
		return nil, errors.New("update_quest tool not registered")
	}
	return &llm.Response{
		Content: "",
		ToolCalls: []llm.ToolCall{
			{
				ID:   "1",
				Name: "update_npc",
				Arguments: map[string]any{
					"npc_id":      "00000000-0000-0000-0000-000000000001",
					"description": "updated via runtime registration test",
				},
			},
		},
	}, nil
}

func (p *testProvider) Stream(_ context.Context, _ []llm.Message, _ []llm.Tool) (<-chan llm.StreamChunk, error) {
	return nil, errors.New("not implemented")
}

func TestNewRegistersExpectedTools(t *testing.T) {
	e, err := New(nil, &testProvider{}, config.LLMConfig{Provider: "ollama", Ollama: config.OllamaConfig{ContextTokenBudget: 8000}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tools := e.assembler.Tools()
	want := []string{
		"update_npc",
		"update_player_stats",
		"add_experience",
		"level_up",
		"add_ability",
		"remove_ability",
		"update_quest",
	}
	registered := make(map[string]bool, len(tools))
	for _, tool := range tools {
		registered[tool.Name] = true
	}
	for _, name := range want {
		if !registered[name] {
			t.Errorf("expected tool %q to be registered", name)
		}
	}
}


func TestProcessTurnStream_Error(t *testing.T) {
	e, err := New(nil, &testProvider{}, config.LLMConfig{Provider: "ollama", Ollama: config.OllamaConfig{ContextTokenBudget: 8000}})

	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ch, err := e.ProcessTurnStream(context.Background(), uuid.New(), "look around")
	if err != nil {
		t.Fatalf("ProcessTurnStream() error = %v", err)
	}

	var events []StreamEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != "error" {
		t.Fatalf("expected error event, got %q", events[0].Type)
	}
	if events[0].Err == nil {
		t.Fatal("expected non-nil Err on error event")
	}
}

func TestProcessTurnStream_ChannelCloses(t *testing.T) {
	e, err := New(nil, &testProvider{}, config.LLMConfig{Provider: "ollama", Ollama: config.OllamaConfig{ContextTokenBudget: 8000}})

	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ch, err := e.ProcessTurnStream(context.Background(), uuid.New(), "test")
	if err != nil {
		t.Fatalf("ProcessTurnStream() error = %v", err)
	}

	// Channel must close (no goroutine leak).
	for range ch {
	}
	// If we get here, channel closed successfully.
}
