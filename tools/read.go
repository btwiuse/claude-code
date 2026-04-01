package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// FileReadTool reads file contents.
type FileReadTool struct{}

type fileReadInput struct {
	FilePath string `json:"file_path"`
	Range    string `json:"range,omitempty"`
}

func (t *FileReadTool) Name() string { return "Read" }

func (t *FileReadTool) Description() string {
	return "Read the contents of a file from the local filesystem. Returns the file content with line numbers."
}

func (t *FileReadTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {
				"type": "string",
				"description": "Absolute path to the file to read"
			},
			"range": {
				"type": "string",
				"description": "Optional line range, e.g. '1-10', '5-', '-20'"
			}
		},
		"required": ["file_path"]
	}`)
}

func (t *FileReadTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in fileReadInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	if in.FilePath == "" {
		return Result{Content: "file_path is required", IsError: true}, nil
	}

	data, err := os.ReadFile(in.FilePath)
	if err != nil {
		return Result{Content: fmt.Sprintf("error reading file: %v", err), IsError: true}, nil
	}

	lines := strings.Split(string(data), "\n")

	// Apply line range filter if specified.
	if in.Range != "" {
		start, end, err := parseRange(in.Range, len(lines))
		if err != nil {
			return Result{Content: fmt.Sprintf("invalid range: %v", err), IsError: true}, nil
		}
		lines = lines[start:end]

		var sb strings.Builder
		for i, line := range lines {
			fmt.Fprintf(&sb, "%d. %s\n", start+i+1, line)
		}
		return Result{Content: sb.String()}, nil
	}

	var sb strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, line)
	}
	return Result{Content: sb.String()}, nil
}

func parseRange(r string, total int) (int, int, error) {
	parts := strings.SplitN(r, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("range must be in format 'start-end'")
	}

	start := 0
	end := total

	if parts[0] != "" {
		n, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, err
		}
		start = n - 1
		if start < 0 {
			start = 0
		}
	}

	if parts[1] != "" {
		n, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, err
		}
		end = n
		if end > total {
			end = total
		}
	}

	if start >= end {
		return 0, 0, fmt.Errorf("start must be less than end")
	}

	return start, end, nil
}
