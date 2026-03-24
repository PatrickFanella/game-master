package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNewOllamaClientDefaults(t *testing.T) {
	client := NewOllamaClient("", "")

	if client.baseURL != defaultOllamaBaseURL {
		t.Fatalf("baseURL = %q, want %q", client.baseURL, defaultOllamaBaseURL)
	}
	if client.model != defaultOllamaModel {
		t.Fatalf("model = %q, want %q", client.model, defaultOllamaModel)
	}
	if client.client == nil {
		t.Fatal("http client must be configured")
	}
}

func TestOllamaClientCompleteRequestResponse(t *testing.T) {
	var gotReq ollamaChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != ollamaChatPath {
			t.Fatalf("path = %q, want %q", r.URL.Path, ollamaChatPath)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("content-type = %q, want application/json", ct)
		}

		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		_, _ = w.Write([]byte(`{
"message": {
"content": "assistant reply",
"tool_calls": [{"function": {"name": "lookup", "arguments": "{\"city\":\"Paris\"}"}}]
},
"done": true,
"done_reason": "stop",
"prompt_eval_count": 8,
"eval_count": 5
}`))
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL+"/", "llama-test")
	resp, err := client.Complete(context.Background(), []Message{
		{Role: RoleSystem, Content: "be brief"},
		// Prior assistant tool call in conversation history should marshal correctly.
		{Role: RoleAssistant, ToolCalls: []ToolCall{{Name: "lookup", Arguments: map[string]any{"city": "Paris"}}}},
	}, []Tool{{
		Name:        "lookup",
		Description: "lookup city",
		Parameters:  map[string]any{"type": "object"},
	}})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if gotReq.Model != "llama-test" {
		t.Fatalf("model = %q, want llama-test", gotReq.Model)
	}
	if gotReq.Stream {
		t.Fatal("stream must be false for Complete")
	}
	if len(gotReq.Messages) != 2 {
		t.Fatalf("messages length = %d, want 2", len(gotReq.Messages))
	}
	if len(gotReq.Tools) != 1 || gotReq.Tools[0].Function.Name != "lookup" {
		t.Fatalf("tools not marshaled correctly: %#v", gotReq.Tools)
	}

	if resp.Content != "assistant reply" {
		t.Fatalf("response content = %q, want assistant reply", resp.Content)
	}
	if resp.FinishReason != "stop" {
		t.Fatalf("finish reason = %q, want stop", resp.FinishReason)
	}
	if resp.Usage.PromptTokens != 8 || resp.Usage.CompletionTokens != 5 || resp.Usage.TotalTokens != 13 {
		t.Fatalf("usage = %#v, want prompt=8 completion=5 total=13", resp.Usage)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool calls length = %d, want 1", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "lookup" {
		t.Fatalf("tool call name = %q, want lookup", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].Arguments["city"] != "Paris" {
		t.Fatalf("tool call args city = %#v, want Paris", resp.ToolCalls[0].Arguments["city"])
	}
}

