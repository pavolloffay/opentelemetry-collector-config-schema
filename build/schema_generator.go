package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
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
	outputDir     string
	commentCache  map[string]map[string]string // packagePath -> typeName.fieldName -> comment
	fileSetCache  map[string]*token.FileSet    // packagePath -> FileSet
}

// NewSchemaGenerator creates a new schema generator that outputs to the specified directory
func NewSchemaGenerator(outputDir string) *SchemaGenerator {
	return &SchemaGenerator{
		outputDir:     outputDir,
		commentCache:  make(map[string]map[string]string),
		fileSetCache:  make(map[string]*token.FileSet),
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

		// Handle embedded/anonymous fields by flattening them
		if field.Anonymous {
			if err := sg.handleEmbeddedField(field, properties, required); err != nil {
				return fmt.Errorf("failed to handle embedded field %s: %w", field.Name, err)
			}
			continue
		}

		// Get field name (use mapstructure tag if available, otherwise field name)
		fieldName := sg.getFieldName(field)
		if fieldName == "" || fieldName == "-" {
			continue
		}

		// Generate property schema for this field
		property, isRequired, err := sg.generatePropertySchema(field, structType)
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

// handleEmbeddedField handles anonymous/embedded struct fields by flattening their properties
func (sg *SchemaGenerator) handleEmbeddedField(field reflect.StructField, properties map[string]interface{}, required *[]string) error {
	fieldType := field.Type

	// Handle pointer to embedded struct
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}

	// Only handle embedded structs
	if fieldType.Kind() != reflect.Struct {
		return nil
	}

	// Recursively analyze the embedded struct's fields
	return sg.analyzeStructFields(fieldType, properties, required)
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
func (sg *SchemaGenerator) generatePropertySchema(field reflect.StructField, parentType reflect.Type) (map[string]interface{}, bool, error) {
	property := make(map[string]interface{})
	fieldType := field.Type
	isRequired := false

	// Handle pointers
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	} else {
		// Non-pointer fields are generally required unless they have omitempty
		tags := field.Tag.Get("mapstructure")
		jsonTags := field.Tag.Get("json")
		if !strings.Contains(tags, "omitempty") && !strings.Contains(jsonTags, "omitempty") {
			isRequired = true
		}
	}

	// Check for special types first, before basic type handling
	typeName := fieldType.Name()
	pkgPath := fieldType.PkgPath()

	// Handle time.Duration specially (it's an int64 but should be treated as a string)
	if typeName == "Duration" && strings.Contains(pkgPath, "time") {
		property["type"] = "string"
		property["pattern"] = "^[0-9]+(ns|us|µs|ms|s|m|h)$"
		property["description"] = "Duration string (e.g., '1s', '5m', '1h')"
		return property, isRequired, nil
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

		// Recursively determine item type
		itemSchema, err := sg.generateTypeSchema(fieldType.Elem())
		if err != nil {
			return nil, false, fmt.Errorf("failed to generate array item schema: %w", err)
		}
		property["items"] = itemSchema
	case reflect.Map:
		property["type"] = "object"
		property["additionalProperties"] = true

		// If we can determine the value type, add it
		if fieldType.Key().Kind() == reflect.String {
			valueSchema, err := sg.generateTypeSchema(fieldType.Elem())
			if err == nil && len(valueSchema) > 0 {
				property["additionalProperties"] = valueSchema
			}
		}
	case reflect.Struct:
		// Handle special types first
		typeName := fieldType.Name()
		pkgPath := fieldType.PkgPath()

		switch {
		case typeName == "Time" && strings.Contains(pkgPath, "time"):
			property["type"] = "string"
			property["format"] = "date-time"
		case strings.HasPrefix(typeName, "Optional") && strings.Contains(pkgPath, "configoptional"):
			// Handle configoptional.Optional[T] types by unwrapping them
			if unwrappedSchema, err := sg.unwrapOptionalType(fieldType); err == nil {
				return unwrappedSchema, false, nil // Optional types are never required
			}
			// Fallback to object if unwrapping fails
			property["type"] = "object"
		default:
			// For other structs, recursively analyze their fields
			property["type"] = "object"
			nestedProperties := make(map[string]interface{})
			nestedRequired := []string{}

			if err := sg.analyzeStructFields(fieldType, nestedProperties, &nestedRequired); err != nil {
				return nil, false, fmt.Errorf("failed to analyze struct fields: %w", err)
			}

			if len(nestedProperties) > 0 {
				property["properties"] = nestedProperties
			}
			if len(nestedRequired) > 0 {
				property["required"] = nestedRequired
			}
		}
	case reflect.Interface:
		// Interface types are typically configuration objects
		property["type"] = "object"
		property["additionalProperties"] = true
	default:
		property["type"] = "object"
	}

	// Add description from source code comments
	// Extract comment for this field from the parent struct where it's declared
	if comment := sg.extractFieldComment(parentType, field.Name); comment != "" {
		property["description"] = comment
	}

	// Add description from field documentation tag if available and no comment was found
	if property["description"] == nil {
		if desc := field.Tag.Get("description"); desc != "" {
			property["description"] = desc
		}
	}

	// Add description from yaml tag if available and no other description was found
	if property["description"] == nil {
		if desc := field.Tag.Get("yaml"); desc != "" && !strings.Contains(desc, ",") {
			property["description"] = desc
		}
	}

	return property, isRequired, nil
}

// generateTypeSchema generates a schema for a specific reflect.Type
func (sg *SchemaGenerator) generateTypeSchema(t reflect.Type) (map[string]interface{}, error) {
	schema := make(map[string]interface{})

	// Handle pointers
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Check for special types first (like time.Duration which is an int64)
	typeName := t.Name()
	pkgPath := t.PkgPath()

	// Handle time.Duration specially
	if typeName == "Duration" && strings.Contains(pkgPath, "time") {
		schema["type"] = "string"
		schema["pattern"] = "^[0-9]+(ns|us|µs|ms|s|m|h)$"
		schema["description"] = "Duration string (e.g., '1s', '5m', '1h')"
		return schema, nil
	}

	switch t.Kind() {
	case reflect.String:
		schema["type"] = "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		 reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		schema["type"] = "integer"
	case reflect.Float32, reflect.Float64:
		schema["type"] = "number"
	case reflect.Bool:
		schema["type"] = "boolean"
	case reflect.Slice, reflect.Array:
		schema["type"] = "array"
		if itemSchema, err := sg.generateTypeSchema(t.Elem()); err == nil {
			schema["items"] = itemSchema
		}
	case reflect.Map:
		schema["type"] = "object"
		schema["additionalProperties"] = true
	case reflect.Struct:
		typeName := t.Name()
		pkgPath := t.PkgPath()

		switch {
		case typeName == "Time" && strings.Contains(pkgPath, "time"):
			schema["type"] = "string"
			schema["format"] = "date-time"
		default:
			schema["type"] = "object"
			properties := make(map[string]interface{})
			required := []string{}

			if err := sg.analyzeStructFields(t, properties, &required); err == nil {
				if len(properties) > 0 {
					schema["properties"] = properties
				}
				if len(required) > 0 {
					schema["required"] = required
				}
			}
		}
	case reflect.Interface:
		schema["type"] = "object"
		schema["additionalProperties"] = true
	default:
		schema["type"] = "object"
	}

	return schema, nil
}

// unwrapOptionalType unwraps configoptional.Optional[T] and similar wrapper types
func (sg *SchemaGenerator) unwrapOptionalType(optionalType reflect.Type) (map[string]interface{}, error) {
	// configoptional.Optional[T] has a field named "value" that contains the actual T value

	// Look for the "value" field specifically
	for i := 0; i < optionalType.NumField(); i++ {
		field := optionalType.Field(i)

		// Look for the "value" field that contains the wrapped type
		if field.Name == "value" {
			fieldType := field.Type

			// Handle pointer to the wrapped type
			if fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
			}

			// Generate schema for the wrapped type
			return sg.generateTypeSchema(fieldType)
		}
	}

	// Also try looking for any exported field that might contain the wrapped type (fallback)
	for i := 0; i < optionalType.NumField(); i++ {
		field := optionalType.Field(i)

		// Skip unexported fields and common non-data fields
		if !field.IsExported() || field.Name == "_" || field.Name == "flavor" {
			continue
		}

		// Check if this field contains the wrapped type
		fieldType := field.Type

		// Handle pointer to the wrapped type
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}

		// If this is a struct that looks like configuration, use it
		if fieldType.Kind() == reflect.Struct && fieldType.NumField() > 0 {
			// Generate schema for the wrapped type
			return sg.generateTypeSchema(fieldType)
		}
	}

	// If we can't unwrap it, return a generic object schema
	return map[string]interface{}{
		"type": "object",
	}, nil
}

