package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBashTool(t *testing.T) {
	tool := &BashTool{}

	if tool.Name() != "Bash" {
		t.Errorf("expected name 'Bash', got %q", tool.Name())
	}

	input, _ := json.Marshal(bashInput{Command: "echo hello"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Content, "hello") {
		t.Errorf("expected output to contain 'hello', got %q", result.Content)
	}
	if result.IsError {
		t.Error("expected no error")
	}
}

func TestBashTool_EmptyCommand(t *testing.T) {
	tool := &BashTool{}
	input, _ := json.Marshal(bashInput{Command: ""})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for empty command")
	}
}

func TestFileReadTool(t *testing.T) {
	tool := &FileReadTool{}

	// Create a temp file.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	input, _ := json.Marshal(fileReadInput{FilePath: path})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Content, "line1") {
		t.Errorf("expected output to contain 'line1', got %q", result.Content)
	}
	if !strings.Contains(result.Content, "1.") {
		t.Error("expected line numbers in output")
	}
}

func TestFileReadTool_Range(t *testing.T) {
	tool := &FileReadTool{}

	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("a\nb\nc\nd\ne\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	input, _ := json.Marshal(fileReadInput{FilePath: path, Range: "2-4"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Content, "b") || !strings.Contains(result.Content, "d") {
		t.Errorf("expected lines 2-4 in output, got %q", result.Content)
	}
	if strings.Contains(result.Content, "1. a") {
		t.Error("should not contain line 1")
	}
}

func TestFileEditTool(t *testing.T) {
	tool := &FileEditTool{}

	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	input, _ := json.Marshal(fileEditInput{
		FilePath:  path,
		OldString: "world",
		NewString: "Go",
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "hello Go" {
		t.Errorf("expected 'hello Go', got %q", string(data))
	}
}

func TestFileEditTool_NotFound(t *testing.T) {
	tool := &FileEditTool{}

	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	input, _ := json.Marshal(fileEditInput{
		FilePath:  path,
		OldString: "nonexistent",
		NewString: "replacement",
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	if !result.IsError {
		t.Error("expected error for non-matching string")
	}
}

func TestFileWriteTool(t *testing.T) {
	tool := &FileWriteTool{}

	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "test.txt")

	input, _ := json.Marshal(fileWriteInput{
		FilePath: path,
		Content:  "hello from Go",
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "hello from Go" {
		t.Errorf("expected 'hello from Go', got %q", string(data))
	}
}

func TestGlobTool(t *testing.T) {
	tool := &GlobTool{}

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "c.txt"), []byte(""), 0o644)

	input, _ := json.Marshal(globInput{
		Pattern: "*.go",
		Path:    dir,
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Content, "a.go") || !strings.Contains(result.Content, "b.go") {
		t.Errorf("expected .go files in output, got %q", result.Content)
	}
	if strings.Contains(result.Content, "c.txt") {
		t.Error("should not contain .txt file")
	}
}

func TestGrepTool(t *testing.T) {
	tool := &GrepTool{}

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.go"), []byte("func main() {\n\tfmt.Println(\"hello\")\n}\n"), 0o644)

	input, _ := json.Marshal(grepInput{
		Pattern: "hello",
		Path:    dir,
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Content, "hello") {
		t.Errorf("expected 'hello' in output, got %q", result.Content)
	}
}

func TestToolRegistry(t *testing.T) {
	registry := NewRegistry()

	tools := registry.All()
	if len(tools) != 7 {
		t.Errorf("expected 7 tools, got %d", len(tools))
	}

	bash, ok := registry.Get("Bash")
	if !ok || bash.Name() != "Bash" {
		t.Error("expected to find Bash tool")
	}

	_, ok = registry.Get("NonExistent")
	if ok {
		t.Error("expected NonExistent tool to not be found")
	}
}

func TestToolRegistry_Execute(t *testing.T) {
	registry := NewRegistry()

	input, _ := json.Marshal(bashInput{Command: "echo test"})
	result, err := registry.Execute(context.Background(), "Bash", input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content, "test") {
		t.Errorf("expected 'test' in output, got %q", result.Content)
	}

	// Test unknown tool.
	result, err = registry.Execute(context.Background(), "Unknown", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Error("expected error for unknown tool")
	}
}
