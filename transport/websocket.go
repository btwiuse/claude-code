// Package transport provides WebSocket transport for --sdk-url mode,
// using github.com/coder/websocket.
package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// State represents the transport connection state.
type State string

const (
	StateIdle         State = "idle"
	StateConnected    State = "connected"
	StateReconnecting State = "reconnecting"
	StateClosing      State = "closing"
	StateClosed       State = "closed"
)

// WSMessage represents a message sent/received over WebSocket.
type WSMessage struct {
	ID   string          `json:"id,omitempty"`
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Transport manages a WebSocket connection to an SDK endpoint.
type Transport struct {
	url     string
	headers http.Header

	conn  *websocket.Conn
	state State
	mu    sync.Mutex

	// Callbacks
	onMessage func(WSMessage)
	onError   func(error)
	onClose   func()

	// Configuration
	pingInterval    time.Duration
	keepalive       time.Duration
	reconnectDelay  time.Duration
	maxReconnect    time.Duration
	giveUpDuration  time.Duration

	// Control
	cancel context.CancelFunc
	done   chan struct{}
}

// Option configures a Transport.
type Option func(*Transport)

// WithHeaders sets custom HTTP headers for the WebSocket connection.
func WithHeaders(h http.Header) Option {
	return func(t *Transport) {
		t.headers = h
	}
}

// WithOnMessage sets the message handler callback.
func WithOnMessage(fn func(WSMessage)) Option {
	return func(t *Transport) {
		t.onMessage = fn
	}
}

// WithOnError sets the error handler callback.
func WithOnError(fn func(error)) Option {
	return func(t *Transport) {
		t.onError = fn
	}
}

// WithOnClose sets the close handler callback.
func WithOnClose(fn func()) Option {
	return func(t *Transport) {
		t.onClose = fn
	}
}

// New creates a new WebSocket transport for the given URL.
func New(url string, opts ...Option) *Transport {
	t := &Transport{
		url:            url,
		state:          StateIdle,
		pingInterval:   10 * time.Second,
		keepalive:      5 * time.Minute,
		reconnectDelay: 1 * time.Second,
		maxReconnect:   30 * time.Second,
		giveUpDuration: 10 * time.Minute,
		done:           make(chan struct{}),
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// Connect establishes the WebSocket connection.
func (t *Transport) Connect(ctx context.Context) error {
	t.mu.Lock()
	if t.state != StateIdle && t.state != StateClosed {
		t.mu.Unlock()
		return fmt.Errorf("transport already in state %s", t.state)
	}
	t.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	err := t.dial(ctx)
	if err != nil {
		cancel()
		return err
	}

	// Start read loop
	go t.readLoop(ctx)

	// Start keepalive
	go t.keepaliveLoop(ctx)

	return nil
}

// Send sends a message over the WebSocket connection.
func (t *Transport) Send(ctx context.Context, msg WSMessage) error {
	t.mu.Lock()
	conn := t.conn
	state := t.state
	t.mu.Unlock()

	if state != StateConnected || conn == nil {
		return fmt.Errorf("transport not connected (state: %s)", state)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	data = append(data, '\n')

	return conn.Write(ctx, websocket.MessageText, data)
}

// Close closes the WebSocket connection.
func (t *Transport) Close() error {
	t.mu.Lock()
	t.state = StateClosing
	conn := t.conn
	t.mu.Unlock()

	if t.cancel != nil {
		t.cancel()
	}

	if conn != nil {
		err := conn.Close(websocket.StatusNormalClosure, "closing")
		t.mu.Lock()
		t.state = StateClosed
		t.conn = nil
		t.mu.Unlock()
		return err
	}

	t.mu.Lock()
	t.state = StateClosed
	t.mu.Unlock()
	return nil
}

// State returns the current transport state.
func (t *Transport) GetState() State {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state
}

// dial establishes a WebSocket connection.
func (t *Transport) dial(ctx context.Context) error {
	opts := &websocket.DialOptions{}
	if t.headers != nil {
		opts.HTTPHeader = t.headers
	}

	conn, _, err := websocket.Dial(ctx, t.url, opts)
	if err != nil {
		return fmt.Errorf("WebSocket dial failed: %w", err)
	}

	// Set read limit to 10MB
	conn.SetReadLimit(10 * 1024 * 1024)

	t.mu.Lock()
	t.conn = conn
	t.state = StateConnected
	t.mu.Unlock()

	return nil
}

// readLoop reads messages from the WebSocket connection.
func (t *Transport) readLoop(ctx context.Context) {
	defer func() {
		t.mu.Lock()
		if t.state == StateConnected {
			t.state = StateClosed
		}
		t.mu.Unlock()
		if t.onClose != nil {
			t.onClose()
		}
		close(t.done)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, data, err := t.conn.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if t.onError != nil {
				t.onError(err)
			}
			// Attempt reconnection
			if t.tryReconnect(ctx) {
				continue
			}
			return
		}

		if t.onMessage == nil {
			continue
		}

		// Parse NDJSON (newline-delimited JSON)
		for _, line := range splitLines(data) {
			if len(line) == 0 {
				continue
			}
			var msg WSMessage
			if err := json.Unmarshal(line, &msg); err != nil {
				continue // Skip malformed messages
			}
			// Skip keepalive messages
			if msg.Type == "keep_alive" {
				continue
			}
			t.onMessage(msg)
		}
	}
}

// keepaliveLoop sends periodic keepalive messages.
func (t *Transport) keepaliveLoop(ctx context.Context) {
	ticker := time.NewTicker(t.keepalive)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.done:
			return
		case <-ticker.C:
			msg := WSMessage{Type: "keep_alive"}
			_ = t.Send(ctx, msg)
		}
	}
}

// tryReconnect attempts to reconnect with exponential backoff.
func (t *Transport) tryReconnect(ctx context.Context) bool {
	t.mu.Lock()
	if t.state == StateClosing || t.state == StateClosed {
		t.mu.Unlock()
		return false
	}
	t.state = StateReconnecting
	t.mu.Unlock()

	delay := t.reconnectDelay
	deadline := time.Now().Add(t.giveUpDuration)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(delay):
		}

		err := t.dial(ctx)
		if err == nil {
			return true
		}

		// Exponential backoff
		delay *= 2
		if delay > t.maxReconnect {
			delay = t.maxReconnect
		}
	}

	t.mu.Lock()
	t.state = StateClosed
	t.mu.Unlock()
	return false
}

// splitLines splits data into individual lines.
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	for len(data) > 0 {
		idx := -1
		for i, b := range data {
			if b == '\n' {
				idx = i
				break
			}
		}
		if idx < 0 {
			if len(data) > 0 {
				lines = append(lines, data)
			}
			break
		}
		line := data[:idx]
		if len(line) > 0 {
			lines = append(lines, line)
		}
		data = data[idx+1:]
	}
	return lines
}

// Wait blocks until the transport is closed.
func (t *Transport) Wait() {
	<-t.done
}
