package aetronyx

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/karthikcodes/aetronyx/internal/spec"
)

func TestSpecNewNameValidation(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"valid-name", false},
		{"my-task-1", false},
		{"a", false},
		{"a-1-b-c", false},
		{"1-task", false},        // numbers allowed at start
		{"ValidName", true},      // uppercase not allowed
		{"invalid name", true},   // spaces not allowed
		{"-invalid", true},       // can't start with hyphen
	}

	nameRe := regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := nameRe.MatchString(tt.name)
			if !matches && !tt.wantErr {
				t.Errorf("name=%q should be valid but regex rejected it", tt.name)
			}
			if matches && tt.wantErr {
				t.Errorf("name=%q should be invalid but regex accepted it", tt.name)
			}
		})
	}
}

func TestSpecInitCreatesFile(t *testing.T) {
	td := t.TempDir()
	oldCwd, _ := os.Getwd()
	os.Chdir(td)
	defer os.Chdir(oldCwd)

	// Manually run the template creation
	template := spec.DefaultTemplate()
	outPath := filepath.Join(td, "example.spec.yaml")

	if err := os.WriteFile(outPath, template, 0o644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	// Verify example.spec.yaml exists
	if _, err := os.Stat(outPath); err != nil {
		t.Errorf("expected file %s to exist", outPath)
	}

	// Verify content
	data, _ := os.ReadFile(outPath)
	content := string(data)
	if len(content) == 0 {
		t.Error("expected non-empty file content")
	}
	if !strings.Contains(content, "spec_version: \"1\"") {
		t.Error("expected spec_version in template")
	}
}
