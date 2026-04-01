package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GlobTool finds files matching glob patterns.
type GlobTool struct{}

type globInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

func (t *GlobTool) Name() string { return "Glob" }

func (t *GlobTool) Description() string {
	return "Find files by glob pattern. Returns matching file paths."
}

func (t *GlobTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "Glob pattern to match files (e.g. '**/*.go', 'src/**/*.ts')"
			},
			"path": {
				"type": "string",
				"description": "Directory to search in (defaults to current directory)"
			}
		},
		"required": ["pattern"]
	}`)
}

func (t *GlobTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in globInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	if in.Pattern == "" {
		return Result{Content: "pattern is required", IsError: true}, nil
	}

	root := in.Path
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return Result{Content: fmt.Sprintf("error getting cwd: %v", err), IsError: true}, nil
		}
	}

	matches, err := globWalk(root, in.Pattern)
	if err != nil {
		return Result{Content: fmt.Sprintf("error: %v", err), IsError: true}, nil
	}

	if len(matches) == 0 {
		return Result{Content: "No files matched the pattern."}, nil
	}

	const maxResults = 200
	truncated := false
	if len(matches) > maxResults {
		matches = matches[:maxResults]
		truncated = true
	}

	result := strings.Join(matches, "\n")
	if truncated {
		result += fmt.Sprintf("\n... (truncated, showing first %d of many matches)", maxResults)
	}

	return Result{Content: result}, nil
}

// globWalk walks the directory tree and matches files against the pattern.
func globWalk(root, pattern string) ([]string, error) {
	// Handle simple non-recursive patterns.
	if !strings.Contains(pattern, "**") {
		fullPattern := filepath.Join(root, pattern)
		return filepath.Glob(fullPattern)
	}

	// For ** patterns, walk the tree.
	var matches []string
	// Extract the suffix after **
	parts := strings.SplitN(pattern, "**", 2)
	prefix := parts[0]
	suffix := ""
	if len(parts) > 1 {
		suffix = strings.TrimPrefix(parts[1], "/")
		suffix = strings.TrimPrefix(suffix, string(filepath.Separator))
	}

	searchRoot := root
	if prefix != "" {
		searchRoot = filepath.Join(root, strings.TrimSuffix(prefix, "/"))
	}

	err := filepath.Walk(searchRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths (permissions, broken symlinks)
		}
		if info.IsDir() {
			// Skip hidden directories.
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}

		if suffix == "" {
			matches = append(matches, path)
			return nil
		}

		matched, _ := filepath.Match(suffix, info.Name())
		if matched {
			matches = append(matches, path)
		}
		return nil
	})

	return matches, err
}
