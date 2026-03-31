package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/claude-code-go/internal/types"
)

// GlobTool finds files matching glob patterns.
type GlobTool struct{}

// GlobInput is the input schema for the Glob tool.
type GlobInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

// NewGlobTool creates a new GlobTool.
func NewGlobTool() *GlobTool {
	return &GlobTool{}
}

func (t *GlobTool) Name() string     { return "Glob" }
func (t *GlobTool) IsReadOnly() bool { return true }
func (t *GlobTool) IsEnabled() bool  { return true }

func (t *GlobTool) Description() string {
	return `Find files matching a glob pattern in the filesystem.

Use this to discover files by name pattern. Supports standard glob syntax:
- * matches any characters within a path segment
- ** matches any characters across path segments (recursive)
- ? matches a single character
- {a,b} matches either a or b
- [abc] matches any character in the set

Results are limited to 100 files by default. Returns file paths relative to the search root.`
}

func (t *GlobTool) InputSchema() types.ToolInputSchema {
	return types.ToolInputSchema{
		Type: "object",
		Properties: map[string]types.ToolPropertySchema{
			"pattern": {
				Type:        "string",
				Description: "Glob pattern to match files against (e.g., '**/*.go', 'src/**/*.ts', '*.json').",
			},
			"path": {
				Type:        "string",
				Description: "Directory to search in. Defaults to the current working directory.",
			},
		},
		Required: []string{"pattern"},
	}
}

func (t *GlobTool) UserFacingName(input json.RawMessage) string {
	var in GlobInput
	if err := json.Unmarshal(input, &in); err == nil && in.Pattern != "" {
		return fmt.Sprintf("Glob: %s", in.Pattern)
	}
	return "Glob"
}

func (t *GlobTool) Execute(ctx context.Context, input json.RawMessage, toolCtx *ToolContext) (*ToolResult, error) {
	var in GlobInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &ToolResult{Content: fmt.Sprintf("Invalid input: %v", err), IsError: true}, nil
	}

	if in.Pattern == "" {
		return &ToolResult{Content: "pattern is required", IsError: true}, nil
	}

	root := toolCtx.CWD
	if in.Path != "" {
		if filepath.IsAbs(in.Path) {
			root = in.Path
		} else {
			root = filepath.Join(toolCtx.CWD, in.Path)
		}
	}

	const maxResults = 100
	var matches []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if len(matches) >= maxResults {
			return filepath.SkipAll
		}

		// Skip hidden directories (but not files)
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") && path != root {
			return filepath.SkipDir
		}

		// Get relative path for matching
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}

		// Try matching against the pattern
		matched, err := filepath.Match(in.Pattern, relPath)
		if err != nil {
			// For patterns with **, we need a different approach
			matched = matchGlob(in.Pattern, relPath)
		}

		if !matched {
			// Also try matching just the filename
			matched, _ = filepath.Match(in.Pattern, info.Name())
		}

		if matched && !info.IsDir() {
			matches = append(matches, relPath)
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return &ToolResult{Content: fmt.Sprintf("Error walking directory: %v", err), IsError: true}, nil
	}

	if len(matches) == 0 {
		return &ToolResult{Content: fmt.Sprintf("No files found matching pattern: %s", in.Pattern)}, nil
	}

	var result strings.Builder
	for _, m := range matches {
		result.WriteString(m)
		result.WriteString("\n")
	}

	if len(matches) >= maxResults {
		fmt.Fprintf(&result, "\n(results truncated at %d files)", maxResults)
	}

	return &ToolResult{Content: result.String()}, nil
}

// matchGlob handles ** patterns that filepath.Match doesn't support.
func matchGlob(pattern, path string) bool {
	// Simple ** handling
	if strings.Contains(pattern, "**") {
		// Split pattern by **
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			prefix := strings.TrimSuffix(parts[0], "/")
			suffix := strings.TrimPrefix(parts[1], "/")

			// Check prefix
			if prefix != "" && !strings.HasPrefix(path, prefix) {
				return false
			}

			// Check suffix
			if suffix != "" {
				matched, _ := filepath.Match(suffix, filepath.Base(path))
				return matched
			}
			return true
		}
	}
	return false
}
