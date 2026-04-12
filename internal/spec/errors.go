package spec

import (
	"errors"
	"fmt"
)

// ValidationError describes a single validation failure or warning from the spec validator.
type ValidationError struct {
	Rule          string
	Severity      string // "fatal" or "warning"
	Message       string
	Field         string
	FixSuggestion string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Rule, e.Field, e.Message)
}

// FatalError wraps an error to signal a fatal parse or validation failure.
type FatalError struct {
	err error
}

func (e FatalError) Error() string {
	return e.err.Error()
}

// IsFatalError reports whether err is a FatalError.
func IsFatalError(err error) bool {
	var fe FatalError
	return errors.As(err, &fe)
}

// ValidationResult is the output of the full 15-rule spec validator.
type ValidationResult struct {
	OK       bool
	Errors   []ValidationError
	Warnings []ValidationError
	Computed map[string]any
}
