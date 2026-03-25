package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestNewClaudeClientDefaults(t *testing.T) {
	client := NewClaudeClient("", "sk-ant-test", "")

	if client.baseURL != defaultClaudeBaseURL {
		t.Fatalf("baseURL = %q, want %q", client.baseURL, defaultClaudeBaseURL)
	}
	if client.model != defaultClaudeModel {
		t.Fatalf("model = %q, want %q", client.model, defaultClaudeModel)
	}
	if client.anthropicVersion != defaultAnthropicVersion {
		t.Fatalf("anthropicVersion = %q, want %q", client.anthropicVersion, defaultAnthropicVersion)
	}
	if client.client == nil {
		t.Fatal("http client must be configured")
	}
	if client.maxTokens != defaultClaudeMaxTokens {
		t.Fatalf("maxTokens = %d, want %d", client.maxTokens, defaultClaudeMaxTokens)
	}
}

func TestNewClaudeClientWithMaxTokens(t *testing.T) {
	tests := []struct {
		name      string
		maxTokens int
		want      int
	}{
		{name: "custom positive", maxTokens: 4096, want: 4096},
		{name: "zero falls back to default", maxTokens: 0, want: defaultClaudeMaxTokens},
		{name: "negative falls back to default", maxTokens: -1, want: defaultClaudeMaxTokens},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			client := NewClaudeClientWithMaxTokens("", "sk-ant-test", "", tt.maxTokens)
			if client.maxTokens != tt.want {
				t.Fatalf("maxTokens = %d, want %d", client.maxTokens, tt.want)
			}
		})
	}
}

func TestClaudeClientCompleteRequestHeadersAndPayload(t *testing.T) {
	var gotReq claudeMessagesRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != claudeMessagesPath {
			t.Fatalf("path = %q, want %q", r.URL.Path, claudeMessagesPath)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if got := r.Header.Get("x-api-key"); got != "sk-ant-test" {
			t.Fatalf("x-api-key = %q, want %q", got, "sk-ant-test")
		}
		if got := r.Header.Get("anthropic-version"); got != defaultAnthropicVersion {
			t.Fatalf("anthropic-version = %q, want %q", got, defaultAnthropicVersion)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("content-type = %q, want application/json", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		_, _ = w.Write([]byte(`{
"content":[
  {"type":"text","text":"assistant reply"},
  {"type":"tool_use","id":"tool_1","name":"lookup_weather","input":{"city":"Paris"}}
],
"stop_reason":"end_turn",
"usage":{"input_tokens":11,"output_tokens":7}
}`))
	}))
	defer server.Close()

	client := NewClaudeClient(server.URL+"/", "sk-ant-test", "claude-test")
	resp, err := client.Complete(context.Background(), []Message{
		{Role: RoleSystem, Content: "be concise"},
		{Role: RoleUser, Content: "weather?"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{{ID: "tool_1", Name: "lookup_weather", Arguments: map[string]any{"city": "Paris"}}}},
		{Role: RoleTool, ToolCallID: "tool_1", Content: `{"temp":"20C"}`},
	}, []Tool{{
		Name:        "lookup_weather",
		Description: "weather by city",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"city": map[string]any{"type": "string"},
			},
			"required": []string{"city"},
		},
	}})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if gotReq.Model != "claude-test" {
		t.Fatalf("model = %q, want claude-test", gotReq.Model)
	}
	if gotReq.System != "be concise" {
		t.Fatalf("system = %q, want %q", gotReq.System, "be concise")
	}
	if gotReq.MaxTokens != defaultClaudeMaxTokens {
		t.Fatalf("max_tokens = %d, want %d", gotReq.MaxTokens, defaultClaudeMaxTokens)
	}
	if len(gotReq.Messages) != 3 {
		t.Fatalf("messages length = %d, want 3", len(gotReq.Messages))
	}
	if gotReq.Messages[0].Role != claudeRoleUser || gotReq.Messages[0].Content[0].Type != claudeContentTypeText {
		t.Fatalf("first message not marshaled as user text: %#v", gotReq.Messages[0])
	}
	if gotReq.Messages[1].Role != claudeRoleAssistant || gotReq.Messages[1].Content[0].Type != claudeContentTypeToolUse {
		t.Fatalf("assistant tool call not marshaled correctly: %#v", gotReq.Messages[1])
	}
	if gotReq.Messages[2].Role != claudeRoleUser || gotReq.Messages[2].Content[0].Type != claudeContentTypeToolResp {
		t.Fatalf("tool result not marshaled correctly: %#v", gotReq.Messages[2])
	}
	if len(gotReq.Tools) != 1 || gotReq.Tools[0].Name != "lookup_weather" {
		t.Fatalf("tools not marshaled correctly: %#v", gotReq.Tools)
	}

	if resp.Content != "assistant reply" {
		t.Fatalf("response content = %q, want assistant reply", resp.Content)
	}
	if resp.FinishReason != "end_turn" {
		t.Fatalf("finish reason = %q, want end_turn", resp.FinishReason)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool calls length = %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].ID != "tool_1" || resp.ToolCalls[0].Name != "lookup_weather" {
		t.Fatalf("tool call = %#v, want id=tool_1 name=lookup_weather", resp.ToolCalls[0])
	}
	if resp.Usage.PromptTokens != 11 || resp.Usage.CompletionTokens != 7 || resp.Usage.TotalTokens != 18 {
		t.Fatalf("usage = %#v, want prompt=11 completion=7 total=18", resp.Usage)
	}
}

