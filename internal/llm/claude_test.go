package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
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
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: message_start\ndata: {\"type\":\"message_start\"}\n\n"))
		_, _ = w.Write([]byte("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"tool_1\",\"name\":\"lookup_weather\",\"input\":{}}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"city\\\":\\\"Paris\\\"}\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n"))
		_, _ = w.Write([]byte("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"tool_2\",\"name\":\"lookup_time\",\"input\":{}}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"city\\\":\\\"Paris\\\"}\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":1}\n\n"))
		_, _ = w.Write([]byte("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":2,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":2,\"delta\":{\"type\":\"text_delta\",\"text\":\"done\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":2}\n\n"))
		_, _ = w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
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
	if len(chunks) != 4 {
		t.Fatalf("chunks len = %d, want 4", len(chunks))
	}
	if chunks[0].Done || chunks[0].ToolCallDelta == nil || chunks[0].ToolCallDelta.Name != "lookup_weather" {
		t.Fatalf("chunk[0] = %#v, want non-done lookup_weather tool delta", chunks[0])
	}
	if chunks[1].Done || chunks[1].ToolCallDelta == nil || chunks[1].ToolCallDelta.Name != "lookup_time" {
		t.Fatalf("chunk[1] = %#v, want non-done lookup_time tool delta", chunks[1])
	}
	if chunks[2].Done || chunks[2].ContentDelta != "done" || chunks[2].ToolCallDelta != nil {
		t.Fatalf("chunk[2] = %#v, want non-done text chunk with content \"done\"", chunks[2])
	}
	if !chunks[3].Done || chunks[3].ContentDelta != "" || chunks[3].ToolCallDelta != nil {
		t.Fatalf("chunk[3] = %#v, want final done chunk", chunks[3])
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

func TestClaudeClientCompleteTextOnlyFixture(t *testing.T) {
	fixture := loadFixture(t, "claude_response_text_only.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := NewClaudeClient(server.URL, "sk-ant-test", "claude-test")
	resp, err := client.Complete(context.Background(), []Message{{Role: RoleUser, Content: "tell me about the tavern"}}, nil)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if resp.Content != "The tavern glows with warm candlelight as travelers trade stories near the hearth." {
		t.Fatalf("content = %q, want narrative text", resp.Content)
	}
	if len(resp.ToolCalls) != 0 {
		t.Fatalf("tool calls length = %d, want 0", len(resp.ToolCalls))
	}
	if resp.FinishReason != "end_turn" {
		t.Fatalf("finish reason = %q, want end_turn", resp.FinishReason)
	}
	if resp.Usage.PromptTokens != 14 || resp.Usage.CompletionTokens != 16 || resp.Usage.TotalTokens != 30 {
		t.Fatalf("usage = %+v, want prompt=14 completion=16 total=30", resp.Usage)
	}
}

func TestClaudeClientCompleteErrorClassificationHTTPStatus(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		body         string
		retryAfter   string
		wantErr      any
		wantContains string
	}{
		{name: "auth unauthorized", status: http.StatusUnauthorized, body: `{"type":"error","error":{"type":"authentication_error","message":"invalid auth token"}}`, wantErr: &ErrAuth{}, wantContains: "claude authentication_error: invalid auth token"},
		{name: "auth forbidden", status: http.StatusForbidden, body: `{"type":"error","error":{"type":"permission_error","message":"forbidden"}}`, wantErr: &ErrAuth{}, wantContains: "claude permission_error: forbidden"},
		{name: "rate limit", status: http.StatusTooManyRequests, retryAfter: "3", body: `{"type":"error","error":{"type":"rate_limit_error","message":"too many requests"}}`, wantErr: &ErrRateLimit{}, wantContains: "claude rate_limit_error: too many requests"},
		{name: "bad request malformed", status: http.StatusBadRequest, body: `{"type":"error","error":{"type":"invalid_request_error","message":"request body invalid"}}`, wantErr: &ErrMalformedResponse{}, wantContains: "claude invalid_request_error: request body invalid"},
		{name: "transient internal error", status: http.StatusInternalServerError, body: `{"type":"error","error":{"type":"api_error","message":"internal error"}}`, wantErr: &ErrTransient{}, wantContains: "claude api_error: internal error"},
		{name: "transient overloaded", status: 529, body: `{"type":"error","error":{"type":"overloaded_error","message":"temporarily overloaded"}}`, wantErr: &ErrTransient{}, wantContains: "claude overloaded_error: temporarily overloaded"},
		{name: "model not found", status: http.StatusNotFound, body: "model 'claude-test' not found", wantErr: &ErrModelNotFound{}, wantContains: "claude-test"},
		{name: "generic non-2xx", status: http.StatusNotImplemented, body: "other provider issue", wantErr: &ErrConnection{}, wantContains: "status 501"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.retryAfter != "" {
					w.Header().Set("Retry-After", tt.retryAfter)
				}
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
				if !e.HasRetryAfter {
					t.Fatal("expected rate limit error to include Retry-After metadata")
				}
				if e.RetryAfter != 3*time.Second {
					t.Fatalf("retry after = %s, want %s", e.RetryAfter, 3*time.Second)
				}
			case *ErrMalformedResponse:
				var e *ErrMalformedResponse
				if !errors.As(err, &e) {
					t.Fatalf("error type = %T, want *ErrMalformedResponse (error=%v)", err, err)
				}
				if e.URL != server.URL+claudeMessagesPath {
					t.Fatalf("malformed response URL = %q, want %q", e.URL, server.URL+claudeMessagesPath)
				}
			case *ErrTransient:
				var e *ErrTransient
				if !errors.As(err, &e) {
					t.Fatalf("error type = %T, want *ErrTransient (error=%v)", err, err)
				}
				if e.StatusCode != tt.status || e.URL != server.URL+claudeMessagesPath {
					t.Fatalf("transient error = %#v, want status=%d url=%q", e, tt.status, server.URL+claudeMessagesPath)
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestClaudeClientCompleteNetworkAndTimeoutErrorClassification(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantErr any
	}{
		{
			name:    "timeout to err timeout",
			err:     context.DeadlineExceeded,
			wantErr: &ErrTimeout{},
		},
		{
			name: "network to err connection",
			err: &url.Error{
				Op:  "Post",
				URL: "http://example.invalid/v1/messages",
				Err: fmt.Errorf("dial tcp: connection refused"),
			},
			wantErr: &ErrConnection{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			client := NewClaudeClient("http://example.invalid", "sk-ant-test", "claude-test")
			client.client = &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, tt.err
				}),
			}

			_, err := client.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
			if err == nil {
				t.Fatal("expected error")
			}

			switch tt.wantErr.(type) {
			case *ErrTimeout:
				var e *ErrTimeout
				if !errors.As(err, &e) {
					t.Fatalf("error type = %T, want *ErrTimeout (error=%v)", err, err)
				}
			case *ErrConnection:
				var e *ErrConnection
				if !errors.As(err, &e) {
					t.Fatalf("error type = %T, want *ErrConnection (error=%v)", err, err)
				}
			default:
				t.Fatalf("unsupported expected error type %T", tt.wantErr)
			}
		})
	}
}

func TestClaudeClientCompleteRateLimitRetryAfterZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"too many requests"}}`))
	}))
	defer server.Close()

	client := NewClaudeClient(server.URL, "sk-ant-test", "claude-test")
	_, err := client.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected rate limit error")
	}

	var rl *ErrRateLimit
	if !errors.As(err, &rl) {
		t.Fatalf("error type = %T, want *ErrRateLimit (error=%v)", err, err)
	}
	if !rl.HasRetryAfter {
		t.Fatal("expected HasRetryAfter true when Retry-After header is 0")
	}
	if rl.RetryAfter != 0 {
		t.Fatalf("retry after = %s, want 0s", rl.RetryAfter)
	}
	if !strings.Contains(rl.Error(), "retry after 0s") {
		t.Fatalf("error = %q, want retry-after text for zero delay", rl.Error())
	}
}

func TestClaudeClientCompleteRateLimitRetryAfterPastDate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "Sun, 06 Nov 1994 08:49:37 GMT")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"too many requests"}}`))
	}))
	defer server.Close()

	client := NewClaudeClient(server.URL, "sk-ant-test", "claude-test")
	_, err := client.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected rate limit error")
	}

	var rl *ErrRateLimit
	if !errors.As(err, &rl) {
		t.Fatalf("error type = %T, want *ErrRateLimit (error=%v)", err, err)
	}
	if !rl.HasRetryAfter {
		t.Fatal("expected HasRetryAfter true when Retry-After header exists, even for past date")
	}
	if rl.RetryAfter != 0 {
		t.Fatalf("retry after = %s, want 0s for past date", rl.RetryAfter)
	}
}

