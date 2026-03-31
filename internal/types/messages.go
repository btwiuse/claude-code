// Package types defines core message and data types used throughout Claude Code.
package types

import (
	"encoding/json"
	"time"
)

// Role represents the role of a message participant.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// ContentType represents the type of a content block.
type ContentType string

const (
	ContentTypeText       ContentType = "text"
	ContentTypeImage      ContentType = "image"
	ContentTypeToolUse    ContentType = "tool_use"
	ContentTypeToolResult ContentType = "tool_result"
	ContentTypeThinking   ContentType = "thinking"
)

// ContentBlock represents a single block of content in a message.
type ContentBlock struct {
	Type    ContentType     `json:"type"`
	Text    string          `json:"text,omitempty"`
	ID      string          `json:"id,omitempty"`
	Name    string          `json:"name,omitempty"`
	Input   json.RawMessage `json:"input,omitempty"`
	Content []ContentBlock  `json:"content,omitempty"`

	// Tool result fields
	ToolUseID string `json:"tool_use_id,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`

	// Image fields
	Source *ImageSource `json:"source,omitempty"`

	// Thinking fields
	Thinking string `json:"thinking,omitempty"`
}

// ImageSource represents the source of an image content block.
type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// Message represents a conversation message.
type Message struct {
	ID         string         `json:"id"`
	Role       Role           `json:"role"`
	Content    []ContentBlock `json:"content"`
	Model      string         `json:"model,omitempty"`
	StopReason string         `json:"stop_reason,omitempty"`
	Usage      *Usage         `json:"usage,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
}

// Usage tracks token usage for an API call.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
}

// ModelUsage tracks cumulative usage for a specific model.
type ModelUsage struct {
	InputTokens              int     `json:"input_tokens"`
	OutputTokens             int     `json:"output_tokens"`
	CacheReadInputTokens     int     `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int     `json:"cache_creation_input_tokens"`
	CostUSD                  float64 `json:"cost_usd"`
}

// StreamEvent represents an event from the streaming API.
type StreamEvent struct {
	Type  string          `json:"type"`
	Index int             `json:"index,omitempty"`
	Delta json.RawMessage `json:"delta,omitempty"`

	// For content_block_start
	ContentBlock *ContentBlock `json:"content_block,omitempty"`

	// For message_start
	Message *MessageStartData `json:"message,omitempty"`

	// For message_delta
	Usage *Usage `json:"usage,omitempty"`
}

// MessageStartData contains the initial message data from a stream.
type MessageStartData struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Role  Role   `json:"role"`
	Model string `json:"model"`
	Usage *Usage `json:"usage,omitempty"`
}

// ToolInputSchema defines the JSON Schema for a tool's input.
type ToolInputSchema struct {
	Type       string                        `json:"type"`
	Properties map[string]ToolPropertySchema `json:"properties"`
	Required   []string                      `json:"required,omitempty"`
}

// ToolPropertySchema defines a single property in a tool's input schema.
type ToolPropertySchema struct {
	Type        string              `json:"type"`
	Description string              `json:"description,omitempty"`
	Enum        []string            `json:"enum,omitempty"`
	Default     any                 `json:"default,omitempty"`
	Items       *ToolPropertySchema `json:"items,omitempty"`
}

// ToolDefinition describes a tool for the API.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema ToolInputSchema `json:"input_schema"`
}

// APIRequest represents a request to the Claude Messages API.
type APIRequest struct {
	Model         string           `json:"model"`
	MaxTokens     int              `json:"max_tokens"`
	Messages      []Message        `json:"messages"`
	System        []SystemBlock    `json:"system,omitempty"`
	Tools         []ToolDefinition `json:"tools,omitempty"`
	Stream        bool             `json:"stream"`
	StopSequences []string         `json:"stop_sequences,omitempty"`
	Temperature   *float64         `json:"temperature,omitempty"`
	TopP          *float64         `json:"top_p,omitempty"`

	// Extended thinking
	Thinking *ThinkingConfig `json:"thinking,omitempty"`
}

// SystemBlock represents a system prompt block.
type SystemBlock struct {
	Type         string        `json:"type"`
	Text         string        `json:"text,omitempty"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// CacheControl specifies caching behavior for a block.
type CacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

// ThinkingConfig configures extended thinking behavior.
type ThinkingConfig struct {
	Type         string `json:"type"` // "enabled"
	BudgetTokens int    `json:"budget_tokens"`
}

// APIResponse represents a non-streaming response from the Claude Messages API.
type APIResponse struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Role       Role           `json:"role"`
	Content    []ContentBlock `json:"content"`
	Model      string         `json:"model"`
	StopReason string         `json:"stop_reason"`
	Usage      *Usage         `json:"usage"`
}

// PermissionMode represents the permission enforcement level.
type PermissionMode string

const (
	PermissionModeDefault PermissionMode = "default"
	PermissionModeAuto    PermissionMode = "auto"
	PermissionModeBypass  PermissionMode = "bypass"
)

// PermissionDecision represents the result of a permission check.
type PermissionDecision string

const (
	PermissionAllow PermissionDecision = "allow"
	PermissionDeny  PermissionDecision = "deny"
	PermissionAsk   PermissionDecision = "ask"
)

// ToolPermissionRule defines a single permission rule.
type ToolPermissionRule struct {
	ToolName string             `json:"tool_name"`
	Pattern  string             `json:"pattern,omitempty"`
	Decision PermissionDecision `json:"decision"`
	Source   string             `json:"source"`
}

// SessionInfo contains metadata about a session.
type SessionInfo struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CWD       string    `json:"cwd"`
	Model     string    `json:"model"`
}
