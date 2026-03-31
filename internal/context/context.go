// Package appcontext provides git and system context for Claude Code sessions.
package appcontext

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// GitStatus contains information about the current git repository.
type GitStatus struct {
	Branch     string
	MainBranch string
	Status     string
	RecentLog  string
	UserName   string
	IsRepo     bool
}

var (
	cachedGitStatus *GitStatus
	gitStatusOnce   sync.Once
)

// GetGitStatus returns the current git repository status, caching the result.
func GetGitStatus() *GitStatus {
	gitStatusOnce.Do(func() {
		cachedGitStatus = fetchGitStatus()
	})
	return cachedGitStatus
}

// ResetGitStatus clears the cached git status, forcing a refresh on next access.
func ResetGitStatus() {
	gitStatusOnce = sync.Once{}
	cachedGitStatus = nil
}

func fetchGitStatus() *GitStatus {
	gs := &GitStatus{}

	// Check if we're in a git repo
	if _, err := runGit("rev-parse", "--is-inside-work-tree"); err != nil {
		return gs
	}
	gs.IsRepo = true

	// Get current branch
	if branch, err := runGit("branch", "--show-current"); err == nil {
		gs.Branch = strings.TrimSpace(branch)
	}

	// Get main branch (try main, then master)
	gs.MainBranch = "main"
	if _, err := runGit("rev-parse", "--verify", "main"); err != nil {
		if _, err := runGit("rev-parse", "--verify", "master"); err == nil {
			gs.MainBranch = "master"
		}
	}

	// Get status (abbreviated)
	if status, err := runGit("status", "--short"); err == nil {
		gs.Status = truncate(strings.TrimSpace(status), 2000)
	}

	// Get recent log
	if log, err := runGit("log", "--oneline", "-10"); err == nil {
		gs.RecentLog = strings.TrimSpace(log)
	}

	// Get user name
	if name, err := runGit("config", "user.name"); err == nil {
		gs.UserName = strings.TrimSpace(name)
	}

	return gs
}

func runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.String(), nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// GetUserContext returns context information about the user environment.
func GetUserContext() map[string]string {
	ctx := map[string]string{
		"os":       getOS(),
		"date":     time.Now().Format("2006-01-02"),
		"time":     time.Now().Format("15:04:05"),
		"timezone": getTimezone(),
	}

	gs := GetGitStatus()
	if gs.IsRepo {
		ctx["git_branch"] = gs.Branch
		ctx["git_status"] = gs.Status
		ctx["git_recent_log"] = gs.RecentLog
		ctx["git_user"] = gs.UserName
	}

	return ctx
}

// GetSystemContext returns system-level context.
func GetSystemContext() map[string]string {
	ctx := map[string]string{
		"platform": getOS(),
		"shell":    os.Getenv("SHELL"),
		"home":     os.Getenv("HOME"),
		"user":     os.Getenv("USER"),
	}

	cwd, err := os.Getwd()
	if err == nil {
		ctx["cwd"] = cwd
	}

	return ctx
}

func getOS() string {
	// Use runtime.GOOS from the Go standard library
	return fmt.Sprintf("%s", os.Getenv("OSTYPE"))
}

func getTimezone() string {
	name, _ := time.Now().Zone()
	return name
}
