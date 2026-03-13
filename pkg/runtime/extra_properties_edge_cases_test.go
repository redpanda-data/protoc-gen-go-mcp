package runtime

import (
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	. "github.com/onsi/gomega"
)

func TestAddExtraProperties_EmptyList(t *testing.T) {
	g := NewWithT(t)

	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{"name": map[string]any{"type": "string"}},
		"required":   []string{"name"},
	}
	schemaBytes, _ := json.Marshal(schema)

	tool := mcp.Tool{
		Name:           "test",
		RawInputSchema: json.RawMessage(schemaBytes),
	}

	result := AddExtraPropertiesToTool(tool, []ExtraProperty{})
	g.Expect(result).To(Equal(tool))
}

func TestAddExtraProperties_NilList(t *testing.T) {
	g := NewWithT(t)

	schema := map[string]any{"type": "object"}
	schemaBytes, _ := json.Marshal(schema)

	tool := mcp.Tool{
		Name:           "test",
		RawInputSchema: json.RawMessage(schemaBytes),
	}

	result := AddExtraPropertiesToTool(tool, nil)
	g.Expect(result).To(Equal(tool))
}

func TestAddExtraProperties_AllOptional(t *testing.T) {
	g := NewWithT(t)

	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
	schemaBytes, _ := json.Marshal(schema)

	tool := mcp.Tool{
		Name:           "test",
		RawInputSchema: json.RawMessage(schemaBytes),
	}

	result := AddExtraPropertiesToTool(tool, []ExtraProperty{
		{Name: "opt1", Description: "Optional 1", Required: false, ContextKey: "opt1"},
		{Name: "opt2", Description: "Optional 2", Required: false, ContextKey: "opt2"},
	})

	var resultSchema map[string]any
	g.Expect(json.Unmarshal(result.RawInputSchema, &resultSchema)).To(Succeed())

	props := resultSchema["properties"].(map[string]any)
	g.Expect(props).To(HaveKey("opt1"))
	g.Expect(props).To(HaveKey("opt2"))

	// No required array should be added (all optional, no pre-existing required)
	g.Expect(resultSchema).ToNot(HaveKey("required"))
}

func TestAddExtraProperties_CollisionOverwritesExisting(t *testing.T) {
	g := NewWithT(t)

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "description": "Original name"},
		},
		"required": []string{"name"},
	}
	schemaBytes, _ := json.Marshal(schema)

	tool := mcp.Tool{
		Name:           "test",
		RawInputSchema: json.RawMessage(schemaBytes),
	}

	// Add an extra property with the same name as existing
	result := AddExtraPropertiesToTool(tool, []ExtraProperty{
		{Name: "name", Description: "Overwritten name", Required: true, ContextKey: "name"},
	})

	var resultSchema map[string]any
	g.Expect(json.Unmarshal(result.RawInputSchema, &resultSchema)).To(Succeed())

	props := resultSchema["properties"].(map[string]any)
	nameField := props["name"].(map[string]any)
	// The extra property overwrites the original
	g.Expect(nameField["description"]).To(Equal("Overwritten name"))
}

func TestAddExtraProperties_NoExistingRequired(t *testing.T) {
	g := NewWithT(t)

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
		// No "required" key at all
	}
	schemaBytes, _ := json.Marshal(schema)

	tool := mcp.Tool{
		Name:           "test",
		RawInputSchema: json.RawMessage(schemaBytes),
	}

	result := AddExtraPropertiesToTool(tool, []ExtraProperty{
		{Name: "token", Description: "Auth token", Required: true, ContextKey: "token"},
	})

	var resultSchema map[string]any
	g.Expect(json.Unmarshal(result.RawInputSchema, &resultSchema)).To(Succeed())

	// Required should be created with just the new property
	required := resultSchema["required"].([]any)
	g.Expect(required).To(ConsistOf("token"))
}

