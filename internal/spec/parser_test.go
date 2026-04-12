package spec

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseValidMinimalSpec tests parsing a minimal valid spec.
func TestParseValidMinimalSpec(t *testing.T) {
	yaml := `
spec_version: "1"
name: test-spec
intent: This is a test specification for validation purposes.
`
	spec, err := ParseBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if spec.Name != "test-spec" {
		t.Errorf("expected name %q, got %q", "test-spec", spec.Name)
	}
	if spec.Version != "1" {
		t.Errorf("expected version %q, got %q", "1", spec.Version)
	}
	if spec.Intent != "This is a test specification for validation purposes." {
		t.Errorf("unexpected intent value")
	}
}

// TestParseMissingName tests that missing name field results in FatalError.
func TestParseMissingName(t *testing.T) {
	yaml := `
spec_version: "1"
intent: This is a test specification.
`
	_, err := ParseBytes([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing name")
	}

	var ve ValidationError
	if !IsFatalError(err) || !parseAsValidationError(err, &ve) {
		t.Fatalf("expected FatalError with ValidationError, got %T: %v", err, err)
	}
	if ve.Field != "name" {
		t.Errorf("expected Field %q, got %q", "name", ve.Field)
	}
}

// TestParseBadYAML tests that malformed YAML results in an error.
func TestParseBadYAML(t *testing.T) {
	yaml := `
spec_version: "1"
name: test
intent: test
  bad: [indentation
`
	_, err := ParseBytes([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for bad YAML")
	}
	// We don't check for FatalError here since YAML parse errors are not wrapped as FatalError
}

// TestParseWrongVersion tests that spec_version != "1" results in FatalError.
func TestParseWrongVersion(t *testing.T) {
	yaml := `
spec_version: "2"
name: test-spec
intent: This is a test specification.
`
	_, err := ParseBytes([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for wrong spec_version")
	}

	var ve ValidationError
	if !IsFatalError(err) || !parseAsValidationError(err, &ve) {
		t.Fatalf("expected FatalError with ValidationError, got %T: %v", err, err)
	}
	if ve.Field != "spec_version" {
		t.Errorf("expected Field %q, got %q", "spec_version", ve.Field)
	}
}

// TestParseFullSpec tests parsing a complete spec file (examples/01-add-rate-limiting.spec.yaml).
func TestParseFullSpec(t *testing.T) {
	// Construct path to example spec
	exePath := filepath.Join("..", "..", "examples", "01-add-rate-limiting.spec.yaml")

	// Get absolute path if this is run from test directory
	if !filepath.IsAbs(exePath) {
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get working directory: %v", err)
		}
		// Navigate to project root
		exePath = filepath.Join(wd, "..", "..", "examples", "01-add-rate-limiting.spec.yaml")
	}

	spec, err := Parse(exePath)
	if err != nil {
		t.Fatalf("failed to parse example spec: %v", err)
	}

	// Validate expected fields from the example
	if spec.Name != "add-rate-limiting-to-auth-login" {
		t.Errorf("expected name %q, got %q", "add-rate-limiting-to-auth-login", spec.Name)
	}

	if spec.Budget.MaxCostUSD != 3.00 {
		t.Errorf("expected max_cost_usd 3.00, got %v", spec.Budget.MaxCostUSD)
	}

	if len(spec.AcceptanceCriteria) != 4 {
		t.Errorf("expected 4 acceptance criteria, got %d", len(spec.AcceptanceCriteria))
	}

	if spec.RoutingHint.PlanningModel != "claude-opus-4" {
		t.Errorf("expected planning_model %q, got %q", "claude-opus-4", spec.RoutingHint.PlanningModel)
	}
}

// parseAsValidationError is a helper to extract ValidationError from an error chain.
func parseAsValidationError(err error, ve *ValidationError) bool {
	var fe FatalError
	if e, ok := err.(FatalError); ok {
		fe = e
	} else {
		return false
	}

	if v, ok := fe.err.(ValidationError); ok {
		*ve = v
		return true
	}
	return false
}
