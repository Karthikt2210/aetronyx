// Package agent implements the M1 single-shot agent loop state machine.
package agent

import "fmt"

// Run state constants — must match values stored in the runs table.
const (
	StateCreated   = "created"
	StatePlanning  = "planning"
	StateRunning   = "running"
	StateCompleted = "completed"
	StateFailed    = "failed"
	StateCancelled = "cancelled"
)

// validTransitions is the adjacency map for the M1 state machine.
// Edges are one-directional. Every legal (from→to) pair is listed.
var validTransitions = map[string][]string{
	StateCreated:  {StatePlanning, StateCancelled},
	StatePlanning: {StateRunning, StateFailed, StateCancelled},
	StateRunning:  {StateCompleted, StateFailed, StateCancelled},
	// Terminal states have no outbound edges.
	StateCompleted: {},
	StateFailed:    {},
	StateCancelled: {},
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
