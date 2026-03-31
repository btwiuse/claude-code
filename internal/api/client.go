// Package api provides the Claude API client with streaming support.
package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/anthropics/claude-code-go/internal/config"
	"github.com/anthropics/claude-code-go/internal/constants"
	"github.com/anthropics/claude-code-go/internal/types"
)

// Client is the Claude API client.
type Client struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string
	model      string
	maxTokens  int
	headers    map[string]string
	sessionID  string
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithAPIKey sets the API key.
func WithAPIKey(key string) ClientOption {
	return func(c *Client) {
		c.apiKey = key
	}
}

// WithBaseURL sets the API base URL.
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithModel sets the default model.
func WithModel(model string) ClientOption {
	return func(c *Client) {
		c.model = model
	}
}

// WithMaxTokens sets the default max tokens.
func WithMaxTokens(tokens int) ClientOption {
	return func(c *Client) {
		c.maxTokens = tokens
	}
}

// WithSessionID sets the session ID for request tracking.
func WithSessionID(id string) ClientOption {
	return func(c *Client) {
		c.sessionID = id
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// NewClient creates a new Claude API client.
func NewClient(opts ...ClientOption) (*Client, error) {
	c := &Client{
		httpClient: &http.Client{
			Timeout: time.Duration(constants.APITimeoutSeconds) * time.Second,
		},
		baseURL:   getBaseURL(),
		model:     getModel(),
		maxTokens: constants.DefaultMaxTokens,
		headers:   make(map[string]string),
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.apiKey == "" {
		c.apiKey = config.GetAPIKey()
	}
	if c.apiKey == "" {
		return nil, fmt.Errorf("no API key configured; set ANTHROPIC_API_KEY environment variable or configure via 'claude-code config'")
	}

	// Parse custom headers from environment
	if customHeaders := os.Getenv("ANTHROPIC_CUSTOM_HEADERS"); customHeaders != "" {
		for _, line := range strings.Split(customHeaders, "\n") {
			line = strings.TrimSpace(line)
			if parts := strings.SplitN(line, ":", 2); len(parts) == 2 {
				c.headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}

	return c, nil
}

func getBaseURL() string {
	if url := os.Getenv("ANTHROPIC_BASE_URL"); url != "" {
		return url
	}
	return "https://api.anthropic.com"
}

func getModel() string {
	if model := os.Getenv("ANTHROPIC_MODEL"); model != "" {
		return model
	}
	return constants.DefaultModel
}

// GetModel returns the configured model name.
func (c *Client) GetModel() string {
	return c.model
}

// CreateMessageRequest contains the parameters for creating a message.
type CreateMessageRequest struct {
	Model         string                 `json:"model"`
	MaxTokens     int                    `json:"max_tokens"`
	Messages      []types.Message        `json:"messages"`
	System        []types.SystemBlock    `json:"system,omitempty"`
	Tools         []types.ToolDefinition `json:"tools,omitempty"`
	Stream        bool                   `json:"stream"`
	StopSequences []string               `json:"stop_sequences,omitempty"`
	Temperature   *float64               `json:"temperature,omitempty"`
	Thinking      *types.ThinkingConfig  `json:"thinking,omitempty"`
}

// CreateMessage sends a non-streaming message request.
func (c *Client) CreateMessage(ctx context.Context, req *CreateMessageRequest) (*types.APIResponse, error) {
	if req.Model == "" {
		req.Model = c.model
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = c.maxTokens
	}
	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	var apiResp types.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &apiResp, nil
}

// StreamMessage sends a streaming message request and returns a channel of events.
func (c *Client) StreamMessage(ctx context.Context, req *CreateMessageRequest) (<-chan StreamResult, error) {
	if req.Model == "" {
		req.Model = c.model
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = c.maxTokens
	}
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, parseAPIError(resp)
	}

	ch := make(chan StreamResult, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		c.processSSEStream(ctx, resp.Body, ch)
	}()

	return ch, nil
}

// StreamResult wraps either a stream event or an error.
type StreamResult struct {
	Event *types.StreamEvent
	Error error
}

func (c *Client) processSSEStream(ctx context.Context, body io.Reader, ch chan<- StreamResult) {
	scanner := bufio.NewScanner(body)
	// Allow larger lines for base64 image content
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	var eventType string
	var dataLines []string

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			ch <- StreamResult{Error: ctx.Err()}
			return
		default:
		}

		line := scanner.Text()

		if line == "" {
			// Empty line means end of event
			if eventType != "" && len(dataLines) > 0 {
				data := strings.Join(dataLines, "\n")
				if event, err := parseStreamEvent(eventType, data); err != nil {
					ch <- StreamResult{Error: fmt.Errorf("parsing stream event: %w", err)}
				} else if event != nil {
					ch <- StreamResult{Event: event}
				}
			}
			eventType = ""
			dataLines = nil
			continue
		}

		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- StreamResult{Error: fmt.Errorf("reading stream: %w", err)}
	}
}

func parseStreamEvent(eventType, data string) (*types.StreamEvent, error) {
	if eventType == "ping" {
		return nil, nil
	}

	var event types.StreamEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return nil, fmt.Errorf("unmarshaling event data for %s: %w", eventType, err)
	}
	event.Type = eventType
	return &event, nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "prompt-caching-2024-07-31,interleaved-thinking-2025-05-14")
	req.Header.Set("User-Agent", fmt.Sprintf("claude-code-go/%s", constants.Version))
	req.Header.Set("x-app", "cli")

	if c.sessionID != "" {
		req.Header.Set("X-Claude-Code-Session-Id", c.sessionID)
	}

	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
}

// APIError represents an error response from the Claude API.
type APIError struct {
	StatusCode int
	Type       string `json:"type"`
	ErrorInfo  struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (%d): %s - %s", e.StatusCode, e.ErrorInfo.Type, e.ErrorInfo.Message)
}

// IsOverloaded returns true if the API is overloaded.
func (e *APIError) IsOverloaded() bool {
	return e.StatusCode == 529
}

// IsRateLimited returns true if the request was rate limited.
func (e *APIError) IsRateLimited() bool {
	return e.StatusCode == 429
}

// IsAuthError returns true if there was an authentication error.
func (e *APIError) IsAuthError() bool {
	return e.StatusCode == 401 || e.StatusCode == 403
}

func parseAPIError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	apiErr := &APIError{StatusCode: resp.StatusCode}
	if err := json.Unmarshal(body, apiErr); err != nil {
		apiErr.ErrorInfo.Type = "unknown"
		apiErr.ErrorInfo.Message = string(body)
	}
	return apiErr
}
