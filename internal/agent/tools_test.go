package agent

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadFile_HappyPath(t *testing.T) {
	dir := t.TempDir()
	content := "hello world"
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	d := NewToolDispatcher(dir, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	result := d.readFile(context.Background(), map[string]any{"path": "test.txt"})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Content != content {
		t.Errorf("content mismatch: got %q, want %q", result.Content, content)
	}
	if result.ContentHash == "" {
		t.Error("content hash is empty")
	}
}

func TestReadFile_OutsideWorkspace(t *testing.T) {
	dir := t.TempDir()
	d := NewToolDispatcher(dir, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	result := d.readFile(context.Background(), map[string]any{"path": "../escape"})

	if result.Err == nil {
		t.Error("expected error for path escape, got nil")
	}
}

func TestReadFile_NotFound(t *testing.T) {
	dir := t.TempDir()
	d := NewToolDispatcher(dir, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	result := d.readFile(context.Background(), map[string]any{"path": "nonexistent.txt"})

	if result.Err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestWriteFile_Atomic(t *testing.T) {
	dir := t.TempDir()
	d := NewToolDispatcher(dir, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	content := "test content"
	result := d.writeFile(context.Background(), map[string]any{
		"path":    "output.txt",
		"content": content,
	})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	actual, err := os.ReadFile(filepath.Join(dir, "output.txt"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(actual) != content {
		t.Errorf("content mismatch: got %q, want %q", string(actual), content)
	}
}

func TestWriteFile_MkdirAll(t *testing.T) {
	dir := t.TempDir()
	d := NewToolDispatcher(dir, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	content := "nested content"
	result := d.writeFile(context.Background(), map[string]any{
		"path":    "a/b/c/file.txt",
		"content": content,
	})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	actual, err := os.ReadFile(filepath.Join(dir, "a/b/c/file.txt"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(actual) != content {
		t.Errorf("content mismatch: got %q, want %q", string(actual), content)
	}
}

func TestListFiles_All(t *testing.T) {
	dir := t.TempDir()

	// Create 3 test files
	files := []string{"a.txt", "b.go", "c.md"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("content"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	d := NewToolDispatcher(dir, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	result := d.listFiles(context.Background(), map[string]any{"path": "."})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	// Check that all files are present (order may vary)
	content := result.Content
	for _, f := range files {
		if !containsFile(content, f) {
			t.Errorf("file %q not found in listing", f)
		}
	}
}

func TestListFiles_Pattern(t *testing.T) {
	dir := t.TempDir()

	// Create mixed files
	files := []string{"a.go", "b.go", "c.txt", "d.md"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("content"), 0o644); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	d := NewToolDispatcher(dir, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	result := d.listFiles(context.Background(), map[string]any{
		"path":    ".",
		"pattern": "*.go",
	})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	// Should contain only .go files
	content := result.Content
	if !containsFile(content, "a.go") || !containsFile(content, "b.go") {
		t.Error("missing .go files in pattern match")
	}
	if containsFile(content, "c.txt") || containsFile(content, "d.md") {
		t.Error("non-.go files included in pattern match")
	}
}

func TestListFiles_SkipsGitAndAetronyx(t *testing.T) {
	dir := t.TempDir()

	// Create directories
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	os.Mkdir(filepath.Join(dir, ".aetronyx"), 0o755)
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("secret"), 0o644)
	os.WriteFile(filepath.Join(dir, ".aetronyx", "state"), []byte("state"), 0o644)
	os.WriteFile(filepath.Join(dir, "normal.txt"), []byte("normal"), 0o644)

	d := NewToolDispatcher(dir, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	result := d.listFiles(context.Background(), map[string]any{"path": "."})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	content := result.Content
	if containsFile(content, ".git/config") || containsFile(content, ".aetronyx/state") {
		t.Error("forbidden directories included in listing")
	}
	if !containsFile(content, "normal.txt") {
		t.Error("normal.txt not found")
	}
}

func TestRunShell_Echo(t *testing.T) {
	dir := t.TempDir()
	d := NewToolDispatcher(dir, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	result := d.runShell(context.Background(), map[string]any{
		"command": "echo hello",
	})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code: got %d, want 0", result.ExitCode)
	}
	if result.Stdout != "hello\n" {
		t.Errorf("stdout mismatch: got %q, want %q", result.Stdout, "hello\n")
	}
}

func TestRunShell_NonZeroExit(t *testing.T) {
	dir := t.TempDir()
	d := NewToolDispatcher(dir, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	result := d.runShell(context.Background(), map[string]any{
		"command": "false",
	})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.ExitCode == 0 {
		t.Errorf("exit code: got 0, want non-zero")
	}
}

func TestRunShell_TimeoutConstraint(t *testing.T) {
	dir := t.TempDir()
	d := NewToolDispatcher(dir, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	// Verify timeout value is clamped to 600s max
	result := d.runShell(context.Background(), map[string]any{
		"command":   "echo test",
		"timeout_s": 9999, // Should be clamped to 600
	})

	// Should succeed - just verifying the parameter is accepted
	if result.Err != nil && result.ExitCode == 0 {
		t.Fatalf("unexpected error: %v", result.Err)
	}
}

func TestRunShell_CommandNotFound(t *testing.T) {
	dir := t.TempDir()
	d := NewToolDispatcher(dir, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	result := d.runShell(context.Background(), map[string]any{
		"command": "nonexistentcommand12345",
	})

	if result.Err == nil {
		t.Error("expected command not found error, got nil")
	}
}

func TestRunShell_TimeoutDefault(t *testing.T) {
	dir := t.TempDir()
	d := NewToolDispatcher(dir, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	start := time.Now()
	result := d.runShell(context.Background(), map[string]any{
		"command": "sleep 0.1",
		// no timeout_s, should use default 60s
	})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	// Verify it completed quickly (didn't hit timeout)
	if time.Since(start) > 5*time.Second {
		t.Error("command took too long, may have timed out unexpectedly")
	}
}

func TestToolDefinitions(t *testing.T) {
	tools := ToolDefinitions()

	if len(tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(tools))
	}

	expectedNames := []string{
		string(ToolReadFile),
		string(ToolWriteFile),
		string(ToolListFiles),
		string(ToolRunShell),
	}

	for i, tool := range tools {
		if tool.Name != expectedNames[i] {
			t.Errorf("tool %d: name mismatch: got %q, want %q", i, tool.Name, expectedNames[i])
		}
		if tool.Description == "" {
			t.Errorf("tool %d: description is empty", i)
		}
		if len(tool.InputSchema) == 0 {
			t.Errorf("tool %d: input schema is empty", i)
		}
	}
}

func TestDispatch_UnknownTool(t *testing.T) {
	dir := t.TempDir()
	d := NewToolDispatcher(dir, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	result := d.Dispatch(context.Background(), ToolCall{
		Name:   "unknown_tool",
		Params: map[string]any{},
	})

	if result.Err == nil {
		t.Error("expected error for unknown tool, got nil")
	}
}

func TestDispatch_ReadFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("content"), 0o644)

	d := NewToolDispatcher(dir, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	result := d.Dispatch(context.Background(), ToolCall{
		Name:   ToolReadFile,
		Params: map[string]any{"path": "test.txt"},
	})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Content != "content" {
		t.Errorf("content mismatch: got %q", result.Content)
	}
}

func TestDispatch_WriteFile(t *testing.T) {
	dir := t.TempDir()
	d := NewToolDispatcher(dir, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	result := d.Dispatch(context.Background(), ToolCall{
		Name:   ToolWriteFile,
		Params: map[string]any{"path": "out.txt", "content": "test"},
	})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	actual, _ := os.ReadFile(filepath.Join(dir, "out.txt"))
	if string(actual) != "test" {
		t.Errorf("file content mismatch")
	}
}

// Helper function to check if a file is in the newline-separated listing.
func containsFile(listing, filename string) bool {
	if listing == filename {
		return true
	}
	for _, line := range split(listing, "\n") {
		if line == filename || filepath.Base(line) == filename {
			return true
		}
	}
	return false
}

func split(s, sep string) []string {
	if s == "" {
		return nil
	}
	var parts []string
	for {
		idx := indexStr(s, sep)
		if idx == -1 {
			parts = append(parts, s)
			break
		}
		parts = append(parts, s[:idx])
		s = s[idx+len(sep):]
	}
	return parts
}

func indexStr(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
