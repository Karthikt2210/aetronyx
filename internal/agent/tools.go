package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/karthikcodes/aetronyx/internal/llm"
)

// ToolName is the canonical name of a tool the agent can invoke.
type ToolName string

const (
	ToolReadFile   ToolName = "read_file"
	ToolWriteFile  ToolName = "write_file"
	ToolListFiles  ToolName = "list_files"
	ToolRunShell   ToolName = "run_shell"
)

var (
	ErrUnknownTool      = errors.New("unknown tool")
	ErrPathValidation   = errors.New("path validation failed")
	ErrFileNotFound     = errors.New("file not found")
	ErrCommandNotFound  = errors.New("command not found")
	ErrTimeout          = errors.New("operation timed out")
)

// ToolCall is a request to invoke a tool with structured parameters.
type ToolCall struct {
	Name   ToolName           `json:"name"`
	Params map[string]any     `json:"params"`
}

// ToolResult is the outcome of a tool invocation.
type ToolResult struct {
	Content    string
	ContentHash string
	ExitCode   int
	Stdout     string
	Stderr     string
	Err        error
}

// ToolDispatcher routes and executes all M2 tools.
type ToolDispatcher struct {
	workspace string
	log       *slog.Logger
}

// NewToolDispatcher creates a new dispatcher for the given workspace.
func NewToolDispatcher(workspace string, log *slog.Logger) *ToolDispatcher {
	return &ToolDispatcher{
		workspace: workspace,
		log:       log,
	}
}

// ValidatePath ensures the given path is within the workspace and doesn't try to escape.
func (d *ToolDispatcher) ValidatePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("%w: empty path", ErrPathValidation)
	}

	// Normalize and get absolute path
	abs := filepath.Join(d.workspace, path)
	abs, err := filepath.Abs(abs)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrPathValidation, err)
	}

	// Ensure it's under workspace
	workspaceAbs, err := filepath.Abs(d.workspace)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrPathValidation, err)
	}

	// Reject paths that escape or are forbidden
	if !strings.HasPrefix(abs, workspaceAbs+string(filepath.Separator)) && abs != workspaceAbs {
		return "", fmt.Errorf("%w: path outside workspace", ErrPathValidation)
	}

	if strings.Contains(abs, ".git") || strings.Contains(abs, ".aetronyx") {
		return "", fmt.Errorf("%w: forbidden directory", ErrPathValidation)
	}

	return abs, nil
}

// Dispatch routes a tool call to the appropriate handler.
func (d *ToolDispatcher) Dispatch(ctx context.Context, call ToolCall) ToolResult {
	switch call.Name {
	case ToolReadFile:
		return d.readFile(ctx, call.Params)
	case ToolWriteFile:
		return d.writeFile(ctx, call.Params)
	case ToolListFiles:
		return d.listFiles(ctx, call.Params)
	case ToolRunShell:
		return d.runShell(ctx, call.Params)
	default:
		return ToolResult{Err: fmt.Errorf("%w: %s", ErrUnknownTool, call.Name)}
	}
}

// readFile implements the read_file tool.
func (d *ToolDispatcher) readFile(ctx context.Context, params map[string]any) ToolResult {
	pathVal, ok := params["path"].(string)
	if !ok || pathVal == "" {
		return ToolResult{Err: fmt.Errorf("%w: missing or invalid path", ErrPathValidation)}
	}

	abs, err := d.ValidatePath(pathVal)
	if err != nil {
		return ToolResult{Err: err}
	}

	content, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return ToolResult{Err: fmt.Errorf("%w: %v", ErrFileNotFound, err)}
		}
		return ToolResult{Err: fmt.Errorf("read file: %w", err)}
	}

	hash := sha256.Sum256(content)
	return ToolResult{
		Content:     string(content),
		ContentHash: hex.EncodeToString(hash[:]),
	}
}

// writeFile implements the write_file tool with atomic writes.
func (d *ToolDispatcher) writeFile(ctx context.Context, params map[string]any) ToolResult {
	pathVal, ok := params["path"].(string)
	if !ok || pathVal == "" {
		return ToolResult{Err: fmt.Errorf("%w: missing or invalid path", ErrPathValidation)}
	}

	contentVal, ok := params["content"].(string)
	if !ok {
		return ToolResult{Err: fmt.Errorf("%w: missing or invalid content", ErrPathValidation)}
	}

	abs, err := d.ValidatePath(pathVal)
	if err != nil {
		return ToolResult{Err: err}
	}

	// Ensure parent directory exists
	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ToolResult{Err: fmt.Errorf("mkdir: %w", err)}
	}

	// Atomic write: write to temp file, then rename
	tmpFile := abs + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(contentVal), 0o644); err != nil {
		return ToolResult{Err: fmt.Errorf("write temp: %w", err)}
	}

	if err := os.Rename(tmpFile, abs); err != nil {
		// Clean up temp file on error
		_ = os.Remove(tmpFile)
		return ToolResult{Err: fmt.Errorf("rename: %w", err)}
	}

	return ToolResult{
		Content: fmt.Sprintf("written %d bytes", len(contentVal)),
	}
}