func TestToOllamaToolsSerializesOpenAICompatibleFormat(t *testing.T) {
	tools := []Tool{
		{
			Name:        "lookup_weather",
			Description: "Gets weather by city",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"city": map[string]any{
						"type":        "string",
						"description": "City name",
					},
					"units": map[string]any{
						"type": "string",
						"enum": []string{"metric", "imperial"},
					},
				},
				"required": []string{"city"},
			},
		},
		{
			Name:        "set_reminder",
			Description: "Creates a reminder",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"message": map[string]any{"type": "string"},
					"at":      map[string]any{"type": "string", "format": "date-time"},
				},
				"required": []string{"message", "at"},
			},
		},
	}

	got := toOllamaTools(tools)
	if len(got) != 2 {
		t.Fatalf("len(toOllamaTools) = %d, want 2", len(got))
	}

	body, err := json.Marshal(struct {
		Tools []ollamaTool `json:"tools"`
	}{Tools: got})
	if err != nil {
		t.Fatalf("json.Marshal(tools): %v", err)
	}

	var decoded struct {
		Tools []map[string]any `json:"tools"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(serialized tools): %v", err)
	}
	if len(decoded.Tools) != 2 {
		t.Fatalf("serialized tools length = %d, want 2", len(decoded.Tools))
	}

	for i := range tools {
		idx := i
		expected := tools[idx]
		decodedTool := decoded.Tools[idx]
		t.Run(expected.Name+"_json", func(t *testing.T) {
			toolType, ok := decodedTool["type"].(string)
			if !ok {
				t.Fatalf("serialized tool[%d].type has unexpected type %T", idx, decodedTool["type"])
			}
			if toolType != "function" {
				t.Fatalf("serialized tool[%d].type = %q, want %q", idx, toolType, "function")
			}

			functionAny, ok := decodedTool["function"]
			if !ok {
				t.Fatalf("serialized tool[%d] missing function field", idx)
			}
			functionObj, ok := functionAny.(map[string]any)
			if !ok {
				t.Fatalf("serialized tool[%d].function has unexpected type %T", idx, functionAny)
			}

			name, ok := functionObj["name"].(string)
			if !ok {
				t.Fatalf("serialized tool[%d].function.name has unexpected type %T", idx, functionObj["name"])
			}
			if name != expected.Name {
				t.Fatalf("serialized tool[%d].function.name = %q, want %q", idx, name, expected.Name)
			}

			description, ok := functionObj["description"].(string)
			if !ok {
				t.Fatalf("serialized tool[%d].function.description has unexpected type %T", idx, functionObj["description"])
			}
			if description != expected.Description {
				t.Fatalf("serialized tool[%d].function.description = %q, want %q", idx, description, expected.Description)
			}

			parametersAny, ok := functionObj["parameters"]
			if !ok {
				t.Fatalf("serialized tool[%d].function missing parameters field", idx)
			}
			parametersObj, ok := parametersAny.(map[string]any)
			if !ok {
				t.Fatalf("serialized tool[%d].function.parameters has unexpected type %T", idx, parametersAny)
			}

			expectedParametersBytes, err := json.Marshal(expected.Parameters)
			if err != nil {
				t.Fatalf("json.Marshal(expected parameters for tool[%d]): %v", idx, err)
			}
			var expectedParameters map[string]any
			if err := json.Unmarshal(expectedParametersBytes, &expectedParameters); err != nil {
				t.Fatalf("json.Unmarshal(expected parameters for tool[%d]): %v", idx, err)
			}
			if !reflect.DeepEqual(parametersObj, expectedParameters) {
				t.Fatalf("serialized tool[%d].function.parameters = %#v, want %#v", idx, parametersObj, expectedParameters)
			}
		})
	}
}

func TestOllamaClientCompleteConnectionError(t *testing.T) {
	client := NewOllamaClient("http://127.0.0.1:1", "llama-test")

	_, err := client.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected connection error")
	}
	var connErr *ErrConnection
	if !errors.As(err, &connErr) {
		t.Fatalf("error type = %T, want *ErrConnection (error=%v)", err, err)
	}
	if connErr.URL != "http://127.0.0.1:1/api/chat" {
		t.Fatalf("connection error URL = %q, want %q", connErr.URL, "http://127.0.0.1:1/api/chat")
	}
	if !strings.Contains(connErr.Error(), "http://127.0.0.1:1/api/chat") {
		t.Fatalf("error = %q, want URL context", connErr.Error())
	}
}

func TestOllamaClientCompleteErrorClassificationHTTPStatus(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		body         string
		wantErr      any
		wantContains string
	}{
		{
			name:         "auth unauthorized",
			status:       http.StatusUnauthorized,
			body:         "invalid auth token",
			wantErr:      &ErrAuth{},
			wantContains: "status 401",
		},
		{
			name:         "auth forbidden",
			status:       http.StatusForbidden,
			body:         "forbidden",
			wantErr:      &ErrAuth{},
			wantContains: "status 403",
		},
		{
			name:         "rate limit",
			status:       http.StatusTooManyRequests,
			body:         "too many requests",
			wantErr:      &ErrRateLimit{},
			wantContains: "status 429",
		},
		{
			name:         "model not found",
			status:       http.StatusNotFound,
			body:         "model 'llama-test' not found",
			wantErr:      &ErrModelNotFound{},
			wantContains: "llama-test",
		},
		{
			name:         "generic non-2xx",
			status:       http.StatusInternalServerError,
			body:         "internal error",
			wantErr:      &ErrConnection{},
			wantContains: "status 500",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewOllamaClient(server.URL, "llama-test")
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
				if e.StatusCode != tt.status {
					t.Fatalf("status code = %d, want %d", e.StatusCode, tt.status)
				}
				if e.URL != server.URL+ollamaChatPath {
					t.Fatalf("URL = %q, want %q", e.URL, server.URL+ollamaChatPath)
				}
			case *ErrRateLimit:
				var e *ErrRateLimit
				if !errors.As(err, &e) {
					t.Fatalf("error type = %T, want *ErrRateLimit (error=%v)", err, err)
				}
				if e.StatusCode != tt.status {
					t.Fatalf("status code = %d, want %d", e.StatusCode, tt.status)
				}
			case *ErrModelNotFound:
				var e *ErrModelNotFound
				if !errors.As(err, &e) {
					t.Fatalf("error type = %T, want *ErrModelNotFound (error=%v)", err, err)
				}
				if e.Model != "llama-test" {
					t.Fatalf("model = %q, want llama-test", e.Model)
				}
				if e.StatusCode != tt.status {
					t.Fatalf("status code = %d, want %d", e.StatusCode, tt.status)
				}
			case *ErrConnection:
				var e *ErrConnection
				if !errors.As(err, &e) {
					t.Fatalf("error type = %T, want *ErrConnection (error=%v)", err, err)
				}
				if e.URL != server.URL+ollamaChatPath {
					t.Fatalf("URL = %q, want %q", e.URL, server.URL+ollamaChatPath)
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

func TestOllamaClientCompleteMalformedResponseErrorType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{invalid json"))
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "llama-test")
	_, err := client.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected malformed response error")
	}

	var malformedErr *ErrMalformedResponse
	if !errors.As(err, &malformedErr) {
		t.Fatalf("error type = %T, want *ErrMalformedResponse (error=%v)", err, err)
	}
	if malformedErr.URL != server.URL+ollamaChatPath {
		t.Fatalf("URL = %q, want %q", malformedErr.URL, server.URL+ollamaChatPath)
	}
}

func TestFormatConnectionErrorTimeoutClassification(t *testing.T) {
	timeoutErr := formatConnectionError("http://localhost:11434/api/chat", context.DeadlineExceeded)

	var typedTimeout *ErrTimeout
	if !errors.As(timeoutErr, &typedTimeout) {
		t.Fatalf("error type = %T, want *ErrTimeout (error=%v)", timeoutErr, timeoutErr)
	}
	if typedTimeout.URL != "http://localhost:11434/api/chat" {
		t.Fatalf("URL = %q, want %q", typedTimeout.URL, "http://localhost:11434/api/chat")
	}
	if !errors.Is(timeoutErr, context.DeadlineExceeded) {
		t.Fatalf("error should unwrap context deadline exceeded, got %v", timeoutErr)
	}
}

func TestTypedLLMErrorsUnwrap(t *testing.T) {
	root := fmt.Errorf("root cause")
	tests := []struct {
		name string
		err  error
	}{
		{name: "connection", err: &ErrConnection{URL: "u", Err: root}},
		{name: "timeout", err: &ErrTimeout{URL: "u", Err: root}},
		{name: "rate_limit", err: &ErrRateLimit{URL: "u", StatusCode: 429, Err: root}},
		{name: "auth", err: &ErrAuth{URL: "u", StatusCode: 401, Err: root}},
		{name: "malformed_response", err: &ErrMalformedResponse{URL: "u", Err: root}},
		{name: "model_not_found", err: &ErrModelNotFound{URL: "u", Model: "m", StatusCode: 404, Err: root}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.err, root) {
				t.Fatalf("expected errors.Is(%T, root) to be true", tt.err)
			}
		})
	}
}

func TestOllamaClientCompleteEncodesEmptyMessagesArray(t *testing.T) {
	var gotReq ollamaChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"message":{"content":"ok"},"done":true}`))
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "llama-test")
	if _, err := client.Complete(context.Background(), nil, nil); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if gotReq.Messages == nil {
		t.Fatal("messages should encode as empty array, got null")
	}
	if len(gotReq.Messages) != 0 {
		t.Fatalf("messages length = %d, want 0", len(gotReq.Messages))
	}
}

