package session

import (
	"bufio"
	"encoding/json"
	"fmt"
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
	Type        string          `json:"type"`
	UUID        string          `json:"uuid,omitempty"`
	ParentUUID  string          `json:"parentUuid,omitempty"`
	SessionID   string          `json:"sessionId,omitempty"`
	Message     json.RawMessage `json:"message,omitempty"`
	CustomTitle string          `json:"customTitle,omitempty"`
	Prompt      string          `json:"prompt,omitempty"`
}

// UserMessage is the message content for a user entry.
type UserMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AssistantMessage is the message content for an assistant entry.
type AssistantMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// Info holds metadata about a session for listing purposes.
type Info struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Path      string    `json:"path"`
}

// Session manages a single conversation session.
type Session struct {
	ID        string
	Title     string
	Path      string
	CreatedAt time.Time
	file      *os.File
}

// New creates a new session with a fresh UUID.
func New() (*Session, error) {
	id := uuid.New().String()
	return newWithID(id)
}

func newWithID(id string) (*Session, error) {
	dir := config.SessionsDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create sessions dir: %w", err)
	}

	path := filepath.Join(dir, id+".jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open session file: %w", err)
	}

	return &Session{
		ID:        id,
		Path:      path,
		CreatedAt: time.Now(),
		file:      f,
	}, nil
}

// Resume loads an existing session by ID.
func Resume(id string) (*Session, []Entry, error) {
	dir := config.SessionsDir()
	path := filepath.Join(dir, id+".jsonl")

	entries, err := readEntries(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read session: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("reopen session: %w", err)
	}

	s := &Session{
		ID:   id,
		Path: path,
		file: f,
	}

	// Extract title and timestamps from entries.
	for _, e := range entries {
		if e.Type == "custom-title" && e.CustomTitle != "" {
			s.Title = e.CustomTitle
		}
	}
	if len(entries) > 0 {
		s.CreatedAt = fileModTime(path)
	}

	return s, entries, nil
}

// Append writes an entry to the session JSONL file.
func (s *Session) Append(entry Entry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = s.file.Write(append(data, '\n'))
	return err
}

// AppendUser writes a user message entry.
func (s *Session) AppendUser(content string) error {
	msg, _ := json.Marshal(UserMessage{Role: "user", Content: content})
	return s.Append(Entry{
		Type:    "user",
		UUID:    uuid.New().String(),
		Message: msg,
	})
}

// AppendAssistant writes an assistant message entry.
func (s *Session) AppendAssistant(content json.RawMessage) error {
	return s.Append(Entry{
		Type:    "assistant",
		UUID:    uuid.New().String(),
		Message: content,
	})
}

// SetTitle writes a custom-title entry.
func (s *Session) SetTitle(title string) error {
	s.Title = title
	return s.Append(Entry{
		Type:        "custom-title",
		SessionID:   s.ID,
		CustomTitle: title,
	})
}

// Close closes the session file.
func (s *Session) Close() error {
	if s.file != nil {
		return s.file.Close()
	}
	return nil
}

// List returns all sessions sorted by most recent first.
func List() ([]Info, error) {
	dir := config.SessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []Info
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}

		id := strings.TrimSuffix(e.Name(), ".jsonl")
		path := filepath.Join(dir, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}

		si := Info{
			ID:        id,
			Path:      path,
			CreatedAt: info.ModTime(),
			UpdatedAt: info.ModTime(),
		}

		// Try to extract title from tail of file.
		si.Title = extractTitle(path)
		if si.Title == "" {
			si.Title = extractFirstPrompt(path)
		}

		sessions = append(sessions, si)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// readEntries reads all entries from a JSONL file.
func readEntries(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, scanner.Err()
}

func extractTitle(path string) string {
	entries, err := readTail(path, 64*1024)
	if err != nil {
		return ""
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Type == "custom-title" && entries[i].CustomTitle != "" {
			return entries[i].CustomTitle
		}
	}
	return ""
}

func extractFirstPrompt(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		if e.Type == "user" && len(e.Message) > 0 {
			var msg UserMessage
			if err := json.Unmarshal(e.Message, &msg); err == nil && msg.Content != "" {
				title := msg.Content
				if len(title) > 80 {
					title = title[:77] + "..."
				}
				return title
			}
		}
	}
	return "(empty session)"
}

// readTail reads the last n bytes of a file and parses JSONL entries.
func readTail(path string, n int64) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	offset := stat.Size() - n
	if offset < 0 {
		offset = 0
	}
	if _, err := f.Seek(offset, 0); err != nil {
		return nil, err
	}

	var entries []Entry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	first := offset > 0
	for scanner.Scan() {
		if first {
			first = false
			continue // skip partial line
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func fileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Now()
	}
	return info.ModTime()
}

// MessagesFromEntries converts session entries into Anthropic-compatible message params.
// Returns pairs of (role, content) for rebuilding the conversation.
type MessagePair struct {
	Role    string
	Content string
}

// ExtractMessages extracts user/assistant message pairs from entries.
func ExtractMessages(entries []Entry) []MessagePair {
	var pairs []MessagePair
	for _, e := range entries {
		switch e.Type {
		case "user":
			var msg UserMessage
			if err := json.Unmarshal(e.Message, &msg); err == nil {
				pairs = append(pairs, MessagePair{Role: "user", Content: msg.Content})
			}
		case "assistant":
			var msg AssistantMessage
			if err := json.Unmarshal(e.Message, &msg); err == nil {
				// Try to extract text content from the assistant message.
				var text string
				// Content might be a string or an array of blocks.
				if err := json.Unmarshal(msg.Content, &text); err != nil {
					// Try as array of content blocks.
					var blocks []json.RawMessage
					if err := json.Unmarshal(msg.Content, &blocks); err == nil {
						for _, b := range blocks {
							var block struct {
								Type string `json:"type"`
								Text string `json:"text"`
							}
							if err := json.Unmarshal(b, &block); err == nil && block.Type == "text" {
								text += block.Text
							}
						}
					}
				}
				if text != "" {
					pairs = append(pairs, MessagePair{Role: "assistant", Content: text})
				}
			}
		}
	}
	return pairs
}