func TestClaudeClientCompleteHonorsConfiguredMaxTokens(t *testing.T) {
	var gotReq claudeMessagesRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{
"content":[{"type":"text","text":"ok"}],
"stop_reason":"end_turn",
"usage":{"input_tokens":1,"output_tokens":1}
}`))
	}))
	defer server.Close()

	client := NewClaudeClientWithMaxTokens(server.URL, "sk-ant-test", "claude-test", 2048)
	_, err := client.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if gotReq.MaxTokens != 2048 {
		t.Fatalf("max_tokens = %d, want %d", gotReq.MaxTokens, 2048)
	}
}

func TestClaudeClientStreamEmitsToolCallDeltasAndFinalDoneChunk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
"content":[
  {"type":"tool_use","id":"tool_1","name":"lookup_weather","input":{"city":"Paris"}},
  {"type":"tool_use","id":"tool_2","name":"lookup_time","input":{"city":"Paris"}},
  {"type":"text","text":"done"}
],
"stop_reason":"end_turn",
"usage":{"input_tokens":9,"output_tokens":4}
}`))
	}))
	defer server.Close()

	client := NewClaudeClient(server.URL, "sk-ant-test", "claude-test")
	ch, err := client.Stream(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var chunks []StreamChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}
	if len(chunks) != 3 {
		t.Fatalf("chunks len = %d, want 3", len(chunks))
	}
	if chunks[0].Done || chunks[0].ToolCallDelta == nil || chunks[0].ToolCallDelta.Name != "lookup_weather" {
		t.Fatalf("chunk[0] = %#v, want non-done lookup_weather tool delta", chunks[0])
	}
	if chunks[1].Done || chunks[1].ToolCallDelta == nil || chunks[1].ToolCallDelta.Name != "lookup_time" {
		t.Fatalf("chunk[1] = %#v, want non-done lookup_time tool delta", chunks[1])
	}
	if !chunks[2].Done || chunks[2].ContentDelta != "done" || chunks[2].ToolCallDelta != nil {
		t.Fatalf("chunk[2] = %#v, want final done text chunk", chunks[2])
	}
}

