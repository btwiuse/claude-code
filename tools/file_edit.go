package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// FileEditTool performs search-and-replace edits on files.
type FileEditTool struct{}

type fileEditInput struct {
	Path   string `json:"path"`
	OldStr string `json:"old_str"`
	NewStr string `json:"new_str"`
}

func (t *FileEditTool) Name() string { return "file_edit" }

func (t *FileEditTool) Description() string {
	return `Make targeted edits to a file by replacing exact string matches. The old_str must match exactly one occurrence in the file. To create a new file, use an empty old_str. To delete text, use an empty new_str.`
}

func (t *FileEditTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "Absolute path to the file to edit"
			},
			"old_str": {
				"type": "string",
				"description": "The exact string to search for and replace. Must match exactly one occurrence."
			},
			"new_str": {
				"type": "string",
				"description": "The replacement string"
			}
		},
		"required": ["path", "old_str", "new_str"]
	}`)
}

func (t *FileEditTool) IsReadOnly() bool { return false }

func (t *FileEditTool) Run(_ context.Context, input json.RawMessage) (*Result, error) {
	var in fileEditInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &Result{Error: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	path := expandPath(in.Path)

	// If old_str is empty, this is a file creation
	if in.OldStr == "" {
		dir := strings.TrimSuffix(path, "/"+path[strings.LastIndex(path, "/")+1:])
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return &Result{Error: fmt.Sprintf("failed to create directories: %v", err), IsError: true}, nil
		}
		if err := os.WriteFile(path, []byte(in.NewStr), 0o644); err != nil {
			return &Result{Error: fmt.Sprintf("failed to create file: %v", err), IsError: true}, nil
		}
		return &Result{Output: fmt.Sprintf("Created file %s", path)}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &Result{Error: fmt.Sprintf("failed to read file: %v", err), IsError: true}, nil
	}

	content := string(data)

	// Count occurrences
	count := strings.Count(content, in.OldStr)
	if count == 0 {
		return &Result{
			Error:   fmt.Sprintf("old_str not found in %s. Make sure the string matches exactly, including whitespace and indentation.", path),
			IsError: true,
		}, nil
	}
	if count > 1 {
		return &Result{
			Error:   fmt.Sprintf("old_str found %d times in %s. It must match exactly once. Add more context to make the match unique.", count, path),
			IsError: true,
		}, nil
	}

	// Perform replacement
	newContent := strings.Replace(content, in.OldStr, in.NewStr, 1)

	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return &Result{Error: fmt.Sprintf("failed to write file: %v", err), IsError: true}, nil
	}

	// Report what changed
	oldLines := len(strings.Split(in.OldStr, "\n"))
	newLines := len(strings.Split(in.NewStr, "\n"))
	return &Result{
		Output: fmt.Sprintf("Edited %s: replaced %d line(s) with %d line(s)", path, oldLines, newLines),
	}, nil
}
