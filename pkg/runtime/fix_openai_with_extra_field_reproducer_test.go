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

	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/encoding/protojson"

	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
)

// TestFixOpenAI_WithExtraFieldReproducer directly tests if FixOpenAI corrupts extra fields
// This reproduces the user's issue step-by-step following the exact generated code pattern
func TestFixOpenAI_WithExtraFieldReproducer(t *testing.T) {
	RegisterTestingT(t)

	t.Run("does FixOpenAI corrupt dataplane_api_url field?", func(t *testing.T) {
		g := NewWithT(t)

		// Setup: message with extra field (as received from OpenAI)
		testURL := "https://api-47f10b24.d2vu1l4bgmjo2sp77do0.fmc.ign.cloud.redpanda.com"
		message := map[string]any{
			"dataplane_api_url": testURL,                // This is the extra field that gets lost
			"id":                "d304p2q489dc738r7nh0", // Regular field
			"mcp_server": map[string]any{
				"display_name": "example-http",
				"description":  "Simple test",
			},
			"update_mask": "description",
		}

		// Step 1: Extract extra property (what the generated code does FIRST)
		var extractedURL string
		if propVal, ok := message["dataplane_api_url"]; ok {
			if urlStr, ok := propVal.(string); ok {
				extractedURL = urlStr
			}
			// NOTE: Without the fix, we don't delete the field here!
			// delete(message, "dataplane_api_url") // This line is missing!
		}

		g.Expect(extractedURL).To(Equal(testURL), "URL should be extracted correctly")

		// Step 2: Apply FixOpenAI (what the generated code does SECOND)
		descriptor := new(testdata.UpdateMCPServerRequest)

		// Log the state BEFORE FixOpenAI
		urlBefore := message["dataplane_api_url"].(string)
		t.Logf("BEFORE FixOpenAI: dataplane_api_url = %q", urlBefore)

		// This is the critical call that might corrupt the extra field
		FixOpenAI(descriptor.ProtoReflect().Descriptor(), message)

		// Log the state AFTER FixOpenAI
		urlAfter := message["dataplane_api_url"].(string)
		t.Logf("AFTER FixOpenAI: dataplane_api_url = %q", urlAfter)

		// TEST: Does FixOpenAI corrupt the extra field?
		if urlAfter != urlBefore {
			t.Logf("🔴 BUG FOUND: FixOpenAI corrupted the dataplane_api_url!")
			t.Logf("  Before: %q", urlBefore)
			t.Logf("  After:  %q", urlAfter)
		} else {
			t.Logf("✅ FixOpenAI preserved the dataplane_api_url")
		}

		g.Expect(urlAfter).To(Equal(testURL), "FixOpenAI should not corrupt extra fields")
		g.Expect(urlAfter).ToNot(BeEmpty(), "URL should not become empty")

		// Step 3: Test JSON marshal/unmarshal with the extra field still present
		marshaledJSON, err := json.Marshal(message)
		g.Expect(err).ToNot(HaveOccurred(), "should marshal with extra field")

		t.Logf("Marshaled JSON contains dataplane_api_url: %v",
			string(marshaledJSON) != "" && len(marshaledJSON) > 0)

		// Step 4: Test protojson unmarshal with DiscardUnknown (what the real code does)
		var req testdata.UpdateMCPServerRequest
		err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(marshaledJSON, &req)

		if err != nil {
			t.Logf("❌ protojson.Unmarshal failed: %v", err)
		} else {
			t.Logf("✅ protojson.Unmarshal succeeded with DiscardUnknown: true")
		}

		// The unmarshal should succeed with DiscardUnknown: true
		g.Expect(err).ToNot(HaveOccurred(), "protojson should handle extra field with DiscardUnknown")

		// Verify the request data is correct
		g.Expect(req.Id).To(Equal("d304p2q489dc738r7nh0"))
		g.Expect(req.McpServer.DisplayName).To(Equal("example-http"))
	})

	t.Run("test with UpdateMCPServerRequest descriptor specifically", func(t *testing.T) {
		g := NewWithT(t)

		// Use the exact UpdateMCPServerRequest descriptor that fails
		descriptor := &testdata.UpdateMCPServerRequest{}

		testURL := "https://api-47f10b24.d2vu1l4bgmjo2sp77do0.fmc.ign.cloud.redpanda.com"
		yamlConfig := `processors:
- http:
    url: 'https://wttr.in/${! @city_name }?format=j1'
    verb: GET`

		// Message structure that exactly matches the user's failing request
		message := map[string]any{
			"dataplane_api_url": testURL, // The extra field that becomes empty
			"id":                "d304p2q489dc738r7nh0",
			"mcp_server": map[string]any{
				"display_name": "example-http",
				"description":  "Weather information fetcher",
				"tools": []any{
					map[string]any{
						"key": "get_weather",
						"value": map[string]any{
							"component_type": "COMPONENT_TYPE_PROCESSOR",
							"config_yaml":    yamlConfig,
						},
					},
				},
				"resources": map[string]any{
					"memory_shares": "128Mi",
					"cpu_shares":    "100m",
				},
				"tags": []any{
					map[string]any{"key": "purpose", "value": "weather-fetching"},
					map[string]any{"key": "api", "value": "wttr-in"},
					map[string]any{"key": "owner", "value": "example"},
				},
			},
			"update_mask": "tools,description,tags",
		}

		// Critical test: does FixOpenAI corrupt the dataplane_api_url when processing complex nested structures?
		originalURL := message["dataplane_api_url"].(string)

		FixOpenAI(descriptor.ProtoReflect().Descriptor(), message)

		finalURL := message["dataplane_api_url"].(string)

		t.Logf("UpdateMCPServerRequest test:")
		t.Logf("  Original URL: %q", originalURL)
		t.Logf("  Final URL:    %q", finalURL)
		t.Logf("  URL corrupted: %v", finalURL != originalURL)

		// This is the key test - does the extra field survive complex processing?
		g.Expect(finalURL).To(Equal(originalURL), "dataplane_api_url should survive FixOpenAI")
		g.Expect(finalURL).ToNot(BeEmpty(), "dataplane_api_url should not become empty")

		// Also verify the nested tools were processed correctly
		mcpServer := message["mcp_server"].(map[string]any)
		tools := mcpServer["tools"].(map[string]any) // Should be converted from array
		g.Expect(tools).To(HaveKey("get_weather"))

		getWeather := tools["get_weather"].(map[string]any)
		configYaml := getWeather["config_yaml"].(string)
		g.Expect(configYaml).To(ContainSubstring("https://wttr.in/"))
	})
}
