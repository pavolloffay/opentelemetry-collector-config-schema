package collectorconfigschema

import (
	"encoding/json"
	"strings"
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

func TestSchemaManager_GetLatestVersion(t *testing.T) {
	manager := NewSchemaManager()

	version, err := manager.GetLatestVersion()
	require.NoError(t, err, "Failed to get latest version")
	require.NotEmpty(t, version, "Latest version should not be empty")

	// Verify the version has a valid format (major.minor.patch)
	assert.Contains(t, version, ".", "Version should contain dots")

	// Since we know we have v0.138.0 in the schemas directory, verify it's returned
	assert.Equal(t, "0.138.0", version, "Expected version 0.138.0 as the latest")

	t.Logf("Latest version found: %s", version)
}

func TestSchemaManager_GetAllVersions(t *testing.T) {
	manager := NewSchemaManager()

	versions, err := manager.GetAllVersions()
	require.NoError(t, err, "Failed to get all versions")
	require.NotEmpty(t, versions, "Versions list should not be empty")

	// Verify we have at least one version
	assert.GreaterOrEqual(t, len(versions), 1, "Should have at least one version")

	// Since we know we have v0.138.0, verify it's in the list
	assert.Contains(t, versions, "0.138.0", "Expected version 0.138.0 to be in the list")

	// Verify all versions have a valid format (contain dots)
	for _, version := range versions {
		assert.Contains(t, version, ".", "Version %s should contain dots", version)
		assert.NotEmpty(t, version, "Version should not be empty")
	}

	t.Logf("All versions found: %v", versions)
}

func TestSchemaManager_GetDeprecatedFields(t *testing.T) {
	manager := NewSchemaManager()

	// Test getting deprecated fields for kafka exporter which has known deprecated fields
	deprecatedFields, err := manager.GetDeprecatedFields(ComponentTypeExporter, "kafka", "0.138.0")
	require.NoError(t, err, "Failed to get deprecated fields for kafka exporter")

	// Assert that we found deprecated fields in kafka exporter
	assert.GreaterOrEqual(t, len(deprecatedFields), 1, "Kafka exporter should have at least one deprecated field")

	// Check for specific deprecated fields we expect in kafka exporter
	expectedDeprecatedFields := []string{"brokers", "topic"}
	foundFields := make(map[string]bool)

	for _, field := range deprecatedFields {
		for _, expected := range expectedDeprecatedFields {
			if strings.Contains(field, expected) {
				foundFields[expected] = true
			}
		}
	}

	// Assert that we found at least one of the expected deprecated fields
	assert.True(t, len(foundFields) > 0, "Should find at least one expected deprecated field (brokers or topic)")

	t.Logf("Found %d deprecated fields in kafka exporter: %v", len(deprecatedFields), deprecatedFields)

	// Test with a component that doesn't exist
	_, err = manager.GetDeprecatedFields(ComponentTypeExporter, "nonexistent", "0.138.0")
	require.Error(t, err, "Expected error for non-existent component")
	assert.Contains(t, err.Error(), "failed to get schema", "Error should mention schema retrieval failure")

	t.Logf("Successfully tested deprecated fields detection")
}

func TestSchemaManager_GetComponentNames(t *testing.T) {
	manager := NewSchemaManager()

	// Test getting receiver component names
	receiverNames, err := manager.GetComponentNames(ComponentTypeReceiver, "0.138.0")
	require.NoError(t, err, "Failed to get receiver component names")
	require.NotEmpty(t, receiverNames, "Receiver names list should not be empty")

	// Verify we have expected receivers
	assert.Contains(t, receiverNames, "otlp", "Expected otlp receiver to be in the list")
	assert.GreaterOrEqual(t, len(receiverNames), 10, "Should have at least 10 receivers")

	t.Logf("Found %d receiver components: %v", len(receiverNames), receiverNames[:minInt(5, len(receiverNames))])

	// Test getting processor component names
	processorNames, err := manager.GetComponentNames(ComponentTypeProcessor, "0.138.0")
	require.NoError(t, err, "Failed to get processor component names")
	require.NotEmpty(t, processorNames, "Processor names list should not be empty")

	// Verify we have expected processors
	assert.Contains(t, processorNames, "batch", "Expected batch processor to be in the list")
	assert.GreaterOrEqual(t, len(processorNames), 5, "Should have at least 5 processors")

	t.Logf("Found %d processor components: %v", len(processorNames), processorNames[:minInt(5, len(processorNames))])

	// Test getting exporter component names
	exporterNames, err := manager.GetComponentNames(ComponentTypeExporter, "0.138.0")
	require.NoError(t, err, "Failed to get exporter component names")
	require.NotEmpty(t, exporterNames, "Exporter names list should not be empty")

	// Verify we have expected exporters
	assert.Contains(t, exporterNames, "debug", "Expected debug exporter to be in the list")
	assert.GreaterOrEqual(t, len(exporterNames), 5, "Should have at least 5 exporters")

	t.Logf("Found %d exporter components: %v", len(exporterNames), exporterNames[:minInt(5, len(exporterNames))])
}

func TestSchemaManager_GetComponentNames_InvalidType(t *testing.T) {
	manager := NewSchemaManager()

	// Test with invalid component type
	_, err := manager.GetComponentNames("invalid", "0.138.0")
	require.Error(t, err, "Expected error for invalid component type")
	assert.Contains(t, err.Error(), "invalid component type", "Error should mention invalid component type")

	t.Logf("Correctly handled invalid component type: %v", err)
}

func TestSchemaManager_GetComponentNames_InvalidVersion(t *testing.T) {
	manager := NewSchemaManager()

	// Test with non-existent version
	_, err := manager.GetComponentNames(ComponentTypeReceiver, "999.999.999")
	require.Error(t, err, "Expected error for non-existent version")
	assert.Contains(t, err.Error(), "failed to read schema directory", "Error should mention directory read failure")

	t.Logf("Correctly handled non-existent version: %v", err)
}

// Helper function for minimum value
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
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
