package permissions

import (
	"testing"

	"github.com/anthropics/claude-code-go/internal/types"
)

func TestIsDangerousPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{".gitconfig", true},
		{".bashrc", true},
		{"/home/user/.zshrc", true},
		{".git/config", true},
		{".vscode/settings.json", true},
		{"src/main.go", false},
		{"README.md", false},
		{"/tmp/safe.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := IsDangerousPath(tt.path)
			if result != tt.expected {
				t.Errorf("IsDangerousPath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestChecker(t *testing.T) {
	t.Run("bypass mode allows everything", func(t *testing.T) {
		c := NewChecker(types.PermissionModeBypass)
		result := c.CheckToolPermission("Bash", "/etc/passwd")
		if result != types.PermissionAllow {
			t.Errorf("expected allow in bypass mode, got %s", result)
		}
	})

	t.Run("deny rules take precedence", func(t *testing.T) {
		c := NewChecker(types.PermissionModeDefault)
		c.DenyRules = []types.ToolPermissionRule{
			{ToolName: "Bash", Decision: types.PermissionDeny},
		}
		result := c.CheckToolPermission("Bash", "")
		if result != types.PermissionDeny {
			t.Errorf("expected deny, got %s", result)
		}
	})

	t.Run("allow rules override default", func(t *testing.T) {
		c := NewChecker(types.PermissionModeDefault)
		c.AllowRules = []types.ToolPermissionRule{
			{ToolName: "Write", Decision: types.PermissionAllow},
		}
		result := c.CheckToolPermission("Write", "test.txt")
		if result != types.PermissionAllow {
			t.Errorf("expected allow, got %s", result)
		}
	})
}
