package spec

import (
	"encoding/json"
	"testing"
)

func TestGenerateSchema(t *testing.T) {
	data, err := GenerateSchema()
	if err != nil {
		t.Fatalf("GenerateSchema() error: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("GenerateSchema() returned empty data")
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("GenerateSchema() returned invalid JSON: %v", err)
	}

	// Verify top-level structure
	if _, ok := schema["$schema"]; !ok {
		t.Error("Schema missing $schema field")
	}
	if _, ok := schema["title"]; !ok {
		t.Error("Schema missing title field")
	}
	if _, ok := schema["type"]; !ok {
		t.Error("Schema missing type field")
	}

	// Verify properties exist
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema properties is not a map")
	}

	requiredProps := []string{"spec_version", "name", "intent"}
	for _, prop := range requiredProps {
		if _, ok := properties[prop]; !ok {
			t.Errorf("Schema missing property: %s", prop)
		}
	}

	// Verify required array
	required, ok := schema["required"].([]interface{})
	if !ok {
		t.Fatal("Schema required is not an array")
	}
	if len(required) < 3 {
		t.Errorf("Schema required array too short: got %d, want at least 3", len(required))
	}
}

func TestSchemaRequiredFields(t *testing.T) {
	data, err := GenerateSchema()
	if err != nil {
		t.Fatalf("GenerateSchema() error: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("Failed to unmarshal schema: %v", err)
	}

	required, ok := schema["required"].([]interface{})
	if !ok {
		t.Fatal("Schema required is not an array")
	}

	requiredFields := make(map[string]bool)
	for _, field := range required {
		if str, ok := field.(string); ok {
			requiredFields[str] = true
		}
	}

	for _, expectedField := range []string{"spec_version", "name", "intent"} {
		if !requiredFields[expectedField] {
			t.Errorf("Required field missing from schema: %s", expectedField)
		}
	}
}

func TestSchemaValidJSON(t *testing.T) {
	data, err := GenerateSchema()
	if err != nil {
		t.Fatalf("GenerateSchema() error: %v", err)
	}

	// Verify it's valid JSON by unmarshaling into a generic map
	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("GenerateSchema() returned invalid JSON: %v", err)
	}

	// Verify it can be re-marshaled
	if _, err := json.MarshalIndent(result, "", "  "); err != nil {
		t.Fatalf("Generated schema cannot be re-marshaled: %v", err)
	}
}
