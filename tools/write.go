package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FileWriteTool writes content to a file.
type FileWriteTool struct{}

type fileWriteInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

func (t *FileWriteTool) Name() string { return "Write" }

func (t *FileWriteTool) Description() string {
	return "Write content to a file on the local filesystem. Creates parent directories if needed."
}

func (t *FileWriteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {
				"type": "string",
				"description": "Absolute path to the file to write"
			},
			"content": {
				"type": "string",
				"description": "Content to write to the file"
			}
		},
		"required": ["file_path", "content"]
	}`)
}

func (t *FileWriteTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in fileWriteInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	if in.FilePath == "" {
		return Result{Content: "file_path is required", IsError: true}, nil
	}

	dir := filepath.Dir(in.FilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Result{Content: fmt.Sprintf("error creating directories: %v", err), IsError: true}, nil
	}

	if err := os.WriteFile(in.FilePath, []byte(in.Content), 0o644); err != nil {
		return Result{Content: fmt.Sprintf("error writing file: %v", err), IsError: true}, nil
	}

	return Result{Content: fmt.Sprintf("Successfully wrote %d bytes to %s", len(in.Content), in.FilePath)}, nil
}
