// Package session manages conversation sessions: persistence to JSONL,
// resumption from disk, and listing available sessions.
//
// Sessions are stored under ~/.claude/sessions/{project_hash}/{session_id}.jsonl
package session

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/btwiuse/claude-code/config"
	"github.com/google/uuid"
)

// Entry represents a single line in a session JSONL file.
type Entry struct {
	// Discriminator
	Type string `json:"type"` // "user", "assistant", "system", "summary", "custom-title", "ai-title", "last-prompt", "tag", etc.

	// Message fields
	UUID       string `json:"uuid,omitempty"`
	ParentUUID string `json:"parentUuid,omitempty"`
	SessionID  string `json:"sessionId,omitempty"`
	Timestamp  string `json:"timestamp,omitempty"`
	CWD        string `json:"cwd,omitempty"`
	GitBranch  string `json:"gitBranch,omitempty"`
	Version    string `json:"version,omitempty"`
	UserType   string `json:"userType,omitempty"`

	// Content can be a string or structured content blocks
	Content json.RawMessage `json:"content,omitempty"`

	// Metadata fields
	Summary     string `json:"summary,omitempty"`
	CustomTitle string `json:"customTitle,omitempty"`
	AiTitle     string `json:"aiTitle,omitempty"`
	LastPrompt  string `json:"lastPrompt,omitempty"`
	Tag         string `json:"tag,omitempty"`

	// Role for Anthropic API messages
	Role string `json:"role,omitempty"`

	// Sidechain flag (subagent transcripts)
	IsSidechain bool `json:"isSidechain,omitempty"`
}

// Message is a simplified message for API communication.
type Message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// Info holds metadata about a session for listing purposes.
type Info struct {
	SessionID    string `json:"sessionId"`
	Summary      string `json:"summary,omitempty"`
	LastModified int64  `json:"lastModified"` // Unix ms
	FileSize     int64  `json:"fileSize,omitempty"`
	CustomTitle  string `json:"customTitle,omitempty"`
	FirstPrompt  string `json:"firstPrompt,omitempty"`
	GitBranch    string `json:"gitBranch,omitempty"`
	CWD          string `json:"cwd,omitempty"`
	Tag          string `json:"tag,omitempty"`
	CreatedAt    int64  `json:"createdAt,omitempty"` // Unix ms
}

// Session represents an active conversation session.
type Session struct {
	ID          string
	ProjectHash string
	Entries     []Entry
	Messages    []Message // Reconstructed conversation for API
	file        *os.File
}

// New creates a new session for the given project directory.
func New(projectDir string) (*Session, error) {
	id := uuid.New().String()
	hash := projectHash(projectDir)

	s := &Session{
		ID:          id,
		ProjectHash: hash,
	}

	dir, err := sessionDir(hash)
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dir, id+".jsonl")
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	s.file = f

	return s, nil
}

// Resume loads an existing session by ID, optionally scoped to a project directory.
func Resume(sessionID string, projectDir string) (*Session, error) {
	hash := ""
	if projectDir != "" {
		hash = projectHash(projectDir)
	}

	path, err := findSessionFile(sessionID, hash)
	if err != nil {
		return nil, fmt.Errorf("session %s not found: %w", sessionID, err)
	}

	entries, err := loadEntries(path)
	if err != nil {
		return nil, fmt.Errorf("loading session %s: %w", sessionID, err)
	}

	s := &Session{
		ID:          sessionID,
		ProjectHash: hash,
		Entries:     entries,
	}

	// Reconstruct messages for API from non-sidechain transcript entries
	s.Messages = reconstructMessages(entries)

	// Open file for appending
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	s.file = f

	return s, nil
}

// Append adds an entry to the session and persists it to disk.
func (s *Session) Append(entry Entry) error {
	entry.SessionID = s.ID
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	if s.file != nil {
		if _, err := s.file.Write(append(data, '\n')); err != nil {
			return err
		}
	}

	s.Entries = append(s.Entries, entry)

	// Update messages if this is a transcript message
	if entry.Type == "user" || entry.Type == "assistant" {
		if !entry.IsSidechain {
			s.Messages = append(s.Messages, Message{
				Role:    entry.Role,
				Content: entry.Content,
			})
		}
	}

	return nil
}

