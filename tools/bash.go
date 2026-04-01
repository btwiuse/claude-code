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
)

// BashTool executes shell commands.
type BashTool struct{}

type bashInput struct {
	Command string `json:"command"`
	CWD     string `json:"cwd,omitempty"`
	Timeout int    `json:"timeout_seconds,omitempty"`
}

func (t *BashTool) Name() string { return "bash" }

func (t *BashTool) Description() string {
	return `Execute a bash command on the system. Use this to run shell commands, scripts, and programs. Commands are executed in a bash shell with the current working directory. Long-running commands will be terminated after the timeout period.`
}

func (t *BashTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {
				"type": "string",
				"description": "The bash command to execute"
			},
			"cwd": {
				"type": "string",
				"description": "Working directory for the command (defaults to current directory)"
			},
			"timeout_seconds": {
				"type": "integer",
				"description": "Maximum execution time in seconds (default: 120)"
			}
		},
		"required": ["command"]
	}`)
}

func (t *BashTool) IsReadOnly() bool { return false }

func (t *BashTool) Run(ctx context.Context, input json.RawMessage) (*Result, error) {
	var in bashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &Result{Error: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	if in.Command == "" {
		return &Result{Error: "command is required", IsError: true}, nil
	}

	timeout := 120 * time.Second
	if in.Timeout > 0 {
		timeout = time.Duration(in.Timeout) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", in.Command)
	if in.CWD != "" {
		cmd.Dir = in.CWD
	}
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() != nil {
			return &Result{
				Error:    fmt.Sprintf("command timed out after %v", timeout),
				IsError:  true,
				ExitCode: -1,
			}, nil
		}
	}

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	// Truncate very large output
	const maxOutput = 100000
	if len(output) > maxOutput {
		output = output[:maxOutput] + "\n... (output truncated)"
	}

	result := &Result{
		Output:   strings.TrimRight(output, "\n"),
		ExitCode: exitCode,
		IsError:  exitCode != 0,
	}

	if exitCode != 0 && result.Output == "" {
		result.Error = fmt.Sprintf("command exited with code %d", exitCode)
	}

	return result, nil
}
