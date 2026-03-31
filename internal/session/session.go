// Package session manages Claude Code conversation sessions.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/anthropics/claude-code-go/internal/config"
	"github.com/anthropics/claude-code-go/internal/types"
)

// Session represents an active conversation session.
type Session struct {
	ID        string          `json:"id"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	CWD       string          `json:"cwd"`
	Model     string          `json:"model"`
	Messages  []types.Message `json:"messages"`
}

// NewSession creates a new session with the given ID.
func NewSession(id, cwd, model string) *Session {
	now := time.Now()
	return &Session{
		ID:        id,
		CreatedAt: now,
		UpdatedAt: now,
		CWD:       cwd,
		Model:     model,
		Messages:  []types.Message{},
	}
}

// AddMessage appends a message to the session.
func (s *Session) AddMessage(msg types.Message) {
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
}

// sessionDir returns the directory for a session.
func sessionDir(id string) string {
	return filepath.Join(config.SessionsDir(), id)
}

// sessionFilePath returns the file path for a session.
func sessionFilePath(id string) string {
	return filepath.Join(sessionDir(id), "session.json")
}

// Save persists the session to disk.
func (s *Session) Save() error {
	dir := sessionDir(s.ID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating session directory: %w", err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling session: %w", err)
	}

	return os.WriteFile(sessionFilePath(s.ID), data, 0600)
}

// Load reads a session from disk.
func Load(id string) (*Session, error) {
	data, err := os.ReadFile(sessionFilePath(id))
	if err != nil {
		return nil, fmt.Errorf("reading session: %w", err)
	}

	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing session: %w", err)
	}
	return &s, nil
}

// ListSessions returns all available session IDs, sorted by most recent first.
func ListSessions() ([]types.SessionInfo, error) {
	sessDir := config.SessionsDir()
	entries, err := os.ReadDir(sessDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading sessions directory: %w", err)
	}

	var sessions []types.SessionInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		id := entry.Name()
		s, err := Load(id)
		if err != nil {
			continue
		}

		sessions = append(sessions, types.SessionInfo{
			ID:        s.ID,
			CreatedAt: s.CreatedAt,
			UpdatedAt: s.UpdatedAt,
			CWD:       s.CWD,
			Model:     s.Model,
		})
	}

	// Sort by most recent first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// DeleteSession removes a session from disk.
func DeleteSession(id string) error {
	return os.RemoveAll(sessionDir(id))
}
