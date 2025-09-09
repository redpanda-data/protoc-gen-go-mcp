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

// TestOpenAISchemaBug reproduces the exact bug where map fields are missing from OpenAI schemas
// This causes OpenAI clients to send empty arguments because the schema is incomplete
func TestOpenAISchemaBug(t *testing.T) {
	RegisterTestingT(t)

	// Create a file generator for testing
	fg := &FileGenerator{}

	t.Run("UpdateMCPServerRequest OpenAI schema should include tools field", func(t *testing.T) {
		g := NewWithT(t)

		// Get the descriptor for the reproducer UpdateMCPServerRequest (closer to real schema)
		descriptor := (&testdata.UpdateMCPServerRequest{}).ProtoReflect().Descriptor()

		// Test standard mode schema generation
		fg.openAICompat = false
		standardSchema := fg.messageSchema(descriptor)

		standardJSON, err := json.MarshalIndent(standardSchema, "", "  ")
		g.Expect(err).ToNot(HaveOccurred())
		t.Logf("=== STANDARD SCHEMA ===\n%s", string(standardJSON))

		// Verify standard schema has the tools field properly defined
		props := standardSchema["properties"].(map[string]any)
		g.Expect(props).To(HaveKey("mcp_server"), "Standard schema should have mcp_server")

		mcpServerProps := props["mcp_server"].(map[string]any)["properties"].(map[string]any)
		g.Expect(mcpServerProps).To(HaveKey("tools"), "Standard schema should have tools property")

		// Test OpenAI mode schema generation
		fg.openAICompat = true
		openAISchema := fg.messageSchema(descriptor)

		openAIJSON, err := json.MarshalIndent(openAISchema, "", "  ")
		g.Expect(err).ToNot(HaveOccurred())
		t.Logf("=== OPENAI SCHEMA ===\n%s", string(openAIJSON))

		// Verify OpenAI schema has the tools field properly defined
		openAIProps := openAISchema["properties"].(map[string]any)
		g.Expect(openAIProps).To(HaveKey("mcp_server"), "OpenAI schema should have mcp_server")

		openAIMcpServerProps := openAIProps["mcp_server"].(map[string]any)["properties"].(map[string]any)
		g.Expect(openAIMcpServerProps).To(HaveKey("tools"), "OpenAI schema should have tools property")

		// Verify the tools field is defined as array of key-value pairs in OpenAI mode
		toolsField := openAIMcpServerProps["tools"].(map[string]any)
		g.Expect(toolsField["type"]).To(Equal("array"), "OpenAI tools should be array")
		g.Expect(toolsField).To(HaveKey("items"), "OpenAI tools array should have items definition")

		// The bug: if this test fails, it means the tools field is missing from OpenAI schema
		// which causes OpenAI clients to send empty arguments
	})

	t.Run("OpenAI schema should have all required properties", func(t *testing.T) {
		g := NewWithT(t)

		descriptor := (&testdata.UpdateMCPServerRequest{}).ProtoReflect().Descriptor()

		// OpenAI schema
		fg.openAICompat = true
		openAISchema := fg.messageSchema(descriptor)
		openAIProps := openAISchema["properties"].(map[string]any)
		openAIMcpServer := openAIProps["mcp_server"].(map[string]any)
		openAIMcpServerProps := openAIMcpServer["properties"].(map[string]any)

		t.Logf("OpenAI mcp_server properties: %v", getKeys(openAIMcpServerProps))

		// The critical test: tools field should be defined in OpenAI schema
		g.Expect(openAIMcpServerProps).To(HaveKey("tools"), "OpenAI schema must include tools property")
		g.Expect(openAIMcpServerProps).To(HaveKey("description"), "OpenAI schema must include description property")
		g.Expect(openAIMcpServerProps).To(HaveKey("display_name"), "OpenAI schema must include display_name property")
		g.Expect(openAIMcpServerProps).To(HaveKey("resources"), "OpenAI schema must include resources property")
		g.Expect(openAIMcpServerProps).To(HaveKey("tags"), "OpenAI schema must include tags property")

		// Verify tools is properly formatted as array in OpenAI mode
		toolsField := openAIMcpServerProps["tools"].(map[string]any)
		g.Expect(toolsField["type"]).To(Equal("array"), "OpenAI tools should be array type")
	})
}

func getKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
