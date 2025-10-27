package collectorconfigschema

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaManager_GetComponentSchema(t *testing.T) {
	manager := NewSchemaManager()

	// Test getting OTLP receiver schema
	schema, err := manager.GetComponentSchema(ComponentTypeReceiver, "otlp", "0.138.0")
	if err != nil {
		t.Fatalf("Failed to get OTLP receiver schema: %v", err)
	}

	if schema == nil {
		t.Fatal("Schema is nil")
	}

	if schema.Name != "otlp" {
		t.Errorf("Expected component name 'otlp', got '%s'", schema.Name)
	}

	if schema.Type != ComponentTypeReceiver {
		t.Errorf("Expected component type 'receiver', got '%s'", schema.Type)
	}

	if schema.Schema == nil {
		t.Fatal("Schema data is nil")
	}

	// Verify schema has expected properties
	if schemaType, exists := schema.Schema["$schema"]; !exists {
		t.Error("Schema missing '$schema' property")
	} else if schemaType != "https://json-schema.org/draft/2020-12/schema" {
		t.Errorf("Unexpected schema type: %v", schemaType)
	}

	// Verify it has properties
	if properties, exists := schema.Schema["properties"]; !exists {
		t.Error("Schema missing 'properties'")
	} else if propertiesMap, ok := properties.(map[string]interface{}); !ok {
		t.Error("Properties is not a map")
	} else if len(propertiesMap) == 0 {
		t.Error("Properties map is empty")
	}

	t.Logf("Successfully loaded schema for %s %s with %d top-level properties",
		schema.Type, schema.Name, len(schema.Schema))
}

func TestSchemaManager_GetComponentSchemaJSON(t *testing.T) {
	manager := NewSchemaManager()

	// Test getting JSON for debug exporter
	jsonData, err := manager.GetComponentSchemaJSON(ComponentTypeExporter, "debug", "0.138.0")
	if err != nil {
		t.Fatalf("Failed to get debug exporter schema JSON: %v", err)
	}

	if len(jsonData) == 0 {
		t.Fatal("JSON data is empty")
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Generated JSON is invalid: %v", err)
	}

	t.Logf("Successfully generated %d bytes of JSON for debug exporter", len(jsonData))
}

