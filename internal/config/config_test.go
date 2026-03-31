package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestConfigDir(t *testing.T) {
	t.Run("default config dir", func(t *testing.T) {
		// Unset custom env var
		os.Unsetenv("CLAUDE_CONFIG_DIR")
		dir := ConfigDir()
		home, _ := os.UserHomeDir()
		expected := filepath.Join(home, ".claude")
		if dir != expected {
			t.Errorf("expected %s, got %s", expected, dir)
		}
	})

	t.Run("custom config dir", func(t *testing.T) {
		customDir := t.TempDir()
		t.Setenv("CLAUDE_CONFIG_DIR", customDir)
		dir := ConfigDir()
		if dir != customDir {
			t.Errorf("expected %s, got %s", customDir, dir)
		}
	})
}

func TestGlobalConfig(t *testing.T) {
	t.Run("load missing config", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

		cfg, err := LoadGlobalConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Settings == nil {
			t.Error("expected settings map to be initialized")
		}
	})

	t.Run("save and load config", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

		cfg := &GlobalConfig{
			Settings: map[string]interface{}{"key": "value"},
			UserID:   "test-user",
		}
		if err := SaveGlobalConfig(cfg); err != nil {
			t.Fatalf("save error: %v", err)
		}

		loaded, err := LoadGlobalConfig()
		if err != nil {
			t.Fatalf("load error: %v", err)
		}
		if loaded.UserID != "test-user" {
			t.Errorf("expected test-user, got %s", loaded.UserID)
		}
	})
}

func TestGetAPIKey(t *testing.T) {
	t.Run("from environment", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "test-key-123")
		key := GetAPIKey()
		if key != "test-key-123" {
			t.Errorf("expected test-key-123, got %s", key)
		}
	})
}

func TestHistory(t *testing.T) {
	t.Run("add and get history", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

		// Reset cached config
		globalConfig = nil
		globalConfigOnce = syncOnce()

		entry := HistoryEntry{Display: "test prompt"}
		if err := AddToHistory(entry); err != nil {
			t.Fatalf("add error: %v", err)
		}

		history := GetHistory()
		if len(history) == 0 {
			t.Fatal("expected history entry")
		}
		if history[0].Display != "test prompt" {
			t.Errorf("expected 'test prompt', got %s", history[0].Display)
		}
	})
}

func syncOnce() sync.Once {
	return sync.Once{}
}
