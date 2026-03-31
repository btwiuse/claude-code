package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/anthropics/claude-code-go/internal/types"
)

// BashTool executes shell commands.
type BashTool struct{}

// BashInput is the input schema for the Bash tool.
type BashInput struct {
	Command    string `json:"command"`
	Timeout    int    `json:"timeout,omitempty"`
	WorkingDir string `json:"working_dir,omitempty"`
}

// NewBashTool creates a new BashTool.
func NewBashTool() *BashTool {
	return &BashTool{}
}

func (t *BashTool) Name() string        { return "Bash" }
func (t *BashTool) IsReadOnly() bool     { return false }
func (t *BashTool) IsEnabled() bool      { return true }

func (t *BashTool) Description() string {
	return `Execute a bash command on the system. Use this for running shell commands, installing packages, compiling code, running tests, searching with find/grep, and other system operations.

Commands are executed in a bash shell with the user's environment. Long-running commands should be avoided. The command timeout defaults to 120 seconds.

Important guidelines:
- Always use absolute paths or paths relative to the working directory
- For multi-line scripts, use heredoc syntax or semicolons
- Prefer non-interactive commands (use -y flags for apt, etc.)
- Check exit codes for command success/failure`
}

func (t *BashTool) InputSchema() types.ToolInputSchema {
	return types.ToolInputSchema{
		Type: "object",
		Properties: map[string]types.ToolPropertySchema{
			"command": {
				Type:        "string",
				Description: "The bash command to execute. Can be a single command or a multi-line script.",
			},
			"timeout": {
				Type:        "integer",
				Description: "Timeout in seconds for the command. Defaults to 120.",
				Default:     120,
			},
			"working_dir": {
				Type:        "string",
				Description: "Working directory for command execution. Defaults to the session's CWD.",
			},
		},
		Required: []string{"command"},
	}
}

func (t *BashTool) UserFacingName(input json.RawMessage) string {
	var in BashInput
	if err := json.Unmarshal(input, &in); err == nil && in.Command != "" {
		cmd := in.Command
		if len(cmd) > 60 {
			cmd = cmd[:57] + "..."
		}
		return fmt.Sprintf("Bash: %s", cmd)
	}
	return "Bash"
}

func (t *BashTool) Execute(ctx context.Context, input json.RawMessage, toolCtx *ToolContext) (*ToolResult, error) {
	var in BashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &ToolResult{Content: fmt.Sprintf("Invalid input: %v", err), IsError: true}, nil
	}

	if in.Command == "" {
		return &ToolResult{Content: "command is required", IsError: true}, nil
	}

	timeout := 120
	if in.Timeout > 0 {
		timeout = in.Timeout
	}

	cwd := toolCtx.CWD
	if in.WorkingDir != "" {
		cwd = in.WorkingDir
	}

	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "bash", "-c", in.Command)
	cmd.Dir = cwd
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	var result strings.Builder
	if stdout.Len() > 0 {
		result.WriteString(stdout.String())
	}
	if stderr.Len() > 0 {
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		result.WriteString("STDERR:\n")
		result.WriteString(stderr.String())
	}

	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return &ToolResult{
				Content: fmt.Sprintf("Command timed out after %d seconds.\n%s", timeout, result.String()),
				IsError: true,
			}, nil
		}
		exitCode := -1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		return &ToolResult{
			Content: fmt.Sprintf("Exit code: %d\n%s", exitCode, result.String()),
			IsError: exitCode != 0,
		}, nil
	}

	output := result.String()
	if output == "" {
		output = "(no output)"
	}

	// Truncate very large outputs
	const maxOutput = 100_000
	if len(output) > maxOutput {
		output = output[:maxOutput] + "\n... (output truncated)"
	}

	return &ToolResult{Content: output}, nil
}