func TestAddExtraProperties_PreservesOtherSchemaFields(t *testing.T) {
	g := NewWithT(t)

	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"id": map[string]any{"type": "string"},
		},
		"required": []string{"id"},
	}
	schemaBytes, _ := json.Marshal(schema)

	tool := mcp.Tool{
		Name:           "test",
		RawInputSchema: json.RawMessage(schemaBytes),
	}

	result := AddExtraPropertiesToTool(tool, []ExtraProperty{
		{Name: "extra", Description: "Extra field", Required: false, ContextKey: "extra"},
	})

	var resultSchema map[string]any
	g.Expect(json.Unmarshal(result.RawInputSchema, &resultSchema)).To(Succeed())

	// additionalProperties should be preserved
	g.Expect(resultSchema["additionalProperties"]).To(Equal(false))
	g.Expect(resultSchema["type"]).To(Equal("object"))

	// Original required should be preserved
	required := resultSchema["required"].([]any)
	g.Expect(required).To(ContainElement("id"))
}

func TestAddExtraProperties_EmptySchema(t *testing.T) {
	g := NewWithT(t)

	tool := mcp.Tool{
		Name:           "test",
		RawInputSchema: json.RawMessage(`{}`),
	}

	result := AddExtraPropertiesToTool(tool, []ExtraProperty{
		{Name: "field", Description: "A field", Required: true, ContextKey: "field"},
	})

	var resultSchema map[string]any
	g.Expect(json.Unmarshal(result.RawInputSchema, &resultSchema)).To(Succeed())

	props := resultSchema["properties"].(map[string]any)
	g.Expect(props).To(HaveKey("field"))

	required := resultSchema["required"].([]any)
	g.Expect(required).To(ConsistOf("field"))
}

func TestAddExtraProperties_SchemaWithArrayType(t *testing.T) {
	g := NewWithT(t)

	// A schema that has properties as null (edge case)
	tool := mcp.Tool{
		Name:           "test",
		RawInputSchema: json.RawMessage(`{"type": "object", "properties": null}`),
	}

	result := AddExtraPropertiesToTool(tool, []ExtraProperty{
		{Name: "field", Description: "A field", Required: true, ContextKey: "field"},
	})

	var resultSchema map[string]any
	g.Expect(json.Unmarshal(result.RawInputSchema, &resultSchema)).To(Succeed())

	// null properties should be replaced with new map
	props := resultSchema["properties"].(map[string]any)
	g.Expect(props).To(HaveKey("field"))
}

func TestAddExtraProperties_DoesNotMutateOriginal(t *testing.T) {
	g := NewWithT(t)

	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{"name": map[string]any{"type": "string"}},
		"required":   []string{"name"},
	}
	schemaBytes, _ := json.Marshal(schema)

	original := mcp.Tool{
		Name:           "test",
		RawInputSchema: json.RawMessage(schemaBytes),
	}

	// Store original schema bytes
	originalSchemaStr := string(original.RawInputSchema)

	_ = AddExtraPropertiesToTool(original, []ExtraProperty{
		{Name: "added", Description: "New field", Required: true, ContextKey: "added"},
	})

	// Original tool's schema should be unchanged
	g.Expect(string(original.RawInputSchema)).To(Equal(originalSchemaStr))
}

func TestAddExtraProperties_ManyProperties(t *testing.T) {
	g := NewWithT(t)

	schema := map[string]any{"type": "object", "properties": map[string]any{}}
	schemaBytes, _ := json.Marshal(schema)

	tool := mcp.Tool{
		Name:           "test",
		RawInputSchema: json.RawMessage(schemaBytes),
	}

	props := make([]ExtraProperty, 20)
	for i := range props {
		props[i] = ExtraProperty{
			Name:        "prop_" + string(rune('a'+i)),
			Description: "Property " + string(rune('a'+i)),
			Required:    i%2 == 0,
			ContextKey:  i,
		}
	}

	result := AddExtraPropertiesToTool(tool, props)

	var resultSchema map[string]any
	g.Expect(json.Unmarshal(result.RawInputSchema, &resultSchema)).To(Succeed())

	resultProps := resultSchema["properties"].(map[string]any)
	g.Expect(resultProps).To(HaveLen(20))
}
