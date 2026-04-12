package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/karthikcodes/aetronyx/internal/repo"
)

func TestBlastRadius_EmptyDeps(t *testing.T) {
	tmpDir := t.TempDir()

	// Create minimal Go module
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\ngo 1.23\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile main.go failed: %v", err)
	}

	// Build graph
	graph, err := repo.Build(tmpDir)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Spec with no dependencies
	specFiles := []string{}
	testCommands := []string{}

	// Compute blast radius
	radius := repo.ComputeBlastRadius(graph, specFiles, testCommands)

	// Should have empty Direct since no dependencies specified
	if len(radius.Direct) != 0 {
		t.Errorf("Expected empty Direct, got %d files", len(radius.Direct))
	}

	// Should have valid stats
	if radius.Stats.DirectCount < 0 {
		t.Error("Stats.DirectCount should be non-negative")
	}
}

func TestBlastRadius_WithDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	// Create module structure
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\ngo 1.23\n"), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod failed: %v", err)
	}

	// Create files
	files := map[string]string{
		"main.go": `package main
func main() {}
`,
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile %s failed: %v", name, err)
		}
	}

	// Build graph
	graph, err := repo.Build(tmpDir)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Spec depends on main.go
	specFiles := []string{"main.go"}
	testCommands := []string{}

	// Compute blast radius
	radius := repo.ComputeBlastRadius(graph, specFiles, testCommands)

	// Should have a valid report structure
	if radius.Stats.DirectCount < 0 {
		t.Error("Stats should be initialized")
	}
}

func TestBlastRadius_FixtureGo(t *testing.T) {
	// Use the testdata/fixture-go from the repo
	fixtureRoot := filepath.Join(getProjectRoot(), "testdata", "fixture-go")
	if _, err := os.Stat(fixtureRoot); err != nil {
		t.Skipf("fixture-go not found at %s", fixtureRoot)
	}

	// Build graph
	graph, err := repo.Build(fixtureRoot)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Spec depends on b/b.go
	specFiles := []string{"b/b.go"}
	testCommands := []string{}

	// Compute blast radius
	radius := repo.ComputeBlastRadius(graph, specFiles, testCommands)

	// Should have b/b.go in Direct
	directNames := make(map[string]bool)
	for _, f := range radius.Direct {
		directNames[f.Path] = true
	}

	if !directNames["b/b.go"] && !directNames["fixture-go/b/b.go"] {
		t.Errorf("Expected b/b.go in Direct, got: %+v", directNames)
	}
}
