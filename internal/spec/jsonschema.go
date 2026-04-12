package spec

import (
	"encoding/json"
)

// GenerateSchema returns a JSON Schema (draft-07) describing the Spec structure.
func GenerateSchema() ([]byte, error) {
	descriptions := map[string]string{
		"spec_version":        "The spec format version, must be '1'",
		"name":                "Unique identifier for the spec, lowercase alphanumeric with hyphens",
		"intent":              "High-level description of what needs to be done",
		"budget":              "Resource constraints and limits for execution",
		"max_cost_usd":        "Maximum cost in USD for the entire run",
		"max_iterations":      "Maximum number of iterations allowed",
		"max_wall_time_minutes": "Maximum wall-clock time in minutes",
		"max_tokens":          "Maximum tokens allowed for LLM calls",
		"acceptance_criteria": "List of testable Given-When-Then conditions",
		"given":               "The initial context or state",
		"when":                "The action or condition being tested",
		"then":                "The expected outcome",
		"invariants":          "Constraints that must remain true throughout execution",
		"out_of_scope":        "Explicitly excluded items or areas",
		"dependencies":        "External files, services, and APIs required",
		"files":               "File paths or glob patterns the spec depends on",
		"services":            "External services required (redis, postgres, docker, etc.)",
		"apis":                "External APIs or integrations needed",
		"test_contracts":      "Commands to verify acceptance criteria and invariants",
		"maps_to":             "Acceptance criteria or invariant indices this test validates",
		"approval_gates":      "Checkpoints requiring human approval during execution",
		"after":               "Phase after which approval is required",
		"before":              "Phase before which approval is required",
		"required":            "Whether approval is mandatory",
		"routing_hint":        "LLM model suggestions for planning and execution",
		"planning_model":      "Model to use for planning phase",
		"execution_model":     "Model to use for execution phase",
		"metadata":            "Custom key-value metadata",
	}

	schema := map[string]interface{}{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"title":   "AetronyxSpec",
		"type":    "object",
		"properties": map[string]interface{}{
			"spec_version": map[string]interface{}{
				"type":        "string",
				"description": descriptions["spec_version"],
				"pattern":     "^1$",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": descriptions["name"],
				"pattern":     "^[a-z0-9][a-z0-9-]{0,63}$",
			},
			"intent": map[string]interface{}{
				"type":        "string",
				"description": descriptions["intent"],
				"minLength":   20,
			},
			"budget": map[string]interface{}{
				"type":        "object",
				"description": descriptions["budget"],
				"properties": map[string]interface{}{
					"max_cost_usd": map[string]interface{}{
						"type":        "number",
						"description": descriptions["max_cost_usd"],
						"minimum":     0,
						"maximum":     10000,
					},
					"max_iterations": map[string]interface{}{
						"type":        "integer",
						"description": descriptions["max_iterations"],
						"minimum":     1,
						"maximum":     500,
					},
					"max_wall_time_minutes": map[string]interface{}{
						"type":        "integer",
						"description": descriptions["max_wall_time_minutes"],
						"minimum":     1,
						"maximum":     720,
					},
					"max_tokens": map[string]interface{}{
						"type":        "integer",
						"description": descriptions["max_tokens"],
						"minimum":     1000,
						"maximum":     10000000,
					},
				},
			},
			"acceptance_criteria": map[string]interface{}{
				"type":        "array",
				"description": descriptions["acceptance_criteria"],
				"items": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{
						"given": map[string]interface{}{
							"type":        "string",
							"description": descriptions["given"],
						},
						"when": map[string]interface{}{
							"type":        "string",
							"description": descriptions["when"],
						},
						"then": map[string]interface{}{
							"type":        "string",
							"description": descriptions["then"],
						},
					},
					"required": []string{"given", "when", "then"},
				},
			},
			"invariants": map[string]interface{}{
				"type":        "array",
				"description": descriptions["invariants"],
				"items": map[string]interface{}{
					"type":      "string",
					"minLength": 10,
				},
			},
			"out_of_scope": map[string]interface{}{
				"type":        "array",
				"description": descriptions["out_of_scope"],
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			"dependencies": map[string]interface{}{
				"type":        "object",
				"description": descriptions["dependencies"],
				"properties": map[string]interface{}{
					"files": map[string]interface{}{
						"type":        "array",
						"description": descriptions["files"],
						"items": map[string]interface{}{
							"type": "string",
						},
					},
					"services": map[string]interface{}{
						"type":        "array",
						"description": descriptions["services"],
						"items": map[string]interface{}{
							"type": "string",
						},
					},
					"apis": map[string]interface{}{
						"type":        "array",
						"description": descriptions["apis"],
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
			"test_contracts": map[string]interface{}{
				"type":        "array",
				"description": descriptions["test_contracts"],
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Name of the test contract",
						},
						"command": map[string]interface{}{
							"type":        "string",
							"description": "Command to execute for testing",
						},
						"maps_to": map[string]interface{}{
							"type":        "array",
							"description": descriptions["maps_to"],
							"items": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"required": []string{"name", "command"},
				},
			},
			"approval_gates": map[string]interface{}{
				"type":        "array",
				"description": descriptions["approval_gates"],
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"after": map[string]interface{}{
							"type":        "string",
							"description": descriptions["after"],
						},
						"before": map[string]interface{}{
							"type":        "string",
							"description": descriptions["before"],
						},
						"required": map[string]interface{}{
							"type":        "boolean",
							"description": descriptions["required"],
						},
					},
				},
			},
			"routing_hint": map[string]interface{}{
				"type":        "object",
				"description": descriptions["routing_hint"],
				"properties": map[string]interface{}{
					"planning_model": map[string]interface{}{
						"type":        "string",
						"description": descriptions["planning_model"],
					},
					"execution_model": map[string]interface{}{
						"type":        "string",
						"description": descriptions["execution_model"],
					},
				},
			},
			"metadata": map[string]interface{}{
				"type":        "object",
				"description": descriptions["metadata"],
				"additionalProperties": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"required": []string{"spec_version", "name", "intent"},
	}

	return json.MarshalIndent(schema, "", "  ")
}
