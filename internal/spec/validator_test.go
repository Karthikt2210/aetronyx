package spec

import (
	"os"
	"path/filepath"
	"testing"
)

// minSpec returns a Spec that satisfies all rules except those under test.
func minSpec() *Spec {
	return &Spec{
		Version: "1",
		Name:    "my-spec",
		Intent:  "This intent is long enough to pass the minimum length requirement very comfortably.",
		AcceptanceCriteria: []AcceptanceCriterion{
			{Given: "a user", When: "they act", Then: "something happens"},
		},
		OutOfScope: []string{"out of scope item"},
	}
}

func hasError(r ValidationResult, rule string) bool {
	for _, e := range r.Errors {
		if e.Rule == rule {
			return true
		}
	}
	return false
}

func hasWarning(r ValidationResult, rule string) bool {
	for _, w := range r.Warnings {
		if w.Rule == rule {
			return true
		}
	}
	return false
}

func TestRule1_SchemaFatal(t *testing.T) {
	ws := t.TempDir()
	tests := []struct {
		name  string
		mutate func(*Spec)
		field string
	}{
		{"missing version", func(s *Spec) { s.Version = "2" }, "spec_version"},
		{"missing name", func(s *Spec) { s.Name = "" }, "name"},
		{"missing intent", func(s *Spec) { s.Intent = "" }, "intent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := minSpec()
			tt.mutate(s)
			r := Validate(s, ws, nil)
			if r.OK {
				t.Errorf("expected OK=false, got true")
			}
			if !hasError(r, "schema") {
				t.Errorf("expected schema error, got %+v", r.Errors)
			}
		})
	}

	// passing case
	t.Run("valid schema", func(t *testing.T) {
		s := minSpec()
		r := Validate(s, ws, nil)
		if !r.OK {
			t.Errorf("expected OK=true, got errors: %+v", r.Errors)
		}
	})
}

func TestRule2_NameFormat(t *testing.T) {
	ws := t.TempDir()
	t.Run("valid name", func(t *testing.T) {
		s := minSpec()
		r := Validate(s, ws, nil)
		if hasError(r, "name.format") {
			t.Error("unexpected name.format error for valid name")
		}
	})
	t.Run("invalid name uppercase", func(t *testing.T) {
		s := minSpec()
		s.Name = "INVALID NAME!!"
		r := Validate(s, ws, nil)
		if !hasError(r, "name.format") {
			t.Error("expected name.format error")
		}
	})
	t.Run("invalid name leading hyphen", func(t *testing.T) {
		s := minSpec()
		s.Name = "-bad"
		r := Validate(s, ws, nil)
		if !hasError(r, "name.format") {
			t.Error("expected name.format error for leading hyphen")
		}
	})
}

func TestRule3_IntentLength(t *testing.T) {
	ws := t.TempDir()
	t.Run("fatal short <20", func(t *testing.T) {
		s := minSpec()
		s.Intent = "too short"
		r := Validate(s, ws, nil)
		if !hasError(r, "intent.length") {
			t.Error("expected intent.length fatal error")
		}
	})
	t.Run("warning 20-79 chars", func(t *testing.T) {
		s := minSpec()
		s.Intent = "This intent is exactly thirty chars long."
		r := Validate(s, ws, nil)
		if !hasWarning(r, "intent.length") {
			t.Error("expected intent.length warning")
		}
		if hasError(r, "intent.length") {
			t.Error("unexpected fatal error for 40-char intent")
		}
	})
	t.Run("no error >=80 chars", func(t *testing.T) {
		s := minSpec()
		r := Validate(s, ws, nil)
		if hasError(r, "intent.length") || hasWarning(r, "intent.length") {
			t.Error("unexpected intent.length issue for long intent")
		}
	})
}

func TestRule4_BudgetSanity(t *testing.T) {
	ws := t.TempDir()
	t.Run("invalid max_cost_usd >10000", func(t *testing.T) {
		s := minSpec()
		s.Budget.MaxCostUSD = 99999
		r := Validate(s, ws, nil)
		if !hasError(r, "budget.sanity") {
			t.Error("expected budget.sanity error for max_cost_usd=99999")
		}
	})
	t.Run("invalid max_iterations >500", func(t *testing.T) {
		s := minSpec()
		s.Budget.MaxIterations = 600
		r := Validate(s, ws, nil)
		if !hasError(r, "budget.sanity") {
			t.Error("expected budget.sanity error for max_iterations=600")
		}
	})
	t.Run("zero fields skipped", func(t *testing.T) {
		s := minSpec()
		// zero value means not set — should not trigger
		r := Validate(s, ws, nil)
		if hasError(r, "budget.sanity") {
			t.Error("unexpected budget.sanity error for zero budget fields")
		}
	})
	t.Run("invalid_budget fixture", func(t *testing.T) {
		r := ValidateFile(filepath.Join("testdata", "invalid_budget.spec.yaml"), ws, nil)
		if !hasError(r, "budget.sanity") {
			t.Error("expected budget.sanity error from fixture")
		}
	})
}

