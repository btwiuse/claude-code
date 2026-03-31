// Package config manages Claude Code configuration files and settings.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// GlobalConfig represents the top-level user configuration.
type GlobalConfig struct {
	APIKeyHelper interface{}            `json:"apiKeyHelper,omitempty"`
	Settings     map[string]interface{} `json:"settings,omitempty"`
	History      []HistoryEntry         `json:"history,omitempty"`
	UserID       string                 `json:"userId,omitempty"`
}

// ProjectConfig represents per-project configuration.
type ProjectConfig struct {
	AllowedTools     []string               `json:"allowedTools,omitempty"`
	MCPServers       map[string]interface{} `json:"mcpServers,omitempty"`
	LastSessionID    string                 `json:"lastSessionId,omitempty"`
	LastCost         float64                `json:"lastCost,omitempty"`
	LastDuration     float64                `json:"lastDuration,omitempty"`
	LastLinesAdded   int                    `json:"lastLinesAdded,omitempty"`
	LastLinesRemoved int                    `json:"lastLinesRemoved,omitempty"`
}

// HistoryEntry represents a single entry in the command history.
type HistoryEntry struct {
	Display        string                `json:"display"`
	PastedContents map[int]PastedContent `json:"pastedContents,omitempty"`
}

// PastedContent represents content that was pasted into a prompt.
type PastedContent struct {
	ID        int    `json:"id"`
	Type      string `json:"type"` // "text" or "image"
	Content   string `json:"content,omitempty"`
	MediaType string `json:"mediaType,omitempty"`
	Filename  string `json:"filename,omitempty"`
}

var (
	globalConfig     *GlobalConfig
	globalConfigOnce sync.Once
	globalConfigMu   sync.Mutex
)

// ConfigDir returns the path to the Claude Code configuration directory.
func ConfigDir() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".claude")
	}
	return filepath.Join(home, ".claude")
}

// ProjectConfigDir returns the path to the project-local configuration directory.
func ProjectConfigDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	return filepath.Join(cwd, ".claude")
}

// GlobalConfigPath returns the path to the global configuration file.
func GlobalConfigPath() string {
	return filepath.Join(ConfigDir(), "config.json")
}

// ProjectConfigPath returns the path to the project configuration file.
func ProjectConfigPath() string {
	return filepath.Join(ProjectConfigDir(), "config.json")
}

// SessionsDir returns the path to the sessions directory.
func SessionsDir() string {
	return filepath.Join(ConfigDir(), "sessions")
}

// EnsureConfigDir creates the configuration directory if it doesn't exist.
func EnsureConfigDir() error {
	return os.MkdirAll(ConfigDir(), 0700)
}

// LoadGlobalConfig loads the global configuration from disk.
func LoadGlobalConfig() (*GlobalConfig, error) {
	path := GlobalConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &GlobalConfig{
				Settings: make(map[string]interface{}),
			}, nil
		}
		return nil, fmt.Errorf("reading global config: %w", err)
	}

	var cfg GlobalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing global config: %w", err)
	}
	if cfg.Settings == nil {
		cfg.Settings = make(map[string]interface{})
	}
	return &cfg, nil
}

// SaveGlobalConfig writes the global configuration to disk.
func SaveGlobalConfig(cfg *GlobalConfig) error {
	if err := EnsureConfigDir(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling global config: %w", err)
	}
	return os.WriteFile(GlobalConfigPath(), data, 0600)
}

// GetGlobalConfig returns the cached global configuration, loading it if necessary.
func GetGlobalConfig() *GlobalConfig {
	globalConfigOnce.Do(func() {
		cfg, err := LoadGlobalConfig()
		if err != nil {
			cfg = &GlobalConfig{Settings: make(map[string]interface{})}
		}
		globalConfig = cfg
	})
	return globalConfig
}

// UpdateGlobalConfig atomically updates the global configuration.
func UpdateGlobalConfig(updater func(*GlobalConfig)) error {
	globalConfigMu.Lock()
	defer globalConfigMu.Unlock()

	cfg := GetGlobalConfig()
	updater(cfg)
	return SaveGlobalConfig(cfg)
}

// LoadProjectConfig loads the project-level configuration.
func LoadProjectConfig() (*ProjectConfig, error) {
	path := ProjectConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ProjectConfig{}, nil
		}
		return nil, fmt.Errorf("reading project config: %w", err)
	}

	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing project config: %w", err)
	}
	return &cfg, nil
}

// SaveProjectConfig writes the project-level configuration.
func SaveProjectConfig(cfg *ProjectConfig) error {
	dir := ProjectConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling project config: %w", err)
	}
	return os.WriteFile(ProjectConfigPath(), data, 0644)
}

// GetOrCreateUserID returns the user ID, creating one if it doesn't exist.
func GetOrCreateUserID() string {
	cfg := GetGlobalConfig()
	if cfg.UserID != "" {
		return cfg.UserID
	}

	// Generate a new user ID
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "unknown"
	}
	userID := hex.EncodeToString(bytes)

	_ = UpdateGlobalConfig(func(c *GlobalConfig) {
		c.UserID = userID
	})
	return userID
}

// GetAPIKey returns the API key from environment or configuration.
func GetAPIKey() string {
	// Check environment variables first
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("ANTHROPIC_AUTH_TOKEN"); key != "" {
		return key
	}

	// Check config
	cfg := GetGlobalConfig()
	if keyStr, ok := cfg.APIKeyHelper.(string); ok {
		return keyStr
	}
	if keyMap, ok := cfg.APIKeyHelper.(map[string]interface{}); ok {
		if apiKey, ok := keyMap["apiKey"].(string); ok {
			return apiKey
		}
	}

	return ""
}

// AddToHistory adds an entry to the command history.
func AddToHistory(entry HistoryEntry) error {
	return UpdateGlobalConfig(func(cfg *GlobalConfig) {
		cfg.History = append(cfg.History, entry)
		// Trim to max history items
		const maxHistory = 100
		if len(cfg.History) > maxHistory {
			cfg.History = cfg.History[len(cfg.History)-maxHistory:]
		}
	})
}

// GetHistory returns the command history.
func GetHistory() []HistoryEntry {
	cfg := GetGlobalConfig()
	return cfg.History
}