func TestOllamaClientStreamSendsDoneOnExplicitDoneOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"message\":{\"content\":\"hello \"},\"done\":false}\n"))
		_, _ = w.Write([]byte("{\"message\":{\"content\":\"world\"},\"done\":true}\n"))
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "llama-test")
	ch, err := client.Stream(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var chunks []StreamChunk
	for c := range ch {
		chunks = append(chunks, c)
	}

	if len(chunks) != 2 {
		t.Fatalf("chunks len = %d, want 2", len(chunks))
	}
	if chunks[0].Done {
		t.Fatal("first chunk Done should be false")
	}
	if !chunks[1].Done {
		t.Fatal("final chunk Done should be true")
	}
	if chunks[0].ContentDelta != "hello " {
		t.Fatalf("first chunk content delta = %q, want %q", chunks[0].ContentDelta, "hello ")
	}
	if chunks[1].ContentDelta != "world" {
		t.Fatalf("second chunk content delta = %q, want %q", chunks[1].ContentDelta, "world")
	}
}

func TestOllamaClientStreamMalformedChunkClosesWithoutDone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"message\":{\"content\":\"start\"},\"done\":false}\n"))
		_, _ = w.Write([]byte("{\"message\":invalid json}\n"))
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "llama-test")
	ch, err := client.Stream(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var chunks []StreamChunk
	for c := range ch {
		chunks = append(chunks, c)
	}

	if len(chunks) != 1 {
		t.Fatalf("chunks len = %d, want 1", len(chunks))
	}
	if chunks[0].Done {
		t.Fatal("unexpected Done=true on malformed stream termination")
	}
}