func TestClaudeClientCompleteMixedContentFixture(t *testing.T) {
	fixture := loadFixture(t, "claude_response_mixed_content.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := NewClaudeClient(server.URL, "sk-ant-test", "claude-test")
	resp, err := client.Complete(context.Background(), []Message{{Role: RoleUser, Content: "What is happening?"}}, nil)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if resp.Content != "You approach the merchant's stall. Let me check the inventory for you." {
		t.Fatalf("content = %q, want narrative text", resp.Content)
	}
	if len(resp.ToolCalls) != 2 {
		t.Fatalf("tool calls length = %d, want 2", len(resp.ToolCalls))
	}

	expected := []ToolCall{
		{
			ID:   "toolu_01A",
			Name: "get_inventory",
			Arguments: map[string]any{
				"merchant_id": "m-17",
				"category":    "weapons",
			},
		},
		{
			ID:   "toolu_01B",
			Name: "check_gold",
			Arguments: map[string]any{
				"player_id": "p-1",
			},
		},
	}

	for i := range expected {
		if resp.ToolCalls[i].ID != expected[i].ID {
			t.Fatalf("tool call[%d] id = %q, want %q", i, resp.ToolCalls[i].ID, expected[i].ID)
		}
		if resp.ToolCalls[i].Name != expected[i].Name {
			t.Fatalf("tool call[%d] name = %q, want %q", i, resp.ToolCalls[i].Name, expected[i].Name)
		}
		if !reflect.DeepEqual(resp.ToolCalls[i].Arguments, expected[i].Arguments) {
			t.Fatalf("tool call[%d] arguments = %#v, want %#v", i, resp.ToolCalls[i].Arguments, expected[i].Arguments)
		}
	}
}

func TestClaudeClientCompleteMultipleToolCallsFixture(t *testing.T) {
	fixture := loadFixture(t, "claude_response_multiple_tool_calls.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := NewClaudeClient(server.URL, "sk-ant-test", "claude-test")
	resp, err := client.Complete(context.Background(), []Message{{Role: RoleUser, Content: "prepare for battle"}}, nil)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if resp.Content != "" {
		t.Fatalf("content = %q, want empty", resp.Content)
	}
	if len(resp.ToolCalls) != 3 {
		t.Fatalf("tool calls length = %d, want 3", len(resp.ToolCalls))
	}

	expected := []ToolCall{
		{
			ID:   "toolu_100",
			Name: "roll_dice",
			Arguments: map[string]any{
				"sides": float64(20),
				"count": float64(1),
			},
		},
		{
			ID:   "toolu_101",
			Name: "lookup_npc",
			Arguments: map[string]any{
				"name":     "Gandalf",
				"location": "Minas Tirith",
			},
		},
		{
			ID:   "toolu_102",
			Name: "update_quest",
			Arguments: map[string]any{
				"quest_id": "q-42",
				"status":   "in_progress",
			},
		},
	}

	for i := range expected {
		if resp.ToolCalls[i].ID != expected[i].ID {
			t.Fatalf("tool call[%d] id = %q, want %q", i, resp.ToolCalls[i].ID, expected[i].ID)
		}
		if resp.ToolCalls[i].Name != expected[i].Name {
			t.Fatalf("tool call[%d] name = %q, want %q", i, resp.ToolCalls[i].Name, expected[i].Name)
		}
		if !reflect.DeepEqual(resp.ToolCalls[i].Arguments, expected[i].Arguments) {
			t.Fatalf("tool call[%d] arguments = %#v, want %#v", i, resp.ToolCalls[i].Arguments, expected[i].Arguments)
		}
	}
}

func TestClaudeClientCompleteErrorClassificationHTTPStatus(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		body         string
		wantErr      any
		wantContains string
	}{
		{name: "auth unauthorized", status: http.StatusUnauthorized, body: "invalid auth token", wantErr: &ErrAuth{}, wantContains: "status 401"},
		{name: "auth forbidden", status: http.StatusForbidden, body: "forbidden", wantErr: &ErrAuth{}, wantContains: "status 403"},
		{name: "rate limit", status: http.StatusTooManyRequests, body: "too many requests", wantErr: &ErrRateLimit{}, wantContains: "status 429"},
		{name: "model not found", status: http.StatusNotFound, body: "model 'claude-test' not found", wantErr: &ErrModelNotFound{}, wantContains: "claude-test"},
		{name: "generic non-2xx", status: http.StatusInternalServerError, body: "internal error", wantErr: &ErrConnection{}, wantContains: "status 500"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClaudeClient(server.URL, "sk-ant-test", "claude-test")
			_, err := client.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
			if err == nil {
				t.Fatalf("expected error for status %d", tt.status)
			}

			switch tt.wantErr.(type) {
			case *ErrAuth:
				var e *ErrAuth
				if !errors.As(err, &e) {
					t.Fatalf("error type = %T, want *ErrAuth (error=%v)", err, err)
				}
				if e.StatusCode != tt.status || e.URL != server.URL+claudeMessagesPath {
					t.Fatalf("auth error = %#v, want status=%d url=%q", e, tt.status, server.URL+claudeMessagesPath)
				}
			case *ErrRateLimit:
				var e *ErrRateLimit
				if !errors.As(err, &e) {
					t.Fatalf("error type = %T, want *ErrRateLimit (error=%v)", err, err)
				}
				if e.StatusCode != tt.status || e.URL != server.URL+claudeMessagesPath {
					t.Fatalf("rate limit error = %#v, want status=%d url=%q", e, tt.status, server.URL+claudeMessagesPath)
				}
			case *ErrModelNotFound:
				var e *ErrModelNotFound
				if !errors.As(err, &e) {
					t.Fatalf("error type = %T, want *ErrModelNotFound (error=%v)", err, err)
				}
				if e.StatusCode != tt.status || e.URL != server.URL+claudeMessagesPath || e.Model != "claude-test" {
					t.Fatalf("model not found error = %#v, want status=%d url=%q model=claude-test", e, tt.status, server.URL+claudeMessagesPath)
				}
			case *ErrConnection:
				var e *ErrConnection
				if !errors.As(err, &e) {
					t.Fatalf("error type = %T, want *ErrConnection (error=%v)", err, err)
				}
				if e.URL != server.URL+claudeMessagesPath {
					t.Fatalf("connection error URL = %q, want %q", e.URL, server.URL+claudeMessagesPath)
				}
			default:
				t.Fatalf("unsupported expected type %T", tt.wantErr)
			}

			if !strings.Contains(err.Error(), tt.wantContains) {
				t.Fatalf("error = %q, want to contain %q", err.Error(), tt.wantContains)
			}
		})
	}
}

