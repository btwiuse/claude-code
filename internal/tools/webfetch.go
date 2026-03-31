package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/anthropics/claude-code-go/internal/types"
)

// WebFetchTool fetches content from URLs.
type WebFetchTool struct{}

// WebFetchInput is the input schema for the WebFetch tool.
type WebFetchInput struct {
	URL       string `json:"url"`
	MaxLength int    `json:"max_length,omitempty"`
}

// NewWebFetchTool creates a new WebFetchTool.
func NewWebFetchTool() *WebFetchTool {
	return &WebFetchTool{}
}

func (t *WebFetchTool) Name() string        { return "WebFetch" }
func (t *WebFetchTool) IsReadOnly() bool     { return true }
func (t *WebFetchTool) IsEnabled() bool      { return true }

func (t *WebFetchTool) Description() string {
	return `Fetch content from a URL. Returns the text content of a web page.

Use this to retrieve documentation, API references, or other web content. The response is returned as plain text with HTML tags stripped.

Note: Some URLs may be blocked or unreachable. The tool has a 30-second timeout.`
}

func (t *WebFetchTool) InputSchema() types.ToolInputSchema {
	return types.ToolInputSchema{
		Type: "object",
		Properties: map[string]types.ToolPropertySchema{
			"url": {
				Type:        "string",
				Description: "The URL to fetch content from.",
			},
			"max_length": {
				Type:        "integer",
				Description: "Maximum number of characters to return. Defaults to 5000.",
				Default:     5000,
			},
		},
		Required: []string{"url"},
	}
}

func (t *WebFetchTool) UserFacingName(input json.RawMessage) string {
	var in WebFetchInput
	if err := json.Unmarshal(input, &in); err == nil && in.URL != "" {
		url := in.URL
		if len(url) > 50 {
			url = url[:47] + "..."
		}
		return fmt.Sprintf("WebFetch: %s", url)
	}
	return "WebFetch"
}

func (t *WebFetchTool) Execute(ctx context.Context, input json.RawMessage, toolCtx *ToolContext) (*ToolResult, error) {
	var in WebFetchInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &ToolResult{Content: fmt.Sprintf("Invalid input: %v", err), IsError: true}, nil
	}

	if in.URL == "" {
		return &ToolResult{Content: "url is required", IsError: true}, nil
	}

	maxLength := 5000
	if in.MaxLength > 0 {
		maxLength = in.MaxLength
	}
	if maxLength > 20000 {
		maxLength = 20000
	}

	// Validate URL scheme
	if !strings.HasPrefix(in.URL, "http://") && !strings.HasPrefix(in.URL, "https://") {
		return &ToolResult{Content: "URL must start with http:// or https://", IsError: true}, nil
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", in.URL, nil)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("Invalid URL: %v", err), IsError: true}, nil
	}
	req.Header.Set("User-Agent", "claude-code-go/0.1.0")

	resp, err := client.Do(req)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("Fetch failed: %v", err), IsError: true}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &ToolResult{
			Content: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status),
			IsError: true,
		}, nil
	}

	// Read body with limit
	limitReader := io.LimitReader(resp.Body, int64(maxLength*2)) // Read extra for HTML stripping
	body, err := io.ReadAll(limitReader)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("Error reading response: %v", err), IsError: true}, nil
	}

	content := string(body)

	// Basic HTML tag stripping
	content = stripHTML(content)

	// Truncate to max length
	if len(content) > maxLength {
		content = content[:maxLength] + "\n... (content truncated)"
	}

	return &ToolResult{Content: content}, nil
}

// stripHTML does basic HTML tag removal.
func stripHTML(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}

	// Clean up excessive whitespace
	text := result.String()
	lines := strings.Split(text, "\n")
	var cleaned []string
	prevEmpty := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if !prevEmpty {
				cleaned = append(cleaned, "")
				prevEmpty = true
			}
		} else {
			cleaned = append(cleaned, trimmed)
			prevEmpty = false
		}
	}

	return strings.Join(cleaned, "\n")
}
