package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anthropics/claude-code-go/internal/types"
)

// ListDirTool lists directory contents.
type ListDirTool struct{}

// ListDirInput is the input schema for the ListDir tool.
type ListDirInput struct {
	Path string `json:"path"`
}

// NewListDirTool creates a new ListDirTool.
func NewListDirTool() *ListDirTool {
	return &ListDirTool{}
}

func (t *ListDirTool) Name() string        { return "ListDir" }
func (t *ListDirTool) IsReadOnly() bool     { return true }
func (t *ListDirTool) IsEnabled() bool      { return true }

func (t *ListDirTool) Description() string {
	return `List the contents of a directory.

Returns a listing of files and subdirectories in the specified directory. Each entry shows whether it is a file or directory, along with its name.

Hidden files (starting with .) are excluded by default. Use the Bash tool with 'ls -la' if you need to see hidden files.`
}

func (t *ListDirTool) InputSchema() types.ToolInputSchema {
	return types.ToolInputSchema{
		Type: "object",
		Properties: map[string]types.ToolPropertySchema{
			"path": {
				Type:        "string",
				Description: "Absolute path or path relative to CWD of the directory to list.",
			},
		},
		Required: []string{"path"},
	}
}

func (t *ListDirTool) UserFacingName(input json.RawMessage) string {
	var in ListDirInput
	if err := json.Unmarshal(input, &in); err == nil && in.Path != "" {
		return fmt.Sprintf("ListDir: %s", filepath.Base(in.Path))
	}
	return "ListDir"
}

func (t *ListDirTool) Execute(ctx context.Context, input json.RawMessage, toolCtx *ToolContext) (*ToolResult, error) {
	var in ListDirInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &ToolResult{Content: fmt.Sprintf("Invalid input: %v", err), IsError: true}, nil
	}

	if in.Path == "" {
		return &ToolResult{Content: "path is required", IsError: true}, nil
	}

	// Resolve path
	dirPath := in.Path
	if !filepath.IsAbs(dirPath) {
		dirPath = filepath.Join(toolCtx.CWD, dirPath)
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &ToolResult{Content: fmt.Sprintf("Directory not found: %s", in.Path), IsError: true}, nil
		}
		return &ToolResult{Content: fmt.Sprintf("Error reading directory: %v", err), IsError: true}, nil
	}

	// Filter and sort entries
	var dirs []string
	var files []string

	for _, entry := range entries {
		name := entry.Name()
		// Skip hidden files
		if strings.HasPrefix(name, ".") {
			continue
		}

		if entry.IsDir() {
			dirs = append(dirs, name+"/")
		} else {
			files = append(files, name)
		}
	}

	sort.Strings(dirs)
	sort.Strings(files)

	var result strings.Builder
	for _, d := range dirs {
		result.WriteString(d)
		result.WriteString("\n")
	}
	for _, f := range files {
		result.WriteString(f)
		result.WriteString("\n")
	}

	output := result.String()
	if output == "" {
		output = "(empty directory)"
	}

	return &ToolResult{Content: output}, nil
}
