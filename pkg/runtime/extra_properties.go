package runtime

import (
	"encoding/json"
)

// Option defines functional options for MCP functions
type Option func(*config)

// ExtraProperty defines an additional property to add to tool schemas
type ExtraProperty struct {
	Name        string
	Description string
	Required    bool
	ContextKey  interface{}
}

type config struct {
	ExtraProperties []ExtraProperty
	NamePrefix      string
}

// WithNamePrefix prepends prefix + "_" to every tool name at registration
// time. Useful when the same service is registered multiple times under
// different names (e.g. separate database instances).
func WithNamePrefix(prefix string) Option {
	return func(c *config) {
		c.NamePrefix = prefix
	}
}

// WithExtraProperties adds extra properties to tool schemas and extracts them from request arguments
func WithExtraProperties(properties ...ExtraProperty) Option {
	return func(c *config) {
		c.ExtraProperties = append(c.ExtraProperties, properties...)
	}
}

// NewConfig creates a new config instance
func NewConfig() *config {
	return &config{}
}

// ApplyConfig applies all config options (name prefix, extra properties) to a tool.
func ApplyConfig(tool Tool, config *config) Tool {
	if config.NamePrefix != "" {
		tool.Name = config.NamePrefix + "_" + tool.Name
	}
	if len(config.ExtraProperties) > 0 {
		tool = AddExtraPropertiesToTool(tool, config.ExtraProperties)
	}
	return tool
}

// AddExtraPropertiesToTool modifies a tool's schema to include additional properties
func AddExtraPropertiesToTool(tool Tool, properties []ExtraProperty) Tool {
	if len(properties) == 0 {
		return tool
	}

	// Parse the existing schema
	var schema map[string]interface{}
	if err := json.Unmarshal(tool.RawInputSchema, &schema); err != nil {
		// If we can't parse the schema, return the original tool
		return tool
	}

	// Add extra properties to schema
	var schemaProperties map[string]interface{}
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		schemaProperties = props
	} else {
		schemaProperties = make(map[string]interface{})
		schema["properties"] = schemaProperties
	}

	// Get existing required fields
	var requiredFields []interface{}
	if req, ok := schema["required"].([]interface{}); ok {
		requiredFields = req
	}

	// Add each extra property
	for _, prop := range properties {
		// All extra properties are treated as strings by default
		propertyDef := map[string]interface{}{
			"type":        "string",
			"description": prop.Description,
		}

		schemaProperties[prop.Name] = propertyDef

		// Add to required fields if needed
		if prop.Required {
			requiredFields = append(requiredFields, prop.Name)
		}
	}

	// Update required array
	if len(requiredFields) > 0 {
		schema["required"] = requiredFields
	}

	// Marshal the modified schema back
	modifiedSchema, err := json.Marshal(schema)
	if err != nil {
		// If marshaling fails, return the original tool
		return tool
	}

	// Create a new tool with the modified schema
	modifiedTool := tool
	modifiedTool.RawInputSchema = json.RawMessage(modifiedSchema)
	return modifiedTool
}
