package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings represents the ~/.claude/settings.json configuration.
type Settings struct {
	APIKey          string            `json:"apiKey,omitempty"`
	APIKeyHelper    string            `json:"apiKeyHelper,omitempty"`
	Model           string            `json:"model,omitempty"`
	AvailableModels []string          `json:"availableModels,omitempty"`
	Env             map[string]string `json:"env,omitempty"`
	Permissions     *Permissions      `json:"permissions,omitempty"`
	CustomPrompt    string            `json:"customPrompt,omitempty"`

	// Extra fields preserved for forward compatibility.
	Extra map[string]json.RawMessage `json:"-"`
}

// Permissions controls tool access rules.
type Permissions struct {
	Allow              []PermissionRule `json:"allow,omitempty"`
	Deny               []PermissionRule `json:"deny,omitempty"`
	DefaultMode        string           `json:"defaultMode,omitempty"`
	AdditionalDirs     []string         `json:"additionalDirectories,omitempty"`
}

// PermissionRule defines an allow/deny rule for a tool.
type PermissionRule struct {
	Tool   string `json:"tool"`
	Match  string `json:"match,omitempty"`
	Action string `json:"action,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// ClaudeDir returns the path to the ~/.claude directory.
func ClaudeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude"
	}
	return filepath.Join(home, ".claude")
}

// SessionsDir returns the path to ~/.claude/sessions/.
func SessionsDir() string {
	return filepath.Join(ClaudeDir(), "sessions")
}

// SettingsPath returns the path to ~/.claude/settings.json.
func SettingsPath() string {
	return filepath.Join(ClaudeDir(), "settings.json")
}

// Load reads and parses ~/.claude/settings.json.
// Returns an empty Settings if the file does not exist.
func Load() (*Settings, error) {
	return LoadFrom(SettingsPath())
}

// LoadFrom reads and parses settings from the given path.
func LoadFrom(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Settings{}, nil
		}
		return nil, err
	}

	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	// Capture extra fields.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		known := map[string]bool{
			"apiKey": true, "apiKeyHelper": true, "model": true,
			"availableModels": true, "env": true, "permissions": true,
			"customPrompt": true,
		}
		extra := make(map[string]json.RawMessage)
		for k, v := range raw {
			if !known[k] {
				extra[k] = v
			}
		}
		if len(extra) > 0 {
			s.Extra = extra
		}
	}

	return &s, nil
}

// Resolve returns the effective API key, checking settings then environment.
func (s *Settings) Resolve() string {
	if s.APIKey != "" {
		return s.APIKey
	}
	return os.Getenv("ANTHROPIC_API_KEY")
}

// EffectiveModel returns the model to use, with fallback.
func (s *Settings) EffectiveModel(flagModel string) string {
	if flagModel != "" {
		return flagModel
	}
	if s.Model != "" {
		return s.Model
	}
	return "claude-sonnet-4-20250514"
}
