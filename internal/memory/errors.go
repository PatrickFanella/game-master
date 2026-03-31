package memory

import "fmt"

// ErrEmbeddingFailed indicates a general failure during an embedding
// operation, such as a provider returning an unexpected HTTP status.
type ErrEmbeddingFailed struct {
	// Text is the (possibly truncated) input that triggered the failure.
	Text string
	Err  error
}

func (e *ErrEmbeddingFailed) Error() string {
	if e.Text == "" {
		return fmt.Sprintf("embedding failed: %v", e.Err)
	}
	return fmt.Sprintf("embedding failed for text %q: %v", truncate(e.Text, 64), e.Err)
}

func (e *ErrEmbeddingFailed) Unwrap() error { return e.Err }

// ErrDimensionMismatch indicates that the embedding provider returned a
// vector whose length differs from the expected dimension.
type ErrDimensionMismatch struct {
	Expected int
	Actual   int
	Err      error
}

func (e *ErrDimensionMismatch) Error() string {
	return fmt.Sprintf("embedding dimension mismatch: expected %d, got %d", e.Expected, e.Actual)
}

func (e *ErrDimensionMismatch) Unwrap() error { return e.Err }

// ErrEmptyInput indicates that an empty string (or an empty slice of strings
// for batch operations) was supplied to the embedder.
type ErrEmptyInput struct {
	Err error
}

func (e *ErrEmptyInput) Error() string {
	return "embedding input must not be empty"
}

func (e *ErrEmptyInput) Unwrap() error { return e.Err }

// ErrBatchPartialFailure indicates that some, but not all, texts in a
// BatchEmbed call failed to produce embeddings.
type ErrBatchPartialFailure struct {
	// Total is the number of texts submitted.
	Total int
	// Failed is the number of texts that could not be embedded.
	Failed int
	Err    error
}

func (e *ErrBatchPartialFailure) Error() string {
	return fmt.Sprintf("batch embedding partially failed: %d of %d texts failed: %v", e.Failed, e.Total, e.Err)
}

func (e *ErrBatchPartialFailure) Unwrap() error { return e.Err }

// truncate shortens s to at most maxLen runes, appending "…" if truncated.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}
