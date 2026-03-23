package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestOllamaClientCompleteConnectionError(t *testing.T) {
	client := NewOllamaClient("http://127.0.0.1:1", "llama-test")

	_, err := client.Complete(context.Background(), []Message{{Role: RoleUser, Content: "hi"}}, nil)
	if err == nil {
		t.Fatal("expected connection error")
	}
	if !strings.Contains(err.Error(), "failed to connect to ollama at http://127.0.0.1:1/api/chat") {
		t.Fatalf("error = %q, want descriptive ollama connection message", err)
	}
}
