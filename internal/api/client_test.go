package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anthropics/claude-code-go/internal/types"
)

func TestNewClient(t *testing.T) {
	t.Run("missing api key", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "")
		t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
		_, err := NewClient()
		if err == nil {
			t.Error("expected error for missing API key")
		}
	})

	t.Run("with api key option", func(t *testing.T) {
		client, err := NewClient(WithAPIKey("test-key"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if client.apiKey != "test-key" {
			t.Errorf("expected test-key, got %s", client.apiKey)
		}
	})

	t.Run("with model option", func(t *testing.T) {
		client, err := NewClient(WithAPIKey("key"), WithModel("claude-opus-4-20250514"))
		if err != nil {
			t.Fatal(err)
		}
		if client.GetModel() != "claude-opus-4-20250514" {
			t.Errorf("expected claude-opus-4-20250514, got %s", client.GetModel())
		}
	})
}

func TestAPIError(t *testing.T) {
	t.Run("overloaded", func(t *testing.T) {
		err := &APIError{StatusCode: 529}
		if !err.IsOverloaded() {
			t.Error("expected overloaded")
		}
	})

	t.Run("rate limited", func(t *testing.T) {
		err := &APIError{StatusCode: 429}
		if !err.IsRateLimited() {
			t.Error("expected rate limited")
		}
	})

	t.Run("auth error", func(t *testing.T) {
		err := &APIError{StatusCode: 401}
		if !err.IsAuthError() {
			t.Error("expected auth error")
		}
	})
}

func TestCreateMessage(t *testing.T) {
	t.Run("successful non-streaming request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify headers
			if r.Header.Get("X-API-Key") != "test-key" {
				t.Error("missing API key header")
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Error("missing content-type header")
			}

			resp := types.APIResponse{
				ID:   "msg_123",
				Type: "message",
				Role: types.RoleAssistant,
				Content: []types.ContentBlock{
					{Type: types.ContentTypeText, Text: "Hello! How can I help?"},
				},
				Model:      "claude-sonnet-4-20250514",
				StopReason: "end_turn",
				Usage:      &types.Usage{InputTokens: 10, OutputTokens: 20},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient(WithAPIKey("test-key"), WithBaseURL(server.URL))
		if err != nil {
			t.Fatal(err)
		}

		resp, err := client.CreateMessage(t.Context(), &CreateMessageRequest{
			Messages: []types.Message{
				{
					Role: types.RoleUser,
					Content: []types.ContentBlock{
						{Type: types.ContentTypeText, Text: "Hello"},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.ID != "msg_123" {
			t.Errorf("expected msg_123, got %s", resp.ID)
		}
		if len(resp.Content) != 1 || resp.Content[0].Text != "Hello! How can I help?" {
			t.Errorf("unexpected response content: %+v", resp.Content)
		}
	})

	t.Run("API error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"type": "error",
				"error": map[string]string{
					"type":    "authentication_error",
					"message": "invalid api key",
				},
			})
		}))
		defer server.Close()

		client, err := NewClient(WithAPIKey("bad-key"), WithBaseURL(server.URL))
		if err != nil {
			t.Fatal(err)
		}

		_, err = client.CreateMessage(t.Context(), &CreateMessageRequest{
			Messages: []types.Message{
				{
					Role: types.RoleUser,
					Content: []types.ContentBlock{
						{Type: types.ContentTypeText, Text: "Hello"},
					},
				},
			},
		})
		if err == nil {
			t.Fatal("expected error")
		}

		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected APIError, got %T", err)
		}
		if !apiErr.IsAuthError() {
			t.Error("expected auth error")
		}
	})
}

func TestStreamMessage(t *testing.T) {
	t.Run("successful streaming request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)

			events := []string{
				`event: message_start` + "\n" + `data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-20250514","usage":{"input_tokens":10}}}` + "\n\n",
				`event: content_block_start` + "\n" + `data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\n",
				`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}` + "\n\n",
				`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}` + "\n\n",
				`event: content_block_stop` + "\n" + `data: {"type":"content_block_stop","index":0}` + "\n\n",
				`event: message_delta` + "\n" + `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}` + "\n\n",
				`event: message_stop` + "\n" + `data: {"type":"message_stop"}` + "\n\n",
			}

			flusher, _ := w.(http.Flusher)
			for _, event := range events {
				w.Write([]byte(event))
				if flusher != nil {
					flusher.Flush()
				}
			}
		}))
		defer server.Close()

		client, err := NewClient(WithAPIKey("test-key"), WithBaseURL(server.URL))
		if err != nil {
			t.Fatal(err)
		}

		stream, err := client.StreamMessage(t.Context(), &CreateMessageRequest{
			Messages: []types.Message{
				{
					Role: types.RoleUser,
					Content: []types.ContentBlock{
						{Type: types.ContentTypeText, Text: "Hello"},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var events []types.StreamEvent
		for result := range stream {
			if result.Error != nil {
				t.Fatalf("stream error: %v", result.Error)
			}
			if result.Event != nil {
				events = append(events, *result.Event)
			}
		}

		if len(events) < 4 {
			t.Errorf("expected at least 4 events, got %d", len(events))
		}

		// Verify we got expected event types
		hasMessageStart := false
		hasContentDelta := false
		for _, e := range events {
			if e.Type == "message_start" {
				hasMessageStart = true
			}
			if e.Type == "content_block_delta" {
				hasContentDelta = true
			}
		}
		if !hasMessageStart {
			t.Error("missing message_start event")
		}
		if !hasContentDelta {
			t.Error("missing content_block_delta event")
		}
	})
}
