package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GlobTool finds files matching glob patterns.
type GlobTool struct{}

type globInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

func (t *GlobTool) Name() string { return "glob" }

func (t *GlobTool) Description() string {
	return `Find files matching a glob pattern. Searches recursively from the specified path (or current directory). Supports standard glob patterns: * matches any characters in a path segment, ** matches across segments, ? matches a single character, {a,b} matches alternatives.`
}

func (t *GlobTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "Glob pattern to match (e.g., '**/*.go', 'src/**/*.ts', '*.{js,ts}')"
			},
			"path": {
				"type": "string",
				"description": "Base directory to search from (defaults to current directory)"
			}
		},
		"required": ["pattern"]
	}`)
}

func (t *GlobTool) IsReadOnly() bool { return true }

func (t *GlobTool) Run(_ context.Context, input json.RawMessage) (*Result, error) {
	var in globInput
	if err := json.Unmarshal(input, &in); err != nil {
		return &Result{Error: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	baseDir := in.Path
	if baseDir == "" {
		var err error
		baseDir, err = os.Getwd()
		if err != nil {
			return &Result{Error: fmt.Sprintf("failed to get working directory: %v", err), IsError: true}, nil
		}
	}
	baseDir = expandPath(baseDir)

	var matches []string

	// Handle ** patterns by walking the directory tree
	if strings.Contains(in.Pattern, "**") {
		err := filepath.WalkDir(baseDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // Skip errors
			}

			// Skip hidden directories
			if d.IsDir() && strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return filepath.SkipDir
			}
			// Skip node_modules and vendor
			if d.IsDir() && (d.Name() == "node_modules" || d.Name() == "vendor") {
				return filepath.SkipDir
			}

			if d.IsDir() {
				return nil
			}

			rel, err := filepath.Rel(baseDir, path)
			if err != nil {
				return nil
			}

			if matchDoubleGlob(in.Pattern, rel) {
				matches = append(matches, path)
			}
			return nil
		})
		if err != nil {
			return &Result{Error: fmt.Sprintf("failed to walk directory: %v", err), IsError: true}, nil
		}
	} else {
		// Simple glob
		pattern := filepath.Join(baseDir, in.Pattern)
		var err error
		matches, err = filepath.Glob(pattern)
		if err != nil {
			return &Result{Error: fmt.Sprintf("invalid glob pattern: %v", err), IsError: true}, nil
		}
	}

	sort.Strings(matches)

	const maxResults = 1000
	if len(matches) > maxResults {
		matches = matches[:maxResults]
	}

	if len(matches) == 0 {
		return &Result{Output: "No files found matching pattern: " + in.Pattern}, nil
	}

	return &Result{Output: strings.Join(matches, "\n")}, nil
}

// matchDoubleGlob matches a path against a pattern containing **.
func matchDoubleGlob(pattern, path string) bool {
	// Split pattern on **
	parts := strings.Split(pattern, "**")
	if len(parts) == 1 {
		// No ** in pattern, use simple match
		matched, _ := filepath.Match(pattern, path)
		return matched
	}

	// Handle patterns like **/*.go
	if parts[0] == "" && len(parts) == 2 {
		suffix := strings.TrimPrefix(parts[1], "/")
		if suffix == "" {
			return true
		}
		// Match suffix against each path component
		matched, _ := filepath.Match(suffix, filepath.Base(path))
		if matched {
			return true
		}
		// Also try matching against relative path
		matched, _ = filepath.Match(suffix, path)
		return matched
	}

	// Handle prefix/**/suffix patterns
	if len(parts) == 2 {
		prefix := strings.TrimSuffix(parts[0], "/")
		suffix := strings.TrimPrefix(parts[1], "/")

		if prefix != "" && !strings.HasPrefix(path, prefix) {
			return false
		}

		if suffix == "" {
			return true
		}

		// Check if any subpath matches the suffix
		pathAfterPrefix := path
		if prefix != "" {
			pathAfterPrefix = strings.TrimPrefix(path, prefix+"/")
		}

		// Try matching suffix against the filename
		matched, _ := filepath.Match(suffix, filepath.Base(pathAfterPrefix))
		return matched
	}

	// For more complex patterns, do a simple check
	matched, _ := filepath.Match(strings.ReplaceAll(pattern, "**", "*"), filepath.Base(path))
	return matched
}
