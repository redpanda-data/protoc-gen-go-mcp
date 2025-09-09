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

	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
)

// TestFixOpenAI_URLCorruption reproduces the exact issue reported by the user
// where URLs in YAML config get corrupted during OpenAI compatibility mode processing
func TestFixOpenAI_URLCorruption(t *testing.T) {
	RegisterTestingT(t)

	t.Run("reproduces URL corruption in MCP server tools", func(t *testing.T) {
		g := NewWithT(t)

		// Use CreateItemRequest which has a map field similar to the MCP server tools
		descriptor := new(testdata.CreateItemRequest)

		yamlWithURL := `label: get_weather
processors:
- label: prepare_parameters
  mutation: |
    meta city_name = this.city_name | "London"
- label: fetch_weather
  http:
    url: 'https://wttr.in/${! @city_name }?format=j1'
    verb: GET
    headers:
      Accept: "application/json"
      User-Agent: "redpanda-mcp-server/1.0"
    timeout: "30s"
    retries: 3
    retry_period: "5s"
- label: format_response
  mutation: |
    root = {
      "city": @city_name,
      "temperature": this.current_condition.0.temp_C.number(),
      "feels_like": this.current_condition.0.FeelsLikeC.number(),
      "humidity": this.current_condition.0.humidity.number(),
      "pressure": this.current_condition.0.pressure.number(),
      "description": this.current_condition.0.weatherDesc.0.value,
      "wind_speed": this.current_condition.0.windspeedKmph.number(),
      "metadata": {
        "source": "wttr.in",
        "fetched_at": now().ts_format("2006-01-02T15:04:05.000Z")
      }
    }

meta:
  mcp:
    enabled: true
    description: "Fetch current weather information for a specified city"
    properties:
      - name: city_name
        type: string
        description: "Name of the city to get weather information for"
        required: false`

		// Simulate the OpenAI request structure that causes the issue
		// This mimics how OpenAI sends maps as arrays of key-value pairs
		input := map[string]any{
			"name":        "example-http",
			"description": "Weather information fetcher using wttr.in free API",
			"labels": []any{
				map[string]any{
					"key":   "get_weather",
					"value": yamlWithURL, // This is where the URL gets corrupted
				},
				map[string]any{
					"key":   "component_type",
					"value": "COMPONENT_TYPE_PROCESSOR",
				},
			},
			"tags": []string{"purpose", "weather-fetching", "api", "wttr-in"},
		}

		// Make a copy
		fixed := make(map[string]any)
		for k, v := range input {
			fixed[k] = v
		}

		// This should reproduce the URL corruption issue
		FixOpenAI(descriptor.ProtoReflect().Descriptor(), fixed)

		// Verify the maps were converted correctly
		g.Expect(fixed["labels"]).To(BeAssignableToTypeOf(map[string]any{}))
		g.Expect(fixed["tags"]).To(BeAssignableToTypeOf([]string{}))

		// The critical test: verify the YAML with URL remains intact
		labels := fixed["labels"].(map[string]any)
		configYaml := labels["get_weather"].(string)

		// These should NOT be empty strings (which indicates URL corruption)
		g.Expect(configYaml).ToNot(BeEmpty(), "config_yaml should not be empty")
		g.Expect(configYaml).To(ContainSubstring("https://wttr.in/"), "URL should be preserved")
		g.Expect(configYaml).To(ContainSubstring("format=j1"), "URL parameters should be preserved")
		g.Expect(configYaml).To(ContainSubstring("city_name"), "Template variables should be preserved")

		// Verify tags remain as array (since they're repeated string, not map)
		tags := fixed["tags"].([]string)
		g.Expect(tags).To(ContainElements("purpose", "weather-fetching", "api", "wttr-in"))
	})

	t.Run("handles malformed map arrays gracefully", func(t *testing.T) {
		g := NewWithT(t)
		descriptor := new(testdata.CreateItemRequest)

		// Test with malformed map array (missing key or value)
		input := map[string]any{
			"labels": []any{
				map[string]any{
					"key":   "valid-key",
					"value": "valid-value",
				},
				map[string]any{
					"key": "missing-value-key",
					// missing "value" field
				},
				map[string]any{
					// missing "key" field
					"value": "missing-key-value",
				},
				"not-a-map", // completely wrong type
			},
		}

		fixed := make(map[string]any)
		for k, v := range input {
			fixed[k] = v
		}

		// Should not panic
		FixOpenAI(descriptor.ProtoReflect().Descriptor(), fixed)

		// Should still create a map with only valid entries
		labels := fixed["labels"].(map[string]any)
		g.Expect(labels["valid-key"]).To(Equal("valid-value"))
		g.Expect(labels).ToNot(HaveKey("missing-value-key"))
		g.Expect(labels).ToNot(HaveKey("missing-key-value"))
	})

	t.Run("simulates exact user request with google.protobuf.Value", func(t *testing.T) {
		g := NewWithT(t)

		// Use WktTestMessage which has a google.protobuf.Value field
		descriptor := new(testdata.WktTestMessage)

		yamlConfig := `label: get_weather
processors:
- label: prepare_parameters
  mutation: |
    meta city_name = this.city_name | "London"
- label: fetch_weather
  http:
    url: 'https://wttr.in/${! @city_name }?format=j1'
    verb: GET
    headers:
      Accept: "application/json"
      User-Agent: "redpanda-mcp-server/1.0"
    timeout: "30s"
    retries: 3
    retry_period: "5s"`

		// This simulates the exact structure from the user's request
		// The tools field contains a nested object with config_yaml
		toolsObject := map[string]any{
			"get_weather": map[string]any{
				"component_type": "COMPONENT_TYPE_PROCESSOR",
				"config_yaml":    yamlConfig,
			},
		}

		input := map[string]any{
			// The value_field represents the "tools" field which is google.protobuf.Value
			"value_field": toolsObject,
		}

		// Make a copy
		fixed := make(map[string]any)
		for k, v := range input {
			fixed[k] = v
		}

		// Apply OpenAI fix
		FixOpenAI(descriptor.ProtoReflect().Descriptor(), fixed)

		// Verify the structure remains intact
		valueField := fixed["value_field"].(map[string]any)
		getWeather := valueField["get_weather"].(map[string]any)
		configYaml := getWeather["config_yaml"].(string)

		// The URL should NOT be corrupted
		g.Expect(configYaml).To(ContainSubstring("https://wttr.in/"))
		g.Expect(configYaml).To(ContainSubstring("format=j1"))
		g.Expect(configYaml).To(ContainSubstring("city_name"))

		// Print for debugging
		t.Logf("Original YAML length: %d", len(yamlConfig))
		t.Logf("Fixed YAML length: %d", len(configYaml))
		t.Logf("URLs match: %v", configYaml == yamlConfig)
		if configYaml != yamlConfig {
			t.Logf("DIFFERENCE FOUND!")
			t.Logf("Original: %q", yamlConfig)
			t.Logf("Fixed: %q", configYaml)
		}
	})

	t.Run("test with JSON-encoded protobuf.Value field", func(t *testing.T) {
		g := NewWithT(t)
		descriptor := new(testdata.WktTestMessage)

		yamlConfig := `processors:
- http:
    url: 'https://wttr.in/${! @city_name }?format=j1'`

		// Test what happens when OpenAI sends the protobuf.Value as a JSON string
		// (which is how it should be sent according to OpenAI schema generation)
		toolsJSON := `{"get_weather":{"component_type":"COMPONENT_TYPE_PROCESSOR","config_yaml":"` + yamlConfig + `"}}`

		input := map[string]any{
			"value_field": toolsJSON, // JSON string representation
		}

		fixed := make(map[string]any)
		for k, v := range input {
			fixed[k] = v
		}

		FixOpenAI(descriptor.ProtoReflect().Descriptor(), fixed)

		// If it was a JSON string, it should be parsed back to an object
		if parsedValue, ok := fixed["value_field"].(map[string]any); ok {
			getWeather := parsedValue["get_weather"].(map[string]any)
			configYaml := getWeather["config_yaml"].(string)
			g.Expect(configYaml).To(ContainSubstring("https://wttr.in/"))
			t.Logf("JSON string was parsed correctly")
		} else {
			// If parsing failed, it should remain as string
			g.Expect(fixed["value_field"]).To(Equal(toolsJSON))
			t.Logf("JSON string remained unchanged (parsing failed)")
		}
	})
}
