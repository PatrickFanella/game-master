package llm

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PatrickFanella/game-master/internal/config"
)

func TestNewLLMProviderOllama(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	t.Cleanup(server.Close)

	cfg := config.Config{
		LLM: config.LLMConfig{
			Provider: "ollama",
			Ollama: config.OllamaConfig{
				Endpoint: server.URL,
				Model:    "llama-test",
			},
		},
	}

	provider, err := NewLLMProvider(cfg)
	if err != nil {
		t.Fatalf("NewLLMProvider() error = %v", err)
	}

	client, ok := provider.(*OllamaClient)
	if !ok {
		t.Fatalf("provider type = %T, want *OllamaClient", provider)
	}
	if client.baseURL != server.URL {
		t.Fatalf("baseURL = %q, want %q", client.baseURL, server.URL)
	}
	if client.model != "llama-test" {
		t.Fatalf("model = %q, want %q", client.model, "llama-test")
	}
}

func TestNewLLMProviderOllamaEndpointBuildsHealthCheckURLFromParsedBase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ollama/api/tags" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.RawQuery != "" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	cfg := config.Config{
		LLM: config.LLMConfig{
			Provider: "ollama",
			Ollama: config.OllamaConfig{
				Endpoint: server.URL + "/ollama?x=1#frag",
				Model:    "llama-test",
			},
		},
	}

	_, err := NewLLMProvider(cfg)
	if err != nil {
		t.Fatalf("NewLLMProvider() error = %v", err)
	}
}

func TestNewLLMProviderClaude(t *testing.T) {
	cfg := config.Config{
		LLM: config.LLMConfig{
			Provider: "claude",
			Claude: config.ClaudeConfig{
				APIKey: "sk-ant-test",
				Model:  "claude-test",
			},
		},
	}

	provider, err := NewLLMProvider(cfg)
	if err != nil {
		t.Fatalf("NewLLMProvider() error = %v", err)
	}

	client, ok := provider.(*ClaudeClient)
	if !ok {
		t.Fatalf("provider type = %T, want *ClaudeClient", provider)
	}
	if client.apiKey != "sk-ant-test" {
		t.Fatalf("apiKey = %q, want %q", client.apiKey, "sk-ant-test")
	}
	if client.model != "claude-test" {
		t.Fatalf("model = %q, want %q", client.model, "claude-test")
	}
}

func TestNewLLMProviderRejectsClaudeWithoutAPIKey(t *testing.T) {
	cfg := config.Config{
		LLM: config.LLMConfig{
			Provider: "claude",
		},
	}

	_, err := NewLLMProvider(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "missing api key") {
		t.Fatalf("error = %q, want missing api key message", err)
	}
}

func TestNewLLMProviderRejectsUnreachableOllama(t *testing.T) {
	cfg := config.Config{
		LLM: config.LLMConfig{
			Provider: "ollama",
			Ollama: config.OllamaConfig{
				Endpoint: "http://127.0.0.1:1",
			},
		},
	}

	_, err := NewLLMProvider(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot reach") {
		t.Fatalf("error = %q, want reachability message", err)
	}
}

func TestNewLLMProviderRejectsOllamaNon2xxWithTrimmedBody(t *testing.T) {
	body := strings.Repeat("x", ollamaHealthCheckErrorBodyLimit+20)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	cfg := config.Config{
		LLM: config.LLMConfig{
			Provider: "ollama",
			Ollama: config.OllamaConfig{
				Endpoint: server.URL,
			},
		},
	}

	_, err := NewLLMProvider(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "returned HTTP 500") {
		t.Fatalf("error = %q, want status message", err)
	}
	if !strings.Contains(err.Error(), body[:ollamaHealthCheckErrorBodyLimit]) {
		t.Fatalf("error = %q, want trimmed body content", err)
	}
	if strings.Contains(err.Error(), body) {
		t.Fatalf("error should not include full response body of length %d", len(body))
	}
}

func TestNewLLMProviderRejectsUnknownProvider(t *testing.T) {
	cfg := config.Config{
		LLM: config.LLMConfig{
			Provider: "unknown",
		},
	}

	_, err := NewLLMProvider(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported llm provider") {
		t.Fatalf("error = %q, want provider message", err)
	}
}
