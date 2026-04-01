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

// GrepTool searches file contents using ripgrep or fallback.
type GrepTool struct{}

type grepInput struct {
	Pattern    string `json:"pattern"`
	Path       string `json:"path,omitempty"`
	Glob       string `json:"glob,omitempty"`
	IgnoreCase bool   `json:"ignore_case,omitempty"`
}

func (t *GrepTool) Name() string { return "Grep" }

func (t *GrepTool) Description() string {
	return "Search for patterns in file contents. Uses ripgrep if available, otherwise falls back to grep."
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
				"description": "File or directory to search in (defaults to current directory)"
			},
			"glob": {
				"type": "string",
				"description": "Glob pattern to filter files (e.g. '*.go', '*.{ts,tsx}')"
			},
			"ignore_case": {
				"type": "boolean",
				"description": "Case-insensitive search"
			}
		},
		"required": ["pattern"]
	}`)
}

func (t *GrepTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in grepInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	if in.Pattern == "" {
		return Result{Content: "pattern is required", IsError: true}, nil
	}

	searchPath := in.Path
	if searchPath == "" {
		var err error
		searchPath, err = os.Getwd()
		if err != nil {
			return Result{Content: fmt.Sprintf("error getting cwd: %v", err), IsError: true}, nil
		}
	}

	// Try ripgrep first, fall back to grep.
	output, err := runRipgrep(ctx, in.Pattern, searchPath, in.Glob, in.IgnoreCase)
	if err != nil {
		output, err = runGrep(ctx, in.Pattern, searchPath, in.IgnoreCase)
		if err != nil {
			return Result{Content: fmt.Sprintf("search error: %v", err), IsError: true}, nil
		}
	}

	if output == "" {
		return Result{Content: "No matches found."}, nil
	}

	// Limit output size.
	lines := strings.Split(output, "\n")
	if len(lines) > 500 {
		output = strings.Join(lines[:500], "\n")
		output += fmt.Sprintf("\n... (truncated, showing first 500 of %d lines)", len(lines))
	}

	return Result{Content: output}, nil
}

func runRipgrep(ctx context.Context, pattern, path, glob string, ignoreCase bool) (string, error) {
	rgPath, err := exec.LookPath("rg")
	if err != nil {
		return "", err
	}

	args := []string{"-n", "--no-heading", "--color", "never"}
	if ignoreCase {
		args = append(args, "-i")
	}
	if glob != "" {
		args = append(args, "--glob", glob)
	}
	args = append(args, pattern, path)

	cmd := exec.CommandContext(ctx, rgPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil // No matches
		}
		return "", fmt.Errorf("%v: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func runGrep(ctx context.Context, pattern, path string, ignoreCase bool) (string, error) {
	args := []string{"-rn", "--color=never"}
	if ignoreCase {
		args = append(args, "-i")
	}
	args = append(args, pattern, path)

	cmd := exec.CommandContext(ctx, "grep", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil // No matches
		}
		return "", fmt.Errorf("%v: %s", err, stderr.String())
	}
	return stdout.String(), nil
}
