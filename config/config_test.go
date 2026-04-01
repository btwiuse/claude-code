package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSettingsFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "settings.json")

	settings := map[string]interface{}{
		"model":              "claude-opus-4-20250514",
		"customInstructions": "Always use tabs",
		"apiKey":             "sk-test-key",
		"env": map[string]string{
			"FOO": "bar",
		},
	}

	data, _ := json.Marshal(settings)
	os.WriteFile(path, data, 0o644)

	s, err := loadSettingsFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.Model != "claude-opus-4-20250514" {
		t.Errorf("expected model 'claude-opus-4-20250514', got %q", s.Model)
	}
	if s.CustomInstructions != "Always use tabs" {
		t.Errorf("expected custom instructions 'Always use tabs', got %q", s.CustomInstructions)
	}
	if s.APIKey != "sk-test-key" {
		t.Errorf("expected API key 'sk-test-key', got %q", s.APIKey)
	}
	if s.Env["FOO"] != "bar" {
		t.Errorf("expected env FOO='bar', got %q", s.Env["FOO"])
	}
}

func TestLoadSettingsFile_NotFound(t *testing.T) {
	_, err := loadSettingsFile("/nonexistent/settings.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadSettingsFile_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "settings.json")
	os.WriteFile(path, []byte("not json"), 0o644)

	_, err := loadSettingsFile(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestMergeSettings(t *testing.T) {
	base := &Settings{
		Model:              "sonnet",
		CustomInstructions: "base instructions",
		Env: map[string]string{
			"A": "1",
			"B": "2",
		},
	}

	override := &Settings{
		Model: "opus",
		Env: map[string]string{
			"B": "3",
			"C": "4",
		},
	}

	merged := mergeSettings(base, override)

	if merged.Model != "opus" {
		t.Errorf("expected model 'opus', got %q", merged.Model)
	}
	if merged.CustomInstructions != "base instructions" {
		t.Errorf("expected base instructions preserved, got %q", merged.CustomInstructions)
	}
	if merged.Env["A"] != "1" {
		t.Errorf("expected env A='1', got %q", merged.Env["A"])
	}
	if merged.Env["B"] != "3" {
		t.Errorf("expected env B='3' (overridden), got %q", merged.Env["B"])
	}
	if merged.Env["C"] != "4" {
		t.Errorf("expected env C='4', got %q", merged.Env["C"])
	}
}

func TestClaudeHomeDir(t *testing.T) {
	tmp := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	dir, err := ClaudeHomeDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(tmp, ".claude")
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}

	// Verify directory was created
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !fi.IsDir() {
		t.Error("expected directory")
	}
}

func TestLoad(t *testing.T) {
	tmp := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	// Create user settings
	claudeDir := filepath.Join(tmp, ".claude")
	os.MkdirAll(claudeDir, 0o755)

	settings := map[string]interface{}{
		"model": "haiku",
	}
	data, _ := json.Marshal(settings)
	os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Model != "haiku" {
		t.Errorf("expected model 'haiku', got %q", cfg.Model)
	}
}

func TestLoad_NoSettings(t *testing.T) {
	tmp := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return empty settings without error
	if cfg.Model != "" {
		t.Errorf("expected empty model, got %q", cfg.Model)
	}
}

func TestMergeSettings_McpServers(t *testing.T) {
	base := &Settings{
		McpServers: map[string]McpServerConfig{
			"server1": {Command: "cmd1"},
		},
	}

	override := &Settings{
		McpServers: map[string]McpServerConfig{
			"server2": {Command: "cmd2"},
		},
	}

	merged := mergeSettings(base, override)

	if _, ok := merged.McpServers["server1"]; !ok {
		t.Error("expected server1 to be preserved")
	}
	if _, ok := merged.McpServers["server2"]; !ok {
		t.Error("expected server2 to be added")
	}
}

func TestMergeSettings_Permissions(t *testing.T) {
	base := &Settings{
		Permissions: PermissionSettings{
			AllowRules: []PermissionRule{{Tool: "bash", Pattern: "echo *"}},
		},
	}

	override := &Settings{
		Permissions: PermissionSettings{
			AllowRules: []PermissionRule{{Tool: "bash", Pattern: "ls *"}},
		},
	}

	merged := mergeSettings(base, override)

	if len(merged.Permissions.AllowRules) != 1 {
		t.Fatalf("expected 1 allow rule, got %d", len(merged.Permissions.AllowRules))
	}
	if merged.Permissions.AllowRules[0].Pattern != "ls *" {
		t.Errorf("expected pattern 'ls *', got %q", merged.Permissions.AllowRules[0].Pattern)
	}
}
