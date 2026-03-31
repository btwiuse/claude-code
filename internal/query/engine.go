// Package query implements the conversation loop (query engine) for Claude Code.
package query

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropics/claude-code-go/internal/api"
	"github.com/anthropics/claude-code-go/internal/cost"
	"github.com/anthropics/claude-code-go/internal/tools"
	"github.com/anthropics/claude-code-go/internal/types"
)

// Engine manages the conversation loop between the user, Claude, and tools.
type Engine struct {
	client       *api.Client
	registry     *tools.Registry
	costTracker  *cost.Tracker
	toolCtx      *tools.ToolContext
	messages     []types.Message
	systemPrompt string
	maxTurns     int
	onText       func(text string)
	onToolUse    func(name string, input json.RawMessage)
	onToolResult func(name string, result *tools.ToolResult)
	onThinking   func(text string)
	onError      func(err error)
}

// EngineConfig configures the query engine.
type EngineConfig struct {
	Client       *api.Client
	Registry     *tools.Registry
	CostTracker  *cost.Tracker
	ToolCtx      *tools.ToolContext
	SystemPrompt string
	MaxTurns     int

	// Callbacks for streaming output
	OnText       func(text string)
	OnToolUse    func(name string, input json.RawMessage)
	OnToolResult func(name string, result *tools.ToolResult)
	OnThinking   func(text string)
	OnError      func(err error)
}

// NewEngine creates a new query engine.
func NewEngine(cfg EngineConfig) *Engine {
	if cfg.MaxTurns == 0 {
		cfg.MaxTurns = 25 // Default max turns per query
	}

	return &Engine{
		client:       cfg.Client,
		registry:     cfg.Registry,
		costTracker:  cfg.CostTracker,
		toolCtx:      cfg.ToolCtx,
		systemPrompt: cfg.SystemPrompt,
		maxTurns:     cfg.MaxTurns,
		onText:       cfg.OnText,
		onToolUse:    cfg.OnToolUse,
		onToolResult: cfg.OnToolResult,
		onThinking:   cfg.OnThinking,
		onError:      cfg.OnError,
	}
}

// GetMessages returns the current conversation messages.
func (e *Engine) GetMessages() []types.Message {
	return e.messages
}

// SetMessages replaces the conversation messages.
func (e *Engine) SetMessages(msgs []types.Message) {
	e.messages = msgs
}

// Submit sends a user message and processes the response, including tool calls.
func (e *Engine) Submit(ctx context.Context, userInput string) error {
	// Add user message
	userMsg := types.Message{
		Role: types.RoleUser,
		Content: []types.ContentBlock{
			{Type: types.ContentTypeText, Text: userInput},
		},
		Timestamp: time.Now(),
	}
	e.messages = append(e.messages, userMsg)

	// Run the conversation loop
	for turn := 0; turn < e.maxTurns; turn++ {
		shouldContinue, err := e.runTurn(ctx)
		if err != nil {
			return err
		}
		if !shouldContinue {
			break
		}
	}

	return nil
}

