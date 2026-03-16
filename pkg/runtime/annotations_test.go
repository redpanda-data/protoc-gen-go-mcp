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

	"github.com/mark3labs/mcp-go/mcp"
	. "github.com/onsi/gomega"
)

// Helper function to create bool pointers
func boolPtr(b bool) *bool {
	return &b
}

func TestWithToolAnnotations_Creation(t *testing.T) {
	g := NewWithT(t)

	// Test with all fields set
	cfg := ToolAnnotationConfig{
		Title:           "Test Tool",
		ReadOnlyHint:    boolPtr(true),
		DestructiveHint: boolPtr(false),
		IdempotentHint:  boolPtr(true),
		OpenWorldHint:   boolPtr(false),
	}

	opt := WithToolAnnotations(cfg)
	g.Expect(opt).ToNot(BeNil())

	// Apply the option to a config
	c := &config{}
	opt(c)

	g.Expect(c.Annotations).ToNot(BeNil())
	g.Expect(c.Annotations.Title).To(Equal("Test Tool"))
	g.Expect(*c.Annotations.ReadOnlyHint).To(BeTrue())
	g.Expect(*c.Annotations.DestructiveHint).To(BeFalse())
	g.Expect(*c.Annotations.IdempotentHint).To(BeTrue())
	g.Expect(*c.Annotations.OpenWorldHint).To(BeFalse())
}

func TestWithToolAnnotations_PartialFields(t *testing.T) {
	g := NewWithT(t)

	// Test with only Title set
	cfg := ToolAnnotationConfig{
		Title: "Read Only Query",
	}

	opt := WithToolAnnotations(cfg)
	c := &config{}
	opt(c)

	g.Expect(c.Annotations).ToNot(BeNil())
	g.Expect(c.Annotations.Title).To(Equal("Read Only Query"))
	g.Expect(c.Annotations.ReadOnlyHint).To(BeNil())
	g.Expect(c.Annotations.DestructiveHint).To(BeNil())
}

func TestApplyToolAnnotations_BasicApplication(t *testing.T) {
	g := NewWithT(t)

	// Create a tool without annotations
	originalSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
	schemaBytes, err := json.Marshal(originalSchema)
	g.Expect(err).ToNot(HaveOccurred())

	tool := mcp.Tool{
		Name:           "test_tool",
		Description:    "A test tool",
		RawInputSchema: json.RawMessage(schemaBytes),
	}

	// Apply annotations
	result := ApplyToolAnnotations(tool, WithToolAnnotations(ToolAnnotationConfig{
		Title:        "Test Tool",
		ReadOnlyHint: boolPtr(true),
	}))

	// Verify annotations are set
	g.Expect(result.Annotations.Title).To(Equal("Test Tool"))
	g.Expect(result.Annotations.ReadOnlyHint).ToNot(BeNil())
	g.Expect(*result.Annotations.ReadOnlyHint).To(BeTrue())
}

func TestApplyToolAnnotations_NoOptions(t *testing.T) {
	g := NewWithT(t)

	// Create a tool
	originalSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
	schemaBytes, err := json.Marshal(originalSchema)
	g.Expect(err).ToNot(HaveOccurred())

	tool := mcp.Tool{
		Name:           "test_tool",
		Description:    "A test tool",
		RawInputSchema: json.RawMessage(schemaBytes),
	}

	// Apply no options
	result := ApplyToolAnnotations(tool)

	// Verify tool is unchanged (no annotations added - empty struct)
	g.Expect(result.Annotations.Title).To(BeEmpty())
	g.Expect(result.Annotations.ReadOnlyHint).To(BeNil())
	g.Expect(result.Name).To(Equal("test_tool"))
	g.Expect(result.Description).To(Equal("A test tool"))
}

func TestApplyToolAnnotations_PreservesOtherFields(t *testing.T) {
	g := NewWithT(t)

	// Create a tool with all fields set
	originalSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []string{"query"},
	}
	schemaBytes, err := json.Marshal(originalSchema)
	g.Expect(err).ToNot(HaveOccurred())

	tool := mcp.Tool{
		Name:           "search_tool",
		Description:    "Search for items",
		RawInputSchema: json.RawMessage(schemaBytes),
	}

	// Apply annotations
	result := ApplyToolAnnotations(tool, WithToolAnnotations(ToolAnnotationConfig{
		Title:        "Search Tool",
		ReadOnlyHint: boolPtr(true),
	}))

	// Verify original fields are preserved
	g.Expect(result.Name).To(Equal("search_tool"))
	g.Expect(result.Description).To(Equal("Search for items"))

	// Verify schema is unchanged
	var resultSchema map[string]interface{}
	err = json.Unmarshal(result.RawInputSchema, &resultSchema)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resultSchema["type"]).To(Equal("object"))

	properties := resultSchema["properties"].(map[string]interface{})
	g.Expect(properties).To(HaveKey("query"))
}

