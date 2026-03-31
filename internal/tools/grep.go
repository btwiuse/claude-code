package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/anthropics/claude-code-go/internal/types"
)

// GrepTool searches file contents using ripgrep or grep.
type GrepTool struct{}

// GrepInput is the input schema for the Grep tool.
type GrepInput struct {
	Pattern   string `json:"pattern"`
	Path      string `json:"path,omitempty"`
	Include   string `json:"include,omitempty"`
	MaxResults int   `json:"max_results,omitempty"`
}

// NewGrepTool creates a new GrepTool.
func NewGrepTool() *GrepTool {
	return &GrepTool{}
}

func (t *GrepTool) Name() string        { return "Grep" }
func (t *GrepTool) IsReadOnly() bool     { return true }
func (t *GrepTool) IsEnabled() bool      { return true }

func (t *GrepTool) Description() string {
	return `Search for patterns in file contents using regular expressions.

Uses ripgrep (rg) if available, falling back to grep. Returns matching lines with file paths and line numbers.

Useful for finding function definitions, imports, configuration values, error messages, and any text patterns across a codebase.

Results are limited to 250 lines by default to prevent context overflow.`
}

func (t *GrepTool) InputSchema() types.ToolInputSchema {
	return types.ToolInputSchema{
		Type: "object",
		Properties: map[string]types.ToolPropertySchema{
			"pattern": {
				Type:        "string",
				Description: "Regular expression pattern to search for.",
			},
			"path": {
				Type:        "string",
				Description: "Directory or file to search in. Defaults to the current working directory.",
			},
			"include": {
				Type:        "string",
				Description: "Glob pattern to filter files (e.g., '*.go', '*.{ts,tsx}').",
			},
			"max_results": {
				Type:        "integer",
				Description: "Maximum number of result lines. Defaults to 250.",
			},
		},
		Required: []string{"pattern"},
	}
}

func (t *GrepTool) UserFacingName(input json.RawMessage) string {
	var in GrepInput
	if err := json.Unmarshal(input, &in); err == nil && in.Pattern != "" {
		pattern := in.Pattern
		if len(pattern) > 40 {
			pattern = pattern[:37] + "..."
		}
		return fmt.Sprintf("Grep: %s", pattern)
	}
	return "Grep"
}

func (t *GrepTool) Execute(ctx context.Context, input json.RawMessage, toolCtx *ToolContext) (*ToolResult, error) {
	var in GrepInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &ToolResult{Content: fmt.Sprintf("Invalid input: %v", err), IsError: true}, nil
	}

	if in.Pattern == "" {
		return &ToolResult{Content: "pattern is required", IsError: true}, nil
	}

	maxResults := 250
	if in.MaxResults > 0 {
		maxResults = in.MaxResults
	}

	searchPath := toolCtx.CWD
	if in.Path != "" {
		if filepath.IsAbs(in.Path) {
			searchPath = in.Path
		} else {
			searchPath = filepath.Join(toolCtx.CWD, in.Path)
		}
	}

	// Try ripgrep first, fall back to grep
	output, err := t.runRipgrep(ctx, in.Pattern, searchPath, in.Include, maxResults)
	if err != nil {
		// Fallback to grep
		output, err = t.runGrep(ctx, in.Pattern, searchPath, in.Include, maxResults)
		if err != nil {
			return &ToolResult{Content: fmt.Sprintf("Search failed: %v", err), IsError: true}, nil
		}
	}

	if output == "" {
		return &ToolResult{Content: fmt.Sprintf("No matches found for pattern: %s", in.Pattern)}, nil
	}

	// Check if results were truncated
	lines := strings.Count(output, "\n")
	if lines >= maxResults {
		output += fmt.Sprintf("\n(results truncated at %d lines)", maxResults)
	}

	return &ToolResult{Content: output}, nil
}

func (t *GrepTool) runRipgrep(ctx context.Context, pattern, path, include string, maxResults int) (string, error) {
	args := []string{
		"--no-heading",
		"--line-number",
		"--color=never",
		"--max-count", fmt.Sprintf("%d", maxResults),
		"--smart-case",
	}

	if include != "" {
		args = append(args, "--glob", include)
	}

	// Skip hidden directories and common noise
	args = append(args, "--hidden", "--glob", "!.git/")
	args = append(args, pattern, path)

	cmd := exec.CommandContext(ctx, "rg", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// ripgrep returns 1 for no matches
			if exitErr.ExitCode() == 1 {
				return "", nil
			}
		}
		return "", fmt.Errorf("ripgrep: %w: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

func (t *GrepTool) runGrep(ctx context.Context, pattern, path, include string, maxResults int) (string, error) {
	args := []string{
		"-rn",
		"--color=never",
		fmt.Sprintf("-m%d", maxResults),
	}

	if include != "" {
		args = append(args, "--include="+include)
	}

	args = append(args, pattern, path)

	cmd := exec.CommandContext(ctx, "grep", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return "", nil
			}
		}
		return "", fmt.Errorf("grep: %w: %s", err, stderr.String())
	}

	return stdout.String(), nil
}
