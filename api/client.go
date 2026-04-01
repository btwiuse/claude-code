// Package api provides the Anthropic API client integration using
// the official anthropic-sdk-go package.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/btwiuse/claude-code/tools"
)

// DefaultModel is the default Claude model to use.
const DefaultModel = "claude-sonnet-4-20250514"

// Client wraps the Anthropic API client with tool support.
type Client struct {
	sdk   anthropic.Client
	model string
}

// NewClient creates a new API client.
func NewClient(apiKey string, model string) *Client {
	opts := []option.RequestOption{}
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}

	c := anthropic.NewClient(opts...)

	if model == "" {
		model = DefaultModel
	}

	return &Client{
		sdk:   c,
		model: model,
	}
}

// toolDef converts a tool to the Anthropic API tool parameter format.
func toolDef(t tools.Tool) anthropic.ToolUnionParam {
	var schema struct {
		Properties interface{} `json:"properties,omitempty"`
		Required   []string    `json:"required,omitempty"`
	}
	_ = json.Unmarshal(t.InputSchema(), &schema)

	return anthropic.ToolUnionParam{
		OfTool: &anthropic.ToolParam{
			Name:        t.Name(),
			Description: anthropic.String(t.Description()),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: schema.Properties,
				Required:   schema.Required,
			},
		},
	}
}

// ContentBlock represents a block of content in a response.
type ContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// StreamEvent represents a streaming event from the API.
type StreamEvent struct {
	Type       string
	Text       string // For text deltas
	ToolUseID  string // For tool_use content block start
	ToolName   string // For tool_use content block start
	InputJSON  string // For input_json_delta
	StopReason string
}

// Usage tracks token usage.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Response represents a complete API response.
type Response struct {
	ID         string         `json:"id"`
	Content    []ContentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
	Usage      *Usage         `json:"usage,omitempty"`
	Model      string         `json:"model"`
}

// ToolUseBlocks extracts tool use blocks from the response.
func (r *Response) ToolUseBlocks() []ContentBlock {
	var blocks []ContentBlock
	for _, b := range r.Content {
		if b.Type == "tool_use" {
			blocks = append(blocks, b)
		}
	}
	return blocks
}

// TextContent extracts all text content from the response.
func (r *Response) TextContent() string {
	var parts []string
	for _, b := range r.Content {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "")
}

// HasToolUse returns true if the response contains tool use blocks.
func (r *Response) HasToolUse() bool {
	for _, b := range r.Content {
		if b.Type == "tool_use" {
			return true
		}
	}
	return false
}

// MessageParams holds parameters for a message request.
type MessageParams struct {
	Messages     []anthropic.MessageParam
	SystemPrompt string
	Tools        []tools.Tool
	MaxTokens    int
	Model        string
}

// SendMessage sends a message and returns the complete response.
func (c *Client) SendMessage(ctx context.Context, params MessageParams) (*Response, error) {
	model := c.model
	if params.Model != "" {
		model = params.Model
	}

	maxTokens := int64(8192)
	if params.MaxTokens > 0 {
		maxTokens = int64(params.MaxTokens)
	}

	// Build tools
	var toolDefs []anthropic.ToolUnionParam
	for _, t := range params.Tools {
		toolDefs = append(toolDefs, toolDef(t))
	}

	reqParams := anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: maxTokens,
		Messages:  params.Messages,
	}

	if params.SystemPrompt != "" {
		reqParams.System = []anthropic.TextBlockParam{
			{Text: params.SystemPrompt},
		}
	}

	if len(toolDefs) > 0 {
		reqParams.Tools = toolDefs
	}

	msg, err := c.sdk.Messages.New(ctx, reqParams)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}

	return convertMessage(msg), nil
}

