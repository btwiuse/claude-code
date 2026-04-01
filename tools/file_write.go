package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FileWriteTool writes content to a file, creating it if necessary.
type FileWriteTool struct{}

type fileWriteInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (t *FileWriteTool) Name() string { return "file_write" }

func (t *FileWriteTool) Description() string {
	return `Write content to a file. Creates the file and any parent directories if they don't exist. Overwrites existing content. Use file_edit for making targeted changes to existing files.`
}

func (t *FileWriteTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Absolute path to the file to write"
			},
			"content": {
				"type": "string",
				"description": "Content to write to the file"
			}
		},
		"required": ["path", "content"]
	}`)
}

func (t *FileWriteTool) IsReadOnly() bool { return false }

func (t *FileWriteTool) Run(_ context.Context, input json.RawMessage) (*Result, error) {
	var in fileWriteInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &Result{Error: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	path := expandPath(in.Path)

	// Create parent directories
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return &Result{Error: fmt.Sprintf("failed to create directories: %v", err), IsError: true}, nil
	}

	if err := os.WriteFile(path, []byte(in.Content), 0o644); err != nil {
		return &Result{Error: fmt.Sprintf("failed to write file: %v", err), IsError: true}, nil
	}

	return &Result{Output: fmt.Sprintf("Successfully wrote %d bytes to %s", len(in.Content), path)}, nil
}