// extractFieldComment extracts comments for a struct field from source code
func (sg *SchemaGenerator) extractFieldComment(parentType reflect.Type, fieldName string) string {
	// Skip basic types that don't have source code
	if parentType.PkgPath() == "" {
		return ""
	}

	// For the parent struct type, try to find comments for the field
	if parentType.Kind() == reflect.Struct {
		typeName := parentType.Name()
		pkgPath := parentType.PkgPath()

		// Load comments for this package if not already loaded
		if err := sg.loadCommentsForPackage(pkgPath); err != nil {
			return ""
		}

		// Look up comment in cache
		if packageComments, exists := sg.commentCache[pkgPath]; exists {
			key := fmt.Sprintf("%s.%s", typeName, fieldName)
			if comment, exists := packageComments[key]; exists {
				return comment
			}
		}
	}

	return ""
}

// loadCommentsForPackage loads comments for all structs in a Go package
func (sg *SchemaGenerator) loadCommentsForPackage(pkgPath string) error {
	// Check if already loaded
	if _, exists := sg.commentCache[pkgPath]; exists {
		return nil
	}

	// Initialize cache for this package
	sg.commentCache[pkgPath] = make(map[string]string)

	// Try to find the source directory for this package
	srcDir, err := sg.findPackageSourceDir(pkgPath)
	if err != nil {
		return err
	}

	// Parse all Go files in the package directory
	fset := token.NewFileSet()
	sg.fileSetCache[pkgPath] = fset

	packages, err := parser.ParseDir(fset, srcDir, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse package %s: %w", pkgPath, err)
	}

	// Extract comments from all packages (there might be multiple due to test files)
	for _, pkg := range packages {
		for _, file := range pkg.Files {
			sg.extractCommentsFromFile(file, fset, pkgPath)
		}
	}

	return nil
}

