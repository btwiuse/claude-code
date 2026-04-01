package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBashTool_Name(t *testing.T) {
	tool := &BashTool{}
	if tool.Name() != "bash" {
		t.Errorf("expected 'bash', got %q", tool.Name())
	}
}

func TestBashTool_IsReadOnly(t *testing.T) {
	tool := &BashTool{}
	if tool.IsReadOnly() {
		t.Error("BashTool should not be read-only")
	}
}

func TestBashTool_Run(t *testing.T) {
	tool := &BashTool{}
	ctx := context.Background()

	input, _ := json.Marshal(bashInput{Command: "echo hello"})
	result, err := tool.Run(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "hello" {
		t.Errorf("expected 'hello', got %q", result.Output)
	}
	if result.IsError {
		t.Error("expected no error")
	}
}

func TestBashTool_RunWithCWD(t *testing.T) {
	tool := &BashTool{}
	ctx := context.Background()

	input, _ := json.Marshal(bashInput{Command: "pwd", CWD: "/tmp"})
	result, err := tool.Run(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "/tmp") {
		t.Errorf("expected output to contain '/tmp', got %q", result.Output)
	}
}

func TestBashTool_RunFailingCommand(t *testing.T) {
	tool := &BashTool{}
	ctx := context.Background()

	input, _ := json.Marshal(bashInput{Command: "exit 1"})
	result, err := tool.Run(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", result.ExitCode)
	}
}

func TestBashTool_EmptyCommand(t *testing.T) {
	tool := &BashTool{}
	ctx := context.Background()

	input, _ := json.Marshal(bashInput{Command: ""})
	result, err := tool.Run(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for empty command")
	}
}

func TestFileReadTool_Run(t *testing.T) {
	tool := &FileReadTool{}
	ctx := context.Background()

	// Create a temp file
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	os.WriteFile(path, []byte("line1\nline2\nline3"), 0o644)

	input, _ := json.Marshal(fileReadInput{Path: path})
	result, err := tool.Run(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "line1") {
		t.Errorf("expected output to contain 'line1', got %q", result.Output)
	}
	if !strings.Contains(result.Output, "1. line1") {
		t.Errorf("expected output to have line numbers, got %q", result.Output)
	}
}

func TestFileReadTool_RunWithRange(t *testing.T) {
	tool := &FileReadTool{}
	ctx := context.Background()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	os.WriteFile(path, []byte("line1\nline2\nline3\nline4"), 0o644)

	input, _ := json.Marshal(fileReadInput{Path: path, StartLine: 2, EndLine: 3})
	result, err := tool.Run(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "2. line2") {
		t.Errorf("expected '2. line2', got %q", result.Output)
	}
	if strings.Contains(result.Output, "1. line1") {
		t.Errorf("should not contain line 1, got %q", result.Output)
	}
}

func TestFileReadTool_NonexistentFile(t *testing.T) {
	tool := &FileReadTool{}
	ctx := context.Background()

	input, _ := json.Marshal(fileReadInput{Path: "/nonexistent/file.txt"})
	result, err := tool.Run(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for nonexistent file")
	}
}

func TestFileWriteTool_Run(t *testing.T) {
	tool := &FileWriteTool{}
	ctx := context.Background()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "output.txt")

	input, _ := json.Marshal(fileWriteInput{Path: path, Content: "hello world"})
	result, err := tool.Run(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %s", result.Error)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(data))
	}
}

func TestFileWriteTool_CreateDirs(t *testing.T) {
	tool := &FileWriteTool{}
	ctx := context.Background()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "sub", "dir", "output.txt")

	input, _ := json.Marshal(fileWriteInput{Path: path, Content: "nested"})
	result, err := tool.Run(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %s", result.Error)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "nested" {
		t.Errorf("expected 'nested', got %q", string(data))
	}
}

func TestFileEditTool_Run(t *testing.T) {
	tool := &FileEditTool{}
	ctx := context.Background()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "edit.txt")
	os.WriteFile(path, []byte("hello world\nfoo bar"), 0o644)

	input, _ := json.Marshal(fileEditInput{
		Path:   path,
		OldStr: "hello world",
		NewStr: "hi universe",
	})
	result, err := tool.Run(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %s", result.Error)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "hi universe") {
		t.Errorf("expected 'hi universe', got %q", string(data))
	}
}

