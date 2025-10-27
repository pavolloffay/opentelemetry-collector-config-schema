package collectorconfigschema

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

//go:embed schemas
var embeddedSchemas embed.FS

// ComponentType represents the type of OpenTelemetry component
type ComponentType string

const (
	ComponentTypeReceiver  ComponentType = "receiver"
	ComponentTypeProcessor ComponentType = "processor"
	ComponentTypeExporter  ComponentType = "exporter"
	ComponentTypeExtension ComponentType = "extension"
	ComponentTypeConnector ComponentType = "connector"
)

// ComponentSchema represents a JSON schema for an OpenTelemetry component
type ComponentSchema struct {
	Name        string                 `json:"name"`
	Type        ComponentType          `json:"type"`
	Version     string                 `json:"version,omitempty"`
	Schema      map[string]interface{} `json:"schema"`
	Description string                 `json:"description,omitempty"`
}

// SchemaManager manages component schemas
type SchemaManager struct {
	cache map[string]*ComponentSchema
}

// NewSchemaManager creates a new schema manager
func NewSchemaManager() *SchemaManager {
	return &SchemaManager{
		cache: make(map[string]*ComponentSchema),
	}
}

// GetComponentSchema returns the JSON schema for a specific component
func (sm *SchemaManager) GetComponentSchema(componentType ComponentType, componentName string, version string) (*ComponentSchema, error) {
	// Create cache key
	cacheKey := fmt.Sprintf("%s_%s_%s", componentType, componentName, version)

	// Check cache first
	if schema, exists := sm.cache[cacheKey]; exists {
		return schema, nil
	}

	// Load schema from file
	schema, err := sm.loadSchemaFromFile(componentType, componentName, version)
	if err != nil {
		return nil, err
	}

	// Cache the result
	sm.cache[cacheKey] = schema

	return schema, nil
}

// GetComponentSchemaJSON returns the JSON schema as a JSON byte array
func (sm *SchemaManager) GetComponentSchemaJSON(componentType ComponentType, componentName string, version string) ([]byte, error) {
	schema, err := sm.GetComponentSchema(componentType, componentName, version)
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(schema.Schema, "", "  ")
}

// ListAvailableComponents returns a list of all available components by type
func (sm *SchemaManager) ListAvailableComponents(version string) (map[ComponentType][]string, error) {
	return sm.listEmbeddedComponents(version)
}

// ValidateComponentJSON validates a component configuration JSON against its schema
func (sm *SchemaManager) ValidateComponentJSON(componentType ComponentType, componentName string, version string, jsonData []byte) (*gojsonschema.Result, error) {
	// Get the component schema
	componentSchema, err := sm.GetComponentSchema(componentType, componentName, version)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema for %s %s v%s: %w", componentType, componentName, version, err)
	}

	// Convert schema to JSON bytes for gojsonschema
	schemaBytes, err := json.Marshal(componentSchema.Schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema for %s %s: %w", componentType, componentName, err)
	}

	// Create schema loader
	schemaLoader := gojsonschema.NewBytesLoader(schemaBytes)

	// Create document loader from the provided JSON data
	documentLoader := gojsonschema.NewBytesLoader(jsonData)

	// Validate the document against the schema
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return nil, fmt.Errorf("validation failed for %s %s: %w", componentType, componentName, err)
	}

	return result, nil
}

// listEmbeddedComponents lists components from embedded filesystem
func (sm *SchemaManager) listEmbeddedComponents(version string) (map[ComponentType][]string, error) {
	components := make(map[ComponentType][]string)

	// Read embedded directory
	schemaPath := fmt.Sprintf("schemas/v%s", version)
	entries, err := fs.ReadDir(embeddedSchemas, schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded schema directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Remove .json extension
		name := strings.TrimSuffix(entry.Name(), ".json")

		// Parse component type and name from filename (format: type_name.json)
		parts := strings.SplitN(name, "_", 2)
		if len(parts) != 2 {
			continue // Skip files that don't match the expected format
		}

		componentType := ComponentType(parts[0])
		componentName := parts[1]

		// Validate component type
		if !isValidComponentType(componentType) {
			continue
		}

		components[componentType] = append(components[componentType], componentName)
	}

	return components, nil
}

// loadSchemaFromFile loads a schema from embedded files
func (sm *SchemaManager) loadSchemaFromFile(componentType ComponentType, componentName string, version string) (*ComponentSchema, error) {
	// Construct filename (format: type_name.json)
	filename := fmt.Sprintf("%s_%s.json", componentType, componentName)

	// Load from embedded filesystem
	schemaPath := fmt.Sprintf("schemas/v%s", version)
	embeddedFilepath := filepath.Join(schemaPath, filename)
	data, err := fs.ReadFile(embeddedSchemas, embeddedFilepath)
	if err != nil {
		return nil, fmt.Errorf("schema not found for component %s %s", componentType, componentName)
	}

	// Parse JSON schema
	var schemaData map[string]interface{}
	if err := json.Unmarshal(data, &schemaData); err != nil {
		return nil, fmt.Errorf("failed to parse schema JSON for %s %s: %w", componentType, componentName, err)
	}

	// Extract description from schema if available
	description := ""
	if title, exists := schemaData["title"]; exists {
		if titleStr, ok := title.(string); ok {
			description = titleStr
		}
	}

	// Use the provided version
	componentVersion := version

	return &ComponentSchema{
		Name:        componentName,
		Type:        componentType,
		Version:     componentVersion,
		Schema:      schemaData,
		Description: description,
	}, nil
}

// isValidComponentType checks if the component type is valid
func isValidComponentType(componentType ComponentType) bool {
	switch componentType {
	case ComponentTypeReceiver, ComponentTypeProcessor, ComponentTypeExporter, ComponentTypeExtension, ComponentTypeConnector:
		return true
	default:
		return false
	}
}
