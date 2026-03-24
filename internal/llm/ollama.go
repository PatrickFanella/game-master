package llm

import (
	"bufio"
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
	defaultOllamaBaseURL = "http://localhost:11434"
	defaultOllamaModel   = "llama3.2"
	ollamaChatPath       = "/api/chat"
)

// OllamaClient implements Provider via Ollama's HTTP API.
type OllamaClient struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewOllamaClient returns an Ollama-backed provider.
func NewOllamaClient(baseURL, model string) *OllamaClient {
	if baseURL == "" {
		baseURL = defaultOllamaBaseURL
	}
	if model == "" {
		model = defaultOllamaModel
	}

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

	return &OllamaClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

func (o *OllamaClient) Complete(ctx context.Context, messages []Message, tools []Tool) (*Response, error) {
	resp, err := o.callChat(ctx, messages, tools, false)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var chatResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, &ErrMalformedResponse{
			URL: o.baseURL + ollamaChatPath,
			Err: fmt.Errorf("failed to decode ollama chat response: %w", err),
		}
	}

	toolCalls, err := fromOllamaToolCalls(chatResp.Message.ToolCalls)
	if err != nil {
		return nil, &ErrMalformedResponse{
			URL: o.baseURL + ollamaChatPath,
			Err: err,
		}
	}

	return &Response{
		Content:      chatResp.Message.Content,
		ToolCalls:    toolCalls,
		FinishReason: chatResp.DoneReason,
		Usage: Usage{
			PromptTokens:     chatResp.PromptEvalCount,
			CompletionTokens: chatResp.EvalCount,
			TotalTokens:      chatResp.PromptEvalCount + chatResp.EvalCount,
		},
	}, nil
}

func (o *OllamaClient) Stream(ctx context.Context, messages []Message, tools []Tool) (<-chan StreamChunk, error) {
	resp, err := o.callChat(ctx, messages, tools, true)
	if err != nil {
		return nil, err
	}

	ch := make(chan StreamChunk)
	go func() {
		defer close(ch)
		defer func() { _ = resp.Body.Close() }()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
		for scanner.Scan() {
			var chunkResp ollamaChatResponse
			if err := json.Unmarshal(scanner.Bytes(), &chunkResp); err != nil {
				return
			}

			var toolDelta *ToolCall
			if len(chunkResp.Message.ToolCalls) > 0 {
				calls, err := fromOllamaToolCalls(chunkResp.Message.ToolCalls)
				if err == nil && len(calls) > 0 {
					toolDelta = &calls[0]
				}
			}

			select {
			case ch <- StreamChunk{
				ContentDelta:  chunkResp.Message.Content,
				ToolCallDelta: toolDelta,
				Done:          chunkResp.Done,
			}:
			case <-ctx.Done():
				return
			}
			if chunkResp.Done {
				return
			}
		}
	}()

	return ch, nil
}