func TestFileEditTool_NotFound(t *testing.T) {
	tool := &FileEditTool{}
	ctx := context.Background()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "edit.txt")
	os.WriteFile(path, []byte("hello world"), 0o644)

	input, _ := json.Marshal(fileEditInput{
		Path:   path,
		OldStr: "nonexistent",
		NewStr: "replacement",
	})
	result, err := tool.Run(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for nonexistent string")
	}
}

func TestFileEditTool_MultipleMatches(t *testing.T) {
	tool := &FileEditTool{}
	ctx := context.Background()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "edit.txt")
	os.WriteFile(path, []byte("hello\nhello\nhello"), 0o644)

	input, _ := json.Marshal(fileEditInput{
		Path:   path,
		OldStr: "hello",
		NewStr: "hi",
	})
	result, err := tool.Run(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for multiple matches")
	}
}

func TestGlobTool_Run(t *testing.T) {
	tool := &GlobTool{}
	ctx := context.Background()

	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "a.go"), []byte("package a"), 0o644)
	os.WriteFile(filepath.Join(tmp, "b.go"), []byte("package b"), 0o644)
	os.WriteFile(filepath.Join(tmp, "c.txt"), []byte("text"), 0o644)

	input, _ := json.Marshal(globInput{Pattern: "*.go", Path: tmp})
	result, err := tool.Run(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "a.go") || !strings.Contains(result.Output, "b.go") {
		t.Errorf("expected to find .go files, got %q", result.Output)
	}
	if strings.Contains(result.Output, "c.txt") {
		t.Errorf("should not find .txt files, got %q", result.Output)
	}
}

func TestGrepTool_Run(t *testing.T) {
	tool := &GrepTool{}
	ctx := context.Background()

	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("hello world\nfoo bar\nhello again"), 0o644)
	os.WriteFile(filepath.Join(tmp, "b.txt"), []byte("no match here"), 0o644)

	input, _ := json.Marshal(grepInput{Pattern: "hello", Path: tmp})
	result, err := tool.Run(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "hello") {
		t.Errorf("expected to find 'hello', got %q", result.Output)
	}
}

func TestGrepTool_NoMatch(t *testing.T) {
	tool := &GrepTool{}
	ctx := context.Background()

	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("hello world"), 0o644)

	input, _ := json.Marshal(grepInput{Pattern: "zzzzz", Path: tmp})
	result, err := tool.Run(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Output, "No matches") {
		t.Errorf("expected 'No matches', got %q", result.Output)
	}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()

	names := r.Names()
	if len(names) == 0 {
		t.Error("expected some tools to be registered")
	}

	// Verify expected tools are present
	expectedTools := []string{"bash", "file_read", "file_write", "file_edit", "glob", "grep", "web_fetch", "agent"}
	for _, name := range expectedTools {
		tool, ok := r.Get(name)
		if !ok {
			t.Errorf("expected tool %q to be registered", name)
			continue
		}
		if tool.Name() != name {
			t.Errorf("expected tool name %q, got %q", name, tool.Name())
		}

		// Verify schema is valid JSON
		schema := tool.InputSchema()
		var v interface{}
		if err := json.Unmarshal(schema, &v); err != nil {
			t.Errorf("tool %q: invalid JSON schema: %v", name, err)
		}
	}
}

func TestAgentTool_NoRunner(t *testing.T) {
	tool := &AgentTool{}
	ctx := context.Background()

	input, _ := json.Marshal(agentInput{Description: "test", Prompt: "do something"})
	result, err := tool.Run(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error when RunAgent is not set")
	}
}

func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input    string
		expected string
	}{
		{"/absolute/path", "/absolute/path"},
		{"~/test", filepath.Join(home, "test")},
		{"/path/../normalized", "/normalized"},
	}

	for _, tt := range tests {
		result := expandPath(tt.input)
		if result != tt.expected {
			t.Errorf("expandPath(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}
