package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GrepTool searches file contents using ripgrep or grep.
type GrepTool struct{}

type grepInput struct {
	Pattern    string `json:"pattern"`
	Path       string `json:"path,omitempty"`
	Include    string `json:"include,omitempty"`
	IgnoreCase bool   `json:"ignore_case,omitempty"`
}

func (t *GrepTool) Name() string { return "grep" }

func (t *GrepTool) Description() string {
	return `Search file contents using regular expressions. Searches recursively through directories. Uses ripgrep (rg) when available, falling back to grep. Results include file paths and matching lines.`
}

func (t *GrepTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "Regular expression pattern to search for"
			},
			"path": {
				"type": "string",
				"description": "Directory or file to search in (defaults to current directory)"
			},
			"include": {
				"type": "string",
				"description": "File glob pattern to filter (e.g., '*.go', '*.{js,ts}')"
			},
			"ignore_case": {
				"type": "boolean",
				"description": "Whether to perform case-insensitive matching"
			}
		},
		"required": ["pattern"]
	}`)
}

func (t *GrepTool) IsReadOnly() bool { return true }

func (t *GrepTool) Run(ctx context.Context, input json.RawMessage) (*Result, error) {
	var in grepInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &Result{Error: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	searchPath := in.Path
	if searchPath == "" {
		var err error
		searchPath, err = os.Getwd()
		if err != nil {
			return &Result{Error: fmt.Sprintf("failed to get working directory: %v", err), IsError: true}, nil
		}
	}
	searchPath = expandPath(searchPath)

	// Try ripgrep first, fall back to grep
	var args []string

	if _, err := exec.LookPath("rg"); err == nil {
		args = []string{"rg", "--no-heading", "--line-number", "--color=never", "--max-count=100"}
		if in.IgnoreCase {
			args = append(args, "-i")
		}
		if in.Include != "" {
			args = append(args, "--glob", in.Include)
		}
		args = append(args, in.Pattern, searchPath)
	} else {
		args = []string{"grep", "-rn", "--color=never"}
		if in.IgnoreCase {
			args = append(args, "-i")
		}
		if in.Include != "" {
			args = append(args, "--include", in.Include)
		}
		args = append(args, in.Pattern, searchPath)
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := stdout.String()

	// grep/rg return exit code 1 for no matches
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return &Result{Output: "No matches found"}, nil
		}
		if stderr.Len() > 0 {
			return &Result{Error: stderr.String(), IsError: true}, nil
		}
	}

	// Truncate large output
	const maxOutput = 100000
	if len(output) > maxOutput {
		output = output[:maxOutput] + "\n... (output truncated)"
	}

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	return &Result{Output: fmt.Sprintf("Found %d matches:\n%s", len(lines), strings.TrimRight(output, "\n"))}, nil
}