func (o *OllamaClient) callChat(ctx context.Context, messages []Message, tools []Tool, stream bool) (*http.Response, error) {
	chatURL, err := o.chatURL()
	if err != nil {
		return nil, err
	}

	ollamaMessages, err := toOllamaMessages(messages)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(ollamaChatRequest{
		Model:    o.model,
		Messages: ollamaMessages,
		Tools:    toOllamaTools(tools),
		Stream:   stream,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ollama chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create ollama chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, formatConnectionError(chatURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer func() { _ = resp.Body.Close() }()
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, classifyHTTPError(chatURL, o.model, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return resp, nil
}

func (o *OllamaClient) chatURL() (string, error) {
	if _, err := url.ParseRequestURI(o.baseURL); err != nil {
		return "", fmt.Errorf("invalid ollama base url %q: %w", o.baseURL, err)
	}
	return o.baseURL + ollamaChatPath, nil
}

func formatConnectionError(endpoint string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return &ErrTimeout{URL: endpoint, Err: err}
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return &ErrTimeout{URL: endpoint, Err: urlErr}
		}
		return &ErrConnection{URL: endpoint, Err: urlErr.Err}
	}
	return &ErrConnection{URL: endpoint, Err: err}
}

func classifyHTTPError(endpoint, model string, statusCode int, body string) error {
	baseErr := fmt.Errorf("ollama chat request failed with status %d: %s", statusCode, body)

	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &ErrAuth{
			URL:        endpoint,
			StatusCode: statusCode,
			Err:        baseErr,
		}
	case http.StatusTooManyRequests:
		return &ErrRateLimit{
			URL:        endpoint,
			StatusCode: statusCode,
			Err:        baseErr,
		}
	case http.StatusNotFound:
		if isModelNotFoundMessage(body) {
			return &ErrModelNotFound{
				URL:        endpoint,
				Model:      model,
				StatusCode: statusCode,
				Err:        baseErr,
			}
		}
		return &ErrConnection{
			URL: endpoint,
			Err: baseErr,
		}
	default:
		return &ErrConnection{
			URL: endpoint,
			Err: baseErr,
		}
	}
}

func isModelNotFoundMessage(body string) bool {
	text := strings.ToLower(body)
	if !strings.Contains(text, "model") {
		return false
	}
	return strings.Contains(text, "not found") ||
		strings.Contains(text, "does not exist") ||
		strings.Contains(text, "unknown model")
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Tools    []ollamaTool    `json:"tools,omitempty"`
	Stream   bool            `json:"stream"`
}

type ollamaMessage struct {
	Role       Role             `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCalls  []ollamaToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type ollamaTool struct {
	Type     string             `json:"type"`
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	Arguments   string         `json:"arguments,omitempty"`
}

type ollamaToolCall struct {
	Function ollamaToolFunction `json:"function"`
}

type ollamaChatResponse struct {
	Message struct {
		Content   string           `json:"content"`
		ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
	} `json:"message"`
	Done            bool   `json:"done"`
	DoneReason      string `json:"done_reason"`
	PromptEvalCount int    `json:"prompt_eval_count"`
	EvalCount       int    `json:"eval_count"`
}

func toOllamaMessages(messages []Message) ([]ollamaMessage, error) {
	if len(messages) == 0 {
		return make([]ollamaMessage, 0), nil
	}
	out := make([]ollamaMessage, 0, len(messages))
	for _, msg := range messages {
		toolCalls, err := toOllamaToolCalls(msg.ToolCalls)
		if err != nil {
			return nil, err
		}
		out = append(out, ollamaMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCalls:  toolCalls,
			ToolCallID: msg.ToolCallID,
		})
	}
	return out, nil
}

func toOllamaTools(tools []Tool) []ollamaTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]ollamaTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, ollamaTool{
			Type: "function",
			Function: ollamaToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return out
}

func toOllamaToolCalls(calls []ToolCall) ([]ollamaToolCall, error) {
	if len(calls) == 0 {
		return nil, nil
	}
	out := make([]ollamaToolCall, 0, len(calls))
	for _, c := range calls {
		args, err := json.Marshal(c.Arguments)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tool arguments for %q: %w", c.Name, err)
		}
		out = append(out, ollamaToolCall{
			Function: ollamaToolFunction{
				Name:      c.Name,
				Arguments: string(args),
			},
		})
	}
	return out, nil
}

func fromOllamaToolCalls(calls []ollamaToolCall) ([]ToolCall, error) {
	if len(calls) == 0 {
		return nil, nil
	}

	out := make([]ToolCall, 0, len(calls))
	for _, c := range calls {
		parsed := map[string]any{}
		if c.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(c.Function.Arguments), &parsed); err != nil {
				return nil, fmt.Errorf("failed to decode ollama tool arguments for %q: %w", c.Function.Name, err)
			}
		}
		out = append(out, ToolCall{Name: c.Function.Name, Arguments: parsed})
	}
	return out, nil
}
