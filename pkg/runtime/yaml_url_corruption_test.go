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

	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
)

// TestYAMLURLCorruption focuses specifically on the URL corruption issue
// The user reported: Error: {"code":"UNAVAILABLE","message":"unsupported protocol scheme \"\""}
// This suggests URLs are becoming empty strings during processing
func TestYAMLURLCorruption(t *testing.T) {
	RegisterTestingT(t)

	t.Run("single-quoted URLs in YAML should not become empty", func(t *testing.T) {
		g := NewWithT(t)
		descriptor := new(testdata.WktTestMessage)

		// This is the exact YAML from the user's request with single-quoted URLs
		yamlWithSingleQuotes := `label: get_weather
processors:
- label: fetch_weather
  http:
    url: 'https://wttr.in/${! @city_name }?format=j1'
    verb: GET
    headers:
      Accept: "application/json"
    timeout: "30s"`

		// Test direct processing as google.protobuf.Value (string format)
		input := map[string]any{
			"value_field": yamlWithSingleQuotes,
		}

		fixed := make(map[string]any)
		for k, v := range input {
			fixed[k] = v
		}

		// Apply FixOpenAI
		FixOpenAI(descriptor.ProtoReflect().Descriptor(), fixed)

		// The YAML should remain intact
		resultYaml := fixed["value_field"].(string)

		g.Expect(resultYaml).ToNot(BeEmpty(), "YAML should not become empty")
		g.Expect(resultYaml).To(ContainSubstring("https://wttr.in/"), "URL should be preserved")
		g.Expect(resultYaml).To(ContainSubstring("format=j1"), "URL parameters should be preserved")

		// Log for debugging
		if resultYaml != yamlWithSingleQuotes {
			t.Logf("YAML CHANGED!")
			t.Logf("Original: %q", yamlWithSingleQuotes)
			t.Logf("Result:   %q", resultYaml)
		} else {
			t.Logf("YAML unchanged (good)")
		}
	})

	t.Run("nested object with YAML config", func(t *testing.T) {
		g := NewWithT(t)
		descriptor := new(testdata.WktTestMessage)

		yamlConfig := `processors:
- http:
    url: 'https://wttr.in/${! @city_name }?format=j1'
    verb: GET`

		// Simulate the nested structure from user's request
		toolsObject := map[string]any{
			"get_weather": map[string]any{
				"component_type": "COMPONENT_TYPE_PROCESSOR",
				"config_yaml":    yamlConfig,
			},
		}

		input := map[string]any{
			"value_field": toolsObject,
		}

		fixed := make(map[string]any)
		for k, v := range input {
			fixed[k] = v
		}

		FixOpenAI(descriptor.ProtoReflect().Descriptor(), fixed)

		// Navigate to the nested config_yaml
		valueField := fixed["value_field"].(map[string]any)
		getWeather := valueField["get_weather"].(map[string]any)
		configYaml := getWeather["config_yaml"].(string)

		g.Expect(configYaml).ToNot(BeEmpty(), "config_yaml should not be empty")
		g.Expect(configYaml).To(ContainSubstring("https://wttr.in/"), "URL should be preserved")

		t.Logf("Nested YAML preserved: %v", configYaml == yamlConfig)
	})

	t.Run("multiple JSON marshal/unmarshal cycles", func(t *testing.T) {
		g := NewWithT(t)

		yamlWithURL := `url: 'https://wttr.in/${! @city_name }?format=j1'`

		// Simulate multiple JSON processing cycles that might happen
		data := map[string]any{
			"config_yaml": yamlWithURL,
		}

		// Cycle 1: Marshal -> Unmarshal
		bytes1, err := json.Marshal(data)
		g.Expect(err).ToNot(HaveOccurred())

		var data1 map[string]any
		err = json.Unmarshal(bytes1, &data1)
		g.Expect(err).ToNot(HaveOccurred())

		// Cycle 2: Marshal -> Unmarshal again
		bytes2, err := json.Marshal(data1)
		g.Expect(err).ToNot(HaveOccurred())

		var data2 map[string]any
		err = json.Unmarshal(bytes2, &data2)
		g.Expect(err).ToNot(HaveOccurred())

		// Check if URL survived
		finalYaml := data2["config_yaml"].(string)
		g.Expect(finalYaml).To(Equal(yamlWithURL), "YAML should survive multiple JSON cycles")
		g.Expect(finalYaml).To(ContainSubstring("https://wttr.in/"))
	})

	t.Run("test with escaped quotes in YAML", func(t *testing.T) {
		g := NewWithT(t)

		// What if the YAML contains escaped quotes when it gets to us?
		yamlWithEscapedQuotes := `url: 'https://wttr.in/${! @city_name }?format=j1'`
		yamlAsJSONString := `"url: 'https://wttr.in/${! @city_name }?format=j1'"`

		// Try parsing as JSON string
		var parsedYaml string
		err := json.Unmarshal([]byte(yamlAsJSONString), &parsedYaml)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(parsedYaml).To(Equal(yamlWithEscapedQuotes))
		g.Expect(parsedYaml).To(ContainSubstring("https://wttr.in/"))

		t.Logf("JSON string parsing preserved URL: %v", parsedYaml == yamlWithEscapedQuotes)
	})
}
