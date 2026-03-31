package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/PatrickFanella/game-master/internal/memory"
)

// --- stub implementation ------------------------------------------------

// stubEmbedder is a minimal Embedder implementation used to verify that
// concrete types can satisfy the interface.
type stubEmbedder struct {
	dim int
}

func (s *stubEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, &memory.ErrEmptyInput{}
	}
	return make([]float32, s.dim), nil
}

func (s *stubEmbedder) BatchEmbed(_ context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, &memory.ErrEmptyInput{}
	}
	out := make([][]float32, len(texts))
	for i, t := range texts {
		if t == "" {
			return nil, &memory.ErrEmptyInput{}
		}
		out[i] = make([]float32, s.dim)
	}
	return out, nil
}

// Compile-time interface satisfaction check.
var _ memory.Embedder = (*stubEmbedder)(nil)

// --- interface tests ----------------------------------------------------

func TestEmbed_ReturnsVectorOfCorrectDimension(t *testing.T) {
	e := &stubEmbedder{dim: memory.DefaultVectorDimension}
	vec, err := e.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vec) != memory.DefaultVectorDimension {
		t.Fatalf("expected vector length %d, got %d", memory.DefaultVectorDimension, len(vec))
	}
}

func TestBatchEmbed_ReturnsCorrectCount(t *testing.T) {
	e := &stubEmbedder{dim: memory.DefaultVectorDimension}
	texts := []string{"one", "two", "three"}
	vecs, err := e.BatchEmbed(context.Background(), texts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vecs) != len(texts) {
		t.Fatalf("expected %d vectors, got %d", len(texts), len(vecs))
	}
	for i, v := range vecs {
		if len(v) != memory.DefaultVectorDimension {
			t.Fatalf("vector %d: expected length %d, got %d", i, memory.DefaultVectorDimension, len(v))
		}
	}
}

// --- dimension tests ----------------------------------------------------

func TestDefaultVectorDimension(t *testing.T) {
	if memory.DefaultVectorDimension != 768 {
		t.Fatalf("expected DefaultVectorDimension=768, got %d", memory.DefaultVectorDimension)
	}
}

// --- error type tests ---------------------------------------------------

func TestErrEmbeddingFailed_ErrorAndUnwrap(t *testing.T) {
	inner := errors.New("provider 500")
	err := &memory.ErrEmbeddingFailed{Text: "some input", Err: inner}

	if err.Error() == "" {
		t.Fatal("expected non-empty error message")
	}
	if !errors.Is(err, inner) {
		t.Fatal("expected Unwrap to return inner error")
	}
}

func TestErrEmbeddingFailed_EmptyText(t *testing.T) {
	inner := errors.New("fail")
	err := &memory.ErrEmbeddingFailed{Err: inner}

	want := "embedding failed: fail"
	if err.Error() != want {
		t.Fatalf("expected %q, got %q", want, err.Error())
	}
}

func TestErrDimensionMismatch_ErrorAndUnwrap(t *testing.T) {
	inner := errors.New("model mismatch")
	err := &memory.ErrDimensionMismatch{Expected: 768, Actual: 1536, Err: inner}
	want := "embedding dimension mismatch: expected 768, got 1536"
	if err.Error() != want {
		t.Fatalf("expected %q, got %q", want, err.Error())
	}
	if !errors.Is(err, inner) {
		t.Fatal("expected Unwrap to return inner error")
	}
}

func TestErrDimensionMismatch_NilUnwrap(t *testing.T) {
	err := &memory.ErrDimensionMismatch{Expected: 768, Actual: 1536}
	if err.Unwrap() != nil {
		t.Fatal("expected Unwrap to return nil when Err is not set")
	}
}

func TestErrEmptyInput_ErrorAndUnwrap(t *testing.T) {
	inner := errors.New("validation failed")
	err := &memory.ErrEmptyInput{Err: inner}
	want := "embedding input must not be empty"
	if err.Error() != want {
		t.Fatalf("expected %q, got %q", want, err.Error())
	}
	if !errors.Is(err, inner) {
		t.Fatal("expected Unwrap to return inner error")
	}
}

func TestErrEmptyInput_NilUnwrap(t *testing.T) {
	err := &memory.ErrEmptyInput{}
	if err.Unwrap() != nil {
		t.Fatal("expected Unwrap to return nil when Err is not set")
	}
}

func TestErrBatchPartialFailure_ErrorAndUnwrap(t *testing.T) {
	inner := errors.New("timeout")
	err := &memory.ErrBatchPartialFailure{Total: 5, Failed: 2, Err: inner}

	if err.Error() == "" {
		t.Fatal("expected non-empty error message")
	}
	if !errors.Is(err, inner) {
		t.Fatal("expected Unwrap to return inner error")
	}
}

func TestErrorsAs(t *testing.T) {
	inner := errors.New("root cause")
	wrapped := &memory.ErrEmbeddingFailed{Text: "test", Err: inner}

	var target *memory.ErrEmbeddingFailed
	if !errors.As(wrapped, &target) {
		t.Fatal("expected errors.As to succeed for *ErrEmbeddingFailed")
	}
	if target.Text != "test" {
		t.Fatalf("expected text %q, got %q", "test", target.Text)
	}
}

// --- EmbeddingResult tests ----------------------------------------------

func TestEmbeddingResult_Fields(t *testing.T) {
	r := memory.EmbeddingResult{
		Vector:    make([]float32, memory.DefaultVectorDimension),
		Text:      "sample text",
		Model:     "nomic-embed-text",
		Dimension: memory.DefaultVectorDimension,
		Duration:  42 * time.Millisecond,
	}

	if len(r.Vector) != r.Dimension {
		t.Fatalf("vector length %d != dimension %d", len(r.Vector), r.Dimension)
	}
	if r.Model != "nomic-embed-text" {
		t.Fatalf("unexpected model: %s", r.Model)
	}
	if r.Duration != 42*time.Millisecond {
		t.Fatalf("unexpected duration: %v", r.Duration)
	}
}
