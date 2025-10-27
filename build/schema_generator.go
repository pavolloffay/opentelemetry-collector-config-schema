package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/receiver"
)

// SchemaGenerator generates JSON schemas for OpenTelemetry collector component configurations
type SchemaGenerator struct {
	outputDir string
}

// NewSchemaGenerator creates a new schema generator that outputs to the specified directory
func NewSchemaGenerator(outputDir string) *SchemaGenerator {
	return &SchemaGenerator{
		outputDir: outputDir,
	}
}

// GenerateAllSchemas generates JSON schemas for all components
func (sg *SchemaGenerator) GenerateAllSchemas() error {
	// Ensure output directory exists
	if err := os.MkdirAll(sg.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Get all component factories
	factories, err := components()
	if err != nil {
		return fmt.Errorf("failed to get component factories: %w", err)
	}

	// Generate schemas for each component type
	if err := sg.generateExtensionSchemas(factories.Extensions); err != nil {
		return fmt.Errorf("failed to generate extension schemas: %w", err)
	}

	if err := sg.generateReceiverSchemas(factories.Receivers); err != nil {
		return fmt.Errorf("failed to generate receiver schemas: %w", err)
	}

	if err := sg.generateProcessorSchemas(factories.Processors); err != nil {
		return fmt.Errorf("failed to generate processor schemas: %w", err)
	}

	if err := sg.generateExporterSchemas(factories.Exporters); err != nil {
		return fmt.Errorf("failed to generate exporter schemas: %w", err)
	}

	if err := sg.generateConnectorSchemas(factories.Connectors); err != nil {
		return fmt.Errorf("failed to generate connector schemas: %w", err)
	}

	return nil
}

// generateExtensionSchemas generates schemas for all extension components
func (sg *SchemaGenerator) generateExtensionSchemas(factories map[component.Type]extension.Factory) error {
	fmt.Printf("Generating schemas for %d extensions...\n", len(factories))

	for componentType, factory := range factories {
		if err := sg.generateSchemaForComponent("extension", componentType, factory); err != nil {
			fmt.Printf("Warning: failed to generate schema for extension %s: %v\n", componentType, err)
			continue
		}
	}
	return nil
}

// generateReceiverSchemas generates schemas for all receiver components
func (sg *SchemaGenerator) generateReceiverSchemas(factories map[component.Type]receiver.Factory) error {
	fmt.Printf("Generating schemas for %d receivers...\n", len(factories))

	for componentType, factory := range factories {
		if err := sg.generateSchemaForComponent("receiver", componentType, factory); err != nil {
			fmt.Printf("Warning: failed to generate schema for receiver %s: %v\n", componentType, err)
			continue
		}
	}
	return nil
}

// generateProcessorSchemas generates schemas for all processor components
func (sg *SchemaGenerator) generateProcessorSchemas(factories map[component.Type]processor.Factory) error {
	fmt.Printf("Generating schemas for %d processors...\n", len(factories))

	for componentType, factory := range factories {
		if err := sg.generateSchemaForComponent("processor", componentType, factory); err != nil {
			fmt.Printf("Warning: failed to generate schema for processor %s: %v\n", componentType, err)
			continue
		}
	}
	return nil
}

// generateExporterSchemas generates schemas for all exporter components
func (sg *SchemaGenerator) generateExporterSchemas(factories map[component.Type]exporter.Factory) error {
	fmt.Printf("Generating schemas for %d exporters...\n", len(factories))

	for componentType, factory := range factories {
		if err := sg.generateSchemaForComponent("exporter", componentType, factory); err != nil {
			fmt.Printf("Warning: failed to generate schema for exporter %s: %v\n", componentType, err)
			continue
		}
	}
	return nil
}

// generateConnectorSchemas generates schemas for all connector components
func (sg *SchemaGenerator) generateConnectorSchemas(factories map[component.Type]connector.Factory) error {
	fmt.Printf("Generating schemas for %d connectors...\n", len(factories))

	for componentType, factory := range factories {
		if err := sg.generateSchemaForComponent("connector", componentType, factory); err != nil {
			fmt.Printf("Warning: failed to generate schema for connector %s: %v\n", componentType, err)
			continue
		}
	}
	return nil
}

// generateSchemaForComponent generates a JSON schema for a specific component
func (sg *SchemaGenerator) generateSchemaForComponent(componentCategory string, componentType component.Type, factory component.Factory) error {
	// Get the default config from the factory
	defaultConfig := factory.CreateDefaultConfig()
	if defaultConfig == nil {
		return fmt.Errorf("factory returned nil config")
	}

	// Generate JSON schema from the config struct
	schema, err := sg.generateJSONSchema(defaultConfig)
	if err != nil {
		return fmt.Errorf("failed to generate JSON schema: %w", err)
	}

	// Create filename for this component
	filename := fmt.Sprintf("%s_%s.json", componentCategory, componentType)
	filepath := filepath.Join(sg.outputDir, filename)

	// Write schema to file
	if err := sg.writeSchemaToFile(filepath, schema); err != nil {
		return fmt.Errorf("failed to write schema to file: %w", err)
	}

	fmt.Printf("Generated schema for %s %s -> %s\n", componentCategory, componentType, filename)
	return nil
}

// generateJSONSchema generates a JSON schema from a Go struct
func (sg *SchemaGenerator) generateJSONSchema(config component.Config) (map[string]interface{}, error) {
	// Use reflection to analyze the struct and generate a basic JSON schema
	configType := reflect.TypeOf(config)
	if configType.Kind() == reflect.Ptr {
		configType = configType.Elem()
	}

	schema := map[string]interface{}{
		"$schema":    "https://json-schema.org/draft/2020-12/schema",
		"type":       "object",
		"title":      fmt.Sprintf("%s Configuration", configType.Name()),
		"properties": make(map[string]interface{}),
	}

	properties := schema["properties"].(map[string]interface{})
	required := []string{}

	// Analyze struct fields
	if err := sg.analyzeStructFields(configType, properties, &required); err != nil {
		return nil, err
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema, nil
}

// analyzeStructFields recursively analyzes struct fields to build JSON schema properties
func (sg *SchemaGenerator) analyzeStructFields(structType reflect.Type, properties map[string]interface{}, required *[]string) error {
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get field name (use mapstructure tag if available, otherwise field name)
		fieldName := sg.getFieldName(field)
		if fieldName == "" || fieldName == "-" {
			continue
		}

		// Generate property schema for this field
		property, isRequired, err := sg.generatePropertySchema(field)
		if err != nil {
			return fmt.Errorf("failed to generate property schema for field %s: %w", field.Name, err)
		}

		properties[fieldName] = property

		if isRequired {
			*required = append(*required, fieldName)
		}
	}

	return nil
}

// getFieldName gets the field name for JSON, preferring mapstructure tag
func (sg *SchemaGenerator) getFieldName(field reflect.StructField) string {
	// Check mapstructure tag first
	if tag := field.Tag.Get("mapstructure"); tag != "" {
		parts := strings.Split(tag, ",")
		if len(parts) > 0 && parts[0] != "" {
			return parts[0]
		}
	}

	// Check json tag
	if tag := field.Tag.Get("json"); tag != "" {
		parts := strings.Split(tag, ",")
		if len(parts) > 0 && parts[0] != "" && parts[0] != "-" {
			return parts[0]
		}
	}

	// Use field name in lowercase
	return strings.ToLower(field.Name)
}

// generatePropertySchema generates a JSON schema property for a struct field
func (sg *SchemaGenerator) generatePropertySchema(field reflect.StructField) (map[string]interface{}, bool, error) {
	property := make(map[string]interface{})
	fieldType := field.Type
	isRequired := false

	// Handle pointers
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	} else {
		// Non-pointer fields are generally required unless they have omitempty
		tags := field.Tag.Get("mapstructure")
		if !strings.Contains(tags, "omitempty") {
			isRequired = true
		}
	}

	// Set type and other properties based on Go type
	switch fieldType.Kind() {
	case reflect.String:
		property["type"] = "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		 reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		property["type"] = "integer"
	case reflect.Float32, reflect.Float64:
		property["type"] = "number"
	case reflect.Bool:
		property["type"] = "boolean"
	case reflect.Slice, reflect.Array:
		property["type"] = "array"

		// Try to determine item type
		elemType := fieldType.Elem()
		if elemType.Kind() == reflect.String {
			property["items"] = map[string]interface{}{"type": "string"}
		} else if elemType.Kind() == reflect.Int || elemType.Kind() == reflect.Int64 {
			property["items"] = map[string]interface{}{"type": "integer"}
		} else {
			property["items"] = map[string]interface{}{}
		}
	case reflect.Map:
		property["type"] = "object"
		property["additionalProperties"] = true
	case reflect.Struct:
		// For embedded structs or complex types, just mark as object
		property["type"] = "object"

		// Special handling for common types
		typeName := fieldType.Name()
		switch typeName {
		case "Duration":
			property["type"] = "string"
			property["pattern"] = "^[0-9]+(ns|us|µs|ms|s|m|h)$"
			property["description"] = "Duration string (e.g., '1s', '5m', '1h')"
		case "Time":
			property["type"] = "string"
			property["format"] = "date-time"
		}
	default:
		property["type"] = "object"
	}

	// Add description from field documentation if available
	if desc := field.Tag.Get("description"); desc != "" {
		property["description"] = desc
	}

	return property, isRequired, nil
}

// writeSchemaToFile writes a JSON schema to a file
func (sg *SchemaGenerator) writeSchemaToFile(filepath string, schema map[string]interface{}) error {
	// Pretty print JSON
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema to JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}