func TestRule5_BudgetFeasibility(t *testing.T) {
	ws := t.TempDir()
	t.Run("warning when budget too low", func(t *testing.T) {
		s := minSpec()
		s.Budget.MaxCostUSD = 0.000001
		s.Budget.MaxIterations = 5
		s.RoutingHint.PlanningModel = "claude-sonnet-4-6"
		s.RoutingHint.ExecutionModel = "claude-sonnet-4-6"
		r := Validate(s, ws, nil)
		if !hasWarning(r, "budget.feasibility") {
			t.Error("expected budget.feasibility warning")
		}
		if _, ok := r.Computed["min_cost_usd"]; !ok {
			t.Error("expected min_cost_usd in Computed")
		}
	})
	t.Run("no warning for adequate budget", func(t *testing.T) {
		s := minSpec()
		s.Budget.MaxCostUSD = 100
		s.Budget.MaxIterations = 5
		s.RoutingHint.PlanningModel = "claude-haiku-4-5-20251001"
		s.RoutingHint.ExecutionModel = "claude-haiku-4-5-20251001"
		r := Validate(s, ws, nil)
		if hasWarning(r, "budget.feasibility") {
			t.Error("unexpected budget.feasibility warning for adequate budget")
		}
	})
}

func TestRule6_AcceptanceCriteriaPresent(t *testing.T) {
	ws := t.TempDir()
	t.Run("no criteria", func(t *testing.T) {
		s := minSpec()
		s.AcceptanceCriteria = nil
		r := Validate(s, ws, nil)
		if !hasError(r, "acceptance_criteria.present") {
			t.Error("expected acceptance_criteria.present error")
		}
	})
	t.Run("empty then field", func(t *testing.T) {
		s := minSpec()
		s.AcceptanceCriteria = []AcceptanceCriterion{{Given: "g", When: "w", Then: ""}}
		r := Validate(s, ws, nil)
		if !hasError(r, "acceptance_criteria.present") {
			t.Error("expected error for empty then field")
		}
	})
	t.Run("valid criteria", func(t *testing.T) {
		r := Validate(minSpec(), ws, nil)
		if hasError(r, "acceptance_criteria.present") {
			t.Error("unexpected acceptance_criteria.present error")
		}
	})
}

func TestRule7_InvariantsLength(t *testing.T) {
	ws := t.TempDir()
	t.Run("short invariant", func(t *testing.T) {
		s := minSpec()
		s.Invariants = []string{"too short"}
		r := Validate(s, ws, nil)
		if !hasWarning(r, "invariants.length") {
			t.Error("expected invariants.length warning")
		}
	})
	t.Run("valid invariants", func(t *testing.T) {
		s := minSpec()
		s.Invariants = []string{"This invariant is long enough to pass."}
		r := Validate(s, ws, nil)
		if hasWarning(r, "invariants.length") {
			t.Error("unexpected invariants.length warning")
		}
	})
}

func TestRule8_OutOfScopeWarning(t *testing.T) {
	ws := t.TempDir()
	t.Run("no out_of_scope warns", func(t *testing.T) {
		s := minSpec()
		s.OutOfScope = nil
		r := Validate(s, ws, nil)
		if !hasWarning(r, "out_of_scope.present") {
			t.Error("expected out_of_scope.present warning")
		}
	})
	t.Run("with out_of_scope no warn", func(t *testing.T) {
		r := Validate(minSpec(), ws, nil)
		if hasWarning(r, "out_of_scope.present") {
			t.Error("unexpected out_of_scope.present warning")
		}
	})
}

