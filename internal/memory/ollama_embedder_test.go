package memory

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewOllamaEmbedderDefaults(t *testing.T) {
	embedder := NewOllamaEmbedder("", "")

	if embedder.baseURL != defaultOllamaEmbedBaseURL {
		t.Fatalf("baseURL = %q, want %q", embedder.baseURL, defaultOllamaEmbedBaseURL)
	}
	if embedder.model != defaultOllamaEmbedModel {
		t.Fatalf("model = %q, want %q", embedder.model, defaultOllamaEmbedModel)
	}
	if embedder.dimension != DefaultVectorDimension {
		t.Fatalf("dimension = %d, want %d", embedder.dimension, DefaultVectorDimension)
	}
	if embedder.client == nil {
		t.Fatal("client must be configured")
	}
}

func TestOllamaEmbedderEmbed_CallsAPIAndParsesVector(t *testing.T) {
	var gotModel string
	var gotInput any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != ollamaEmbedPath {
			t.Fatalf("path = %q, want %q", r.URL.Path, ollamaEmbedPath)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		var req struct {
			Model string `json:"model"`
			Input any    `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		gotModel = req.Model
		gotInput = req.Input
		_, _ = w.Write([]byte(`{"embeddings":[[1.25,-0.5]]}`))
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(server.URL, "custom-model", WithOllamaEmbedderDimension(2))
	vec, err := embedder.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	if gotModel != "custom-model" {
		t.Fatalf("model = %q, want custom-model", gotModel)
	}
	if gotInput != "hello" {
		t.Fatalf("input = %#v, want hello", gotInput)
	}
	if len(vec) != 2 || vec[0] != 1.25 || vec[1] != -0.5 {
		t.Fatalf("vector = %#v, want [1.25 -0.5]", vec)
	}
}

func TestOllamaEmbedderBatchEmbed_UsesSingleBatchRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req struct {
			Model string          `json:"model"`
			Input json.RawMessage `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		var inputs []string
		if err := json.Unmarshal(req.Input, &inputs); err != nil {
			t.Fatalf("input is not an array: %v", err)
		}
		if len(inputs) != 2 || inputs[0] != "alpha" || inputs[1] != "beta" {
			t.Fatalf("inputs = %#v, want [alpha beta]", inputs)
		}
		_, _ = w.Write([]byte(`{"embeddings":[[0.1,0.2],[0.3,0.4]]}`))
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(server.URL, "nomic-embed-text", WithOllamaEmbedderDimension(2))
	out, err := embedder.BatchEmbed(context.Background(), []string{"alpha", "beta"})
	if err != nil {
		t.Fatalf("BatchEmbed() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("server calls = %d, want 1", calls)
	}
	if len(out) != 2 || len(out[0]) != 2 || len(out[1]) != 2 {
		t.Fatalf("unexpected output shape: %#v", out)
	}
}

func TestOllamaEmbedderBatchEmbed_FallsBackToLoopWhenBatchUnsupported(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req struct {
			Input json.RawMessage `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		var asArray []string
		if err := json.Unmarshal(req.Input, &asArray); err == nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"batch not supported"}`))
			return
		}

		var asText string
		if err := json.Unmarshal(req.Input, &asText); err != nil {
			t.Fatalf("unexpected input format: %v", err)
		}
		switch asText {
		case "a":
			_, _ = w.Write([]byte(`{"embeddings":[[1,2]]}`))
		case "b":
			_, _ = w.Write([]byte(`{"embeddings":[[3,4]]}`))
		default:
			t.Fatalf("unexpected text %q", asText)
		}
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder(server.URL, "nomic-embed-text", WithOllamaEmbedderDimension(2))
	out, err := embedder.BatchEmbed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("BatchEmbed() error = %v", err)
	}
	if calls != 3 {
		t.Fatalf("server calls = %d, want 3 (1 batch + 2 single)", calls)
	}
	if len(out) != 2 || out[0][0] != 1 || out[1][0] != 3 {
		t.Fatalf("unexpected output vectors: %#v", out)
	}
}

func TestOllamaEmbedderEmbed_TimeoutAndConnectionErrors(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(250 * time.Millisecond)
			_, _ = w.Write([]byte(`{"embeddings":[[1,2]]}`))
		}))
		defer server.Close()

		embedder := NewOllamaEmbedder(server.URL, "nomic-embed-text",
			WithOllamaEmbedderDimension(2),
			WithOllamaEmbedderTimeout(50*time.Millisecond),
		)
		_, err := embedder.Embed(context.Background(), "slow")
		if err == nil {
			t.Fatal("expected timeout error")
		}

		var failed *ErrEmbeddingFailed
		if !errors.As(err, &failed) {
			t.Fatalf("error type = %T, want *ErrEmbeddingFailed", err)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected deadline exceeded, got %v", err)
		}
	})

	t.Run("connection refused", func(t *testing.T) {
		embedder := NewOllamaEmbedder("http://127.0.0.1:1", "nomic-embed-text", WithOllamaEmbedderDimension(2))
		_, err := embedder.Embed(context.Background(), "text")
		if err == nil {
			t.Fatal("expected connection error")
		}
		var failed *ErrEmbeddingFailed
		if !errors.As(err, &failed) {
			t.Fatalf("error type = %T, want *ErrEmbeddingFailed", err)
		}
	})
}
