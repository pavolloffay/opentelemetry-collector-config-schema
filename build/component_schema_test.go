package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestSchemaGenerationWithCustomComponent tests the schema generator with our custom test component
func TestSchemaGenerationWithCustomComponent(t *testing.T) {
	// Create our custom test component factory
	factory := NewFactory()

	// Get the default config
	defaultConfig := factory.CreateDefaultConfig()
	if defaultConfig == nil {
		t.Fatalf("Factory returned nil config")
	}

	// Create schema generator
	generator := NewSchemaGenerator("test_output")

	// Generate schema for our test component
	generatedSchema, err := generator.generateJSONSchema(defaultConfig)
	if err != nil {
		t.Fatalf("Failed to generate JSON schema: %v", err)
	}

	// Load expected schema
	expectedSchemaPath := filepath.Join("testdata", "expected_testcomponent_schema.json")
	expectedSchemaBytes, err := os.ReadFile(expectedSchemaPath)
	if err != nil {
		t.Fatalf("Failed to read expected schema file: %v", err)
	}

	var expectedSchema map[string]interface{}
	if err := json.Unmarshal(expectedSchemaBytes, &expectedSchema); err != nil {
		t.Fatalf("Failed to unmarshal expected schema: %v", err)
	}

	// Write generated schema to file for inspection
	generatedBytes, _ := json.MarshalIndent(generatedSchema, "", "  ")
	generatedFile := filepath.Join("test_output", "actual_generated_schema.json")
	if err := os.MkdirAll("test_output", 0755); err == nil {
		_ = os.WriteFile(generatedFile, generatedBytes, 0644)
		t.Logf("Generated schema written to: %s", generatedFile)
	}

	// Compare generated schema with expected schema
	if !compareSchemas(t, expectedSchema, generatedSchema) {
		t.Error("Generated schema does not match expected schema")
	}
}

// compareSchemas recursively compares two schema maps
func compareSchemas(t *testing.T, expected, actual map[string]interface{}) bool {
	for key, expectedValue := range expected {
		actualValue, exists := actual[key]
		if !exists {
			t.Errorf("Missing key in generated schema: %s", key)
			return false
		}

		if !compareValues(t, key, expectedValue, actualValue) {
			return false
		}
	}

	// Check for unexpected keys in actual schema
	for key := range actual {
		if _, exists := expected[key]; !exists {
			t.Logf("Extra key in generated schema: %s", key)
			// We allow extra keys as the generator might include additional metadata
		}
	}

	return true
}

// compareValues compares two values recursively
func compareValues(t *testing.T, path string, expected, actual interface{}) bool {
	// Handle special case for arrays - []interface{} vs []string
	if expectedSlice, ok := expected.([]interface{}); ok {
		if actualStringSlice, ok := actual.([]string); ok {
			// Convert []string to []interface{} for comparison
			actualSlice := make([]interface{}, len(actualStringSlice))
			for i, s := range actualStringSlice {
				actualSlice[i] = s
			}
			return compareSlices(t, path, expectedSlice, actualSlice)
		}
		if actualSlice, ok := actual.([]interface{}); ok {
			return compareSlices(t, path, expectedSlice, actualSlice)
		}
		t.Errorf("Expected slice at %s, got %T", path, actual)
		return false
	}

	if actualSlice, ok := actual.([]string); ok {
		if expectedSlice, ok := expected.([]interface{}); ok {
			// Convert []string to []interface{} for comparison
			actualInterfaceSlice := make([]interface{}, len(actualSlice))
			for i, s := range actualSlice {
				actualInterfaceSlice[i] = s
			}
			return compareSlices(t, path, expectedSlice, actualInterfaceSlice)
		}
	}

	expectedType := reflect.TypeOf(expected)
	actualType := reflect.TypeOf(actual)

	if expectedType != actualType {
		t.Errorf("Type mismatch at %s: expected %T, got %T", path, expected, actual)
		return false
	}

	switch expectedValue := expected.(type) {
	case map[string]interface{}:
		actualMap, ok := actual.(map[string]interface{})
		if !ok {
			t.Errorf("Expected map at %s, got %T", path, actual)
			return false
		}
		return compareSchemas(t, expectedValue, actualMap)

	case []interface{}:
		actualSlice, ok := actual.([]interface{})
		if !ok {
			t.Errorf("Expected slice at %s, got %T", path, actual)
			return false
		}
		return compareSlices(t, path, expectedValue, actualSlice)

	default:
		if expected != actual {
			t.Errorf("Value mismatch at %s: expected %v, got %v", path, expected, actual)
			return false
		}
		return true
	}
}

// compareSlices compares two slices of interfaces
func compareSlices(t *testing.T, path string, expected, actual []interface{}) bool {
	if len(expected) != len(actual) {
		t.Errorf("Slice length mismatch at %s: expected %d, got %d", path, len(expected), len(actual))
		return false
	}

	for i, expectedItem := range expected {
		if !compareValues(t, fmt.Sprintf("%s[%d]", path, i), expectedItem, actual[i]) {
			return false
		}
	}
	return true
}


// TestOptionalFieldHandling specifically tests configoptional.Optional field handling
func TestOptionalFieldHandling(t *testing.T) {
	// Create a simple config with Optional field for isolated testing
	factory := NewFactory()
	defaultConfig := factory.CreateDefaultConfig()

	generator := NewSchemaGenerator("test_optional")
	defer func() {
		_ = os.RemoveAll("test_optional")
	}()

	schema, err := generator.generateJSONSchema(defaultConfig)
	if err != nil {
		t.Fatalf("Failed to generate schema: %v", err)
	}

	properties := schema["properties"].(map[string]interface{})

	// Test that Optional[confighttp.ServerConfig] is unwrapped
	httpServer, exists := properties["http_server"]
	if !exists {
		t.Fatal("http_server field missing")
	}

	httpServerObj, ok := httpServer.(map[string]interface{})
	if !ok {
		t.Fatalf("http_server should be object, got: %T", httpServer)
	}

	// Should have nested properties from ServerConfig
	httpProps, exists := httpServerObj["properties"].(map[string]interface{})
	if !exists {
		t.Fatal("http_server should have properties from unwrapped ServerConfig")
	}

	// Check for some expected ServerConfig fields
	if _, exists := httpProps["endpoint"]; !exists {
		t.Error("Missing endpoint field from unwrapped ServerConfig")
	}

	t.Logf("Successfully unwrapped configoptional.Optional[confighttp.ServerConfig] with %d properties", len(httpProps))
}