// listFiles implements the list_files tool.
func (d *ToolDispatcher) listFiles(ctx context.Context, params map[string]any) ToolResult {
	pathVal, ok := params["path"].(string)
	if !ok || pathVal == "" {
		return ToolResult{Err: fmt.Errorf("%w: missing or invalid path", ErrPathValidation)}
	}

	pattern, _ := params["pattern"].(string)

	abs, err := d.ValidatePath(pathVal)
	if err != nil {
		return ToolResult{Err: err}
	}

	// Check if path exists
	if _, err := os.Stat(abs); err != nil {
		return ToolResult{Err: fmt.Errorf("path stat: %w", err)}
	}

	var results []string
	err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and forbidden paths
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == ".aetronyx" {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip forbidden directories in path
		if strings.Contains(path, ".git") || strings.Contains(path, ".aetronyx") {
			return nil
		}

		// Get relative path
		rel, err := filepath.Rel(abs, path)
		if err != nil {
			return err
		}

		// Apply pattern if provided
		if pattern != "" {
			match, err := filepath.Match(pattern, rel)
			if err != nil || !match {
				return nil
			}
		}

		results = append(results, rel)
		return nil
	})

	if err != nil {
		return ToolResult{Err: fmt.Errorf("walk dir: %w", err)}
	}

	return ToolResult{
		Content: strings.Join(results, "\n"),
	}
}

// runShell implements the run_shell tool.
func (d *ToolDispatcher) runShell(ctx context.Context, params map[string]any) ToolResult {
	cmdVal, ok := params["command"].(string)
	if !ok || cmdVal == "" {
		return ToolResult{Err: fmt.Errorf("%w: missing or invalid command", ErrPathValidation)}
	}

	timeoutVal, _ := params["timeout_s"].(float64)
	timeout := int(timeoutVal)
	if timeout == 0 {
		timeout = 60
	}
	if timeout > 600 {
		timeout = 600
	}

	// Validate that the first token (command) exists in PATH
	parts := strings.Fields(cmdVal)
	if len(parts) == 0 {
		return ToolResult{Err: fmt.Errorf("empty command")}
	}

	if _, err := exec.LookPath(parts[0]); err != nil {
		return ToolResult{Err: fmt.Errorf("%w: %v", ErrCommandNotFound, err)}
	}

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", cmdVal)
	cmd.Dir = d.workspace

	// Set restricted environment: only PATH, HOME, GOPATH, GOROOT, GOMODCACHE
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"GOPATH=" + os.Getenv("GOPATH"),
		"GOROOT=" + os.Getenv("GOROOT"),
		"GOMODCACHE=" + os.Getenv("GOMODCACHE"),
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	err := cmd.Run()

	// Limit output to 64KB each
	stdoutStr := limitOutput(stdout.String(), 64*1024)
	stderrStr := limitOutput(stderr.String(), 64*1024)

	exitCode := 0
	if err != nil {
		if ctxErr := cmdCtx.Err(); ctxErr == context.DeadlineExceeded {
			return ToolResult{
				Err:      fmt.Errorf("%w", ErrTimeout),
				Stdout:   stdoutStr,
				Stderr:   stderrStr,
				ExitCode: 1,
			}
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	return ToolResult{
		Stdout:   stdoutStr,
		Stderr:   stderrStr,
		ExitCode: exitCode,
	}
}

// limitOutput truncates output to maxLen bytes.
func limitOutput(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "\n... (truncated)"
	}
	return s
}

// ToolDefinitions returns all M2 tool schemas as llm.Tool structs.
func ToolDefinitions() []llm.Tool {
	return []llm.Tool{
		{
			Name:        string(ToolReadFile),
			Description: "Read the contents of a file from the workspace.",
			InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "Relative path within the workspace."
    }
  },
  "required": ["path"]
}`),
		},
		{
			Name:        string(ToolWriteFile),
			Description: "Write or overwrite a file in the workspace. Creates parent directories if needed.",
			InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "Relative path within the workspace where the file should be written."
    },
    "content": {
      "type": "string",
      "description": "Full content to write to the file."
    }
  },
  "required": ["path", "content"]
}`),
		},
		{
			Name:        string(ToolListFiles),
			Description: "List files in a directory, optionally filtered by a glob pattern.",
			InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "path": {
      "type": "string",
      "description": "Relative path within the workspace."
    },
    "pattern": {
      "type": "string",
      "description": "Optional glob pattern to filter results (e.g. '*.go')."
    }
  },
  "required": ["path"]
}`),
		},
		{
			Name:        string(ToolRunShell),
			Description: "Execute a shell command in the workspace. Environment is restricted to PATH, HOME, GOPATH, GOROOT, GOMODCACHE.",
			InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "command": {
      "type": "string",
      "description": "Shell command to execute."
    },
    "timeout_s": {
      "type": "integer",
      "description": "Timeout in seconds (default 60, max 600)."
    }
  },
  "required": ["command"]
}`),
		},
	}
}
