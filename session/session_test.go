package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewSession(t *testing.T) {
	tmp := t.TempDir()

	// Override home directory for test
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	sess, err := New("/test/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer sess.Close()

	if sess.ID == "" {
		t.Error("expected session ID to be set")
	}
	if sess.ProjectHash == "" {
		t.Error("expected project hash to be set")
	}
}

func TestSessionAppend(t *testing.T) {
	tmp := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	sess, err := New("/test/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer sess.Close()

	// Append a user message
	err = sess.AppendUserMessage("hello world", "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sess.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(sess.Entries))
	}

	entry := sess.Entries[0]
	if entry.Type != "user" {
		t.Errorf("expected type 'user', got %q", entry.Type)
	}
	if entry.SessionID != sess.ID {
		t.Errorf("expected session ID %q, got %q", sess.ID, entry.SessionID)
	}

	// Check messages were updated
	if len(sess.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sess.Messages))
	}
	if sess.Messages[0].Role != "user" {
		t.Errorf("expected role 'user', got %q", sess.Messages[0].Role)
	}
}

func TestSessionAppendAssistant(t *testing.T) {
	tmp := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	sess, err := New("/test/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer sess.Close()

	content, _ := json.Marshal([]map[string]string{
		{"type": "text", "text": "hello"},
	})
	err = sess.AppendAssistantMessage(content, "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sess.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sess.Messages))
	}
	if sess.Messages[0].Role != "assistant" {
		t.Errorf("expected role 'assistant', got %q", sess.Messages[0].Role)
	}
}

func TestSessionResume(t *testing.T) {
	tmp := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	// Create and populate a session
	sess, err := New("/test/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = sess.AppendUserMessage("first message", "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := json.Marshal([]map[string]string{
		{"type": "text", "text": "response"},
	})
	err = sess.AppendAssistantMessage(content, "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sessionID := sess.ID
	sess.Close()

	// Resume the session
	resumed, err := Resume(sessionID, "/test/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resumed.Close()

	if resumed.ID != sessionID {
		t.Errorf("expected session ID %q, got %q", sessionID, resumed.ID)
	}
	if len(resumed.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(resumed.Entries))
	}
	if len(resumed.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(resumed.Messages))
	}
}

func TestSessionList(t *testing.T) {
	tmp := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	// Create two sessions
	sess1, err := New("/test/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = sess1.AppendUserMessage("session one", "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sess1.Close()

	sess2, err := New("/test/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = sess2.AppendUserMessage("session two", "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sess2.Close()

	// List all sessions
	infos, err := List("", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(infos))
	}

	// Verify ordering (newest first)
	if infos[0].LastModified < infos[1].LastModified {
		t.Error("expected sessions sorted by last modified (newest first)")
	}
}

func TestSessionListWithLimit(t *testing.T) {
	tmp := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	// Create three sessions
	for i := 0; i < 3; i++ {
		sess, err := New("/test/project")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		err = sess.AppendUserMessage("session message", "/test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		sess.Close()
	}

	// List with limit
	infos, err := List("", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) != 2 {
		t.Fatalf("expected 2 sessions (limited), got %d", len(infos))
	}
}

func TestSessionExists(t *testing.T) {
	tmp := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	sess, err := New("/test/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = sess.AppendUserMessage("test", "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sess.Close()

	if !Exists(sess.ID) {
		t.Error("expected session to exist")
	}
	if Exists("nonexistent-id") {
		t.Error("expected nonexistent session to not exist")
	}
}

func TestSessionSetTitle(t *testing.T) {
	tmp := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	sess, err := New("/test/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer sess.Close()

	err = sess.AppendUserMessage("hello", "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = sess.SetTitle("My Custom Title")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sess.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(sess.Entries))
	}
	if sess.Entries[1].CustomTitle != "My Custom Title" {
		t.Errorf("expected title 'My Custom Title', got %q", sess.Entries[1].CustomTitle)
	}
}

func TestProjectHash(t *testing.T) {
	h1 := projectHash("/test/project1")
	h2 := projectHash("/test/project2")
	h3 := projectHash("/test/project1")

	if h1 == h2 {
		t.Error("different projects should have different hashes")
	}
	if h1 != h3 {
		t.Error("same project should have same hash")
	}
}

func TestLastNMessages(t *testing.T) {
	tmp := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	sess, err := New("/test/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer sess.Close()

	// Add 6 messages
	for i := 0; i < 3; i++ {
		sess.AppendUserMessage("user msg", "/test")
		content, _ := json.Marshal("assistant msg")
		sess.AppendAssistantMessage(content, "/test")
	}

	msgs := sess.LastNMessages(4)
	if len(msgs) > 4 {
		t.Errorf("expected at most 4 messages, got %d", len(msgs))
	}
	// Should start with a user message
	if len(msgs) > 0 && msgs[0].Role != "user" {
		t.Errorf("expected first message role 'user', got %q", msgs[0].Role)
	}
}

func TestExtractTextContent(t *testing.T) {
	tests := []struct {
		name     string
		input    json.RawMessage
		expected string
	}{
		{
			name:     "plain string",
			input:    json.RawMessage(`"hello world"`),
			expected: "hello world",
		},
		{
			name:     "content blocks",
			input:    json.RawMessage(`[{"type":"text","text":"from blocks"}]`),
			expected: "from blocks",
		},
		{
			name:     "empty",
			input:    json.RawMessage(``),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTextContent(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSessionFilePath(t *testing.T) {
	tmp := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	sess, err := New("/test/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer sess.Close()

	path := sess.FilePath()
	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got %q", path)
	}
	if !contains(path, ".claude/sessions") {
		t.Errorf("expected path to contain '.claude/sessions', got %q", path)
	}
	if !contains(path, sess.ID+".jsonl") {
		t.Errorf("expected path to contain session ID, got %q", path)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
