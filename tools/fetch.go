package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// WebFetchTool fetches content from a URL.
type WebFetchTool struct{}

type webFetchInput struct {
	URL       string `json:"url"`
	MaxLength int    `json:"max_length,omitempty"`
}

func (t *WebFetchTool) Name() string { return "WebFetch" }

func (t *WebFetchTool) Description() string {
	return "Fetch and extract content from a URL. Returns the page content as text."
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
				"description": "Maximum number of characters to return (default: 10000)"
			}
		},
		"required": ["url"]
	}`)
}

func (t *WebFetchTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in webFetchInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	if in.URL == "" {
		return Result{Content: "url is required", IsError: true}, nil
	}

	if in.MaxLength <= 0 {
		in.MaxLength = 10000
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", in.URL, nil)
	if err != nil {
		return Result{Content: fmt.Sprintf("invalid URL: %v", err), IsError: true}, nil
	}
	req.Header.Set("User-Agent", "claude-code/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return Result{Content: fmt.Sprintf("fetch error: %v", err), IsError: true}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Result{Content: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status), IsError: true}, nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(in.MaxLength*3)))
	if err != nil {
		return Result{Content: fmt.Sprintf("read error: %v", err), IsError: true}, nil
	}

	contentType := resp.Header.Get("Content-Type")
	content := string(body)

	// If HTML, extract text content.
	if strings.Contains(contentType, "text/html") {
		content = extractText(content)
	}

	if len(content) > in.MaxLength {
		content = content[:in.MaxLength] + "\n... (truncated)"
	}

	return Result{Content: content}, nil
}

// extractText extracts visible text from HTML.
func extractText(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}

	var sb strings.Builder
	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "noscript":
				return
			}
		}
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				sb.WriteString(text)
				sb.WriteString(" ")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
		if n.Type == html.ElementNode {
			switch n.Data {
			case "p", "br", "div", "h1", "h2", "h3", "h4", "h5", "h6", "li", "tr":
				sb.WriteString("\n")
			}
		}
	}
	extract(doc)
	return sb.String()
}
