package spec

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/karthikcodes/aetronyx/internal/llm"
)

var (
	knownGates  = map[string]bool{"planning": true, "schema_change": true, "pre_merge": true, "iteration": true}
	mapsToRegex = regexp.MustCompile(`^(acceptance_criteria|invariants)\[(\d+)\]$`)
)

func checkDependencyFiles(s *Spec, workspace string, r *ValidationResult, add func(ValidationError)) {
	count := 0
	for _, pattern := range s.Dependencies.Files {
		full := pattern
		if !filepath.IsAbs(pattern) {
			full = filepath.Join(workspace, pattern)
		}
		matches, err := filepath.Glob(full)
		if err != nil || len(matches) == 0 {
			add(ValidationError{Rule: "dependencies.files", Severity: "fatal", Field: "dependencies.files",
				Message: fmt.Sprintf("no files matched pattern %q in workspace", pattern)})
		} else {
			count += len(matches)
		}
	}
	r.Computed["dependency_count"] = count
}

func checkDependencyServices(s *Spec, add func(ValidationError)) {
	portMap := map[string]string{"redis": "6379", "postgres": "5432"}
	var unreachable []string
	for _, svc := range s.Dependencies.Services {
		name := strings.ToLower(svc)
		if port, ok := portMap[name]; ok {
			conn, err := net.DialTimeout("tcp", "localhost:"+port, 2*time.Second)
			if err != nil {
				unreachable = append(unreachable, svc)
			} else {
				conn.Close()
			}
			continue
		}
		if name == "docker" {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			err := exec.CommandContext(ctx, "docker", "ps").Run()
			cancel()
			if err != nil {
				unreachable = append(unreachable, svc)
			}
		}
	}
	if len(unreachable) > 0 {
		add(ValidationError{Rule: "dependencies.services", Severity: "warning", Field: "dependencies.services",
			Message: fmt.Sprintf("services not reachable: %s", strings.Join(unreachable, ", "))})
	}
}

func checkTestContractCommands(s *Spec, add func(ValidationError)) {
	var missing []string
	for _, tc := range s.TestContracts {
		if tc.Command == "" {
			continue
		}
		parts := strings.Fields(tc.Command)
		if len(parts) == 0 {
			continue
		}
		if _, err := exec.LookPath(parts[0]); err != nil {
			missing = append(missing, parts[0])
		}
	}
	if len(missing) > 0 {
		add(ValidationError{Rule: "test_contracts.commands", Severity: "fatal", Field: "test_contracts",
			Message: fmt.Sprintf("commands not found in PATH: %s", strings.Join(missing, ", "))})
	}
}

func checkTestContractReferences(s *Spec, add func(ValidationError)) {
	for i, tc := range s.TestContracts {
		for _, ref := range tc.MapsTo {
			m := mapsToRegex.FindStringSubmatch(ref)
			if m == nil {
				add(ValidationError{Rule: "test_contracts.references", Severity: "fatal",
					Field:   fmt.Sprintf("test_contracts[%d].maps_to", i),
					Message: fmt.Sprintf("invalid reference %q; must be acceptance_criteria[N] or invariants[N]", ref)})
				continue
			}
			idx, _ := strconv.Atoi(m[2])
			switch m[1] {
			case "acceptance_criteria":
				if idx >= len(s.AcceptanceCriteria) {
					add(ValidationError{Rule: "test_contracts.references", Severity: "fatal",
						Field:   fmt.Sprintf("test_contracts[%d].maps_to", i),
						Message: fmt.Sprintf("acceptance_criteria[%d] out of range (len=%d)", idx, len(s.AcceptanceCriteria))})
				}
			case "invariants":
				if idx >= len(s.Invariants) {
					add(ValidationError{Rule: "test_contracts.references", Severity: "fatal",
						Field:   fmt.Sprintf("test_contracts[%d].maps_to", i),
						Message: fmt.Sprintf("invariants[%d] out of range (len=%d)", idx, len(s.Invariants))})
				}
			}
		}
	}
}

func checkApprovalGateHooks(s *Spec, add func(ValidationError)) {
	validate := func(name, field string) {
		if name == "" || knownGates[name] || strings.HasPrefix(name, "custom:") {
			return
		}
		add(ValidationError{Rule: "approval_gates.hooks", Severity: "fatal", Field: field,
			Message:       fmt.Sprintf("unknown hook %q; must be one of {planning, schema_change, pre_merge, iteration} or start with \"custom:\"", name),
			FixSuggestion: "use a known hook name or prefix with \"custom:\""})
	}
	for i, gate := range s.ApprovalGates {
		validate(gate.After, fmt.Sprintf("approval_gates[%d].after", i))
		validate(gate.Before, fmt.Sprintf("approval_gates[%d].before", i))
	}
}

func checkRoutingHintModels(s *Spec, adapters []string, add func(ValidationError)) {
	adapterSet := make(map[string]bool, len(adapters))
	for _, a := range adapters {
		adapterSet[a] = true
	}
	available := func() string {
		var names []string
		for k := range llm.Pricing {
			names = append(names, k)
		}
		names = append(names, adapters...)
		return strings.Join(names, ", ")
	}
	check := func(model, field string) {
		if model == "" {
			return
		}
		if _, ok := llm.Pricing[model]; !ok && !adapterSet[model] {
			add(ValidationError{Rule: "routing_hint.models", Severity: "fatal", Field: field,
				Message: fmt.Sprintf("unknown model %q; available: %s", model, available())})
		}
	}
	check(s.RoutingHint.PlanningModel, "routing_hint.planning_model")
	check(s.RoutingHint.ExecutionModel, "routing_hint.execution_model")
}

func checkOutOfScopeConflict(s *Spec, add func(ValidationError)) {
	fileSet := make(map[string]bool, len(s.Dependencies.Files))
	for _, f := range s.Dependencies.Files {
		fileSet[f] = true
	}
	for _, oos := range s.OutOfScope {
		if fileSet[oos] {
			add(ValidationError{Rule: "out_of_scope.conflict", Severity: "warning", Field: "out_of_scope",
				Message: fmt.Sprintf("%q appears in both dependencies.files and out_of_scope", oos)})
		}
	}
}
