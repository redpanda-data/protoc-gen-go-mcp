// Copyright 2025 Redpanda Data, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package generator

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"

	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
)

// TestOpenAIMapWithMessageValueTypeBug reproduces the specific bug where
// map fields with message value types (like map<string, Tool>) are missing
// from OpenAI schema properties, causing OpenAI clients to send empty arguments
func TestOpenAIMapWithMessageValueTypeBug(t *testing.T) {
	RegisterTestingT(t)

	fg := &FileGenerator{}

	t.Run("UpdateMCPServerRequest map field with message value type", func(t *testing.T) {
		g := NewWithT(t)

		descriptor := (&testdata.UpdateMCPServerRequest{}).ProtoReflect().Descriptor()

		// Test standard schema generation
		fg.openAICompat = false
		standardSchema := fg.messageSchema(descriptor)

		standardJSON, err := json.MarshalIndent(standardSchema, "", "  ")
		g.Expect(err).ToNot(HaveOccurred())
		t.Logf("=== STANDARD SCHEMA ===\n%s", string(standardJSON))

		// Extract mcp_server field from standard schema
		standardProps := standardSchema["properties"].(map[string]any)
		g.Expect(standardProps).To(HaveKey("mcp_server"), "Standard schema should have mcp_server field")

		standardMcpServer := standardProps["mcp_server"].(map[string]any)
		standardMcpServerProps := standardMcpServer["properties"].(map[string]any)
		g.Expect(standardMcpServerProps).To(HaveKey("tools"), "Standard schema mcp_server should have tools field")

		// Test OpenAI schema generation
		fg.openAICompat = true
		openAISchema := fg.messageSchema(descriptor)

		openAIJSON, err := json.MarshalIndent(openAISchema, "", "  ")
		g.Expect(err).ToNot(HaveOccurred())
		t.Logf("=== OPENAI SCHEMA ===\n%s", string(openAIJSON))

		// Extract mcp_server field from OpenAI schema
		openAIProps := openAISchema["properties"].(map[string]any)
		g.Expect(openAIProps).To(HaveKey("mcp_server"), "OpenAI schema should have mcp_server field")

		openAIMcpServer := openAIProps["mcp_server"].(map[string]any)
		openAIMcpServerProps := openAIMcpServer["properties"].(map[string]any)

		t.Logf("Standard mcp_server properties: %v", getKeys(standardMcpServerProps))
		t.Logf("OpenAI mcp_server properties: %v", getKeys(openAIMcpServerProps))

		// THE BUG: OpenAI schema is missing the tools field entirely
		// This happens specifically for map<string, MessageType> fields
		g.Expect(openAIMcpServerProps).To(HaveKey("tools"),
			"CRITICAL BUG: OpenAI schema missing 'tools' field (map<string, MCPServer.Tool>)")

		// Verify the tools field has proper array structure in OpenAI mode
		toolsField, hasTools := openAIMcpServerProps["tools"]
		g.Expect(hasTools).To(BeTrue(), "tools field should exist in OpenAI schema")

		toolsMap := toolsField.(map[string]any)
		g.Expect(toolsMap["type"]).To(Equal("array"), "OpenAI tools field should be array type")
		g.Expect(toolsMap).To(HaveKey("items"), "OpenAI tools array should have items definition")
	})

	t.Run("MCPServer direct schema generation", func(t *testing.T) {
		g := NewWithT(t)

		descriptor := (&testdata.MCPServer{}).ProtoReflect().Descriptor()

		// Standard mode
		fg.openAICompat = false
		standardSchema := fg.messageSchema(descriptor)
		standardProps := standardSchema["properties"].(map[string]any)

		// OpenAI mode
		fg.openAICompat = true
		openAISchema := fg.messageSchema(descriptor)
		openAIProps := openAISchema["properties"].(map[string]any)

		t.Logf("Standard MCPServer properties: %v", getKeys(standardProps))
		t.Logf("OpenAI MCPServer properties: %v", getKeys(openAIProps))

		// Both should have the tools field
		g.Expect(standardProps).To(HaveKey("tools"), "Standard MCPServer should have tools field")
		g.Expect(openAIProps).To(HaveKey("tools"), "OpenAI MCPServer should have tools field")

		// Compare field types
		standardTools := standardProps["tools"].(map[string]any)
		openAITools := openAIProps["tools"].(map[string]any)

		t.Logf("Standard tools field: %+v", standardTools)
		t.Logf("OpenAI tools field: %+v", openAITools)

		// Standard should be object with additionalProperties
		g.Expect(standardTools["type"]).To(Equal("object"))
		g.Expect(standardTools).To(HaveKey("additionalProperties"))

		// OpenAI should be array with items
		g.Expect(openAITools["type"]).To(Equal("array"))
		g.Expect(openAITools).To(HaveKey("items"))
	})
}
