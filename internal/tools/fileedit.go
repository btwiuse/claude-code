package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/claude-code-go/internal/types"
)

// FileEditTool applies targeted edits to existing files.
type FileEditTool struct{}

// FileEditInput is the input schema for the FileEdit tool.
type FileEditInput struct {
	FilePath string `json:"file_path"`
	OldText  string `json:"old_text"`
	NewText  string `json:"new_text"`
}

// NewFileEditTool creates a new FileEditTool.
func NewFileEditTool() *FileEditTool {
	return &FileEditTool{}
}

func (t *FileEditTool) Name() string        { return "Edit" }
func (t *FileEditTool) IsReadOnly() bool     { return false }
func (t *FileEditTool) IsEnabled() bool      { return true }

func (t *FileEditTool) Description() string {
	return `Make targeted edits to a file by specifying the exact text to find and replace.

Use this tool for surgical modifications to existing files. The tool finds exactly one occurrence of old_text and replaces it with new_text.

Important:
- old_text must match exactly one location in the file (including whitespace and indentation)
- If old_text is not found or matches multiple locations, the edit will fail
- For creating new files, use the Write tool instead
- Include enough surrounding context in old_text to ensure a unique match`
}

func (t *FileEditTool) InputSchema() types.ToolInputSchema {
	return types.ToolInputSchema{
		Type: "object",
		Properties: map[string]types.ToolPropertySchema{
			"file_path": {
				Type:        "string",
				Description: "Absolute path or path relative to CWD of the file to edit.",
			},
			"old_text": {
				Type:        "string",
				Description: "The exact text to find in the file. Must match exactly one location.",
			},
			"new_text": {
				Type:        "string",
				Description: "The text to replace old_text with. Use empty string to delete text.",
			},
		},
		Required: []string{"file_path", "old_text", "new_text"},
	}
}

func (t *FileEditTool) UserFacingName(input json.RawMessage) string {
	var in FileEditInput
	if err := json.Unmarshal(input, &in); err == nil && in.FilePath != "" {
		return fmt.Sprintf("Edit: %s", filepath.Base(in.FilePath))
	}
	return "Edit"
}

func (t *FileEditTool) Execute(ctx context.Context, input json.RawMessage, toolCtx *ToolContext) (*ToolResult, error) {
	var in FileEditInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &ToolResult{Content: fmt.Sprintf("Invalid input: %v", err), IsError: true}, nil
	}

	if in.FilePath == "" {
		return &ToolResult{Content: "file_path is required", IsError: true}, nil
	}
	if in.OldText == "" {
		return &ToolResult{Content: "old_text is required. To create a new file, use the Write tool.", IsError: true}, nil
	}

	// Resolve path
	filePath := in.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(toolCtx.CWD, filePath)
	}

	// Read existing file
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &ToolResult{Content: fmt.Sprintf("File not found: %s. Use the Write tool to create new files.", in.FilePath), IsError: true}, nil
		}
		return &ToolResult{Content: fmt.Sprintf("Error reading file: %v", err), IsError: true}, nil
	}

	content := string(data)

	// Check for staleness
	if toolCtx.ReadFileState != nil {
		if cached, ok := toolCtx.ReadFileState.Get(filePath); ok {
			info, err := os.Stat(filePath)
			if err == nil {
				modTime := info.ModTime().UnixMilli()
				if modTime > cached.Timestamp {
					return &ToolResult{
						Content: fmt.Sprintf("File %s has been modified since last read. Read the file again before editing.", in.FilePath),
						IsError: true,
					}, nil
				}
			}
		}
	}

	// Count occurrences
	count := strings.Count(content, in.OldText)
	if count == 0 {
		return &ToolResult{
			Content: fmt.Sprintf("old_text not found in %s. Make sure it matches exactly, including whitespace and indentation.", in.FilePath),
			IsError: true,
		}, nil
	}
	if count > 1 {
		return &ToolResult{
			Content: fmt.Sprintf("old_text found %d times in %s. It must match exactly once. Include more surrounding context to make the match unique.", count, in.FilePath),
			IsError: true,
		}, nil
	}

	// Apply the replacement
	newContent := strings.Replace(content, in.OldText, in.NewText, 1)

	// Write back
	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return &ToolResult{Content: fmt.Sprintf("Error writing file: %v", err), IsError: true}, nil
	}

	// Update file state cache
	if toolCtx.ReadFileState != nil {
		toolCtx.ReadFileState.Set(filePath, &FileStateEntry{
			Content:   newContent,
			Timestamp: time.Now().UnixMilli(),
			Size:      int64(len(newContent)),
		})
	}

	// Calculate change stats
	oldLines := strings.Count(in.OldText, "\n") + 1
	newLines := strings.Count(in.NewText, "\n") + 1

	return &ToolResult{
		Content: fmt.Sprintf("Edited %s: replaced %d lines with %d lines", in.FilePath, oldLines, newLines),
	}, nil
}
