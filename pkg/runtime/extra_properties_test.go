// Copyright 2025 Redpanda Data, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runtime

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
)

// Test context key types
type (
	baseURLKey struct{}
)

func TestAddExtraPropertiesToTool(t *testing.T) {
	g := NewWithT(t)

	// Create a test tool with a basic schema
	originalSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
			"age": map[string]interface{}{
				"type": "integer",
			},
		},
		"required": []string{"name"},
	}

	schemaBytes, err := json.Marshal(originalSchema)
	g.Expect(err).ToNot(HaveOccurred())

	tool := Tool{
		Name:           "test_tool",
		Description:    "A test tool",
		RawInputSchema: json.RawMessage(schemaBytes),
	}

	// Add extra properties to the tool
	extraProps := []ExtraProperty{
		{
			Name:        "base_url",
			Description: "Base URL for the API",
			Required:    true,
			ContextKey:  baseURLKey{},
		},
		{
			Name:        "api_key",
			Description: "API key for authentication",
			Required:    false,
			ContextKey:  "api_key",
		},
	}
	modifiedTool := AddExtraPropertiesToTool(tool, extraProps)

	// Parse the modified schema
	var modifiedSchema map[string]interface{}
	err = json.Unmarshal(modifiedTool.RawInputSchema, &modifiedSchema)
	g.Expect(err).ToNot(HaveOccurred())

	// Verify the properties were added
	properties := modifiedSchema["properties"].(map[string]interface{})
	g.Expect(properties).To(HaveKey("base_url"))
	g.Expect(properties).To(HaveKey("api_key"))

	// Verify base_url field
	urlField := properties["base_url"].(map[string]interface{})
	g.Expect(urlField["type"]).To(Equal("string"))
	g.Expect(urlField["description"]).To(Equal("Base URL for the API"))
	g.Expect(urlField).ToNot(HaveKey("format")) // No special format handling

	// Verify API key field
	apiKeyField := properties["api_key"].(map[string]interface{})
	g.Expect(apiKeyField["type"]).To(Equal("string"))
	g.Expect(apiKeyField["description"]).To(Equal("API key for authentication"))
	g.Expect(apiKeyField).ToNot(HaveKey("format"))

	// Verify original fields are still there
	g.Expect(properties).To(HaveKey("name"))
	g.Expect(properties).To(HaveKey("age"))

	// Verify only required fields were added to required array
	g.Expect(modifiedSchema["required"]).To(Equal([]interface{}{"name", "base_url"}))
}

func TestAddExtraPropertiesToToolWithNoProperties(t *testing.T) {
	g := NewWithT(t)

	// Create a tool with no properties
	originalSchema := map[string]interface{}{
		"type": "object",
	}

	schemaBytes, err := json.Marshal(originalSchema)
	g.Expect(err).ToNot(HaveOccurred())

	tool := Tool{
		Name:           "test_tool",
		Description:    "A test tool",
		RawInputSchema: json.RawMessage(schemaBytes),
	}

	// Add extra properties to the tool
	extraProps := []ExtraProperty{
		{
			Name:        "base_url",
			Description: "Base URL for the API",
			Required:    true,
			ContextKey:  baseURLKey{},
		},
	}
	modifiedTool := AddExtraPropertiesToTool(tool, extraProps)

	// Parse the modified schema
	var modifiedSchema map[string]interface{}
	err = json.Unmarshal(modifiedTool.RawInputSchema, &modifiedSchema)
	g.Expect(err).ToNot(HaveOccurred())

	// Verify the base_url field was added
	properties := modifiedSchema["properties"].(map[string]interface{})
	g.Expect(properties).To(HaveKey("base_url"))

	urlField := properties["base_url"].(map[string]interface{})
	g.Expect(urlField["type"]).To(Equal("string"))
	g.Expect(urlField["description"]).To(Equal("Base URL for the API"))
	g.Expect(urlField).ToNot(HaveKey("format")) // No special format handling

	// Verify the URL field was added to required fields
	g.Expect(modifiedSchema["required"]).To(Equal([]interface{}{"base_url"}))
}

func TestAddExtraPropertiesToToolWithInvalidSchema(t *testing.T) {
	g := NewWithT(t)

	// Create a tool with invalid JSON schema
	tool := Tool{
		Name:           "test_tool",
		Description:    "A test tool",
		RawInputSchema: json.RawMessage([]byte("invalid json")),
	}

	// Add extra properties to the tool - should return original tool due to invalid schema
	extraProps := []ExtraProperty{
		{
			Name:        "base_url",
			Description: "Base URL for the API",
			Required:    true,
			ContextKey:  baseURLKey{},
		},
	}
	modifiedTool := AddExtraPropertiesToTool(tool, extraProps)

	// Verify the tool is unchanged
	g.Expect(modifiedTool).To(Equal(tool))
}

func TestAddExtraPropertiesToToolWithCustomFieldName(t *testing.T) {
	g := NewWithT(t)

	// Create a test tool with a basic schema
	originalSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []string{"name"},
	}

	schemaBytes, err := json.Marshal(originalSchema)
	g.Expect(err).ToNot(HaveOccurred())

	tool := Tool{
		Name:           "test_tool",
		Description:    "A test tool",
		RawInputSchema: json.RawMessage(schemaBytes),
	}

	// Add custom field name to the tool
	extraProps := []ExtraProperty{
		{
			Name:        "api_url",
			Description: "Custom API endpoint URL",
			Required:    true,
			ContextKey:  baseURLKey{},
		},
	}
	modifiedTool := AddExtraPropertiesToTool(tool, extraProps)

	// Parse the modified schema
	var modifiedSchema map[string]interface{}
	err = json.Unmarshal(modifiedTool.RawInputSchema, &modifiedSchema)
	g.Expect(err).ToNot(HaveOccurred())

	// Verify the custom field was added
	properties := modifiedSchema["properties"].(map[string]interface{})
	g.Expect(properties).To(HaveKey("api_url"))
	g.Expect(properties).ToNot(HaveKey("base_url")) // Should not have the default base_url field

	apiField := properties["api_url"].(map[string]interface{})
	g.Expect(apiField["type"]).To(Equal("string"))
	g.Expect(apiField["description"]).To(Equal("Custom API endpoint URL"))
	g.Expect(apiField).ToNot(HaveKey("format")) // No special format handling

	// Verify original fields are still there
	g.Expect(properties).To(HaveKey("name"))

	// Verify the URL field was added to required fields
	g.Expect(modifiedSchema["required"]).To(Equal([]interface{}{"name", "api_url"}))
}
