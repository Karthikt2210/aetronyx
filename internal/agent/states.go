// Package agent implements the M2 iterative agent loop state machine.
package agent

import "fmt"

// Run state constants — must match values stored in the runs table.
const (
	// M1 states (kept for backward compat).
	StateCreated   = "created"
	StatePlanning  = "planning"
	StateRunning   = "running" // M1 compat — use StateIterating in M2
	StateCompleted = "completed"
	StateFailed    = "failed"
	StateCancelled = "cancelled"

	// M2 states.
	StateBlastRadius    = "blast_radius"
	StateIterating      = "iterating"
	StateVerifying      = "verifying"
	StateHaltedMaxIters = "halted_max_iters"
	StateHaltedMaxTime  = "halted_max_time"
)

// validTransitions is the adjacency map for the M2 state machine.
// M1 edges are preserved for backward compatibility.
var validTransitions = map[string][]string{
	// M1 compat edges.
	StateRunning: {StateCompleted, StateFailed, StateCancelled},

	// M2 edges.
	StateCreated: {
		StateBlastRadius, StatePlanning, StateCancelled,
	},
	StateBlastRadius: {
		StatePlanning, StateFailed, StateCancelled,
	},
	StatePlanning: {
		StateIterating, StateRunning, StateFailed, StateCancelled,
	},
	StateIterating: {
		StateVerifying, StateHaltedMaxIters, StateHaltedMaxTime, StateFailed, StateCancelled,
	},
	StateVerifying: {
		StateIterating, StateCompleted, StateFailed, StateCancelled,
	},

	// Terminal states have no outbound edges.
	StateCompleted:      {},
	StateFailed:         {},
	StateCancelled:      {},
	StateHaltedMaxIters: {},
	StateHaltedMaxTime:  {},
}

// ValidTransitions returns a copy of the state transition graph.
// Callers may inspect but not mutate it.
func ValidTransitions() map[string][]string {
	out := make(map[string][]string, len(validTransitions))
	for k, v := range validTransitions {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// CanTransition reports whether transitioning from → to is permitted.
func CanTransition(from, to string) bool {
	edges, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, e := range edges {
		if e == to {
			return true
		}
	}
	return false
}

// ErrInvalidTransition is returned when a state transition is not permitted.
type ErrInvalidTransition struct {
	From string
	To   string
}

func (e *ErrInvalidTransition) Error() string {
	return fmt.Sprintf("invalid state transition: %s → %s", e.From, e.To)
}
