package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func newTestToolCtx(t *testing.T) *ToolContext {
	t.Helper()
	dir := t.TempDir()
	return &ToolContext{
		CWD:           dir,
		AbortCtx:      context.Background(),
		ReadFileState: NewFileStateCache(),
	}
}

func TestBashTool(t *testing.T) {
	tool := NewBashTool()

	t.Run("name and schema", func(t *testing.T) {
		if tool.Name() != "Bash" {
			t.Errorf("expected name Bash, got %s", tool.Name())
		}
		if tool.IsReadOnly() {
			t.Error("Bash should not be read-only")
		}
		schema := tool.InputSchema()
		if _, ok := schema.Properties["command"]; !ok {
			t.Error("missing command property in schema")
		}
	})

	t.Run("execute echo", func(t *testing.T) {
		ctx := newTestToolCtx(t)
		input, _ := json.Marshal(BashInput{Command: "echo hello"})
		result, err := tool.Execute(context.Background(), input, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %s", result.Content)
		}
		if result.Content != "hello\n" {
			t.Errorf("expected 'hello\\n', got %q", result.Content)
		}
	})

	t.Run("execute failing command", func(t *testing.T) {
		ctx := newTestToolCtx(t)
		input, _ := json.Marshal(BashInput{Command: "exit 42"})
		result, err := tool.Execute(context.Background(), input, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error result for failing command")
		}
	})

	t.Run("empty command", func(t *testing.T) {
		ctx := newTestToolCtx(t)
		input, _ := json.Marshal(BashInput{})
		result, err := tool.Execute(context.Background(), input, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for empty command")
		}
	})
}

func TestFileReadTool(t *testing.T) {
	tool := NewFileReadTool()

	t.Run("read existing file", func(t *testing.T) {
		ctx := newTestToolCtx(t)
		// Create test file
		testFile := filepath.Join(ctx.CWD, "test.txt")
		if err := os.WriteFile(testFile, []byte("line1\nline2\nline3\n"), 0644); err != nil {
			t.Fatal(err)
		}

		input, _ := json.Marshal(FileReadInput{FilePath: "test.txt"})
		result, err := tool.Execute(context.Background(), input, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %s", result.Content)
		}
		if !contains(result.Content, "line1") || !contains(result.Content, "line2") {
			t.Errorf("expected file contents, got: %s", result.Content)
		}
	})

	t.Run("read with offset and limit", func(t *testing.T) {
		ctx := newTestToolCtx(t)
		testFile := filepath.Join(ctx.CWD, "test.txt")
		if err := os.WriteFile(testFile, []byte("line1\nline2\nline3\nline4\nline5\n"), 0644); err != nil {
			t.Fatal(err)
		}

		input, _ := json.Marshal(FileReadInput{FilePath: "test.txt", Offset: 1, Limit: 2})
		result, err := tool.Execute(context.Background(), input, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !contains(result.Content, "line2") || !contains(result.Content, "line3") {
			t.Errorf("expected lines 2-3, got: %s", result.Content)
		}
		if contains(result.Content, "line1") {
			t.Errorf("should not contain line1: %s", result.Content)
		}
	})

	t.Run("read nonexistent file", func(t *testing.T) {
		ctx := newTestToolCtx(t)
		input, _ := json.Marshal(FileReadInput{FilePath: "nonexistent.txt"})
		result, err := tool.Execute(context.Background(), input, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for nonexistent file")
		}
	})
}

func TestFileWriteTool(t *testing.T) {
	tool := NewFileWriteTool()

	t.Run("write new file", func(t *testing.T) {
		ctx := newTestToolCtx(t)
		input, _ := json.Marshal(FileWriteInput{
			FilePath: "new_file.txt",
			Content:  "hello world\n",
		})
		result, err := tool.Execute(context.Background(), input, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %s", result.Content)
		}

		// Verify file was created
		data, err := os.ReadFile(filepath.Join(ctx.CWD, "new_file.txt"))
		if err != nil {
			t.Fatalf("file not created: %v", err)
		}
		if string(data) != "hello world\n" {
			t.Errorf("unexpected content: %q", string(data))
		}
	})

	t.Run("create nested directories", func(t *testing.T) {
		ctx := newTestToolCtx(t)
		input, _ := json.Marshal(FileWriteInput{
			FilePath: "a/b/c/deep.txt",
			Content:  "deep content",
		})
		result, err := tool.Execute(context.Background(), input, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %s", result.Content)
		}

		data, err := os.ReadFile(filepath.Join(ctx.CWD, "a/b/c/deep.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "deep content" {
			t.Errorf("unexpected content: %q", string(data))
		}
	})
}

func TestFileEditTool(t *testing.T) {
	tool := NewFileEditTool()

	t.Run("edit existing file", func(t *testing.T) {
		ctx := newTestToolCtx(t)
		testFile := filepath.Join(ctx.CWD, "edit.txt")
		if err := os.WriteFile(testFile, []byte("hello world\nfoo bar\nbaz\n"), 0644); err != nil {
			t.Fatal(err)
		}

		input, _ := json.Marshal(FileEditInput{
			FilePath: "edit.txt",
			OldText:  "foo bar",
			NewText:  "replaced",
		})
		result, err := tool.Execute(context.Background(), input, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %s", result.Content)
		}

		data, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatal(err)
		}
		if !contains(string(data), "replaced") {
			t.Errorf("expected replacement, got: %s", string(data))
		}
		if contains(string(data), "foo bar") {
			t.Error("old text should be replaced")
		}
	})

	t.Run("old text not found", func(t *testing.T) {
		ctx := newTestToolCtx(t)
		testFile := filepath.Join(ctx.CWD, "edit2.txt")
		if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
			t.Fatal(err)
		}

		input, _ := json.Marshal(FileEditInput{
			FilePath: "edit2.txt",
			OldText:  "not found",
			NewText:  "replacement",
		})
		result, err := tool.Execute(context.Background(), input, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error when old_text not found")
		}
	})

	t.Run("multiple matches", func(t *testing.T) {
		ctx := newTestToolCtx(t)
		testFile := filepath.Join(ctx.CWD, "edit3.txt")
		if err := os.WriteFile(testFile, []byte("aaa\naaa\naaa\n"), 0644); err != nil {
			t.Fatal(err)
		}

		input, _ := json.Marshal(FileEditInput{
			FilePath: "edit3.txt",
			OldText:  "aaa",
			NewText:  "bbb",
		})
		result, err := tool.Execute(context.Background(), input, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for multiple matches")
		}
	})
}

