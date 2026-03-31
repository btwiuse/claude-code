package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/claude-code-go/internal/types"
)

// AgentTool creates a sub-agent for complex tasks.
type AgentTool struct{}

// AgentInput is the input schema for the Agent tool.
type AgentInput struct {
	Prompt string `json:"prompt"`
}

// NewAgentTool creates a new AgentTool.
func NewAgentTool() *AgentTool {
	return &AgentTool{}
}

func (t *AgentTool) Name() string     { return "Agent" }
func (t *AgentTool) IsReadOnly() bool { return false }
func (t *AgentTool) IsEnabled() bool  { return true }

func (t *AgentTool) Description() string {
	return `Launch a sub-agent to handle a complex task independently.

The agent has access to all the same tools (Bash, Read, Write, Edit, Glob, Grep, etc.) and will work autonomously to complete the given task. Use this for tasks that require multiple steps or when you want to delegate work.

The agent runs in the same working directory and has the same permissions as the parent session.`
}

func (t *AgentTool) InputSchema() types.ToolInputSchema {
	return types.ToolInputSchema{
		Type: "object",
		Properties: map[string]types.ToolPropertySchema{
			"prompt": {
				Type:        "string",
				Description: "The task description for the sub-agent to complete.",
			},
		},
		Required: []string{"prompt"},
	}
}

func (t *AgentTool) UserFacingName(input json.RawMessage) string {
	var in AgentInput
	if err := json.Unmarshal(input, &in); err == nil && in.Prompt != "" {
		prompt := in.Prompt
		if len(prompt) > 50 {
			prompt = prompt[:47] + "..."
		}
		return fmt.Sprintf("Agent: %s", prompt)
	}
	return "Agent"
}

func (t *AgentTool) Execute(ctx context.Context, input json.RawMessage, toolCtx *ToolContext) (*ToolResult, error) {
	var in AgentInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &ToolResult{Content: fmt.Sprintf("Invalid input: %v", err), IsError: true}, nil
	}

	if strings.TrimSpace(in.Prompt) == "" {
		return &ToolResult{Content: "prompt is required", IsError: true}, nil
	}

	// In the Go port, the agent tool acts as a marker/placeholder.
	// A full implementation would create a new query engine instance.
	return &ToolResult{
		Content: fmt.Sprintf("Sub-agent task acknowledged: %s\n\n(Note: Full sub-agent execution requires a query engine instance. In this Go port, the agent tool is a placeholder for the agentic loop.)", in.Prompt),
	}, nil
}
