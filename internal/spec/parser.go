package spec

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ValidationError represents a validation failure for a specific field.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// FatalError represents a fatal validation error that should exit with code 10.
type FatalError struct {
	err error
}

func (e FatalError) Error() string {
	return e.err.Error()
}

// Parse reads and parses a YAML spec file from the given path.
func Parse(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseBytes(data)
}

// ParseBytes parses a YAML spec from byte data.
func ParseBytes(data []byte) (*Spec, error) {
	var spec Spec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// M1 validation: check required fields and version
	if err := validateM1(&spec); err != nil {
		return nil, err
	}

	return &spec, nil
}

// validateM1 performs M1-level validation: spec_version, name, and intent.
func validateM1(spec *Spec) error {
	// Check spec_version
	if spec.Version != "1" {
		return FatalError{
			err: ValidationError{
				Field:   "spec_version",
				Message: fmt.Sprintf("must be \"1\", got %q", spec.Version),
			},
		}
	}

	// Check name is not empty
	if spec.Name == "" {
		return FatalError{
			err: ValidationError{
				Field:   "name",
				Message: "is required and cannot be empty",
			},
		}
	}

	// Check intent is not empty
	if spec.Intent == "" {
		return FatalError{
			err: ValidationError{
				Field:   "intent",
				Message: "is required and cannot be empty",
			},
		}
	}

	return nil
}

// IsFatalError checks if an error is a FatalError.
func IsFatalError(err error) bool {
	var fe FatalError
	return errors.As(err, &fe)
}
