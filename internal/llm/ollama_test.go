package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	if !strings.Contains(err.Error(), "failed to connect to ollama at http://127.0.0.1:1/api/chat") {
		t.Fatalf("error = %q, want descriptive ollama connection message", err)
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