func TestSchemaManager_NonExistentComponent(t *testing.T) {
	manager := NewSchemaManager()

	// Test getting schema for non-existent component
	_, err := manager.GetComponentSchema(ComponentTypeReceiver, "nonexistent", "0.138.0")
	if err == nil {
		t.Fatal("Expected error for non-existent component, got nil")
	}

	expectedError := "schema not found for component receiver nonexistent"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestSchemaManager_ListAvailableComponents(t *testing.T) {
	manager := NewSchemaManager()

	components, err := manager.ListAvailableComponents("0.138.0")
	if err != nil {
		t.Fatalf("Failed to list available components: %v", err)
	}

	if len(components) == 0 {
		t.Fatal("No components found")
	}

	// Verify we have expected component types
	expectedTypes := []ComponentType{
		ComponentTypeReceiver,
		ComponentTypeProcessor,
		ComponentTypeExporter,
		ComponentTypeExtension,
		ComponentTypeConnector,
	}

	for _, expectedType := range expectedTypes {
		if componentList, exists := components[expectedType]; !exists {
			t.Errorf("Missing component type: %s", expectedType)
		} else if len(componentList) == 0 {
			t.Errorf("No components found for type: %s", expectedType)
		} else {
			t.Logf("Found %d %s components", len(componentList), expectedType)
		}
	}

	// Verify specific components exist
	if receivers, exists := components[ComponentTypeReceiver]; exists {
		found := false
		for _, name := range receivers {
			if name == "otlp" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to find 'otlp' receiver in component list")
		}
	}
}

func TestSchemaManager_Caching(t *testing.T) {
	manager := NewSchemaManager()

	// Get the same schema twice
	schema1, err := manager.GetComponentSchema(ComponentTypeProcessor, "batch", "0.138.0")
	if err != nil {
		t.Fatalf("Failed to get batch processor schema (first call): %v", err)
	}

	schema2, err := manager.GetComponentSchema(ComponentTypeProcessor, "batch", "0.138.0")
	if err != nil {
		t.Fatalf("Failed to get batch processor schema (second call): %v", err)
	}

	// Verify they are the same object (should be cached)
	if schema1 != schema2 {
		t.Error("Expected cached schema to return the same object")
	}

	t.Log("Schema caching is working correctly")
}

func TestSchemaManager_WithVersion(t *testing.T) {
	manager := NewSchemaManager()

	// Test getting schema with version 0.138.0
	schema, err := manager.GetComponentSchema(ComponentTypeReceiver, "otlp", "0.138.0")
	if err != nil {
		t.Fatalf("Failed to get OTLP receiver schema with version: %v", err)
	}

	if schema.Version != "0.138.0" {
		t.Errorf("Expected version '0.138.0', got '%s'", schema.Version)
	}

	// Test that different versions are cached separately (this would fail for versions we don't have schemas for)
	// For now, test with the same version to ensure caching works
	schema2, err := manager.GetComponentSchema(ComponentTypeReceiver, "otlp", "0.138.0")
	if err != nil {
		t.Fatalf("Failed to get OTLP receiver schema with same version: %v", err)
	}

	if schema2.Version != "0.138.0" {
		t.Errorf("Expected version '0.138.0', got '%s'", schema2.Version)
	}

	// They should be the same object due to caching
	if schema != schema2 {
		t.Error("Expected same objects for same version (cached)")
	}

	t.Log("Version handling works correctly")
}

func TestSchemaManager_ValidateComponentJSON(t *testing.T) {
	manager := NewSchemaManager()

	// Test valid JSON for OTLP receiver
	validJSON := []byte(`{
		"protocols": {
			"grpc": {
				"endpoint": "0.0.0.0:4317"
			},
			"http": {
				"endpoint": "0.0.0.0:4318"
			}
		}
	}`)

	result, err := manager.ValidateComponentJSON(ComponentTypeReceiver, "otlp", "0.138.0", validJSON)
	require.NoError(t, err, "Failed to validate valid OTLP receiver JSON")
	require.NotNil(t, result, "Validation result should not be nil")

	if !result.Valid() {
		for _, desc := range result.Errors() {
			t.Errorf("Validation error: %s", desc)
		}
	}
	assert.True(t, result.Valid(), "Expected valid JSON to pass validation")

	t.Logf("Successfully validated OTLP receiver configuration")
}

func TestSchemaManager_ValidateComponentJSON_Invalid(t *testing.T) {
	manager := NewSchemaManager()

	// Test invalid JSON (include_metadata should be a boolean, not a string)
	invalidJSON := []byte(`{
		"grpc": {
			"include_metadata": "invalid_boolean_value",
			"keepalive": {
				"server_parameters": {
					"max_connection_idle": "invalid_duration_format"
				}
			}
		}
	}`)

	result, err := manager.ValidateComponentJSON(ComponentTypeReceiver, "otlp", "0.138.0", invalidJSON)
	require.NoError(t, err, "Failed to validate invalid OTLP receiver JSON")
	require.NotNil(t, result, "Validation result should not be nil")

	if result.Valid() {
		assert.Fail(t, "Expected invalid JSON to fail validation, but it passed")
	} else {
		t.Logf("Correctly identified %d validation errors:", len(result.Errors()))
		for _, desc := range result.Errors() {
			t.Logf("  - %s", desc)
		}
		assert.False(t, result.Valid(), "Expected invalid JSON to fail validation")
	}
}

func TestSchemaManager_ValidateComponentJSON_MalformedJSON(t *testing.T) {
	manager := NewSchemaManager()

	// Test malformed JSON
	malformedJSON := []byte(`{
		"protocols": {
			"grpc": {
				"endpoint": "0.0.0.0:4317"
			}
		// Missing closing braces`)

	result, err := manager.ValidateComponentJSON(ComponentTypeReceiver, "otlp", "0.138.0", malformedJSON)
	if err != nil {
		// This should fail at the validation level, not the JSON parsing level
		t.Logf("Expected error for malformed JSON: %v", err)
		return
	}

	if result != nil && result.Valid() {
		t.Error("Expected malformed JSON to fail validation")
	}
}

func TestSchemaManager_ValidateComponentJSON_NonExistentComponent(t *testing.T) {
	manager := NewSchemaManager()

	validJSON := []byte(`{"some": "config"}`)

	_, err := manager.ValidateComponentJSON(ComponentTypeReceiver, "nonexistent", "0.138.0", validJSON)
	require.Error(t, err, "Expected error for non-existent component")

	expectedError := "failed to get schema for receiver nonexistent v0.138.0"
	assert.Contains(t, err.Error(), expectedError, "Error should contain expected text")

	t.Logf("Correctly handled non-existent component: %v", err)
}

func TestSchemaManager_ValidateComponentJSON_EmptyJSON(t *testing.T) {
	manager := NewSchemaManager()

	// Test empty JSON object
	emptyJSON := []byte(`{}`)

	result, err := manager.ValidateComponentJSON(ComponentTypeReceiver, "otlp", "0.138.0", emptyJSON)
	require.NoError(t, err, "Failed to validate empty JSON")
	require.NotNil(t, result, "Validation result should not be nil")

	// Empty JSON might be valid or invalid depending on schema requirements
	// Just verify we get a result without errors
	t.Logf("Empty JSON validation result: valid=%v, errors=%d", result.Valid(), len(result.Errors()))
}

func TestSchemaManager_ValidateComponentJSON_DifferentComponents(t *testing.T) {
	manager := NewSchemaManager()

	// Test debug exporter with minimal valid config
	debugExporterJSON := []byte(`{
		"verbosity": "normal"
	}`)

	result, err := manager.ValidateComponentJSON(ComponentTypeExporter, "debug", "0.138.0", debugExporterJSON)
	require.NoError(t, err, "Failed to validate debug exporter JSON")
	require.NotNil(t, result, "Validation result should not be nil")

	t.Logf("Debug exporter validation result: valid=%v, errors=%d", result.Valid(), len(result.Errors()))

	// Test batch processor with valid config
	batchProcessorJSON := []byte(`{
		"timeout": "1s",
		"send_batch_size": 1024
	}`)

	result, err = manager.ValidateComponentJSON(ComponentTypeProcessor, "batch", "0.138.0", batchProcessorJSON)
	require.NoError(t, err, "Failed to validate batch processor JSON")
	require.NotNil(t, result, "Validation result should not be nil")

	t.Logf("Batch processor validation result: valid=%v, errors=%d", result.Valid(), len(result.Errors()))
}

func BenchmarkSchemaManager_GetComponentSchema(b *testing.B) {
	manager := NewSchemaManager()

	// Pre-load one schema to test caching performance
	_, _ = manager.GetComponentSchema(ComponentTypeReceiver, "otlp", "0.138.0")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.GetComponentSchema(ComponentTypeReceiver, "otlp", "0.138.0")
		if err != nil {
			b.Fatalf("Failed to get schema: %v", err)
		}
	}
}