func TestClaudeAPIErrorSanitizesRawBody(t *testing.T) {
	body := "line1\r\nline2"
	err := claudeAPIError(body, http.StatusBadGateway)
	if got := err.Error(); strings.Contains(got, "\n") || strings.Contains(got, "\r") {
		t.Fatalf("error contains control characters: %q", got)
	}
	if got := err.Error(); !strings.Contains(got, "line1 line2") {
		t.Fatalf("error = %q, want sanitized body text", got)
	}
}

func TestClaudeAPIErrorSanitizesJSONErrorFields(t *testing.T) {
	body := `{"type":"error","error":{"type":"overloaded\nerror","message":"retry\tlater"}}`
	err := claudeAPIError(body, http.StatusServiceUnavailable)
	if got := err.Error(); strings.Contains(got, "\n") || strings.Contains(got, "\r") || strings.Contains(got, "\t") {
		t.Fatalf("error contains control characters: %q", got)
	}
	if got := err.Error(); !strings.Contains(got, "claude overloaded error: retry later") {
		t.Fatalf("error = %q, want sanitized type and message", got)
	}
}

func TestClaudeAPIErrorParsesLongJSONBeforeFallbackSanitization(t *testing.T) {
	longMsg := strings.Repeat("a", 1300)
	body := `{"type":"error","error":{"type":"invalid_request_error","message":"` + longMsg + `"}}`
	err := claudeAPIError(body, http.StatusBadRequest)
	if got := err.Error(); !strings.Contains(got, "claude invalid_request_error: ") {
		t.Fatalf("error = %q, want parsed claude type/message", got)
	}
	if got := err.Error(); strings.Contains(got, "…") {
		t.Fatalf("error should not be fallback-truncated when json parse succeeds: %q", got)
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

func TestClaudeClientStreamSetsStreamTrue(t *testing.T) {
	var gotReq claudeMessagesRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer server.Close()

	client := NewClaudeClient(server.URL, "sk-ant-test", "claude-test")
	ch, err := client.Stream(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	for range ch {
	}

	if !gotReq.Stream {
		t.Fatal("stream must be true for Stream()")
	}
}

func TestClaudeClientStreamRejectsNonSSEContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[]}`))
	}))
	defer server.Close()

	client := NewClaudeClient(server.URL, "sk-ant-test", "claude-test")
	_, err := client.Stream(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected error for non-SSE Content-Type")
	}

	var malformedErr *ErrMalformedResponse
	if !errors.As(err, &malformedErr) {
		t.Fatalf("error type = %T, want *ErrMalformedResponse (error=%v)", err, err)
	}
	if !strings.Contains(malformedErr.Error(), "text/event-stream") {
		t.Fatalf("error = %q, want message referencing text/event-stream", malformedErr.Error())
	}
}

func TestClaudeClientStreamContextCancellation(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("ResponseWriter does not support Flusher")
			return
		}
		_, _ = w.Write([]byte("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"first\"}}\n\n"))
		flusher.Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	client := NewClaudeClient(server.URL, "sk-ant-test", "claude-test")
	ch, err := client.Stream(ctx, []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	<-started
	cancel()

	for range ch {
	}
}

func TestClaudeClientStreamErrorEventClosesChannel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"partial\"}}\n\n"))
		_, _ = w.Write([]byte("event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"Overloaded\"}}\n\n"))
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

	if len(chunks) != 1 {
		t.Fatalf("chunks len = %d, want 1 (partial text before error)", len(chunks))
	}
	if chunks[0].ContentDelta != "partial" {
		t.Fatalf("chunk[0].ContentDelta = %q, want \"partial\"", chunks[0].ContentDelta)
	}
	// Channel must be closed without a Done=true chunk when an error event is received.
	for _, c := range chunks {
		if c.Done {
			t.Fatal("no chunk should have Done=true when error event received")
		}
	}
}

func TestClaudeClientStreamMalformedDataSkipsEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: content_block_delta\ndata: {invalid json\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"ok\"}}\n\n"))
		_, _ = w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
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

	if len(chunks) != 2 {
		t.Fatalf("chunks len = %d, want 2 (text delta + done)", len(chunks))
	}
	if chunks[0].ContentDelta != "ok" {
		t.Fatalf("chunk[0].ContentDelta = %q, want \"ok\"", chunks[0].ContentDelta)
	}
	if !chunks[1].Done {
		t.Fatal("last chunk must have Done=true")
	}
}

func TestClaudeClientStreamMalformedToolArgsSkipsToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// tool_use block with invalid JSON argument fragment
		_, _ = w.Write([]byte("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"tool_bad\",\"name\":\"bad_tool\",\"input\":{}}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{invalid\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n"))
		// valid text follows
		_, _ = w.Write([]byte("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"text_delta\",\"text\":\"safe\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":1}\n\n"))
		_, _ = w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
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

	// The malformed tool call must be silently dropped; only the text delta and done chunk are emitted.
	if len(chunks) != 2 {
		t.Fatalf("chunks len = %d, want 2 (text delta + done); bad tool call must be skipped", len(chunks))
	}
	if chunks[0].ToolCallDelta != nil {
		t.Fatalf("chunk[0] should not carry malformed tool call, got %#v", chunks[0])
	}
	if chunks[0].ContentDelta != "safe" {
		t.Fatalf("chunk[0].ContentDelta = %q, want \"safe\"", chunks[0].ContentDelta)
	}
	if !chunks[1].Done {
		t.Fatal("last chunk must have Done=true")
	}
}

func TestClaudeClientStreamFixtureResponses(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []StreamChunk
	}{
		{
			name:    "text only",
			fixture: "claude_stream_text_chunks.sse",
			want: []StreamChunk{
				{ContentDelta: "The dragon ", Done: false},
				{ContentDelta: "breathes fire across the cavern.", Done: false},
				{Done: true},
			},
		},
		{
			name:    "tool calls only",
			fixture: "claude_stream_tool_calls_only.sse",
			want: []StreamChunk{
				{
					ToolCallDelta: &ToolCall{
						ID:        "toolu_01",
						Name:      "roll_dice",
						Arguments: map[string]any{"sides": float64(20), "count": float64(1)},
					},
					Done: false,
				},
				{
					ToolCallDelta: &ToolCall{
						ID:        "toolu_02",
						Name:      "lookup_npc",
						Arguments: map[string]any{"name": "Gandalf"},
					},
					Done: false,
				},
				{Done: true},
			},
		},
		{
			name:    "mixed text and tool calls",
			fixture: "claude_stream_mixed_content.sse",
			want: []StreamChunk{
				{ContentDelta: "You approach the merchant's stall. ", Done: false},
				{
					ToolCallDelta: &ToolCall{
						ID:        "toolu_03",
						Name:      "get_inventory",
						Arguments: map[string]any{"merchant_id": "m-17", "category": "weapons"},
					},
					Done: false,
				},
				{ContentDelta: "Let me see what is available.", Done: false},
				{Done: true},
			},
		},
		{
			name:    "multiple tool calls",
			fixture: "claude_stream_multiple_tool_calls.sse",
			want: []StreamChunk{
				{
					ToolCallDelta: &ToolCall{
						ID:        "toolu_100",
						Name:      "roll_dice",
						Arguments: map[string]any{"sides": float64(20), "count": float64(1)},
					},
					Done: false,
				},
				{
					ToolCallDelta: &ToolCall{
						ID:        "toolu_101",
						Name:      "lookup_npc",
						Arguments: map[string]any{"name": "Gandalf", "location": "Minas Tirith"},
					},
					Done: false,
				},
				{
					ToolCallDelta: &ToolCall{
						ID:        "toolu_102",
						Name:      "update_quest",
						Arguments: map[string]any{"quest_id": "q-42", "status": "in_progress"},
					},
					Done: false,
				},
				{Done: true},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			fixture := loadFixture(t, tt.fixture)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = w.Write(fixture)
			}))
			defer server.Close()

			client := NewClaudeClient(server.URL, "sk-ant-test", "claude-test")
			ch, err := client.Stream(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
			if err != nil {
				t.Fatalf("Stream() error = %v", err)
			}

			var got []StreamChunk
			for chunk := range ch {
				got = append(got, chunk)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("chunks len = %d, want %d", len(got), len(tt.want))
			}

			for i := range tt.want {
				if got[i].ContentDelta != tt.want[i].ContentDelta {
					t.Fatalf("chunk[%d] content delta = %q, want %q", i, got[i].ContentDelta, tt.want[i].ContentDelta)
				}
				if got[i].Done != tt.want[i].Done {
					t.Fatalf("chunk[%d] done = %v, want %v", i, got[i].Done, tt.want[i].Done)
				}
				if !reflect.DeepEqual(got[i].ToolCallDelta, tt.want[i].ToolCallDelta) {
					t.Fatalf("chunk[%d] tool delta = %#v, want %#v", i, got[i].ToolCallDelta, tt.want[i].ToolCallDelta)
				}
			}
		})
	}
}