func TestGlobTool(t *testing.T) {
	tool := NewGlobTool()

	t.Run("find files", func(t *testing.T) {
		ctx := newTestToolCtx(t)
		// Create test files
		os.WriteFile(filepath.Join(ctx.CWD, "file1.go"), []byte("package main"), 0644)
		os.WriteFile(filepath.Join(ctx.CWD, "file2.go"), []byte("package main"), 0644)
		os.WriteFile(filepath.Join(ctx.CWD, "file3.txt"), []byte("text"), 0644)

		input, _ := json.Marshal(GlobInput{Pattern: "*.go"})
		result, err := tool.Execute(context.Background(), input, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %s", result.Content)
		}
		if !contains(result.Content, "file1.go") || !contains(result.Content, "file2.go") {
			t.Errorf("expected .go files, got: %s", result.Content)
		}
		if contains(result.Content, "file3.txt") {
			t.Error("should not match .txt files")
		}
	})
}

func TestGrepTool(t *testing.T) {
	tool := NewGrepTool()

	t.Run("search for pattern", func(t *testing.T) {
		ctx := newTestToolCtx(t)
		os.WriteFile(filepath.Join(ctx.CWD, "search.go"), []byte("package main\n\nfunc hello() {}\n"), 0644)

		input, _ := json.Marshal(GrepInput{Pattern: "func hello"})
		result, err := tool.Execute(context.Background(), input, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %s", result.Content)
		}
		if !contains(result.Content, "func hello") {
			t.Errorf("expected match, got: %s", result.Content)
		}
	})

	t.Run("no matches", func(t *testing.T) {
		ctx := newTestToolCtx(t)
		os.WriteFile(filepath.Join(ctx.CWD, "empty.txt"), []byte("nothing here"), 0644)

		input, _ := json.Marshal(GrepInput{Pattern: "xyzzy"})
		result, err := tool.Execute(context.Background(), input, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Error("should not be error for no matches")
		}
		if !contains(result.Content, "No matches") {
			t.Errorf("expected 'No matches' message, got: %s", result.Content)
		}
	})
}

func TestListDirTool(t *testing.T) {
	tool := NewListDirTool()

	t.Run("list directory", func(t *testing.T) {
		ctx := newTestToolCtx(t)
		os.WriteFile(filepath.Join(ctx.CWD, "file.txt"), []byte(""), 0644)
		os.MkdirAll(filepath.Join(ctx.CWD, "subdir"), 0755)

		input, _ := json.Marshal(ListDirInput{Path: "."})
		result, err := tool.Execute(context.Background(), input, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %s", result.Content)
		}
		if !contains(result.Content, "subdir/") {
			t.Errorf("expected directory listing, got: %s", result.Content)
		}
		if !contains(result.Content, "file.txt") {
			t.Errorf("expected file in listing, got: %s", result.Content)
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		ctx := newTestToolCtx(t)
		input, _ := json.Marshal(ListDirInput{Path: "nonexistent"})
		result, err := tool.Execute(context.Background(), input, ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for nonexistent directory")
		}
	})
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()

	t.Run("register and get", func(t *testing.T) {
		tool := NewBashTool()
		r.Register(tool)

		got, ok := r.Get("Bash")
		if !ok {
			t.Fatal("tool not found after register")
		}
		if got.Name() != "Bash" {
			t.Errorf("expected Bash, got %s", got.Name())
		}
	})

	t.Run("all tools", func(t *testing.T) {
		r2 := NewRegistry()
		RegisterBuiltinTools(r2)
		all := r2.All()
		if len(all) < 5 {
			t.Errorf("expected at least 5 tools, got %d", len(all))
		}
	})

	t.Run("to definitions", func(t *testing.T) {
		r2 := NewRegistry()
		RegisterBuiltinTools(r2)
		defs := r2.ToDefinitions()
		if len(defs) < 5 {
			t.Errorf("expected at least 5 definitions, got %d", len(defs))
		}
		for _, d := range defs {
			if d.Name == "" {
				t.Error("definition has empty name")
			}
			if d.Description == "" {
				t.Errorf("definition %s has empty description", d.Name)
			}
		}
	})

	t.Run("execute unknown tool", func(t *testing.T) {
		r2 := NewRegistry()
		result, err := r2.Execute(context.Background(), "UnknownTool", nil, newTestToolCtx(t))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for unknown tool")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsCheck(s, substr)
}

func containsCheck(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
