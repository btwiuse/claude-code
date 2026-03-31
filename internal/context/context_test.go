package appcontext

import (
	"testing"
)

func TestGetUserContext(t *testing.T) {
	ctx := GetUserContext()
	if ctx["date"] == "" {
		t.Error("expected date in user context")
	}
	if ctx["time"] == "" {
		t.Error("expected time in user context")
	}
}

func TestGetSystemContext(t *testing.T) {
	ctx := GetSystemContext()
	if ctx["platform"] == "" {
		// platform might be empty in some environments, but the key should exist
		if _, ok := ctx["platform"]; !ok {
			t.Error("expected platform key in system context")
		}
	}
	if _, ok := ctx["shell"]; !ok {
		t.Error("expected shell key in system context")
	}
}

func TestResetGitStatus(t *testing.T) {
	// Just verify it doesn't panic
	ResetGitStatus()
	gs := GetGitStatus()
	if gs == nil {
		t.Error("expected non-nil git status")
	}
}
