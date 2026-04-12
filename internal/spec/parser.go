package spec

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

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
	if spec.Version != "1" {
		return FatalError{err: ValidationError{
			Rule:     "schema",
			Severity: "fatal",
			Field:    "spec_version",
			Message:  fmt.Sprintf("must be \"1\", got %q", spec.Version),
		}}
	}
	if spec.Name == "" {
		return FatalError{err: ValidationError{
			Rule:     "schema",
			Severity: "fatal",
			Field:    "name",
			Message:  "is required and cannot be empty",
		}}
	}
	if spec.Intent == "" {
		return FatalError{err: ValidationError{
			Rule:     "schema",
			Severity: "fatal",
			Field:    "intent",
			Message:  "is required and cannot be empty",
		}}
	}
	return nil
}