// Close closes the session file.
func (s *Session) Close() error {
	if s.file != nil {
		return s.file.Close()
	}
	return nil
}

// List returns metadata for all sessions, optionally filtered by project directory.
func List(projectDir string, limit int) ([]Info, error) {
	claudeHome, err := config.ClaudeHomeDir()
	if err != nil {
		return nil, err
	}
	sessionsRoot := filepath.Join(claudeHome, "sessions")

	if _, err := os.Stat(sessionsRoot); os.IsNotExist(err) {
		return nil, nil
	}

	var infos []Info

	if projectDir != "" {
		hash := projectHash(projectDir)
		dir := filepath.Join(sessionsRoot, hash)
		dirInfos, err := listSessionsInDir(dir)
		if err != nil {
			return nil, err
		}
		infos = append(infos, dirInfos...)
	} else {
		// Scan all project directories
		entries, err := os.ReadDir(sessionsRoot)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(sessionsRoot, e.Name())
			dirInfos, err := listSessionsInDir(dir)
			if err != nil {
				continue
			}
			infos = append(infos, dirInfos...)
		}
	}

	// Sort by last modified (newest first)
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].LastModified > infos[j].LastModified
	})

	if limit > 0 && len(infos) > limit {
		infos = infos[:limit]
	}

	return infos, nil
}

// Exists checks whether a session file exists for the given ID.
func Exists(sessionID string) bool {
	_, err := findSessionFile(sessionID, "")
	return err == nil
}

// listSessionsInDir reads session metadata from a single project hash directory.
func listSessionsInDir(dir string) ([]Info, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var infos []Info
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}

		sessionID := strings.TrimSuffix(e.Name(), ".jsonl")
		path := filepath.Join(dir, e.Name())

		info, err := parseSessionInfo(path, sessionID)
		if err != nil {
			continue
		}
		infos = append(infos, *info)
	}

	return infos, nil
}

// parseSessionInfo reads head and tail of a session file to extract metadata.
func parseSessionInfo(path string, sessionID string) (*Info, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info := &Info{
		SessionID:    sessionID,
		LastModified: fi.ModTime().UnixMilli(),
		FileSize:     fi.Size(),
	}

	// Read first line for creation info
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer
	if scanner.Scan() {
		var first Entry
		if err := json.Unmarshal(scanner.Bytes(), &first); err == nil {
			info.CWD = first.CWD
			info.GitBranch = first.GitBranch
			if first.Timestamp != "" {
				if t, err := time.Parse(time.RFC3339Nano, first.Timestamp); err == nil {
					info.CreatedAt = t.UnixMilli()
				}
			}
			// Extract first prompt from user messages
			if first.Type == "user" {
				info.FirstPrompt = extractTextContent(first.Content)
			}
		}
	}

	// Read remaining lines for tail metadata
	var lastEntry Entry
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err == nil {
			lastEntry = e

			// Track first user prompt if not found yet
			if info.FirstPrompt == "" && e.Type == "user" {
				info.FirstPrompt = extractTextContent(e.Content)
			}

			// Update metadata from tail entries
			switch e.Type {
			case "summary":
				info.Summary = e.Summary
			case "custom-title":
				info.CustomTitle = e.CustomTitle
			case "ai-title":
				if info.Summary == "" {
					info.Summary = e.AiTitle
				}
			case "tag":
				info.Tag = e.Tag
			case "last-prompt":
				// Keep track but don't override firstPrompt
			}
		}
	}

	// Use last entry's summary if available and no dedicated summary entry
	if info.Summary == "" && lastEntry.Summary != "" {
		info.Summary = lastEntry.Summary
	}

	// Skip empty or metadata-only sessions
	if info.FirstPrompt == "" && info.Summary == "" && info.CustomTitle == "" {
		return nil, fmt.Errorf("empty session")
	}

	return info, nil
}

// extractTextContent attempts to extract plain text from message content.
func extractTextContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	// Try as plain string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if len(s) > 200 {
			return s[:200] + "..."
		}
		return s
	}

	// Try as array of content blocks
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				if len(b.Text) > 200 {
					return b.Text[:200] + "..."
				}
				return b.Text
			}
		}
	}

	return ""
}