func TestApplyToolAnnotations_AllHintTypes(t *testing.T) {
	g := NewWithT(t)

	tool := mcp.Tool{
		Name:        "full_annotated_tool",
		Description: "A tool with all annotations",
	}

	// Apply all annotation types
	result := ApplyToolAnnotations(tool, WithToolAnnotations(ToolAnnotationConfig{
		Title:           "Full Annotated Tool",
		ReadOnlyHint:    boolPtr(false),
		DestructiveHint: boolPtr(true),
		IdempotentHint:  boolPtr(false),
		OpenWorldHint:   boolPtr(true),
	}))

	// Verify all annotations are set correctly
	g.Expect(result.Annotations.Title).To(Equal("Full Annotated Tool"))
	g.Expect(*result.Annotations.ReadOnlyHint).To(BeFalse())
	g.Expect(*result.Annotations.DestructiveHint).To(BeTrue())
	g.Expect(*result.Annotations.IdempotentHint).To(BeFalse())
	g.Expect(*result.Annotations.OpenWorldHint).To(BeTrue())
}

func TestApplyOptions_CombinedAnnotationsAndExtraProperties(t *testing.T) {
	g := NewWithT(t)

	// Create a tool with a basic schema
	originalSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type": "string",
			},
		},
	}
	schemaBytes, err := json.Marshal(originalSchema)
	g.Expect(err).ToNot(HaveOccurred())

	tool := mcp.Tool{
		Name:           "api_tool",
		Description:    "A tool that calls an API",
		RawInputSchema: json.RawMessage(schemaBytes),
	}

	// Apply both annotations and extra properties
	result := ApplyOptions(tool,
		WithToolAnnotations(ToolAnnotationConfig{
			Title:         "API Tool",
			OpenWorldHint: boolPtr(true),
		}),
		WithExtraProperties(ExtraProperty{
			Name:        "base_url",
			Description: "API base URL",
			Required:    true,
		}),
	)

	// Verify annotations are set
	g.Expect(result.Annotations.Title).To(Equal("API Tool"))
	g.Expect(*result.Annotations.OpenWorldHint).To(BeTrue())

	// Verify extra properties are added to schema
	var resultSchema map[string]interface{}
	err = json.Unmarshal(result.RawInputSchema, &resultSchema)
	g.Expect(err).ToNot(HaveOccurred())

	properties := resultSchema["properties"].(map[string]interface{})
	g.Expect(properties).To(HaveKey("base_url"))
	g.Expect(properties).To(HaveKey("query"))
}

func TestApplyOptions_OnlyAnnotations(t *testing.T) {
	g := NewWithT(t)

	tool := mcp.Tool{
		Name:        "simple_tool",
		Description: "A simple tool",
	}

	// Apply only annotations
	result := ApplyOptions(tool, WithToolAnnotations(ToolAnnotationConfig{
		Title:        "Simple Tool",
		ReadOnlyHint: boolPtr(true),
	}))

	// Verify annotations are set
	g.Expect(result.Annotations.Title).To(Equal("Simple Tool"))
}

func TestApplyOptions_OnlyExtraProperties(t *testing.T) {
	g := NewWithT(t)

	originalSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
	schemaBytes, err := json.Marshal(originalSchema)
	g.Expect(err).ToNot(HaveOccurred())

	tool := mcp.Tool{
		Name:           "props_tool",
		Description:    "A tool with extra properties",
		RawInputSchema: json.RawMessage(schemaBytes),
	}

	// Apply only extra properties
	result := ApplyOptions(tool, WithExtraProperties(ExtraProperty{
		Name:        "api_key",
		Description: "API key",
		Required:    false,
	}))

	// Verify annotations are NOT set (empty struct)
	g.Expect(result.Annotations.Title).To(BeEmpty())
	g.Expect(result.Annotations.ReadOnlyHint).To(BeNil())

	// Verify extra properties are added
	var resultSchema map[string]interface{}
	err = json.Unmarshal(result.RawInputSchema, &resultSchema)
	g.Expect(err).ToNot(HaveOccurred())

	properties := resultSchema["properties"].(map[string]interface{})
	g.Expect(properties).To(HaveKey("api_key"))
}