func TestOllamaClientStreamSetsStreamTrue(t *testing.T) {
	var gotReq ollamaChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"message\":{\"content\":\"ok\"},\"done\":true}\n"))
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "llama-test")
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

func TestOllamaClientStreamToolCallDelta(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"message\":{\"content\":\"\",\"tool_calls\":[{\"function\":{\"name\":\"roll_dice\",\"arguments\":\"{\\\"sides\\\":20}\"}}]},\"done\":false}\n"))
		_, _ = w.Write([]byte("{\"message\":{\"content\":\"\"},\"done\":true}\n"))
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "llama-test")
	ch, err := client.Stream(context.Background(), []Message{{Role: RoleUser, Content: "roll for initiative"}}, nil)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var chunks []StreamChunk
	for c := range ch {
		chunks = append(chunks, c)
	}

	if len(chunks) != 2 {
		t.Fatalf("chunks len = %d, want 2", len(chunks))
	}
	if chunks[0].ToolCallDelta == nil {
		t.Fatal("first chunk ToolCallDelta should not be nil")
	}
	if chunks[0].ToolCallDelta.Name != "roll_dice" {
		t.Fatalf("ToolCallDelta.Name = %q, want roll_dice", chunks[0].ToolCallDelta.Name)
	}
	if chunks[0].ToolCallDelta.Arguments["sides"] != float64(20) {
		t.Fatalf("ToolCallDelta.Arguments[sides] = %v, want 20", chunks[0].ToolCallDelta.Arguments["sides"])
	}
	if chunks[1].ToolCallDelta != nil {
		t.Fatal("second chunk ToolCallDelta should be nil")
	}
	if !chunks[1].Done {
		t.Fatal("second chunk Done should be true")
	}
}

func TestOllamaClientStreamContextCancellation(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("ResponseWriter does not support Flusher")
			return
		}
		_, _ = w.Write([]byte("{\"message\":{\"content\":\"first\"},\"done\":false}\n"))
		flusher.Flush()
		close(started)
		// Block until request context is done, simulating a long stream.
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	client := NewOllamaClient(server.URL, "llama-test")
	ch, err := client.Stream(ctx, []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	// Wait for the server to flush the first chunk, then cancel the context.
	<-started
	cancel()

	// Drain the channel; it must close without blocking forever.
	for range ch {
	}
}

// loadFixture reads a JSON fixture file from testdata/.
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("failed to read fixture %q: %v", name, err)
	}
	return data
}