// reconstructMessages builds API messages from session entries.
func reconstructMessages(entries []Entry) []Message {
	var msgs []Message
	for _, e := range entries {
		if e.IsSidechain {
			continue
		}
		switch e.Type {
		case "user":
			msgs = append(msgs, Message{
				Role:    "user",
				Content: e.Content,
			})
		case "assistant":
			msgs = append(msgs, Message{
				Role:    "assistant",
				Content: e.Content,
			})
		}
	}
	return msgs
}

// loadEntries reads all entries from a session JSONL file.
func loadEntries(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue // Skip malformed lines
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// sessionDir returns the directory for sessions of a given project hash.
func sessionDir(hash string) (string, error) {
	claudeHome, err := config.ClaudeHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(claudeHome, "sessions", hash)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// findSessionFile locates a session file by ID, optionally scoped to a project hash.
func findSessionFile(sessionID string, projectHash string) (string, error) {
	claudeHome, err := config.ClaudeHomeDir()
	if err != nil {
		return "", err
	}
	sessionsRoot := filepath.Join(claudeHome, "sessions")

	if projectHash != "" {
		path := filepath.Join(sessionsRoot, projectHash, sessionID+".jsonl")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Search all project directories
	entries, err := os.ReadDir(sessionsRoot)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(sessionsRoot, e.Name(), sessionID+".jsonl")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("session file not found: %s", sessionID)
}

// projectHash returns a stable hash for a project directory path.
func projectHash(dir string) string {
	// Resolve symlinks for consistency
	resolved, err := filepath.EvalSymlinks(dir)
	if err == nil {
		dir = resolved
	}
	h := sha256.Sum256([]byte(dir))
	return hex.EncodeToString(h[:8]) // First 8 bytes = 16 hex chars
}

// AppendUserMessage is a convenience to append a user message entry.
func (s *Session) AppendUserMessage(text string, cwd string) error {
	content, _ := json.Marshal(text)
	return s.Append(Entry{
		Type:    "user",
		Role:    "user",
		UUID:    uuid.New().String(),
		Content: content,
		CWD:     cwd,
	})
}

// AppendAssistantMessage is a convenience to append an assistant message entry.
func (s *Session) AppendAssistantMessage(content json.RawMessage, cwd string) error {
	return s.Append(Entry{
		Type:    "assistant",
		Role:    "assistant",
		UUID:    uuid.New().String(),
		Content: content,
		CWD:     cwd,
	})
}

// SetTitle sets a custom title on the session.
func (s *Session) SetTitle(title string) error {
	return s.Append(Entry{
		Type:        "custom-title",
		CustomTitle: title,
	})
}

// SetSummary sets an AI-generated summary on the session.
func (s *Session) SetSummary(summary string) error {
	return s.Append(Entry{
		Type:    "summary",
		Summary: summary,
	})
}

// FilePath returns the path to this session's JSONL file.
func (s *Session) FilePath() string {
	claudeHome, err := config.ClaudeHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(claudeHome, "sessions", s.ProjectHash, s.ID+".jsonl")
}

// GetMessages returns the full message slice suitable for resuming a conversation.
// It filters out duplicate tool results to prevent the API from receiving bad input.
func (s *Session) GetMessages() []Message {
	return s.Messages
}

// LastNMessages returns at most the last n messages from the conversation.
func (s *Session) LastNMessages(n int) []Message {
	if len(s.Messages) <= n {
		return s.Messages
	}
	start := len(s.Messages) - n
	// Ensure we start with a user message
	for start < len(s.Messages) && s.Messages[start].Role != "user" {
		start++
	}
	if start >= len(s.Messages) {
		return s.Messages
	}
	return s.Messages[start:]
}

// Tail reads the last n bytes of a file, used for fast metadata extraction.
func Tail(path string, n int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := fi.Size()
	if size <= n {
		return io.ReadAll(f)
	}

	buf := make([]byte, n)
	_, err = f.ReadAt(buf, size-n)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return buf, nil
}
