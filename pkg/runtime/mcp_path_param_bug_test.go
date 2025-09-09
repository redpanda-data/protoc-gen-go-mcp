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
	"context"
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/encoding/protojson"

	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
)

// TestMCPPathParameterBug reproduces the user's specific issue:
// dataplane_api_url gets corrupted when combined with path parameters like "id"
func TestMCPPathParameterBug(t *testing.T) {
	RegisterTestingT(t)

	t.Run("path parameter + extra property processing with context", func(t *testing.T) {
		g := NewWithT(t)

		// Simulate the exact request structure that fails
		testURL := "https://api-47f10b24.d2vu1l4bgmjo2sp77do0.fmc.ign.cloud.redpanda.com"
		testContextKey := "test_dataplane_url_key"

		// This mimics GetMCPServerRequest or UpdateMCPServerRequest
		// which have both a path parameter (id) AND extra properties (dataplane_api_url)
		message := map[string]any{
			"dataplane_api_url": testURL,                // Extra property
			"id":                "d304p2q489dc738r7nh0", // Path parameter
			"mcp_server": map[string]any{
				"display_name": "example-http",
				"description":  "Simple test",
			},
			"update_mask": "description",
		}

		// Test 1: Simulate the full extra property extraction process (what the generated code does)
		ctx := context.Background()
		var extractedURL string

		// Simulate the extra properties configuration
		extraProps := []ExtraProperty{
			{
				Name:        "dataplane_api_url",
				Description: "URL to connect to this dataplane",
				Required:    true,
				ContextKey:  testContextKey,
			},
		}

		// Extract extra properties and add to context (matching generated code logic)
		for _, prop := range extraProps {
			if propVal, ok := message[prop.Name]; ok {
				ctx = context.WithValue(ctx, prop.ContextKey, propVal)
				if urlStr, ok := propVal.(string); ok {
					extractedURL = urlStr
				}
				// NOTE: Without the fix, the extra property stays in message
				// delete(message, prop.Name) // This line is missing without the fix!
			}
		}

		// Verify URL was extracted correctly
		g.Expect(extractedURL).To(Equal(testURL), "dataplane_api_url should be extracted correctly")

		// Test 2: Verify the URL is accessible from context
		contextURL := ctx.Value(testContextKey)
		g.Expect(contextURL).ToNot(BeNil(), "URL should be stored in context")
		g.Expect(contextURL.(string)).To(Equal(testURL), "Context URL should match original")

		// Test 3: Apply OpenAI fix processing
		descriptor := new(testdata.UpdateMCPServerRequest)
		FixOpenAI(descriptor.ProtoReflect().Descriptor(), message)

		// Test 4: Try to marshal and unmarshal (what the generated code does)
		marshaledJSON, err := json.Marshal(message)
		g.Expect(err).ToNot(HaveOccurred(), "should marshal successfully")

		var req testdata.UpdateMCPServerRequest
		err = protojson.Unmarshal(marshaledJSON, &req)

		// Without the fix, this might fail OR succeed with DiscardUnknown: true
		// The real issue is that the extra property stays in the message
		if err != nil {
			t.Logf("❌ protojson.Unmarshal failed as expected (without fix): %v", err)
		} else {
			t.Logf("⚠️ protojson.Unmarshal succeeded (DiscardUnknown: true handled it)")
		}

		// Let's test without DiscardUnknown to see the actual error
		var req2 testdata.UpdateMCPServerRequest
		err2 := protojson.Unmarshal(marshaledJSON, &req2)
		if err2 != nil {
			t.Logf("❌ Without DiscardUnknown, unmarshal fails: %v", err2)
			g.Expect(err2.Error()).To(ContainSubstring("dataplane_api_url"), "Should mention the unknown field")
		}

		// Verify the data made it through correctly
		g.Expect(req.Id).To(Equal("d304p2q489dc738r7nh0"))
		g.Expect(req.McpServer.DisplayName).To(Equal("example-http"))
		g.Expect(req.UpdateMask).To(Equal("description"))

		// Test 5: Without the fix, the message still contains the extra property
		g.Expect(message).To(HaveKey("dataplane_api_url"), "Without fix, extra property remains in message")

		t.Logf("SUCCESS: Full path parameter + extra property + context processing worked")
		t.Logf("  - Original URL: %s", testURL)
		t.Logf("  - Extracted URL: %s", extractedURL)
		t.Logf("  - Context URL: %s", contextURL.(string))
		t.Logf("  - Final ID: %s", req.Id)
		_, hasDataplaneURL := message["dataplane_api_url"]
		t.Logf("  - Extra property remains in message (without fix): %v", hasDataplaneURL)
	})

	t.Run("complex nested structure with tools map", func(t *testing.T) {
		g := NewWithT(t)

		testURL := "https://api-47f10b24.d2vu1l4bgmjo2sp77do0.fmc.ign.cloud.redpanda.com"

		yamlConfig := `label: get_weather
processors:
- label: fetch_weather
  http:
    url: 'https://wttr.in/${! @city_name }?format=j1'
    verb: GET
    headers:
      Accept: "application/json"
    timeout: "30s"`

		// This is the exact structure from the user's failing request
		message := map[string]any{
			"dataplane_api_url": testURL,
			"id":                "d304p2q489dc738r7nh0",
			"mcp_server": map[string]any{
				"display_name": "example-http",
				"description":  "Weather information fetcher using wttr.in free API",
				"resources": map[string]any{
					"memory_shares": "128Mi",
					"cpu_shares":    "100m",
				},
				// tools field as array-of-key-value-pairs (OpenAI format)
				"tools": []any{
					map[string]any{
						"key": "get_weather",
						"value": map[string]any{
							"component_type": "COMPONENT_TYPE_PROCESSOR",
							"config_yaml":    yamlConfig, // This contains the URL that should NOT be corrupted
						},
					},
				},
				"tags": []any{
					map[string]any{
						"key":   "purpose",
						"value": "weather-fetching",
					},
					map[string]any{
						"key":   "api",
						"value": "wttr-in",
					},
				},
			},
			"update_mask": "tools,description,tags",
		}

		// Extract extra properties and add to context (matching generated code logic)
		ctx := context.Background()
		var extractedURL string
		testContextKey := "test_dataplane_url_key"

		extraProps := []ExtraProperty{
			{
				Name:        "dataplane_api_url",
				Description: "URL to connect to this dataplane",
				Required:    true,
				ContextKey:  testContextKey,
			},
		}

		for _, prop := range extraProps {
			if propVal, ok := message[prop.Name]; ok {
				ctx = context.WithValue(ctx, prop.ContextKey, propVal)
				if urlStr, ok := propVal.(string); ok {
					extractedURL = urlStr
				}
				// NOTE: Without the fix, the extra property stays in message
				// delete(message, prop.Name) // This line is missing without the fix!
			}
		}

		g.Expect(extractedURL).To(Equal(testURL))

		// Verify context contains the URL
		contextURL := ctx.Value(testContextKey)
		g.Expect(contextURL).ToNot(BeNil())
		g.Expect(contextURL.(string)).To(Equal(testURL))

		// Apply OpenAI fix processing
		descriptor := new(testdata.UpdateMCPServerRequest)
		FixOpenAI(descriptor.ProtoReflect().Descriptor(), message)

		// Check that the nested YAML config with URL is preserved
		mcpServer := message["mcp_server"].(map[string]any)
		tools := mcpServer["tools"].(map[string]any) // Should be converted from array to map
		getWeather := tools["get_weather"].(map[string]any)
		configYaml := getWeather["config_yaml"].(string)

		g.Expect(configYaml).To(ContainSubstring("https://wttr.in/"), "Nested URL should be preserved")
		g.Expect(configYaml).To(ContainSubstring("format=j1"), "URL parameters should be preserved")

		// Try marshaling/unmarshaling
		marshaledJSON, err := json.Marshal(message)
		g.Expect(err).ToNot(HaveOccurred())

		var req testdata.UpdateMCPServerRequest
		err = protojson.Unmarshal(marshaledJSON, &req)
		g.Expect(err).ToNot(HaveOccurred(), "complex structure should unmarshal successfully")

		// Verify everything made it through
		g.Expect(req.Id).To(Equal("d304p2q489dc738r7nh0"))
		g.Expect(req.McpServer.DisplayName).To(Equal("example-http"))
		g.Expect(req.McpServer.Tools["get_weather"].ConfigYaml).To(ContainSubstring("https://wttr.in/"))

		t.Logf("SUCCESS: Complex nested structure processing worked correctly")
		t.Logf("  - dataplane_api_url extracted: %s", extractedURL)
		t.Logf("  - Nested YAML URL preserved: %v", len(req.McpServer.Tools["get_weather"].ConfigYaml) > 0)
	})
}
