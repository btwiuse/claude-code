package constants

import "testing"

func TestGetClaudeAIBaseURL(t *testing.T) {
	tests := []struct {
		name       string
		sessionID  string
		ingressURL string
		expected   string
	}{
		{"production", "session_abc", "", ClaudeAIBaseURL},
		{"staging session", "session_staging_abc", "", ClaudeAIStagingBaseURL},
		{"staging ingress", "", "https://staging.example.com", ClaudeAIStagingBaseURL},
		{"local session", "session_local_abc", "", ClaudeAILocalBaseURL},
		{"local ingress", "", "http://localhost:3000", ClaudeAILocalBaseURL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetClaudeAIBaseURL(tt.sessionID, tt.ingressURL)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetRemoteSessionURL(t *testing.T) {
	url := GetRemoteSessionURL("session_123", "")
	expected := ClaudeAIBaseURL + "/code/session_123"
	if url != expected {
		t.Errorf("expected %s, got %s", expected, url)
	}
}
