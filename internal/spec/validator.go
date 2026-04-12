package spec

import (
	"fmt"
	"regexp"

	"github.com/karthikcodes/aetronyx/internal/llm"
)

var nameRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// Validate runs all 15 validation rules against s.
// workspace is the root directory used for glob expansion (rule 9).
// adapters is the set of known model IDs available in the current config (rule 14).
func Validate(s *Spec, workspace string, adapters []string) ValidationResult {
	r := ValidationResult{OK: true, Computed: make(map[string]any)}
	add := func(ve ValidationError) {
		if ve.Severity == "fatal" {
			r.OK = false
			r.Errors = append(r.Errors, ve)
		} else {
			r.Warnings = append(r.Warnings, ve)
		}
	}

	checkSchema(s, add)
	checkNameFormat(s, add)
	checkIntentLength(s, add)
	checkBudgetSanity(s, add)
	checkBudgetFeasibility(s, &r, add)
	checkAcceptanceCriteriaPresent(s, add)
	checkInvariantsLength(s, add)
	checkOutOfScopePresent(s, add)
	checkDependencyFiles(s, workspace, &r, add)
	checkDependencyServices(s, add)
	checkTestContractCommands(s, add)
	checkTestContractReferences(s, add)
	checkApprovalGateHooks(s, add)
	checkRoutingHintModels(s, adapters, add)
	checkOutOfScopeConflict(s, add)

	return r
}

// ValidateFile parses a spec file at path and runs the full validator.
func ValidateFile(path, workspace string, adapters []string) ValidationResult {
	s, err := Parse(path)
	if err != nil {
		return ValidationResult{
			OK: false,
			Errors: []ValidationError{{
				Rule: "parse", Severity: "fatal", Field: "file", Message: err.Error(),
			}},
			Computed: make(map[string]any),
		}
	}
	return Validate(s, workspace, adapters)
}

func checkSchema(s *Spec, add func(ValidationError)) {
	if s.Version != "1" {
		add(ValidationError{Rule: "schema", Severity: "fatal", Field: "spec_version",
			Message:       fmt.Sprintf("must be \"1\", got %q", s.Version),
			FixSuggestion: "set spec_version: \"1\""})
	}
	if s.Name == "" {
		add(ValidationError{Rule: "schema", Severity: "fatal", Field: "name",
			Message: "required and cannot be empty"})
	}
	if s.Intent == "" {
		add(ValidationError{Rule: "schema", Severity: "fatal", Field: "intent",
			Message: "required and cannot be empty"})
	}
}

func checkNameFormat(s *Spec, add func(ValidationError)) {
	if s.Name != "" && !nameRegexp.MatchString(s.Name) {
		add(ValidationError{Rule: "name.format", Severity: "fatal", Field: "name",
			Message:       fmt.Sprintf("must match ^[a-z0-9][a-z0-9-]{0,63}$, got %q", s.Name),
			FixSuggestion: "use lowercase letters, digits, and hyphens only"})
	}
}

func checkIntentLength(s *Spec, add func(ValidationError)) {
	l := len(s.Intent)
	switch {
	case l < 20:
		add(ValidationError{Rule: "intent.length", Severity: "fatal", Field: "intent",
			Message: fmt.Sprintf("must be at least 20 characters, got %d", l)})
	case l < 80:
		add(ValidationError{Rule: "intent.length", Severity: "warning", Field: "intent",
			Message: fmt.Sprintf("intent is short (%d chars); consider expanding to ≥80 for clarity", l)})
	}
}

func checkBudgetSanity(s *Spec, add func(ValidationError)) {
	b := s.Budget
	if b.MaxCostUSD != 0 && (b.MaxCostUSD <= 0 || b.MaxCostUSD > 10000) {
		add(ValidationError{Rule: "budget.sanity", Severity: "fatal", Field: "budget.max_cost_usd",
			Message: fmt.Sprintf("must be in (0, 10000], got %g", b.MaxCostUSD)})
	}
	if b.MaxIterations != 0 && (b.MaxIterations < 1 || b.MaxIterations > 500) {
		add(ValidationError{Rule: "budget.sanity", Severity: "fatal", Field: "budget.max_iterations",
			Message: fmt.Sprintf("must be in [1, 500], got %d", b.MaxIterations)})
	}
	if b.MaxWallTimeMins != 0 && (b.MaxWallTimeMins < 1 || b.MaxWallTimeMins > 720) {
		add(ValidationError{Rule: "budget.sanity", Severity: "fatal", Field: "budget.max_wall_time_minutes",
			Message: fmt.Sprintf("must be in [1, 720], got %d", b.MaxWallTimeMins)})
	}
	if b.MaxTokens != 0 && (b.MaxTokens < 1000 || b.MaxTokens > 10_000_000) {
		add(ValidationError{Rule: "budget.sanity", Severity: "fatal", Field: "budget.max_tokens",
			Message: fmt.Sprintf("must be in [1000, 10000000], got %d", b.MaxTokens)})
	}
}

func checkBudgetFeasibility(s *Spec, r *ValidationResult, add func(ValidationError)) {
	if s.Budget.MaxCostUSD == 0 || s.RoutingHint.PlanningModel == "" {
		return
	}
	planPricing, ok := llm.Pricing[s.RoutingHint.PlanningModel]
	if !ok {
		return
	}
	const (
		planOutputTokens = 256
		execOutputTokens = 512
		perMillion       = 1_000_000.0
	)
	floor := float64(planOutputTokens) / perMillion * planPricing.OutputPer1MUSD
	if ep, has := llm.Pricing[s.RoutingHint.ExecutionModel]; has && s.Budget.MaxIterations > 0 {
		floor += float64(s.Budget.MaxIterations) * float64(execOutputTokens) / perMillion * ep.OutputPer1MUSD
	}
	r.Computed["min_cost_usd"] = floor
	if s.Budget.MaxCostUSD < floor {
		add(ValidationError{Rule: "budget.feasibility", Severity: "warning", Field: "budget.max_cost_usd",
			Message: fmt.Sprintf("budget %.4f USD may be insufficient; estimated floor is %.4f USD",
				s.Budget.MaxCostUSD, floor)})
	}
}

func checkAcceptanceCriteriaPresent(s *Spec, add func(ValidationError)) {
	if len(s.AcceptanceCriteria) == 0 {
		add(ValidationError{Rule: "acceptance_criteria.present", Severity: "fatal",
			Field:   "acceptance_criteria",
			Message: "at least one acceptance criterion is required"})
		return
	}
	for i, ac := range s.AcceptanceCriteria {
		if ac.Given == "" || ac.When == "" || ac.Then == "" {
			add(ValidationError{Rule: "acceptance_criteria.present", Severity: "fatal",
				Field:   fmt.Sprintf("acceptance_criteria[%d]", i),
				Message: "given, when, and then fields are all required"})
		}
	}
}

func checkInvariantsLength(s *Spec, add func(ValidationError)) {
	for i, inv := range s.Invariants {
		if len(inv) < 10 {
			add(ValidationError{Rule: "invariants.length", Severity: "warning",
				Field:   fmt.Sprintf("invariants[%d]", i),
				Message: fmt.Sprintf("invariant is too short (%d chars); minimum is 10", len(inv))})
		}
	}
}

func checkOutOfScopePresent(s *Spec, add func(ValidationError)) {
	if len(s.OutOfScope) == 0 {
		add(ValidationError{Rule: "out_of_scope.present", Severity: "warning", Field: "out_of_scope",
			Message: "no out_of_scope entries; consider defining scope boundaries"})
	}
}
