package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// FileReadTool reads file contents.
type FileReadTool struct{}

type fileReadInput struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}

func (t *FileReadTool) Name() string { return "file_read" }

func (t *FileReadTool) Description() string {
	return `Read the contents of a file. Reads the full file or a specific line range. Line numbers are 1-indexed. Use start_line and end_line to read a specific range.`
}

func (t *FileReadTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Absolute path to the file to read"
			},
			"start_line": {
				"type": "integer",
				"description": "Starting line number (1-indexed, inclusive)"
			},
			"end_line": {
				"type": "integer",
				"description": "Ending line number (1-indexed, inclusive)"
			}
		},
		"required": ["path"]
	}`)
}

func (t *FileReadTool) IsReadOnly() bool { return true }

func (t *FileReadTool) Run(_ context.Context, input json.RawMessage) (*Result, error) {
	var in fileReadInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &Result{Error: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	path := expandPath(in.Path)

	data, err := os.ReadFile(path)
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to read file: %v", err), IsError: true}, nil
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Apply line range filtering
	if in.StartLine > 0 || in.EndLine > 0 {
		start := in.StartLine
		end := in.EndLine

		if start < 1 {
			start = 1
		}
		if end < 1 || end > len(lines) {
			end = len(lines)
		}
		if start > len(lines) {
			return &Result{
				Error:   fmt.Sprintf("start_line %d exceeds file length of %d lines", start, len(lines)),
				IsError: true,
			}, nil
		}

		// Add line numbers
		var sb strings.Builder
		for i := start - 1; i < end; i++ {
			sb.WriteString(strconv.Itoa(i + 1))
			sb.WriteString(". ")
			sb.WriteString(lines[i])
			sb.WriteString("\n")
		}
		return &Result{Output: sb.String()}, nil
	}

	// Full file with line numbers
	var sb strings.Builder
	for i, line := range lines {
		sb.WriteString(strconv.Itoa(i + 1))
		sb.WriteString(". ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return &Result{Output: sb.String()}, nil
}

// expandPath resolves ~ to home directory and cleans the path.
func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			p = filepath.Join(home, p[2:])
		}
	}
	return filepath.Clean(p)
}
