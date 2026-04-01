package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/btwiuse/claude-code/config"
)

func setupTestDir(t *testing.T) func() {
	t.Helper()
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", dir)

	sessDir := filepath.Join(dir, ".claude", "sessions")
	if err := os.MkdirAll(sessDir, 0o700); err != nil {
		t.Fatal(err)
	}

	return func() {
		os.Setenv("HOME", origHome)
	}
}

func TestNewSession(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	sess, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer sess.Close()

	if sess.ID == "" {
		t.Error("expected non-empty session ID")
	}

	// Session file should exist.
	if _, err := os.Stat(sess.Path); os.IsNotExist(err) {
		t.Errorf("session file not created: %s", sess.Path)
	}
}

func TestAppendAndResume(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	// Create session and write messages.
	sess, err := New()
	if err != nil {
		t.Fatalf("creating session: %v", err)
	}

	if err := sess.AppendUser("Hello"); err != nil {
		t.Fatalf("appending user msg: %v", err)
	}

	assistantMsg, _ := json.Marshal(map[string]string{
		"role":    "assistant",
		"content": "Hi there!",
	})
	if err := sess.AppendAssistant(assistantMsg); err != nil {
		t.Fatalf("appending assistant msg: %v", err)
	}

	if err := sess.SetTitle("Test Session"); err != nil {
		t.Fatalf("setting title: %v", err)
	}
	sess.Close()

	// Resume the session.
	resumed, entries, err := Resume(sess.ID)
	if err != nil {
		t.Fatalf("resuming session: %v", err)
	}
	defer resumed.Close()

	if resumed.ID != sess.ID {
		t.Errorf("expected ID %s, got %s", sess.ID, resumed.ID)
	}

	if resumed.Title != "Test Session" {
		t.Errorf("expected title 'Test Session', got %q", resumed.Title)
	}

	if len(entries) != 3 { // user + assistant + custom-title
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
}

func TestListSessions(t *testing.T) {
	cleanup := setupTestDir(t)
	defer cleanup()

	// Create two sessions.
	s1, err := New()
	if err != nil {
		t.Fatal(err)
	}
	_ = s1.AppendUser("First session")
	s1.Close()

	s2, err := New()
	if err != nil {
		t.Fatal(err)
	}
	_ = s2.AppendUser("Second session")
	s2.Close()

	sessions, err := List()
	if err != nil {
		t.Fatalf("listing sessions: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestExtractMessages(t *testing.T) {
	userMsg, _ := json.Marshal(UserMessage{Role: "user", Content: "Hello"})
	assistantContent, _ := json.Marshal("Hi there!")
	assistantMsg, _ := json.Marshal(AssistantMessage{Role: "assistant", Content: assistantContent})

	entries := []Entry{
		{Type: "user", UUID: "1", Message: userMsg},
		{Type: "assistant", UUID: "2", Message: assistantMsg},
	}

	pairs := ExtractMessages(entries)
	if len(pairs) != 2 {
		t.Fatalf("expected 2 message pairs, got %d", len(pairs))
	}

	if pairs[0].Role != "user" || pairs[0].Content != "Hello" {
		t.Errorf("unexpected first pair: %+v", pairs[0])
	}

	if pairs[1].Role != "assistant" || pairs[1].Content != "Hi there!" {
		t.Errorf("unexpected second pair: %+v", pairs[1])
	}
}

func TestSessionsDir(t *testing.T) {
	dir := config.SessionsDir()
	if dir == "" {
		t.Error("expected non-empty sessions dir")
	}
}
