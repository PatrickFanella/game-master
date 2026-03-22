package llm

import (
	"context"
	"testing"
)

func TestRoleConstants(t *testing.T) {
	tests := []struct {
		role Role
		want string
	}{
		{RoleSystem, "system"},
		{RoleUser, "user"},
		{RoleAssistant, "assistant"},
		{RoleTool, "tool"},
	}

	for _, tt := range tests {
		if string(tt.role) != tt.want {
			t.Errorf("Role = %q, want %q", tt.role, tt.want)
		}
	}
}

func TestMessageFields(t *testing.T) {
	msg := Message{
		Role:       RoleAssistant,
		Content:    "hello",
		ToolCalls:  []ToolCall{{ID: "1", Name: "fn", Arguments: map[string]any{"a": "b"}}},
		ToolCallID: "call-1",
	}

	if msg.Role != RoleAssistant {
		t.Fatalf("Role = %q, want %q", msg.Role, RoleAssistant)
	}
	if msg.Content != "hello" {
		t.Fatalf("Content = %q, want %q", msg.Content, "hello")
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].ID != "1" {
		t.Fatalf("unexpected ToolCalls: %+v", msg.ToolCalls)
	}
	if msg.ToolCallID != "call-1" {
		t.Fatalf("ToolCallID = %q, want %q", msg.ToolCallID, "call-1")
	}
}

func TestToolFields(t *testing.T) {
	tool := Tool{
		Name:        "search",
		Description: "Search the web",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string"},
			},
		},
	}

	if tool.Name != "search" {
		t.Fatalf("Name = %q, want %q", tool.Name, "search")
	}
	if tool.Description != "Search the web" {
		t.Fatalf("Description = %q, want %q", tool.Description, "Search the web")
	}
	if tool.Parameters["type"] != "object" {
		t.Fatalf("unexpected Parameters: %+v", tool.Parameters)
	}
}

func TestToolCallFields(t *testing.T) {
	tc := ToolCall{
		ID:        "call-123",
		Name:      "search",
		Arguments: map[string]any{"query": "go testing"},
	}

	if tc.ID != "call-123" {
		t.Fatalf("ID = %q, want %q", tc.ID, "call-123")
	}
	if tc.Name != "search" {
		t.Fatalf("Name = %q, want %q", tc.Name, "search")
	}
	if tc.Arguments["query"] != "go testing" {
		t.Fatalf("unexpected Arguments: %+v", tc.Arguments)
	}
}

func TestResponseFields(t *testing.T) {
	resp := Response{
		Content:      "result text",
		ToolCalls:    []ToolCall{{ID: "1", Name: "fn"}},
		FinishReason: "stop",
		Usage:        Usage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
	}

	if resp.Content != "result text" {
		t.Fatalf("Content = %q, want %q", resp.Content, "result text")
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 ToolCall, got %d", len(resp.ToolCalls))
	}
	if resp.FinishReason != "stop" {
		t.Fatalf("FinishReason = %q, want %q", resp.FinishReason, "stop")
	}
	if resp.Usage.TotalTokens != 30 {
		t.Fatalf("TotalTokens = %d, want %d", resp.Usage.TotalTokens, 30)
	}
}

func TestStreamChunkFields(t *testing.T) {
	chunk := StreamChunk{
		ContentDelta:  "hello ",
		ToolCallDelta: &ToolCall{ID: "1", Name: "fn"},
		Done:          false,
	}

	if chunk.ContentDelta != "hello " {
		t.Fatalf("ContentDelta = %q, want %q", chunk.ContentDelta, "hello ")
	}
	if chunk.ToolCallDelta == nil || chunk.ToolCallDelta.ID != "1" {
		t.Fatalf("unexpected ToolCallDelta: %+v", chunk.ToolCallDelta)
	}
	if chunk.Done {
		t.Fatal("Done should be false")
	}

	final := StreamChunk{Done: true}
	if !final.Done {
		t.Fatal("Done should be true")
	}
}

func TestUsageFields(t *testing.T) {
	u := Usage{PromptTokens: 5, CompletionTokens: 15, TotalTokens: 20}

	if u.PromptTokens != 5 {
		t.Fatalf("PromptTokens = %d, want %d", u.PromptTokens, 5)
	}
	if u.CompletionTokens != 15 {
		t.Fatalf("CompletionTokens = %d, want %d", u.CompletionTokens, 15)
	}
	if u.TotalTokens != 20 {
		t.Fatalf("TotalTokens = %d, want %d", u.TotalTokens, 20)
	}
}

// mockProvider is a test helper that verifies Provider can be implemented.
type mockProvider struct{}

func (m *mockProvider) Complete(_ context.Context, _ []Message, _ []Tool) (*Response, error) {
	return &Response{Content: "mock"}, nil
}

func (m *mockProvider) Stream(_ context.Context, _ []Message, _ []Tool) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 1)
	ch <- StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func TestProviderInterface(t *testing.T) {
	var p Provider = &mockProvider{}

	resp, err := p.Complete(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if resp.Content != "mock" {
		t.Fatalf("Content = %q, want %q", resp.Content, "mock")
	}

	ch, err := p.Stream(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	chunk := <-ch
	if !chunk.Done {
		t.Fatal("expected Done chunk from stream")
	}
}
