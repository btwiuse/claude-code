package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/btwiuse/claude-code/tools"
)

// Client wraps the Anthropic SDK client for conversational AI with tool use.
type Client struct {
	client anthropic.Client
	model  string
}

// NewClient creates a new API client.
func NewClient(apiKey, model string) *Client {
	opts := []option.RequestOption{}
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	return &Client{
		client: anthropic.NewClient(opts...),
		model:  model,
	}
}

// ToolDef converts a tool to an Anthropic API tool union parameter.
func ToolDef(t tools.Tool) anthropic.ToolUnionParam {
	var schema struct {
		Properties interface{} `json:"properties"`
		Required   []string    `json:"required"`
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

// ToolDefs converts all tools in a registry to Anthropic API tool params.
func ToolDefs(registry *tools.Registry) []anthropic.ToolUnionParam {
	all := registry.All()
	defs := make([]anthropic.ToolUnionParam, 0, len(all))
	for _, t := range all {
		defs = append(defs, ToolDef(t))
	}
	return defs
}

// StreamResponse holds the result of a streaming message call.
type StreamResponse struct {
	Content    string
	ToolCalls  []ToolCall
	StopReason string
	Usage      Usage
}

// ToolCall represents a tool use request from the model.
type ToolCall struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// Usage tracks token counts.
type Usage struct {
	InputTokens  int64
	OutputTokens int64
}

// SendMessage sends a message and returns the response, handling streaming.
func (c *Client) SendMessage(ctx context.Context, messages []anthropic.MessageParam, toolDefs []anthropic.ToolUnionParam, systemPrompt string, onText func(string)) (*StreamResponse, error) {
	params := anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 16384,
		Messages:  messages,
	}

	if len(toolDefs) > 0 {
		params.Tools = toolDefs
	}

	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemPrompt},
		}
	}

	stream := c.client.Messages.NewStreaming(ctx, params)
	defer stream.Close()

	resp := &StreamResponse{}

	// Accumulate tool call JSON input incrementally.
	type partialToolCall struct {
		ID          string
		Name        string
		PartialJSON string
	}
	var activeToolCalls []partialToolCall

	for stream.Next() {
		evt := stream.Current()
		switch evt.Type {
		case "content_block_start":
			cb := evt.ContentBlock
			if cb.Type == "tool_use" {
				activeToolCalls = append(activeToolCalls, partialToolCall{
					ID:   cb.ID,
					Name: cb.Name,
				})
			}
		case "content_block_delta":
			delta := evt.Delta
			if delta.Type == "text_delta" && onText != nil {
				onText(delta.Text)
			}
			if delta.Type == "input_json_delta" && len(activeToolCalls) > 0 {
				activeToolCalls[len(activeToolCalls)-1].PartialJSON += delta.PartialJSON
			}
		case "message_delta":
			resp.StopReason = string(evt.Delta.StopReason)
		case "message_start":
			if evt.Message.Usage.InputTokens > 0 {
				resp.Usage.InputTokens = evt.Message.Usage.InputTokens
			}
		}
	}

	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("stream error: %w", err)
	}

	// Finalize tool calls.
	for _, tc := range activeToolCalls {
		input := json.RawMessage(tc.PartialJSON)
		if len(input) == 0 {
			input = json.RawMessage("{}")
		}
		resp.ToolCalls = append(resp.ToolCalls, ToolCall{
			ID:    tc.ID,
			Name:  tc.Name,
			Input: input,
		})
	}

	return resp, nil
}

// BuildUserMessage creates a user message param.
func BuildUserMessage(content string) anthropic.MessageParam {
	return anthropic.NewUserMessage(anthropic.NewTextBlock(content))
}

// BuildAssistantMessage creates an assistant message param with text content.
func BuildAssistantMessage(content string) anthropic.MessageParam {
	return anthropic.NewAssistantMessage(anthropic.NewTextBlock(content))
}

// BuildAssistantToolUseMessage creates an assistant message param with tool use.
func BuildAssistantToolUseMessage(toolCalls []ToolCall) anthropic.MessageParam {
	blocks := make([]anthropic.ContentBlockParamUnion, 0, len(toolCalls))
	for _, tc := range toolCalls {
		var input interface{}
		_ = json.Unmarshal(tc.Input, &input)
		blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, input, tc.Name))
	}
	return anthropic.NewAssistantMessage(blocks...)
}

// BuildToolResultMessage creates a tool result message.
func BuildToolResultMessage(toolCallID, content string, isError bool) anthropic.MessageParam {
	return anthropic.NewUserMessage(
		anthropic.NewToolResultBlock(toolCallID, content, isError),
	)
}