func TestRule9_DependencyFiles(t *testing.T) {
	ws := t.TempDir()
	// create a file for the glob to match
	if err := os.WriteFile(filepath.Join(ws, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Run("matching glob", func(t *testing.T) {
		s := minSpec()
		s.Dependencies.Files = []string{"*.go"}
		r := Validate(s, ws, nil)
		if hasError(r, "dependencies.files") {
			t.Error("unexpected dependencies.files error for matching glob")
		}
		if r.Computed["dependency_count"] == 0 {
			t.Error("expected non-zero dependency_count")
		}
	})
	t.Run("non-matching glob", func(t *testing.T) {
		s := minSpec()
		s.Dependencies.Files = []string{"*.nonexistent"}
		r := Validate(s, ws, nil)
		if !hasError(r, "dependencies.files") {
			t.Error("expected dependencies.files error for no-match glob")
		}
	})
}

func TestRule10_ServicesWarning(t *testing.T) {
	ws := t.TempDir()
	t.Run("unreachable redis", func(t *testing.T) {
		s := minSpec()
		s.Dependencies.Services = []string{"redis"}
		r := Validate(s, ws, nil)
		// Redis is likely not running; expect a warning (or none if it happens to be)
		// We can't assert presence because CI may have redis; just verify no crash.
		_ = r
	})
	t.Run("unknown service no warning", func(t *testing.T) {
		s := minSpec()
		s.Dependencies.Services = []string{"custom-api"}
		r := Validate(s, ws, nil)
		if hasWarning(r, "dependencies.services") {
			t.Error("unexpected services warning for unknown service name")
		}
	})
}

func TestRule11_TestContractCommands(t *testing.T) {
	ws := t.TempDir()
	t.Run("known command go", func(t *testing.T) {
		s := minSpec()
		s.TestContracts = []TestContract{{Name: "tests", Command: "go test ./...", MapsTo: []string{"acceptance_criteria[0]"}}}
		r := Validate(s, ws, nil)
		if hasError(r, "test_contracts.commands") {
			t.Error("unexpected test_contracts.commands error for 'go'")
		}
	})
	t.Run("missing binary", func(t *testing.T) {
		s := minSpec()
		s.TestContracts = []TestContract{{Name: "tests", Command: "nonexistent-binary-xyz ./...", MapsTo: []string{"acceptance_criteria[0]"}}}
		r := Validate(s, ws, nil)
		if !hasError(r, "test_contracts.commands") {
			t.Error("expected test_contracts.commands error for missing binary")
		}
	})
}

func TestRule12_TestContractReferences(t *testing.T) {
	ws := t.TempDir()
	t.Run("valid reference", func(t *testing.T) {
		s := minSpec()
		s.TestContracts = []TestContract{{Name: "t", Command: "go test", MapsTo: []string{"acceptance_criteria[0]"}}}
		r := Validate(s, ws, nil)
		if hasError(r, "test_contracts.references") {
			t.Error("unexpected test_contracts.references error")
		}
	})
	t.Run("out of range index", func(t *testing.T) {
		s := minSpec()
		s.TestContracts = []TestContract{{Name: "t", Command: "go test", MapsTo: []string{"acceptance_criteria[5]"}}}
		r := Validate(s, ws, nil)
		if !hasError(r, "test_contracts.references") {
			t.Error("expected test_contracts.references error for out-of-range index")
		}
	})
	t.Run("invalid format", func(t *testing.T) {
		s := minSpec()
		s.TestContracts = []TestContract{{Name: "t", Command: "go test", MapsTo: []string{"bad-format"}}}
		r := Validate(s, ws, nil)
		if !hasError(r, "test_contracts.references") {
			t.Error("expected test_contracts.references error for bad format")
		}
	})
}

func TestRule13_ApprovalGateHooks(t *testing.T) {
	ws := t.TempDir()
	t.Run("known hook", func(t *testing.T) {
		s := minSpec()
		s.ApprovalGates = []ApprovalGate{{After: "planning", Required: true}}
		r := Validate(s, ws, nil)
		if hasError(r, "approval_gates.hooks") {
			t.Error("unexpected approval_gates.hooks error for known hook")
		}
	})
	t.Run("custom prefix hook", func(t *testing.T) {
		s := minSpec()
		s.ApprovalGates = []ApprovalGate{{After: "custom:my-hook", Required: true}}
		r := Validate(s, ws, nil)
		if hasError(r, "approval_gates.hooks") {
			t.Error("unexpected approval_gates.hooks error for custom: hook")
		}
	})
	t.Run("unknown hook", func(t *testing.T) {
		s := minSpec()
		s.ApprovalGates = []ApprovalGate{{After: "invalid-hook", Required: true}}
		r := Validate(s, ws, nil)
		if !hasError(r, "approval_gates.hooks") {
			t.Error("expected approval_gates.hooks error for unknown hook")
		}
	})
}

func TestRule14_RoutingHintModels(t *testing.T) {
	ws := t.TempDir()
	t.Run("model in pricing", func(t *testing.T) {
		s := minSpec()
		s.RoutingHint.PlanningModel = "claude-sonnet-4-6"
		r := Validate(s, ws, nil)
		if hasError(r, "routing_hint.models") {
			t.Error("unexpected routing_hint.models error for known model")
		}
	})
	t.Run("model in adapters", func(t *testing.T) {
		s := minSpec()
		s.RoutingHint.PlanningModel = "ollama/llama3"
		r := Validate(s, ws, []string{"ollama/llama3"})
		if hasError(r, "routing_hint.models") {
			t.Error("unexpected routing_hint.models error for adapter model")
		}
	})
	t.Run("unknown model", func(t *testing.T) {
		s := minSpec()
		s.RoutingHint.PlanningModel = "gpt-99-turbo"
		r := Validate(s, ws, nil)
		if !hasError(r, "routing_hint.models") {
			t.Error("expected routing_hint.models error for unknown model")
		}
	})
}

func TestRule15_OutOfScopeConflict(t *testing.T) {
	ws := t.TempDir()
	t.Run("no conflict", func(t *testing.T) {
		r := Validate(minSpec(), ws, nil)
		if hasWarning(r, "out_of_scope.conflict") {
			t.Error("unexpected out_of_scope.conflict warning")
		}
	})
	t.Run("conflict detected", func(t *testing.T) {
		s := minSpec()
		s.Dependencies.Files = []string{"internal/auth.go"}
		s.OutOfScope = []string{"internal/auth.go"}
		r := Validate(s, ws, nil)
		if !hasWarning(r, "out_of_scope.conflict") {
			t.Error("expected out_of_scope.conflict warning")
		}
	})
}

func TestValidateFile_Valid(t *testing.T) {
	ws := t.TempDir()
	r := ValidateFile(filepath.Join("testdata", "valid_full.spec.yaml"), ws, nil)
	if !r.OK {
		t.Errorf("expected OK=true for valid_full.spec.yaml, errors: %+v", r.Errors)
	}
}

func TestValidateFile_InvalidName(t *testing.T) {
	ws := t.TempDir()
	r := ValidateFile(filepath.Join("testdata", "invalid_name.spec.yaml"), ws, nil)
	if r.OK {
		t.Error("expected OK=false for invalid_name.spec.yaml")
	}
	if !hasError(r, "name.format") {
		t.Errorf("expected name.format error, got %+v", r.Errors)
	}
}

func TestValidateFile_MissingFile(t *testing.T) {
	ws := t.TempDir()
	r := ValidateFile(filepath.Join(ws, "nonexistent.spec.yaml"), ws, nil)
	if r.OK {
		t.Error("expected OK=false for missing file")
	}
	if !hasError(r, "parse") {
		t.Errorf("expected parse error, got %+v", r.Errors)
	}
}

func TestRule4_BudgetSanity_AllFields(t *testing.T) {
	ws := t.TempDir()
	t.Run("invalid max_wall_time_minutes", func(t *testing.T) {
		s := minSpec()
		s.Budget.MaxWallTimeMins = 999
		r := Validate(s, ws, nil)
		if !hasError(r, "budget.sanity") {
			t.Error("expected budget.sanity error for max_wall_time_minutes=999")
		}
	})
	t.Run("invalid max_tokens too large", func(t *testing.T) {
		s := minSpec()
		s.Budget.MaxTokens = 20_000_000
		r := Validate(s, ws, nil)
		if !hasError(r, "budget.sanity") {
			t.Error("expected budget.sanity error for max_tokens=20M")
		}
	})
	t.Run("invalid max_tokens too small", func(t *testing.T) {
		s := minSpec()
		s.Budget.MaxTokens = 50
		r := Validate(s, ws, nil)
		if !hasError(r, "budget.sanity") {
			t.Error("expected budget.sanity error for max_tokens=50")
		}
	})
}

func TestErrorStrings(t *testing.T) {
	ve := ValidationError{Rule: "schema", Severity: "fatal", Field: "name", Message: "required"}
	if ve.Error() == "" {
		t.Error("ValidationError.Error() should not be empty")
	}
	fe := FatalError{err: ve}
	if fe.Error() == "" {
		t.Error("FatalError.Error() should not be empty")
	}
}
