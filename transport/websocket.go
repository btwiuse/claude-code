package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Message represents an NDJSON message over the WebSocket transport.
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// WebSocketTransport provides bidirectional NDJSON streaming over WebSocket.
type WebSocketTransport struct {
	url     string
	headers http.Header
	conn    *websocket.Conn

	incoming chan Message
	done     chan struct{}
	once     sync.Once

	mu       sync.Mutex
	closed   bool
	onClose  func()
}

// New creates a new WebSocket transport for the given SDK URL.
func New(sdkURL string) *WebSocketTransport {
	return &WebSocketTransport{
		url:      sdkURL,
		headers:  make(http.Header),
		incoming: make(chan Message, 100),
		done:     make(chan struct{}),
	}
}

// SetHeader sets a header for the WebSocket connection.
func (t *WebSocketTransport) SetHeader(key, value string) {
	t.headers.Set(key, value)
}

// OnClose sets a callback to be called when the connection closes.
func (t *WebSocketTransport) OnClose(fn func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onClose = fn
}

// Connect establishes the WebSocket connection.
func (t *WebSocketTransport) Connect(ctx context.Context) error {
	dialCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(dialCtx, t.url, &websocket.DialOptions{
		HTTPHeader: t.headers,
	})
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}
	t.conn = conn

	// Start reading in background.
	go t.readLoop(ctx)

	return nil
}

// Send writes an NDJSON message to the WebSocket.
func (t *WebSocketTransport) Send(ctx context.Context, msg Message) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return fmt.Errorf("transport closed")
	}
	conn := t.conn
	t.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return conn.Write(ctx, websocket.MessageText, data)
}

// Receive returns the next incoming message.
func (t *WebSocketTransport) Receive(ctx context.Context) (Message, error) {
	select {
	case msg, ok := <-t.incoming:
		if !ok {
			return Message{}, io.EOF
		}
		return msg, nil
	case <-ctx.Done():
		return Message{}, ctx.Err()
	case <-t.done:
		return Message{}, io.EOF
	}
}

// Close closes the WebSocket connection.
func (t *WebSocketTransport) Close() error {
	t.once.Do(func() {
		t.mu.Lock()
		t.closed = true
		onClose := t.onClose
		t.mu.Unlock()

		close(t.done)

		if t.conn != nil {
			t.conn.Close(websocket.StatusNormalClosure, "closing")
		}

		if onClose != nil {
			onClose()
		}
	})
	return nil
}

func (t *WebSocketTransport) readLoop(ctx context.Context) {
	defer close(t.incoming)
	defer t.Close()

	for {
		_, data, err := t.conn.Read(ctx)
		if err != nil {
			return
		}

		// Parse NDJSON - may contain multiple messages.
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var msg Message
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				continue
			}
			select {
			case t.incoming <- msg:
			case <-ctx.Done():
				return
			}
		}
	}
}