func TestOllamaClientCompletePureTextResponse(t *testing.T) {
	fixture := loadFixture(t, "ollama_response_pure_text.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "test-model")
	resp, err := client.Complete(context.Background(), []Message{{Role: RoleUser, Content: "tell me a story"}}, nil)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if resp.Content != "The dragon breathes fire across the cavern, illuminating the ancient runes on the walls." {
		t.Fatalf("content = %q, want narrative text", resp.Content)
	}
	if len(resp.ToolCalls) != 0 {
		t.Fatalf("tool calls length = %d, want 0", len(resp.ToolCalls))
	}
	if resp.FinishReason != "stop" {
		t.Fatalf("finish reason = %q, want stop", resp.FinishReason)
	}
	if resp.Usage.PromptTokens != 12 || resp.Usage.CompletionTokens != 18 || resp.Usage.TotalTokens != 30 {
		t.Fatalf("usage = %+v, want prompt=12 completion=18 total=30", resp.Usage)
	}
}

func TestOllamaCompleteSingleToolCall(t *testing.T) {
	fixture := loadFixture(t, "ollama_response_single_tool_call.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "test-model")
	resp, err := client.Complete(context.Background(), []Message{{Role: RoleUser, Content: "roll for initiative"}}, nil)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if resp.Content != "" {
		t.Fatalf("content = %q, want empty", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool calls length = %d, want 1", len(resp.ToolCalls))
	}

	tc := resp.ToolCalls[0]
	if tc.Name != "roll_dice" {
		t.Fatalf("tool call name = %q, want roll_dice", tc.Name)
	}
	// JSON numbers decode as float64 in map[string]any.
	if tc.Arguments["sides"] != float64(20) {
		t.Fatalf("tool call args sides = %v, want 20", tc.Arguments["sides"])
	}
	if tc.Arguments["count"] != float64(1) {
		t.Fatalf("tool call args count = %v, want 1", tc.Arguments["count"])
	}
}

func TestOllamaCompleteMultipleToolCalls(t *testing.T) {
	fixture := loadFixture(t, "ollama_response_multiple_tool_calls.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "test-model")
	resp, err := client.Complete(context.Background(), []Message{{Role: RoleUser, Content: "prepare for battle"}}, nil)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if len(resp.ToolCalls) != 3 {
		t.Fatalf("tool calls length = %d, want 3", len(resp.ToolCalls))
	}

	expected := []struct {
		name string
		args map[string]any
	}{
		{
			name: "roll_dice",
			args: map[string]any{"sides": float64(20), "count": float64(1)},
		},
		{
			name: "lookup_npc",
			args: map[string]any{"name": "Gandalf", "location": "Minas Tirith"},
		},
		{
			name: "update_quest",
			args: map[string]any{"quest_id": "q-42", "status": "in_progress"},
		},
	}

	for i, exp := range expected {
		tc := resp.ToolCalls[i]
		if tc.Name != exp.name {
			t.Fatalf("tool call[%d] name = %q, want %q", i, tc.Name, exp.name)
		}
		if !reflect.DeepEqual(tc.Arguments, exp.args) {
			t.Fatalf("tool call[%d] args = %v, want %v", i, tc.Arguments, exp.args)
		}
	}
}

func TestOllamaCompleteMixedContent(t *testing.T) {
	fixture := loadFixture(t, "ollama_response_mixed_content.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, "test-model")
	resp, err := client.Complete(context.Background(), []Message{{Role: RoleUser, Content: "I want to buy a sword"}}, nil)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	if resp.Content != "You approach the merchant's stall. Let me check the inventory for you." {
		t.Fatalf("content = %q, want narrative text", resp.Content)
	}
	if len(resp.ToolCalls) != 2 {
		t.Fatalf("tool calls length = %d, want 2", len(resp.ToolCalls))
	}

	if resp.ToolCalls[0].Name != "get_inventory" {
		t.Fatalf("tool call[0] name = %q, want get_inventory", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].Arguments["merchant_id"] != "m-17" {
		t.Fatalf("tool call[0] merchant_id = %v, want m-17", resp.ToolCalls[0].Arguments["merchant_id"])
	}
	if resp.ToolCalls[0].Arguments["category"] != "weapons" {
		t.Fatalf("tool call[0] category = %v, want weapons", resp.ToolCalls[0].Arguments["category"])
	}

	if resp.ToolCalls[1].Name != "check_gold" {
		t.Fatalf("tool call[1] name = %q, want check_gold", resp.ToolCalls[1].Name)
	}
	if resp.ToolCalls[1].Arguments["player_id"] != "p-1" {
		t.Fatalf("tool call[1] player_id = %v, want p-1", resp.ToolCalls[1].Arguments["player_id"])
	}
}

func TestFromOllamaToolCallsEmptyArguments(t *testing.T) {
	calls := []ollamaToolCall{
		{Function: ollamaToolFunction{Name: "ping", Arguments: ""}},
	}

	result, err := fromOllamaToolCalls(calls)
	if err != nil {
		t.Fatalf("fromOllamaToolCalls() error = %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("result length = %d, want 1", len(result))
	}
	if result[0].Name != "ping" {
		t.Fatalf("name = %q, want ping", result[0].Name)
	}
	if len(result[0].Arguments) != 0 {
		t.Fatalf("arguments = %v, want empty map", result[0].Arguments)
	}
}

func TestFromOllamaToolCallsInvalidJSON(t *testing.T) {
	calls := []ollamaToolCall{
		{Function: ollamaToolFunction{Name: "bad", Arguments: "{invalid json}"}},
	}

	_, err := fromOllamaToolCalls(calls)
	if err == nil {
		t.Fatal("expected error for invalid JSON arguments")
	}
	if !strings.Contains(err.Error(), "bad") {
		t.Fatalf("error = %q, want mention of function name", err.Error())
	}
}

func TestFromOllamaToolCallsNilSlice(t *testing.T) {
	result, err := fromOllamaToolCalls(nil)
	if err != nil {
		t.Fatalf("fromOllamaToolCalls(nil) error = %v", err)
	}
	if result != nil {
		t.Fatalf("result = %v, want nil", result)
	}
}
