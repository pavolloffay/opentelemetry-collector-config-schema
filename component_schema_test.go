package main

import (
	"encoding/json"
	"testing"
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

func TestConvenienceFunctions(t *testing.T) {
	// Test convenience function for getting schema
	schema, err := GetComponentSchemaByName(ComponentTypeExtension, "zpages", "0.138.0")
	if err != nil {
		t.Fatalf("Failed to get zpages extension schema using convenience function: %v", err)
	}

	if schema.Name != "zpages" || schema.Type != ComponentTypeExtension {
		t.Errorf("Unexpected schema: name=%s, type=%s", schema.Name, schema.Type)
	}

	// Test convenience function for getting JSON
	jsonData, err := GetComponentSchemaJSONByName(ComponentTypeExtension, "zpages", "0.138.0")
	if err != nil {
		t.Fatalf("Failed to get zpages extension schema JSON using convenience function: %v", err)
	}

	if len(jsonData) == 0 {
		t.Fatal("JSON data is empty")
	}

	t.Logf("Convenience functions work correctly")
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