// findPackageSourceDir finds the source directory for a given package path
func (sg *SchemaGenerator) findPackageSourceDir(pkgPath string) (string, error) {
	// For standard library packages, we can't easily access source
	if !strings.Contains(pkgPath, ".") {
		return "", fmt.Errorf("cannot access source for standard library package: %s", pkgPath)
	}

	// For our test case in the main package, try current directory first
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// If the package path ends with the current directory name, use current directory
	if strings.HasSuffix(pkgPath, "contrib") && strings.Contains(wd, "build") {
		return wd, nil
	}

	// Use go list to find the package directory
	return sg.findPackageWithGoList(pkgPath)
}

// findPackageWithGoList uses go list to find the source directory for a package
func (sg *SchemaGenerator) findPackageWithGoList(pkgPath string) (string, error) {
	// Use go list to get the package directory
	cmd := exec.Command("go", "list", "-f", "{{.Dir}}", pkgPath)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go list failed for package %s: %w", pkgPath, err)
	}

	dir := strings.TrimSpace(string(output))
	if dir == "" {
		return "", fmt.Errorf("go list returned empty directory for package: %s", pkgPath)
	}

	// Verify the directory exists
	if _, err := os.Stat(dir); err != nil {
		return "", fmt.Errorf("directory from go list does not exist: %s", dir)
	}

	return dir, nil
}

// extractCommentsFromFile extracts comments from a single Go file
func (sg *SchemaGenerator) extractCommentsFromFile(file *ast.File, fset *token.FileSet, pkgPath string) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.TypeSpec:
			// This is a type declaration (struct, interface, etc.)
			if structType, ok := node.Type.(*ast.StructType); ok {
				sg.extractStructComments(node.Name.Name, structType, node.Doc, fset, pkgPath)
			}
		}
		return true
	})
}

// extractStructComments extracts comments for all fields in a struct
func (sg *SchemaGenerator) extractStructComments(typeName string, structType *ast.StructType, typeDoc *ast.CommentGroup, fset *token.FileSet, pkgPath string) {
	for _, field := range structType.Fields.List {
		// Get field comment (prefer field comment over type comment)
		var comment string
		if field.Doc != nil {
			comment = sg.cleanComment(field.Doc.Text())
		} else if field.Comment != nil {
			comment = sg.cleanComment(field.Comment.Text())
		}

		// Store comment for each field name
		for _, name := range field.Names {
			if comment != "" {
				key := fmt.Sprintf("%s.%s", typeName, name.Name)
				sg.commentCache[pkgPath][key] = comment
			}
		}
	}
}

// cleanComment cleans up a comment string by removing comment markers and extra whitespace
func (sg *SchemaGenerator) cleanComment(comment string) string {
	// Remove leading/trailing whitespace
	comment = strings.TrimSpace(comment)

	// Remove comment markers
	lines := strings.Split(comment, "\n")
	var cleanedLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Remove // and /* */ markers
		line = strings.TrimPrefix(line, "//")
		line = strings.TrimPrefix(line, "/*")
		line = strings.TrimSuffix(line, "*/")
		line = strings.TrimSpace(line)

		if line != "" {
			cleanedLines = append(cleanedLines, line)
		}
	}

	return strings.Join(cleanedLines, " ")
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