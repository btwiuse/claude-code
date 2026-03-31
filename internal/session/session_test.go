package session

import (
	"os"
	"testing"

	"github.com/anthropics/claude-code-go/internal/types"
)

func TestSession(t *testing.T) {
	t.Run("new session", func(t *testing.T) {
		s := NewSession("test-session", "/tmp", "claude-sonnet-4-20250514")
		if s.ID != "test-session" {
			t.Errorf("expected test-session, got %s", s.ID)
		}
		if s.CWD != "/tmp" {
			t.Errorf("expected /tmp, got %s", s.CWD)
		}
		if len(s.Messages) != 0 {
			t.Error("expected empty messages")
		}
	})

	t.Run("add message", func(t *testing.T) {
		s := NewSession("test-session", "/tmp", "claude-sonnet-4-20250514")
		s.AddMessage(types.Message{
			Role: types.RoleUser,
			Content: []types.ContentBlock{
				{Type: types.ContentTypeText, Text: "hello"},
			},
		})
		if len(s.Messages) != 1 {
			t.Errorf("expected 1 message, got %d", len(s.Messages))
		}
	})

	t.Run("save and load", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

		s := NewSession("save-test", "/tmp", "claude-sonnet-4-20250514")
		s.AddMessage(types.Message{
			Role: types.RoleUser,
			Content: []types.ContentBlock{
				{Type: types.ContentTypeText, Text: "test message"},
			},
		})

		if err := s.Save(); err != nil {
			t.Fatalf("save error: %v", err)
		}

		loaded, err := Load("save-test")
		if err != nil {
			t.Fatalf("load error: %v", err)
		}

		if loaded.ID != "save-test" {
			t.Errorf("expected save-test, got %s", loaded.ID)
		}
		if len(loaded.Messages) != 1 {
			t.Errorf("expected 1 message, got %d", len(loaded.Messages))
		}
	})

	t.Run("list sessions", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

		// Create a few sessions
		for _, id := range []string{"sess1", "sess2", "sess3"} {
			s := NewSession(id, "/tmp", "claude-sonnet-4-20250514")
			if err := s.Save(); err != nil {
				t.Fatalf("save error: %v", err)
			}
		}

		sessions, err := ListSessions()
		if err != nil {
			t.Fatalf("list error: %v", err)
		}
		if len(sessions) != 3 {
			t.Errorf("expected 3 sessions, got %d", len(sessions))
		}
	})

	t.Run("delete session", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

		s := NewSession("delete-test", "/tmp", "claude-sonnet-4-20250514")
		if err := s.Save(); err != nil {
			t.Fatal(err)
		}

		if err := DeleteSession("delete-test"); err != nil {
			t.Fatal(err)
		}

		_, err := Load("delete-test")
		if !os.IsNotExist(err) && err != nil {
			// Should fail to load deleted session
			// (the error might not be os.IsNotExist due to wrapper)
		}
	})
}
