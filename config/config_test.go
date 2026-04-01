package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFrom_NonExistent(t *testing.T) {
	s, err := LoadFrom("/nonexistent/settings.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Model != "" {
		t.Errorf("expected empty model, got %q", s.Model)
	}
}

func TestLoadFrom_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	data := `{
		"apiKey": "test-key",
		"model": "claude-sonnet-4-20250514",
		"env": {"FOO": "bar"},
		"permissions": {
			"allow": [{"tool": "Bash"}],
			"defaultMode": "auto"
		},
		"unknownField": "preserved"
	}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.APIKey != "test-key" {
		t.Errorf("expected apiKey 'test-key', got %q", s.APIKey)
	}
	if s.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected model, got %q", s.Model)
	}
	if s.Env["FOO"] != "bar" {
		t.Errorf("expected env FOO=bar, got %v", s.Env)
	}
	if s.Permissions == nil || s.Permissions.DefaultMode != "auto" {
		t.Error("expected permissions.defaultMode=auto")
	}
	if s.Extra == nil || s.Extra["unknownField"] == nil {
		t.Error("expected extra field 'unknownField' to be preserved")
	}
}

func TestResolve(t *testing.T) {
	s := &Settings{APIKey: "from-settings"}
	if got := s.Resolve(); got != "from-settings" {
		t.Errorf("expected 'from-settings', got %q", got)
	}

	s = &Settings{}
	t.Setenv("ANTHROPIC_API_KEY", "from-env")
	if got := s.Resolve(); got != "from-env" {
		t.Errorf("expected 'from-env', got %q", got)
	}
}

func TestEffectiveModel(t *testing.T) {
	s := &Settings{Model: "settings-model"}

	if got := s.EffectiveModel("flag-model"); got != "flag-model" {
		t.Errorf("expected flag model, got %q", got)
	}
	if got := s.EffectiveModel(""); got != "settings-model" {
		t.Errorf("expected settings model, got %q", got)
	}

	s = &Settings{}
	if got := s.EffectiveModel(""); got != "claude-sonnet-4-20250514" {
		t.Errorf("expected default model, got %q", got)
	}
}
