package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidatePath checks that relPath is safe to write within workspace.
// It rejects paths that escape the workspace root, or target .git/ or .aetronyx/.
func ValidatePath(workspace, relPath string) error {
	if relPath == "" {
		return fmt.Errorf("ValidatePath: path is empty")
	}

	// Resolve to absolute path.
	abs, err := filepath.Abs(filepath.Join(workspace, relPath))
	if err != nil {
		return fmt.Errorf("ValidatePath resolve: %w", err)
	}

	// Ensure workspace itself is absolute.
	wsAbs, err := filepath.Abs(workspace)
	if err != nil {
		return fmt.Errorf("ValidatePath workspace resolve: %w", err)
	}

	// Must stay inside workspace.
	if !strings.HasPrefix(abs+string(filepath.Separator), wsAbs+string(filepath.Separator)) {
		return fmt.Errorf("ValidatePath: path %q escapes workspace %q", relPath, workspace)
	}

	// Reject writes inside .git/ or .aetronyx/.
	rel, err := filepath.Rel(wsAbs, abs)
	if err != nil {
		return fmt.Errorf("ValidatePath rel: %w", err)
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) > 0 {
		top := parts[0]
		if top == ".git" || top == ".aetronyx" {
			return fmt.Errorf("ValidatePath: path %q is inside protected directory %q", relPath, top)
		}
	}

	return nil
}

// WriteFile writes content to workspace/relPath, creating parent directories as needed.
// The write is atomic: content is written to a temp file first, then renamed.
func WriteFile(workspace, relPath, content string) error {
	if err := ValidatePath(workspace, relPath); err != nil {
		return fmt.Errorf("WriteFile: %w", err)
	}

	destPath := filepath.Join(workspace, relPath)
	dir := filepath.Dir(destPath)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("WriteFile mkdir: %w", err)
	}

	// Atomic write: temp file in the same directory, then rename.
	tmp, err := os.CreateTemp(dir, ".aetronyx-write-*")
	if err != nil {
		return fmt.Errorf("WriteFile tempfile: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("WriteFile write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("WriteFile close: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("WriteFile chmod: %w", err)
	}
	if err := os.Rename(tmpName, destPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("WriteFile rename: %w", err)
	}

	return nil
}
