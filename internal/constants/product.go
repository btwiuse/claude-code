// Package constants defines product-wide constants for Claude Code.
package constants

import "strings"

const (
	// ProductName is the display name of the product.
	ProductName = "Claude Code"

	// ProductURL is the canonical product page URL.
	ProductURL = "https://claude.com/claude-code"

	// ClaudeAIBaseURL is the production Claude AI URL.
	ClaudeAIBaseURL = "https://claude.ai"

	// ClaudeAIStagingBaseURL is the staging Claude AI URL.
	ClaudeAIStagingBaseURL = "https://claude-ai.staging.ant.dev"

	// ClaudeAILocalBaseURL is the local development Claude AI URL.
	ClaudeAILocalBaseURL = "http://localhost:4000"

	// DefaultModel is the default Claude model to use.
	DefaultModel = "claude-sonnet-4-20250514"

	// DefaultMaxTokens is the default maximum output tokens.
	DefaultMaxTokens = 16384

	// APITimeoutSeconds is the default API timeout.
	APITimeoutSeconds = 600

	// MaxHistoryItems is the maximum number of history entries to retain.
	MaxHistoryItems = 100

	// Version is the application version (set at build time via ldflags).
	Version = "0.1.0-go"
)

// IsRemoteSessionStaging checks if a session is targeting the staging environment.
func IsRemoteSessionStaging(sessionID, ingressURL string) bool {
	return strings.Contains(sessionID, "_staging_") || strings.Contains(ingressURL, "staging")
}

// IsRemoteSessionLocal checks if a session is targeting local development.
func IsRemoteSessionLocal(sessionID, ingressURL string) bool {
	return strings.Contains(sessionID, "_local_") || strings.Contains(ingressURL, "localhost")
}

// GetClaudeAIBaseURL returns the appropriate base URL based on session environment.
func GetClaudeAIBaseURL(sessionID, ingressURL string) string {
	if IsRemoteSessionLocal(sessionID, ingressURL) {
		return ClaudeAILocalBaseURL
	}
	if IsRemoteSessionStaging(sessionID, ingressURL) {
		return ClaudeAIStagingBaseURL
	}
	return ClaudeAIBaseURL
}

// GetRemoteSessionURL returns the full URL for a remote session.
func GetRemoteSessionURL(sessionID, ingressURL string) string {
	baseURL := GetClaudeAIBaseURL(sessionID, ingressURL)
	return baseURL + "/code/" + sessionID
}
