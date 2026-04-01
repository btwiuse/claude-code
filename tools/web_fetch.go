package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// WebFetchTool fetches content from URLs.
type WebFetchTool struct{}

type webFetchInput struct {
	URL       string `json:"url"`
	MaxLength int    `json:"max_length,omitempty"`
	Raw       bool   `json:"raw,omitempty"`
}

func (t *WebFetchTool) Name() string { return "web_fetch" }

func (t *WebFetchTool) Description() string {
	return `Fetch content from a URL. Returns the page content as text. Use max_length to limit the response size.`
}

func (t *WebFetchTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "The URL to fetch"
			},
			"max_length": {
				"type": "integer",
				"description": "Maximum number of characters to return (default: 5000)"
			},
			"raw": {
				"type": "boolean",
				"description": "If true, returns raw HTML. If false, returns simplified text."
			}
		},
		"required": ["url"]
	}`)
}

func (t *WebFetchTool) IsReadOnly() bool { return true }

func (t *WebFetchTool) Run(ctx context.Context, input json.RawMessage) (*Result, error) {
	var in webFetchInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &Result{Error: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	if in.URL == "" {
		return &Result{Error: "url is required", IsError: true}, nil
	}

	if !strings.HasPrefix(in.URL, "http://") && !strings.HasPrefix(in.URL, "https://") {
		in.URL = "https://" + in.URL
	}

	maxLen := 5000
	if in.MaxLength > 0 {
		maxLen = in.MaxLength
		if maxLen > 20000 {
			maxLen = 20000
		}
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.URL, nil)
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to create request: %v", err), IsError: true}, nil
	}
	req.Header.Set("User-Agent", "Claude-Code/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to fetch URL: %v", err), IsError: true}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return &Result{
			Error:   fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status),
			IsError: true,
		}, nil
	}

	// Read limited body
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxLen)+1))
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to read response: %v", err), IsError: true}, nil
	}

	content := string(body)
	truncated := false
	if len(content) > maxLen {
		content = content[:maxLen]
		truncated = true
	}

	if truncated {
		content += "\n... (content truncated)"
	}

	return &Result{Output: content}, nil
}
