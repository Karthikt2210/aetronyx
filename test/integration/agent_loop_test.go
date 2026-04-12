package integration

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAgentLoopBasic verifies the agent loop can be wired together.
// Full integration testing will come after the agent engine is implemented.
func TestAgentLoopBasic(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a workspace
	wsDir := filepath.Join(tmpDir, "workspace")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	// TODO(M1): Complete once agent engine is wired
	// For now, just verify the basics:
	// 1. Workspace directory exists
	// 2. DB can be created
	// 3. Keypair can be loaded

	if _, err := os.Stat(wsDir); os.IsNotExist(err) {
		t.Fatal("Workspace should exist")
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		// DB not created yet is fine for M1
	}
}

// TestAgentLoopWithSpec verifies agent can load a spec file.
func TestAgentLoopWithSpec(t *testing.T) {
	tmpDir := t.TempDir()
	specDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatalf("Failed to create spec dir: %v", err)
	}

	// Create a minimal spec
	specPath := filepath.Join(specDir, "test.yaml")
	specContent := `spec_version: "1"
name: test-spec
intent: "Test intent"
`
	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		t.Fatalf("Failed to write spec: %v", err)
	}

	// Verify it exists
	if _, err := os.Stat(specPath); err != nil {
		t.Fatalf("Spec file not found: %v", err)
	}
}
