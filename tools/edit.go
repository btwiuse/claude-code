package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// FileEditTool performs string-replacement edits on files.
type FileEditTool struct{}

type fileEditInput struct {
	FilePath  string `json:"file_path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

func (t *FileEditTool) Name() string { return "Edit" }

func (t *FileEditTool) Description() string {
	return "Edit a file by replacing an exact string match. The old_string must match exactly one occurrence in the file."
}

func (t *FileEditTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {
				"type": "string",
				"description": "Absolute path to the file to edit"
			},
			"old_string": {
				"type": "string",
				"description": "The exact string to find and replace (must match exactly one occurrence)"
			},
			"new_string": {
				"type": "string",
				"description": "The replacement string"
			}
		},
		"required": ["file_path", "old_string", "new_string"]
	}`)
}

func (t *FileEditTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in fileEditInput
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

	content := string(data)
	count := strings.Count(content, in.OldString)

	if count == 0 {
		return Result{
			Content: "old_string not found in file",
			IsError: true,
		}, nil
	}

	if count > 1 {
		return Result{
			Content: fmt.Sprintf("old_string found %d times; must match exactly once. Add more context to make the match unique.", count),
			IsError: true,
		}, nil
	}

	newContent := strings.Replace(content, in.OldString, in.NewString, 1)

	info, err := os.Stat(in.FilePath)
	if err != nil {
		return Result{Content: fmt.Sprintf("error stating file: %v", err), IsError: true}, nil
	}

	if err := os.WriteFile(in.FilePath, []byte(newContent), info.Mode()); err != nil {
		return Result{Content: fmt.Sprintf("error writing file: %v", err), IsError: true}, nil
	}

	return Result{Content: fmt.Sprintf("Successfully edited %s", in.FilePath)}, nil
}
