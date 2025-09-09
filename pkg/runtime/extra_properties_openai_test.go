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
package runtime

import (
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/encoding/protojson"

	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
)

// TestExtraPropertiesWithOpenAI reproduces the URL corruption issue
// when extra properties (like dataplane_api_url) are used with OpenAI compatibility mode
func TestExtraPropertiesWithOpenAI(t *testing.T) {
	RegisterTestingT(t)

	t.Run("dataplane_api_url corruption in UpdateMCPServer scenario", func(t *testing.T) {
		g := NewWithT(t)

		// Simulate the exact scenario from the user's request
		descriptor := new(testdata.CreateItemRequest)

		yamlConfig := `label: get_weather
processors:
- label: fetch_weather
  http:
    url: 'https://wttr.in/${! @city_name }?format=j1'
    verb: GET
    headers:
      Accept: "application/json"
    timeout: "30s"`

		// This simulates the OpenAI request structure with extra properties + map conversion
		// The tools field is sent as array-of-key-value pairs due to map conversion
		// AND there's an extra dataplane_api_url field
		message := map[string]any{
			"name":              "example-http",
			"description":       "Weather information fetcher",
			"dataplane_api_url": "https://api-47f10b24.d2vu1l4bgmjo2sp77do0.fmc.ign.cloud.redpanda.com", // This is the extra property
			"labels": []any{
				map[string]any{
					"key":   "get_weather",
					"value": yamlConfig, // This contains the URL that gets corrupted
				},
			},
		}

		// First, apply FixOpenAI processing (this happens first in the generated code)
		FixOpenAI(descriptor.ProtoReflect().Descriptor(), message)

		// Now extract extra properties (this happens second in the generated code)
		var extractedURL string
		if propVal, ok := message["dataplane_api_url"]; ok {
			if urlStr, ok := propVal.(string); ok {
				extractedURL = urlStr
			}
			// Remove extra property from message (this is what the fix does)
			delete(message, "dataplane_api_url")
		}

		// Verify the dataplane_api_url is correctly extracted
		g.Expect(extractedURL).To(Equal("https://api-47f10b24.d2vu1l4bgmjo2sp77do0.fmc.ign.cloud.redpanda.com"))

		// Verify the nested YAML config with URL is still intact
		labels := message["labels"].(map[string]any)
		configYaml := labels["get_weather"].(string)

		// This should NOT be empty or corrupted
		g.Expect(configYaml).ToNot(BeEmpty(), "config_yaml should not be empty")
		g.Expect(configYaml).To(ContainSubstring("https://wttr.in/"), "URL in config should be preserved")
		g.Expect(configYaml).To(ContainSubstring("format=j1"), "URL parameters should be preserved")

		// Finally, test that the message can be unmarshaled into a protobuf
		marshaledJSON, err := json.Marshal(message)
		g.Expect(err).ToNot(HaveOccurred())

		var req testdata.CreateItemRequest
		err = protojson.Unmarshal(marshaledJSON, &req)
		g.Expect(err).ToNot(HaveOccurred(), "should be able to unmarshal after OpenAI processing")

		// Verify the content made it through
		g.Expect(req.Name).To(Equal("example-http"))
		g.Expect(req.Labels["get_weather"]).To(ContainSubstring("https://wttr.in/"))
	})

	t.Run("test AddExtraPropertiesToTool with OpenAI schema", func(t *testing.T) {
		g := NewWithT(t)

		// Create a basic tool schema (OpenAI compatible)
		baseSchema := map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"name": map[string]any{
					"type": "string",
				},
			},
			"required": []any{"name"},
		}

		baseSchemaJSON, err := json.Marshal(baseSchema)
		g.Expect(err).ToNot(HaveOccurred())

		baseTool := mcp.Tool{
			Name:           "test_tool",
			Description:    "Test tool",
			RawInputSchema: json.RawMessage(baseSchemaJSON),
		}

		// Add the dataplane_api_url extra property
		extraProperty := ExtraProperty{
			Name:        "dataplane_api_url",
			Description: "URL to connect to this dataplane",
			Required:    true,
			ContextKey:  "dataplane_url_key",
		}

		modifiedTool := AddExtraPropertiesToTool(baseTool, []ExtraProperty{extraProperty})

		// Verify the schema was modified correctly
		var modifiedSchema map[string]any
		err = json.Unmarshal(modifiedTool.RawInputSchema, &modifiedSchema)
		g.Expect(err).ToNot(HaveOccurred())

		// Check that the extra property was added
		properties := modifiedSchema["properties"].(map[string]any)
		g.Expect(properties).To(HaveKey("dataplane_api_url"))

		urlProperty := properties["dataplane_api_url"].(map[string]any)
		g.Expect(urlProperty["type"]).To(Equal("string"))
		g.Expect(urlProperty["description"]).To(Equal("URL to connect to this dataplane"))

		// Check that it was added to required fields
		required := modifiedSchema["required"].([]any)
		g.Expect(required).To(ContainElement("dataplane_api_url"))
		g.Expect(required).To(ContainElement("name")) // original required field should still be there
	})
}
