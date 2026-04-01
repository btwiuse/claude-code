// Package config handles reading and merging Claude Code settings
// from ~/.claude/settings.json and project-level .claude/settings.json.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings represents the merged configuration from all sources.
type Settings struct {
	// Permissions
	Permissions PermissionSettings `json:"permissions,omitempty"`

	// Model settings
	Model string `json:"model,omitempty"`

	// API key
	APIKey string `json:"apiKey,omitempty"`

	// Custom system prompt
	SystemPrompt string `json:"systemPrompt,omitempty"`

	// Append to system prompt
	AppendSystemPrompt string `json:"appendSystemPrompt,omitempty"`

	// Allowed tools
	AllowedTools []string `json:"allowedTools,omitempty"`

	// Denied tools
	DeniedTools []string `json:"deniedTools,omitempty"`

	// Custom instructions
	CustomInstructions string `json:"customInstructions,omitempty"`

	// Environment variables
	Env map[string]string `json:"env,omitempty"`

	// MCP servers
	McpServers map[string]McpServerConfig `json:"mcpServers,omitempty"`

	// Raw JSON for any unrecognized fields
	Raw map[string]json.RawMessage `json:"-"`
}

// PermissionSettings controls tool permission behavior.
type PermissionSettings struct {
	AllowRules []PermissionRule `json:"allow,omitempty"`
	DenyRules  []PermissionRule `json:"deny,omitempty"`
}

// PermissionRule is a single permission pattern.
type PermissionRule struct {
	Tool    string `json:"tool"`
	Pattern string `json:"pattern,omitempty"`
}

// McpServerConfig defines an MCP server connection.
type McpServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
}

// ClaudeHomeDir returns the path to ~/.claude, creating it if necessary.
func ClaudeHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// Load reads and merges settings from user and project-level config files.
// User settings: ~/.claude/settings.json
// Project settings: ./.claude/settings.json (current directory and ancestors)
func Load() (*Settings, error) {
	merged := &Settings{}

	// Load user settings first (lowest precedence)
	userSettings, err := loadUserSettings()
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if userSettings != nil {
		merged = mergeSettings(merged, userSettings)
	}

	// Load project settings (higher precedence)
	projectSettings, err := loadProjectSettings()
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if projectSettings != nil {
		merged = mergeSettings(merged, projectSettings)
	}

	return merged, nil
}

// loadUserSettings reads ~/.claude/settings.json.
func loadUserSettings() (*Settings, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".claude", "settings.json")
	return loadSettingsFile(path)
}

// loadProjectSettings reads .claude/settings.json from the current or ancestor directory.
func loadProjectSettings() (*Settings, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	dir := cwd
	for {
		path := filepath.Join(dir, ".claude", "settings.json")
		s, err := loadSettingsFile(path)
		if err == nil {
			return s, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, os.ErrNotExist
}

// loadSettingsFile reads and parses a single settings.json file.
func loadSettingsFile(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	// Also keep raw for unrecognized fields
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		s.Raw = raw
	}
	return &s, nil
}

// mergeSettings merges override into base, with override taking precedence.
func mergeSettings(base, override *Settings) *Settings {
	result := *base

	if override.Model != "" {
		result.Model = override.Model
	}
	if override.APIKey != "" {
		result.APIKey = override.APIKey
	}
	if override.SystemPrompt != "" {
		result.SystemPrompt = override.SystemPrompt
	}
	if override.AppendSystemPrompt != "" {
		result.AppendSystemPrompt = override.AppendSystemPrompt
	}
	if override.CustomInstructions != "" {
		result.CustomInstructions = override.CustomInstructions
	}
	if len(override.AllowedTools) > 0 {
		result.AllowedTools = override.AllowedTools
	}
	if len(override.DeniedTools) > 0 {
		result.DeniedTools = override.DeniedTools
	}
	if len(override.Permissions.AllowRules) > 0 {
		result.Permissions.AllowRules = override.Permissions.AllowRules
	}
	if len(override.Permissions.DenyRules) > 0 {
		result.Permissions.DenyRules = override.Permissions.DenyRules
	}
	if len(override.Env) > 0 {
		if result.Env == nil {
			result.Env = make(map[string]string)
		}
		for k, v := range override.Env {
			result.Env[k] = v
		}
	}
	if len(override.McpServers) > 0 {
		if result.McpServers == nil {
			result.McpServers = make(map[string]McpServerConfig)
		}
		for k, v := range override.McpServers {
			result.McpServers[k] = v
		}
	}

	return &result
}
