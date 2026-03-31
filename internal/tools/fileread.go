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

// FileReadTool reads file contents.
type FileReadTool struct{}

// FileReadInput is the input schema for the FileRead tool.
type FileReadInput struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// NewFileReadTool creates a new FileReadTool.
func NewFileReadTool() *FileReadTool {
	return &FileReadTool{}
}

func (t *FileReadTool) Name() string        { return "Read" }
func (t *FileReadTool) IsReadOnly() bool     { return true }
func (t *FileReadTool) IsEnabled() bool      { return true }

func (t *FileReadTool) Description() string {
	return `Read the contents of a file from the filesystem.

Use this tool to examine source code, configuration files, documentation, and other text-based files. Supports text files of all types.

For large files, use the offset and limit parameters to read specific sections:
- offset: Line number to start reading from (0-indexed)
- limit: Maximum number of lines to read

The tool returns the file contents with line numbers prefixed to each line.`
}

func (t *FileReadTool) InputSchema() types.ToolInputSchema {
	return types.ToolInputSchema{
		Type: "object",
		Properties: map[string]types.ToolPropertySchema{
			"file_path": {
				Type:        "string",
				Description: "Absolute path or path relative to CWD of the file to read.",
			},
			"offset": {
				Type:        "integer",
				Description: "Line number to start reading from (0-indexed). Defaults to 0.",
			},
			"limit": {
				Type:        "integer",
				Description: "Maximum number of lines to read. Defaults to reading the entire file.",
			},
		},
		Required: []string{"file_path"},
	}
}

func (t *FileReadTool) UserFacingName(input json.RawMessage) string {
	var in FileReadInput
	if err := json.Unmarshal(input, &in); err == nil && in.FilePath != "" {
		return fmt.Sprintf("Read: %s", filepath.Base(in.FilePath))
	}
	return "Read"
}

func (t *FileReadTool) Execute(ctx context.Context, input json.RawMessage, toolCtx *ToolContext) (*ToolResult, error) {
	var in FileReadInput
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

	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &ToolResult{Content: fmt.Sprintf("File not found: %s", in.FilePath), IsError: true}, nil
		}
		return &ToolResult{Content: fmt.Sprintf("Error accessing file: %v", err), IsError: true}, nil
	}

	if info.IsDir() {
		return &ToolResult{Content: fmt.Sprintf("%s is a directory, not a file. Use ListDir to view directory contents.", in.FilePath), IsError: true}, nil
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return &ToolResult{Content: fmt.Sprintf("Error reading file: %v", err), IsError: true}, nil
	}

	// Update file state cache
	if toolCtx.ReadFileState != nil {
		toolCtx.ReadFileState.Set(filePath, &FileStateEntry{
			Content:   string(data),
			Timestamp: time.Now().UnixMilli(),
			Size:      info.Size(),
		})
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// Apply offset and limit
	start := 0
	if in.Offset > 0 {
		start = in.Offset
	}
	if start > len(lines) {
		start = len(lines)
	}

	end := len(lines)
	if in.Limit > 0 && start+in.Limit < end {
		end = start + in.Limit
	}

	lines = lines[start:end]

	// Format with line numbers
	var result strings.Builder
	for i, line := range lines {
		lineNum := start + i + 1
		fmt.Fprintf(&result, "%d\t%s\n", lineNum, line)
	}

	output := result.String()
	if output == "" {
		output = "(empty file)"
	}

	return &ToolResult{Content: output}, nil
}
