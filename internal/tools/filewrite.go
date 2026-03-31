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

// FileWriteTool creates or overwrites files.
type FileWriteTool struct{}

// FileWriteInput is the input schema for the FileWrite tool.
type FileWriteInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

// NewFileWriteTool creates a new FileWriteTool.
func NewFileWriteTool() *FileWriteTool {
	return &FileWriteTool{}
}

func (t *FileWriteTool) Name() string     { return "Write" }
func (t *FileWriteTool) IsReadOnly() bool { return false }
func (t *FileWriteTool) IsEnabled() bool  { return true }

func (t *FileWriteTool) Description() string {
	return `Write content to a file, creating it if it doesn't exist or overwriting if it does.

Use this tool to create new files or completely replace existing file contents. For partial modifications to existing files, prefer the Edit tool instead.

Important notes:
- Parent directories will be created automatically if they don't exist
- If the file was previously read and has been modified externally since, the write will be rejected to prevent data loss
- Always provide the complete file content, not just the changes`
}

func (t *FileWriteTool) InputSchema() types.ToolInputSchema {
	return types.ToolInputSchema{
		Type: "object",
		Properties: map[string]types.ToolPropertySchema{
			"file_path": {
				Type:        "string",
				Description: "Absolute path or path relative to CWD of the file to write.",
			},
			"content": {
				Type:        "string",
				Description: "The complete content to write to the file.",
			},
		},
		Required: []string{"file_path", "content"},
	}
}

func (t *FileWriteTool) UserFacingName(input json.RawMessage) string {
	var in FileWriteInput
	if err := json.Unmarshal(input, &in); err == nil && in.FilePath != "" {
		return fmt.Sprintf("Write: %s", filepath.Base(in.FilePath))
	}
	return "Write"
}

func (t *FileWriteTool) Execute(ctx context.Context, input json.RawMessage, toolCtx *ToolContext) (*ToolResult, error) {
	var in FileWriteInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &ToolResult{Content: fmt.Sprintf("Invalid input: %v", err), IsError: true}, nil
	}

	if in.FilePath == "" {
		return &ToolResult{Content: "file_path is required", IsError: true}, nil
	}

	// Resolve path
	filePath := in.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(toolCtx.CWD, filePath)
	}

	// Staleness check: if we previously read this file, check it hasn't been modified
	if toolCtx.ReadFileState != nil {
		if cached, ok := toolCtx.ReadFileState.Get(filePath); ok {
			info, err := os.Stat(filePath)
			if err == nil {
				modTime := info.ModTime().UnixMilli()
				if modTime > cached.Timestamp {
					return &ToolResult{
						Content: fmt.Sprintf("File %s has been modified since it was last read (cached at %d, modified at %d). Read the file again before writing to avoid losing changes.",
							in.FilePath, cached.Timestamp, modTime),
						IsError: true,
					}, nil
				}
			}
		}
	}

	// Create parent directories
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &ToolResult{Content: fmt.Sprintf("Error creating directories: %v", err), IsError: true}, nil
	}

	// Determine if this is a new file or overwrite
	isNew := true
	if _, err := os.Stat(filePath); err == nil {
		isNew = false
	}

	// Write the file
	if err := os.WriteFile(filePath, []byte(in.Content), 0644); err != nil {
		return &ToolResult{Content: fmt.Sprintf("Error writing file: %v", err), IsError: true}, nil
	}

	// Update file state cache
	if toolCtx.ReadFileState != nil {
		toolCtx.ReadFileState.Set(filePath, &FileStateEntry{
			Content:   in.Content,
			Timestamp: time.Now().UnixMilli(),
			Size:      int64(len(in.Content)),
		})
	}

	lines := strings.Count(in.Content, "\n") + 1
	action := "Created"
	if !isNew {
		action = "Wrote"
	}
	return &ToolResult{
		Content: fmt.Sprintf("%s %s (%d lines)", action, in.FilePath, lines),
	}, nil
}