// StreamMessage sends a message and streams the response.
func (c *Client) StreamMessage(ctx context.Context, params MessageParams, handler func(event StreamEvent)) (*Response, error) {
	model := c.model
	if params.Model != "" {
		model = params.Model
	}

	maxTokens := int64(8192)
	if params.MaxTokens > 0 {
		maxTokens = int64(params.MaxTokens)
	}

	// Build tools
	var toolDefs []anthropic.ToolUnionParam
	for _, t := range params.Tools {
		toolDefs = append(toolDefs, toolDef(t))
	}

	reqParams := anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: maxTokens,
		Messages:  params.Messages,
	}

	if params.SystemPrompt != "" {
		reqParams.System = []anthropic.TextBlockParam{
			{Text: params.SystemPrompt},
		}
	}

	if len(toolDefs) > 0 {
		reqParams.Tools = toolDefs
	}

	stream := c.sdk.Messages.NewStreaming(ctx, reqParams)
	defer stream.Close()

	// Accumulate content blocks for the final response
	var accumulated []ContentBlock
	var currentBlock *ContentBlock
	var inputAccum strings.Builder
	var responseID string
	var stopReason string
	var usage Usage

	for stream.Next() {
		evt := stream.Current()

		switch evt.Type {
		case "message_start":
			responseID = evt.Message.ID
			usage.InputTokens = int(evt.Message.Usage.InputTokens)

		case "content_block_start":
			cb := evt.ContentBlock
			switch cb.Type {
			case "text":
				currentBlock = &ContentBlock{Type: "text"}
			case "tool_use":
				currentBlock = &ContentBlock{
					Type: "tool_use",
					ID:   cb.ID,
					Name: cb.Name,
				}
				inputAccum.Reset()
				if handler != nil {
					handler(StreamEvent{
						Type:      "tool_use_start",
						ToolUseID: cb.ID,
						ToolName:  cb.Name,
					})
				}
			}

		case "content_block_delta":
			switch evt.Delta.Type {
			case "text_delta":
				if currentBlock != nil {
					currentBlock.Text += evt.Delta.Text
				}
				if handler != nil {
					handler(StreamEvent{
						Type: "text_delta",
						Text: evt.Delta.Text,
					})
				}
			case "input_json_delta":
				inputAccum.WriteString(evt.Delta.PartialJSON)
				if handler != nil {
					handler(StreamEvent{
						Type:      "input_json_delta",
						InputJSON: evt.Delta.PartialJSON,
					})
				}
			}

		case "content_block_stop":
			if currentBlock != nil {
				if currentBlock.Type == "tool_use" && inputAccum.Len() > 0 {
					currentBlock.Input = json.RawMessage(inputAccum.String())
				}
				accumulated = append(accumulated, *currentBlock)
				currentBlock = nil
			}

		case "message_delta":
			stopReason = string(evt.Delta.StopReason)
			usage.OutputTokens = int(evt.Usage.OutputTokens)
			if handler != nil {
				handler(StreamEvent{
					Type:       "message_delta",
					StopReason: stopReason,
				})
			}

		case "message_stop":
			// Final event
		}
	}

	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("stream error: %w", err)
	}

	return &Response{
		ID:         responseID,
		Content:    accumulated,
		StopReason: stopReason,
		Model:      model,
		Usage:      &usage,
	}, nil
}

// convertMessage converts an SDK Message to our Response type.
func convertMessage(msg *anthropic.Message) *Response {
	if msg == nil {
		return &Response{}
	}

	var content []ContentBlock
	for _, b := range msg.Content {
		switch b.Type {
		case "text":
			content = append(content, ContentBlock{
				Type: "text",
				Text: b.Text,
			})
		case "tool_use":
			inputJSON, _ := json.Marshal(b.Input)
			content = append(content, ContentBlock{
				Type:  "tool_use",
				ID:    b.ID,
				Name:  b.Name,
				Input: inputJSON,
			})
		}
	}

	return &Response{
		ID:         msg.ID,
		Content:    content,
		StopReason: string(msg.StopReason),
		Model:      string(msg.Model),
		Usage: &Usage{
			InputTokens:  int(msg.Usage.InputTokens),
			OutputTokens: int(msg.Usage.OutputTokens),
		},
	}
}

// TextBlock creates a text content block parameter.
func TextBlock(text string) anthropic.ContentBlockParamUnion {
	return anthropic.ContentBlockParamUnion{
		OfText: &anthropic.TextBlockParam{
			Text: text,
		},
	}
}

// ToolUseBlock creates a tool use content block parameter.
func ToolUseBlock(id, name string, input interface{}) anthropic.ContentBlockParamUnion {
	return anthropic.ContentBlockParamUnion{
		OfToolUse: &anthropic.ToolUseBlockParam{
			ID:    id,
			Name:  name,
			Input: input,
		},
	}
}

// ToolResultBlock creates a tool result content block parameter.
func ToolResultBlock(toolUseID string, content string) anthropic.ContentBlockParamUnion {
	return anthropic.ContentBlockParamUnion{
		OfToolResult: &anthropic.ToolResultBlockParam{
			ToolUseID: toolUseID,
			Content: []anthropic.ToolResultBlockParamContentUnion{
				{OfText: &anthropic.TextBlockParam{Text: content}},
			},
		},
	}
}

// ResolveModel maps short model names to full model identifiers.
func ResolveModel(name string) string {
	switch strings.ToLower(name) {
	case "sonnet", "claude-sonnet":
		return "claude-sonnet-4-20250514"
	case "opus", "claude-opus":
		return "claude-opus-4-20250514"
	case "haiku", "claude-haiku":
		return "claude-haiku-3-5-20241022"
	default:
		if name == "" {
			return DefaultModel
		}
		return name
	}
}

// APIKeyFromEnv returns the Anthropic API key from environment variables.
func APIKeyFromEnv() string {
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key
	}
	return ""
}