func TestClaudeClientCompleteMalformedResponseErrorType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{invalid json"))
	}))
	defer server.Close()

	client := NewClaudeClient(server.URL, "sk-ant-test", "claude-test")
	_, err := client.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected malformed response error")
	}

	var malformedErr *ErrMalformedResponse
	if !errors.As(err, &malformedErr) {
		t.Fatalf("error type = %T, want *ErrMalformedResponse (error=%v)", err, err)
	}
	if malformedErr.URL != server.URL+claudeMessagesPath {
		t.Fatalf("URL = %q, want %q", malformedErr.URL, server.URL+claudeMessagesPath)
	}
}

func TestClaudeClientCompleteUnsupportedContentTypeReturnsMalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
"content":[{"type":"image","text":"ignored"}],
"stop_reason":"end_turn",
"usage":{"input_tokens":1,"output_tokens":1}
}`))
	}))
	defer server.Close()

	client := NewClaudeClient(server.URL, "sk-ant-test", "claude-test")
	_, err := client.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected malformed response error")
	}

	var malformedErr *ErrMalformedResponse
	if !errors.As(err, &malformedErr) {
		t.Fatalf("error type = %T, want *ErrMalformedResponse (error=%v)", err, err)
	}
	if !strings.Contains(malformedErr.Error(), "unsupported claude content block type") {
		t.Fatalf("error = %q, want unsupported content type context", malformedErr.Error())
	}
}
