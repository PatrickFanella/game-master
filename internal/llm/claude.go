package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultClaudeBaseURL      = "https://api.anthropic.com"
	defaultClaudeModel        = "claude-sonnet-4-6"
	defaultAnthropicVersion   = "2023-06-01"
	defaultClaudeMaxTokens    = 1024
	claudeMessagesPath        = "/v1/messages"
	claudeRoleUser            = "user"
	claudeRoleAssistant       = "assistant"
	claudeContentTypeText     = "text"
	claudeContentTypeToolUse  = "tool_use"
	claudeContentTypeToolResp = "tool_result"
)

// ClaudeClient implements Provider via Anthropic's Messages API.
type ClaudeClient struct {
	baseURL           string
	apiKey            string
	model             string
	anthropicVersion  string
	maxTokens         int
	client            *http.Client
}

// NewClaudeClient returns a Claude-backed provider.
func NewClaudeClient(baseURL, apiKey, model string) *ClaudeClient {
	if baseURL == "" {
		baseURL = defaultClaudeBaseURL
	}
	if model == "" {
		model = defaultClaudeModel
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

	return &ClaudeClient{
		baseURL:          strings.TrimRight(baseURL, "/"),
		apiKey:           apiKey,
		model:            model,
		anthropicVersion: defaultAnthropicVersion,
		maxTokens:        defaultClaudeMaxTokens,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

func (c *ClaudeClient) Complete(ctx context.Context, messages []Message, tools []Tool) (*Response, error) {
	resp, err := c.callMessages(ctx, messages, tools)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var messagesResp claudeMessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&messagesResp); err != nil {
		return nil, &ErrMalformedResponse{
			URL: c.baseURL + claudeMessagesPath,
			Err: fmt.Errorf("failed to decode claude messages response: %w", err),
		}
	}

	content, toolCalls, err := fromClaudeContent(messagesResp.Content)
	if err != nil {
		return nil, &ErrMalformedResponse{
			URL: c.baseURL + claudeMessagesPath,
			Err: err,
		}
	}

	return &Response{
		Content:      content,
		ToolCalls:    toolCalls,
		FinishReason: messagesResp.StopReason,
		Usage: Usage{
			PromptTokens:     messagesResp.Usage.InputTokens,
			CompletionTokens: messagesResp.Usage.OutputTokens,
			TotalTokens:      messagesResp.Usage.InputTokens + messagesResp.Usage.OutputTokens,
		},
	}, nil
}

func (c *ClaudeClient) Stream(ctx context.Context, messages []Message, tools []Tool) (<-chan StreamChunk, error) {
	resp, err := c.Complete(ctx, messages, tools)
	if err != nil {
		return nil, err
	}

	ch := make(chan StreamChunk, len(resp.ToolCalls)+1)
	for _, tc := range resp.ToolCalls {
		tcCopy := tc
		ch <- StreamChunk{
			ContentDelta:  "",
			ToolCallDelta: &tcCopy,
			Done:          false,
		}
	}
	ch <- StreamChunk{
		ContentDelta:  resp.Content,
		ToolCallDelta: nil,
		Done:          true,
	}
	close(ch)
	return ch, nil
}

func (c *ClaudeClient) callMessages(ctx context.Context, messages []Message, tools []Tool) (*http.Response, error) {
	messagesURL, err := c.messagesURL()
	if err != nil {
		return nil, err
	}

	claudeMessages, system, err := toClaudeMessages(messages)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(claudeMessagesRequest{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		System:    system,
		Messages:  claudeMessages,
		Tools:     toClaudeTools(tools),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal claude messages request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, messagesURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create claude messages request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", c.anthropicVersion)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, formatConnectionError(messagesURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer func() { _ = resp.Body.Close() }()
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, classifyClaudeHTTPError(messagesURL, c.model, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return resp, nil
}

func (c *ClaudeClient) messagesURL() (string, error) {
	if _, err := url.ParseRequestURI(c.baseURL); err != nil {
		return "", fmt.Errorf("invalid claude base url %q: %w", c.baseURL, err)
	}
	return c.baseURL + claudeMessagesPath, nil
}

func classifyClaudeHTTPError(endpoint, model string, statusCode int, body string) error {
	baseErr := fmt.Errorf("claude messages request failed with status %d: %s", statusCode, body)

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

type claudeMessagesRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"`
	Messages  []claudeMessage `json:"messages"`
	Tools     []claudeTool    `json:"tools,omitempty"`
}

type claudeMessage struct {
	Role    string               `json:"role"`
	Content []claudeContentBlock `json:"content"`
}

type claudeTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type claudeContentBlock struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	Content   string         `json:"content,omitempty"`
}

type claudeMessagesResponse struct {
	Content    []claudeContentBlock `json:"content"`
	StopReason string               `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func toClaudeMessages(messages []Message) ([]claudeMessage, string, error) {
	if len(messages) == 0 {
		return make([]claudeMessage, 0), "", nil
	}

	out := make([]claudeMessage, 0, len(messages))
	systemContent := make([]string, 0)

	for _, msg := range messages {
		switch msg.Role {
		case RoleSystem:
			if msg.Content != "" {
				systemContent = append(systemContent, msg.Content)
			}
		case RoleUser:
			out = append(out, claudeMessage{
				Role: claudeRoleUser,
				Content: []claudeContentBlock{
					{Type: claudeContentTypeText, Text: msg.Content},
				},
			})
		case RoleAssistant:
			content := make([]claudeContentBlock, 0, 1+len(msg.ToolCalls))
			if msg.Content != "" {
				content = append(content, claudeContentBlock{Type: claudeContentTypeText, Text: msg.Content})
			}
			for _, toolCall := range msg.ToolCalls {
				content = append(content, claudeContentBlock{
					Type:  claudeContentTypeToolUse,
					ID:    toolCall.ID,
					Name:  toolCall.Name,
					Input: toolCall.Arguments,
				})
			}
			if len(content) == 0 {
				return nil, "", fmt.Errorf("assistant message must include content or tool calls")
			}
			out = append(out, claudeMessage{Role: claudeRoleAssistant, Content: content})
		case RoleTool:
			if msg.ToolCallID == "" {
				return nil, "", fmt.Errorf("tool message missing tool_call_id")
			}
			out = append(out, claudeMessage{
				Role: claudeRoleUser,
				Content: []claudeContentBlock{
					{
						Type:      claudeContentTypeToolResp,
						ToolUseID: msg.ToolCallID,
						Content:   msg.Content,
					},
				},
			})
		default:
			return nil, "", fmt.Errorf("unsupported message role %q for claude", msg.Role)
		}
	}

	return out, strings.Join(systemContent, "\n\n"), nil
}

func toClaudeTools(tools []Tool) []claudeTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]claudeTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, claudeTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}
	return out
}

func fromClaudeContent(content []claudeContentBlock) (string, []ToolCall, error) {
	if len(content) == 0 {
		return "", nil, fmt.Errorf("claude response contained no content blocks")
	}

	var text strings.Builder
	toolCalls := make([]ToolCall, 0)
	for _, block := range content {
		switch block.Type {
		case claudeContentTypeText:
			text.WriteString(block.Text)
		case claudeContentTypeToolUse:
			toolCalls = append(toolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: block.Input,
			})
		default:
			return "", nil, fmt.Errorf("unsupported claude content block type %q", block.Type)
		}
	}
	if text.Len() == 0 && len(toolCalls) == 0 {
		return "", nil, fmt.Errorf("claude response contained no text or tool calls")
	}
	return text.String(), toolCalls, nil
}
