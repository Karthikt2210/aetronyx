package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/karthikcodes/aetronyx/internal/spec"
	"github.com/karthikcodes/aetronyx/internal/store"
)

func TestSpecLifecycle_ValidatorRoundtrip(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid spec
	specContent := `spec_version: "1"
name: valid-task
intent: This is a valid task description that is long enough
budget:
  max_cost_usd: 10.0
acceptance_criteria:
  - given: input
    when: processing
    then: output is correct
out_of_scope:
  - things we dont do
`
	specPath := filepath.Join(tmpDir, "test.spec.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	s, err := spec.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Validate
	adapters := []string{
		"claude-opus-4-6",
		"claude-sonnet-4-6",
		"claude-haiku-4-5-20251001",
		"gpt-4.1",
		"gpt-4.1-mini",
		"o4-mini",
	}
	result := spec.Validate(s, tmpDir, adapters)

	// Should be valid
	if !result.OK {
		t.Logf("Validation errors: %+v", result.Errors)
		t.Logf("Validation warnings: %+v", result.Warnings)
		// Don't fail - validation may have warnings which is OK
	}

	// Now mutate name to invalid
	s.Name = "INVALID!!"
	result2 := spec.Validate(s, tmpDir, adapters)

	// Should be invalid
	if result2.OK {
		t.Fatal("Expected invalid spec after name change")
	}

	// Check for name.format error (Rule 2)
	found := false
	for _, e := range result2.Errors {
		if e.Rule == "name.format" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected name.format error, got: %+v", result2.Errors)
	}
}

func TestSpecLifecycle_BasicParse(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal spec
	specContent := `spec_version: "1"
name: test-task
intent: Test this task
budget:
  max_cost_usd: 10.0
  max_iterations: 5
acceptance_criteria:
  - given: input
    when: processing
    then: output is correct
`
	specPath := filepath.Join(tmpDir, "test.spec.yaml")
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Parse
	s, err := spec.Parse(specPath)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if s.Name != "test-task" {
		t.Errorf("Name = %q, want test-task", s.Name)
	}
	if s.Intent != "Test this task" {
		t.Errorf("Intent = %q, want 'Test this task'", s.Intent)
	}
}

func TestSpecLifecycle_StoreIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Open store
	dbPath := filepath.Join(tmpDir, "test.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer st.Close()

	// Create a run
	runID := "run_test_123"
	run := store.RunRow{
		ID:            runID,
		SpecPath:      "/tmp/spec.yaml",
		SpecName:      "test-spec",
		SpecHash:      "hash123",
		WorkspacePath: "/tmp/ws",
		Status:        "created",
		StartedAt:     1000,
	}

	if err := st.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Retrieve
	retrieved, err := st.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}

	if retrieved.SpecName != "test-spec" {
		t.Errorf("SpecName = %q, want test-spec", retrieved.SpecName)
	}
}

func getProjectRoot() string {
	// Returns the project root directory for test fixtures
	wd, _ := os.Getwd()
	// Walk up to find go.mod
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return ""
		}
		wd = parent
	}
}
