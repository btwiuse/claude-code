package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// AgentTool spawns sub-agent instances to handle complex tasks.
type AgentTool struct {
	// RunAgent is set externally to provide the agent execution logic.
	// This avoids circular dependencies between tools and the main loop.
	RunAgent func(ctx context.Context, prompt string, model string) (string, error)
}

type agentInput struct {
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	Model       string `json:"model,omitempty"`
}

func (t *AgentTool) Name() string { return "agent" }

func (t *AgentTool) Description() string {
	return `Launch a sub-agent to handle a specific task. The sub-agent runs independently with its own context and can use all available tools. Use this for complex multi-step tasks that benefit from focused attention. Provide a clear, specific prompt describing the task.`
}

func (t *AgentTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"description": {
				"type": "string",
				"description": "A short (3-5 word) description of the task"
			},
			"prompt": {
				"type": "string",
				"description": "Detailed task description for the sub-agent"
			},
			"model": {
				"type": "string",
				"description": "Model to use for the sub-agent (e.g., 'sonnet', 'opus', 'haiku')"
			}
		},
		"required": ["description", "prompt"]
	}`)
}

func (t *AgentTool) IsReadOnly() bool { return false }

func (t *AgentTool) Run(ctx context.Context, input json.RawMessage) (*Result, error) {
	var in agentInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &Result{Error: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	if in.Prompt == "" {
		return &Result{Error: "prompt is required", IsError: true}, nil
	}

	if t.RunAgent == nil {
		return &Result{Error: "agent execution not configured", IsError: true}, nil
	}

	model := in.Model
	if model == "" {
		model = "sonnet"
	}

	result, err := t.RunAgent(ctx, in.Prompt, model)
	if err != nil {
		return &Result{
			Error:   fmt.Sprintf("sub-agent failed: %v", err),
			IsError: true,
		}, nil
	}

	// Truncate very long results
	const maxResult = 100000
	if len(result) > maxResult {
		result = result[:maxResult] + "\n... (output truncated)"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Sub-agent (%s) completed task: %s\n\n", model, in.Description))
	sb.WriteString(result)

	return &Result{Output: sb.String()}, nil
}
