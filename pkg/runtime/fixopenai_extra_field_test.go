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
	"testing"

	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"

	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
)

// TestFixOpenAI_ExtraField tests if FixOpenAI corrupts extra fields
// This reproduces the user's specific issue where dataplane_api_url becomes empty
func TestFixOpenAI_ExtraField(t *testing.T) {
	RegisterTestingT(t)

	t.Run("extra field should survive FixOpenAI processing", func(t *testing.T) {
		g := NewWithT(t)

		descriptor := new(testdata.UpdateMCPServerRequest)
		testURL := "https://api-47f10b24.d2vu1l4bgmjo2sp77do0.fmc.ign.cloud.redpanda.com"

		message := map[string]any{
			"dataplane_api_url": testURL,                // Extra field - this is the one that gets lost!
			"id":                "d304p2q489dc738r7nh0", // Path parameter
			"mcp_server": map[string]any{
				"display_name": "example-http",
				"description":  "Simple test",
			},
			"update_mask": "description",
		}

		// Store original values for comparison
		originalURL := message["dataplane_api_url"].(string)
		originalID := message["id"].(string)

		t.Logf("BEFORE FixOpenAI:")
		t.Logf("  dataplane_api_url: %q", originalURL)
		t.Logf("  id: %q", originalID)

		// Call FixOpenAI - this might corrupt the extra field
		FixOpenAI(descriptor.ProtoReflect().Descriptor(), message)

		// Check what happened to our fields
		afterURL := message["dataplane_api_url"].(string)
		afterID := message["id"].(string)

		t.Logf("AFTER FixOpenAI:")
		t.Logf("  dataplane_api_url: %q", afterURL)
		t.Logf("  id: %q", afterID)

		// The critical test: did the extra field get corrupted?
		g.Expect(afterURL).To(Equal(originalURL), "dataplane_api_url should NOT be corrupted by FixOpenAI")
		g.Expect(afterID).To(Equal(originalID), "id should remain unchanged")
		g.Expect(afterURL).ToNot(BeEmpty(), "dataplane_api_url should not become empty")
		g.Expect(afterURL).To(ContainSubstring("https://"), "URL should still be a valid URL")

		if afterURL != originalURL {
			t.Logf("🔴 BUG REPRODUCED: dataplane_api_url was corrupted!")
			t.Logf("   Original: %q", originalURL)
			t.Logf("   After:    %q", afterURL)
		} else {
			t.Logf("✅ dataplane_api_url survived FixOpenAI unchanged")
		}
	})

	t.Run("extra field with different descriptors", func(t *testing.T) {
		testURL := "https://api-47f10b24.d2vu1l4bgmjo2sp77do0.fmc.ign.cloud.redpanda.com"

		testCases := []struct {
			name       string
			descriptor proto.Message
			message    map[string]any
		}{
			{
				name:       "GetMCPServerRequest (fails in real usage)",
				descriptor: &testdata.GetMCPServerRequest{},
				message: map[string]any{
					"dataplane_api_url": testURL,
					"id":                "d304p2q489dc738r7nh0",
				},
			},
			{
				name:       "ListMCPServersRequest (works in real usage)",
				descriptor: &testdata.ListMCPServersRequest{},
				message: map[string]any{
					"dataplane_api_url": testURL,
					"page_size":         10,
					"page_token":        "",
				},
			},
			{
				name:       "UpdateMCPServerRequest (fails in real usage)",
				descriptor: &testdata.UpdateMCPServerRequest{},
				message: map[string]any{
					"dataplane_api_url": testURL,
					"id":                "d304p2q489dc738r7nh0",
					"mcp_server": map[string]any{
						"display_name": "test",
					},
					"update_mask": "display_name",
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				g := NewWithT(t)

				// Make a copy to avoid modifying test data
				message := make(map[string]any)
				for k, v := range tc.message {
					message[k] = v
				}

				originalURL := message["dataplane_api_url"].(string)

				// Apply FixOpenAI
				FixOpenAI(tc.descriptor.ProtoReflect().Descriptor(), message)

				afterURL := message["dataplane_api_url"].(string)

				t.Logf("%s:", tc.name)
				t.Logf("  Original URL: %q", originalURL)
				t.Logf("  After URL:    %q", afterURL)

				if afterURL != originalURL {
					t.Logf("  🔴 URL CORRUPTED!")
				} else {
					t.Logf("  ✅ URL preserved")
				}

				g.Expect(afterURL).To(Equal(originalURL), "URL should not be corrupted")
				g.Expect(afterURL).ToNot(BeEmpty(), "URL should not become empty")
			})
		}
	})

	t.Run("extra field with complex nested structures", func(t *testing.T) {
		g := NewWithT(t)

		descriptor := new(testdata.UpdateMCPServerRequest)
		testURL := "https://api-47f10b24.d2vu1l4bgmjo2sp77do0.fmc.ign.cloud.redpanda.com"

		yamlConfig := `label: get_weather
processors:
- label: fetch_weather
  http:
    url: 'https://wttr.in/${! @city_name }?format=j1'
    verb: GET
    headers:
      Accept: "application/json"`

		// Complex message similar to the user's failing request
		message := map[string]any{
			"dataplane_api_url": testURL, // Extra field
			"id":                "d304p2q489dc738r7nh0",
			"mcp_server": map[string]any{
				"display_name": "example-http",
				"description":  "Weather fetcher",
				"tools": []any{ // Map converted to array in OpenAI mode
					map[string]any{
						"key": "get_weather",
						"value": map[string]any{
							"component_type": "COMPONENT_TYPE_PROCESSOR",
							"config_yaml":    yamlConfig, // Nested YAML with URLs
						},
					},
				},
				"tags": []any{
					map[string]any{"key": "purpose", "value": "weather"},
				},
			},
			"update_mask": "tools,description",
		}

		originalURL := message["dataplane_api_url"].(string)

		// Apply FixOpenAI with complex nested structure
		FixOpenAI(descriptor.ProtoReflect().Descriptor(), message)

		afterURL := message["dataplane_api_url"].(string)

		t.Logf("Complex structure test:")
		t.Logf("  Original URL: %q", originalURL)
		t.Logf("  After URL:    %q", afterURL)

		// Check if nested processing affected the extra field
		g.Expect(afterURL).To(Equal(originalURL), "Complex processing should not affect extra field")
		g.Expect(afterURL).ToNot(BeEmpty(), "URL should not become empty during complex processing")

		// Also verify the nested YAML URL is preserved
		mcpServer := message["mcp_server"].(map[string]any)
		tools := mcpServer["tools"].(map[string]any) // Should be converted from array
		getWeather := tools["get_weather"].(map[string]any)
		configYaml := getWeather["config_yaml"].(string)

		g.Expect(configYaml).To(ContainSubstring("https://wttr.in/"), "Nested URL should also be preserved")

		if afterURL != originalURL {
			t.Logf("🔴 COMPLEX BUG: Extra field corrupted during complex processing!")
		}
	})
}