// runTurn executes a single turn of the conversation loop.
// Returns true if another turn should be executed (tool calls were made).
func (e *Engine) runTurn(ctx context.Context) (bool, error) {
	// Build API request
	req := &api.CreateMessageRequest{
		Messages: e.messages,
		System: []types.SystemBlock{
			{Type: "text", Text: e.systemPrompt},
		},
		Tools: e.registry.ToDefinitions(),
	}

	// Stream the response
	startTime := time.Now()
	stream, err := e.client.StreamMessage(ctx, req)
	if err != nil {
		if e.onError != nil {
			e.onError(err)
		}
		return false, err
	}

	// Process stream events
	assistantMsg, err := e.processStream(ctx, stream)
	if err != nil {
		return false, err
	}

	// Record API call cost
	duration := time.Since(startTime)
	if assistantMsg.Usage != nil {
		e.costTracker.AddAPICall(e.client.GetModel(), cost.Usage{
			InputTokens:              assistantMsg.Usage.InputTokens,
			OutputTokens:             assistantMsg.Usage.OutputTokens,
			CacheReadInputTokens:     assistantMsg.Usage.CacheReadInputTokens,
			CacheCreationInputTokens: assistantMsg.Usage.CacheCreationInputTokens,
		}, duration)
	}

	// Add assistant message to conversation
	e.messages = append(e.messages, *assistantMsg)

	// Check for tool use blocks and execute them
	var toolResults []types.ContentBlock
	hasToolUse := false

	for _, block := range assistantMsg.Content {
		if block.Type == types.ContentTypeToolUse {
			hasToolUse = true

			if e.onToolUse != nil {
				e.onToolUse(block.Name, block.Input)
			}

			// Execute the tool
			toolStart := time.Now()
			result, err := e.registry.Execute(ctx, block.Name, block.Input, e.toolCtx)
			toolDuration := time.Since(toolStart)
			e.costTracker.AddToolDuration(toolDuration)

			if err != nil {
				result = &tools.ToolResult{
					Content: fmt.Sprintf("Tool execution error: %v", err),
					IsError: true,
				}
			}

			if e.onToolResult != nil {
				e.onToolResult(block.Name, result)
			}

			toolResults = append(toolResults, types.ContentBlock{
				Type:      types.ContentTypeToolResult,
				ToolUseID: block.ID,
				Content: []types.ContentBlock{
					{Type: types.ContentTypeText, Text: result.Content},
				},
				IsError: result.IsError,
			})
		}
	}

	// If there were tool calls, add results and continue
	if hasToolUse && len(toolResults) > 0 {
		toolResultMsg := types.Message{
			Role:      types.RoleUser,
			Content:   toolResults,
			Timestamp: time.Now(),
		}
		e.messages = append(e.messages, toolResultMsg)
		return true, nil // Continue loop
	}

	return false, nil // No tool calls, conversation turn complete
}

// processStream reads stream events and assembles the assistant message.
func (e *Engine) processStream(ctx context.Context, stream <-chan api.StreamResult) (*types.Message, error) {
	msg := &types.Message{
		Role:      types.RoleAssistant,
		Content:   []types.ContentBlock{},
		Timestamp: time.Now(),
	}

	var currentBlock *types.ContentBlock
	var currentText string

	for result := range stream {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if result.Error != nil {
			return nil, result.Error
		}

		event := result.Event
		if event == nil {
			continue
		}

		switch event.Type {
		case "message_start":
			if event.Message != nil {
				msg.Model = event.Message.Model
				if event.Message.Usage != nil {
					msg.Usage = event.Message.Usage
				}
			}

		case "content_block_start":
			if event.ContentBlock != nil {
				block := *event.ContentBlock
				currentBlock = &block
				currentText = ""

				if block.Type == types.ContentTypeThinking && e.onThinking != nil {
					e.onThinking("")
				}
			}

		case "content_block_delta":
			if currentBlock != nil && event.Delta != nil {
				var delta struct {
					Type        string `json:"type"`
					Text        string `json:"text"`
					PartialJSON string `json:"partial_json"`
					Thinking    string `json:"thinking"`
				}
				if err := json.Unmarshal(event.Delta, &delta); err == nil {
					switch delta.Type {
					case "text_delta":
						currentText += delta.Text
						if e.onText != nil {
							e.onText(delta.Text)
						}
					case "input_json_delta":
						currentText += delta.PartialJSON
					case "thinking_delta":
						if e.onThinking != nil {
							e.onThinking(delta.Thinking)
						}
					}
				}
			}

		case "content_block_stop":
			if currentBlock != nil {
				switch currentBlock.Type {
				case types.ContentTypeText:
					currentBlock.Text = currentText
				case types.ContentTypeToolUse:
					if currentText != "" {
						currentBlock.Input = json.RawMessage(currentText)
					}
				case types.ContentTypeThinking:
					currentBlock.Thinking = currentText
				}
				msg.Content = append(msg.Content, *currentBlock)
				currentBlock = nil
				currentText = ""
			}

		case "message_delta":
			if event.Delta != nil {
				var delta struct {
					StopReason string `json:"stop_reason"`
				}
				if err := json.Unmarshal(event.Delta, &delta); err == nil {
					msg.StopReason = delta.StopReason
				}
			}
			if event.Usage != nil {
				if msg.Usage == nil {
					msg.Usage = &types.Usage{}
				}
				msg.Usage.OutputTokens = event.Usage.OutputTokens
			}
		}
	}

	return msg, nil
}
