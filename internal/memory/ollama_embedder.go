package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultOllamaEmbedBaseURL = "http://localhost:11434"
	defaultOllamaEmbedModel   = "nomic-embed-text"
	ollamaEmbedPath           = "/api/embed"
	ollamaEmbedErrorBodyLimit = 4096
)

// OllamaEmbedder implements Embedder using Ollama's /api/embed endpoint.
type OllamaEmbedder struct {
	baseURL   string
	model     string
	dimension int
	client    *http.Client
	timeout   time.Duration
}

// OllamaEmbedderOption configures an OllamaEmbedder instance.
type OllamaEmbedderOption func(*OllamaEmbedder)

// WithOllamaEmbedderTimeout sets the per-request timeout.
func WithOllamaEmbedderTimeout(timeout time.Duration) OllamaEmbedderOption {
	return func(o *OllamaEmbedder) {
		if timeout > 0 {
			o.timeout = timeout
			if o.client != nil {
				o.client.Timeout = timeout
			}
		}
	}
}

// WithOllamaEmbedderDimension sets the expected output vector dimension.
func WithOllamaEmbedderDimension(dimension int) OllamaEmbedderOption {
	return func(o *OllamaEmbedder) {
		if dimension > 0 {
			o.dimension = dimension
		}
	}
}

// NewOllamaEmbedder constructs an Ollama-backed embedder.
func NewOllamaEmbedder(baseURL, model string, opts ...OllamaEmbedderOption) *OllamaEmbedder {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultOllamaEmbedBaseURL
	}
	if strings.TrimSpace(model) == "" {
		model = defaultOllamaEmbedModel
	}

	timeout := 30 * time.Second
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	e := &OllamaEmbedder{
		baseURL:   strings.TrimRight(baseURL, "/"),
		model:     model,
		dimension: DefaultVectorDimension,
		timeout:   timeout,
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// Embed embeds a single input text.
func (o *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if strings.TrimSpace(text) == "" {
		return nil, &ErrEmptyInput{}
	}

	resp, err := o.embed(ctx, text)
	if err != nil {
		return nil, &ErrEmbeddingFailed{Text: text, Err: err}
	}

	vector, err := o.singleVector(resp)
	if err != nil {
		return nil, &ErrEmbeddingFailed{Text: text, Err: err}
	}
	if len(vector) != o.dimension {
		return nil, &ErrDimensionMismatch{Expected: o.dimension, Actual: len(vector)}
	}

	return vector, nil
}

// BatchEmbed embeds multiple input texts. It uses one request when supported,
// and falls back to per-item requests if the server rejects batched input.
func (o *OllamaEmbedder) BatchEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, &ErrEmptyInput{}
	}
	for _, text := range texts {
		if strings.TrimSpace(text) == "" {
			return nil, &ErrEmptyInput{}
		}
	}

	resp, err := o.embed(ctx, texts)
	if err == nil {
		vectors, vectorsErr := o.batchVectors(resp, len(texts))
		if vectorsErr == nil {
			return vectors, nil
		}
		return nil, &ErrEmbeddingFailed{Err: vectorsErr}
	}

	if !isBatchFallbackError(err) {
		return nil, &ErrEmbeddingFailed{Err: err}
	}

	// For partial failures, failed positions remain nil to preserve input order
	// per the Embedder interface contract.
	vectors := make([][]float32, len(texts))
	failed := 0
	var lastErr error
	for i, text := range texts {
		vector, embedErr := o.Embed(ctx, text)
		if embedErr != nil {
			failed++
			lastErr = embedErr
			continue
		}
		vectors[i] = vector
	}

	if failed == 0 {
		return vectors, nil
	}
	if failed == len(texts) {
		return nil, &ErrEmbeddingFailed{Err: lastErr}
	}

	return vectors, &ErrBatchPartialFailure{Total: len(texts), Failed: failed, Err: lastErr}
}

func (o *OllamaEmbedder) endpoint() (string, error) {
	if _, err := url.ParseRequestURI(o.baseURL); err != nil {
		return "", fmt.Errorf("invalid ollama base URL %q: %w", o.baseURL, err)
	}
	return o.baseURL + ollamaEmbedPath, nil
}

func (o *OllamaEmbedder) embed(ctx context.Context, input any) (*ollamaEmbedResponse, error) {
	endpoint, err := o.endpoint()
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(ollamaEmbedRequest{
		Model: o.model,
		Input: input,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ollama embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create ollama embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, formatOllamaEmbedConnectionError(endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, ollamaEmbedErrorBodyLimit))
		return nil, &ollamaEmbedHTTPStatusError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(respBody)),
		}
	}

	var decoded ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("failed to decode ollama embed response: %w", err)
	}
	return &decoded, nil
}

func formatOllamaEmbedConnectionError(endpoint string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return fmt.Errorf("ollama embed timeout (%s): %w", endpoint, err)
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return fmt.Errorf("ollama embed timeout (%s): %w", endpoint, urlErr)
		}
		return fmt.Errorf("ollama embed connection error (%s): %w", endpoint, urlErr.Err)
	}
	return fmt.Errorf("ollama embed connection error (%s): %w", endpoint, err)
}

func (o *OllamaEmbedder) singleVector(resp *ollamaEmbedResponse) ([]float32, error) {
	if len(resp.Embeddings) > 0 {
		return resp.Embeddings[0], nil
	}
	if len(resp.Embedding) > 0 {
		return resp.Embedding, nil
	}
	return nil, errors.New("ollama embed response missing embeddings")
}

func (o *OllamaEmbedder) batchVectors(resp *ollamaEmbedResponse, expected int) ([][]float32, error) {
	if len(resp.Embeddings) == expected {
		for _, vector := range resp.Embeddings {
			if len(vector) != o.dimension {
				return nil, &ErrDimensionMismatch{Expected: o.dimension, Actual: len(vector)}
			}
		}
		return resp.Embeddings, nil
	}
	if expected == 1 && len(resp.Embedding) > 0 {
		if len(resp.Embedding) != o.dimension {
			return nil, &ErrDimensionMismatch{Expected: o.dimension, Actual: len(resp.Embedding)}
		}
		return [][]float32{resp.Embedding}, nil
	}
	return nil, fmt.Errorf("embedding count mismatch: expected %d vectors, got %d", expected, len(resp.Embeddings))
}

func isBatchFallbackError(err error) bool {
	var statusErr *ollamaEmbedHTTPStatusError
	if !errors.As(err, &statusErr) {
		return false
	}
	return statusErr.StatusCode == http.StatusBadRequest ||
		statusErr.StatusCode == http.StatusNotFound ||
		statusErr.StatusCode == http.StatusUnprocessableEntity
}

type ollamaEmbedRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

type ollamaEmbedResponse struct {
	Embedding  []float32   `json:"embedding"`
	Embeddings [][]float32 `json:"embeddings"`
}

type ollamaEmbedHTTPStatusError struct {
	StatusCode int
	Body       string
}

func (e *ollamaEmbedHTTPStatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("ollama embed request failed with status %d", e.StatusCode)
	}
	return fmt.Sprintf("ollama embed request failed with status %d: %s", e.StatusCode, e.Body)
}

var _ Embedder = (*OllamaEmbedder)(nil)
