// Package permissions implements the tool permission system for Claude Code.
package permissions

import (
	"path/filepath"
	"strings"

	"github.com/anthropics/claude-code-go/internal/types"
)

// DangerousFiles are files that require elevated permissions to modify.
var DangerousFiles = []string{
	".gitconfig",
	".gitmodules",
	".bashrc",
	".bash_profile",
	".zshrc",
	".zprofile",
	".profile",
	".ripgreprc",
	".mcp.json",
	".claude.json",
}

// DangerousDirectories are directories that require elevated permissions.
var DangerousDirectories = []string{
	".git",
	".vscode",
	".idea",
	".claude",
}

// Checker evaluates tool permissions against configured rules.
type Checker struct {
	Mode       types.PermissionMode
	AllowRules []types.ToolPermissionRule
	DenyRules  []types.ToolPermissionRule
}

// NewChecker creates a permission checker with the given mode.
func NewChecker(mode types.PermissionMode) *Checker {
	return &Checker{
		Mode: mode,
	}
}

// CheckToolPermission evaluates whether a tool call is allowed.
func (c *Checker) CheckToolPermission(toolName string, path string) types.PermissionDecision {
	// Bypass mode allows everything
	if c.Mode == types.PermissionModeBypass {
		return types.PermissionAllow
	}

	// Check deny rules first
	for _, rule := range c.DenyRules {
		if matchesRule(rule, toolName, path) {
			return types.PermissionDeny
		}
	}

	// Check allow rules
	for _, rule := range c.AllowRules {
		if matchesRule(rule, toolName, path) {
			return types.PermissionAllow
		}
	}

	// Auto mode: allow reads, ask for writes
	if c.Mode == types.PermissionModeAuto {
		return types.PermissionAllow
	}

	// Default mode: check if path is dangerous
	if path != "" && isDangerousPath(path) {
		return types.PermissionAsk
	}

	return types.PermissionAllow
}

// IsDangerousPath checks if a file path is considered dangerous.
func IsDangerousPath(path string) bool {
	return isDangerousPath(path)
}

func isDangerousPath(path string) bool {
	base := filepath.Base(path)

	// Check dangerous files
	for _, df := range DangerousFiles {
		if base == df {
			return true
		}
	}

	// Check if path is inside a dangerous directory
	for _, dd := range DangerousDirectories {
		if strings.Contains(path, "/"+dd+"/") || strings.HasPrefix(path, dd+"/") {
			return true
		}
	}

	return false
}

func matchesRule(rule types.ToolPermissionRule, toolName string, path string) bool {
	// Check tool name match
	if rule.ToolName != "" && rule.ToolName != "*" && rule.ToolName != toolName {
		return false
	}

	// Check path pattern match
	if rule.Pattern != "" && path != "" {
		matched, _ := filepath.Match(rule.Pattern, path)
		if !matched {
			matched, _ = filepath.Match(rule.Pattern, filepath.Base(path))
		}
		return matched
	}

	// Tool name only rule
	return rule.ToolName == toolName || rule.ToolName == "*"
}
