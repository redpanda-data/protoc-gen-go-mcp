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
	"google.golang.org/protobuf/proto"

	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
)

func TestFixOpenAI(t *testing.T) {
	RegisterTestingT(t)
	tests := []struct {
		name                 string
		descriptor           proto.Message
		input                map[string]any
		expected             map[string]any
		expectUnmarshalError bool
	}{
		// Basic map transformation test - labels field in CreateItemRequest
		{
			name:       "basic map transformation",
			descriptor: new(testdata.CreateItemRequest),
			input: map[string]any{
				"labels": []any{
					map[string]any{
						"key":   "my-key",
						"value": "my-value",
					},
				},
			},
			expected: map[string]any{
				"labels": map[string]any{
					"my-key": "my-value",
				},
			},
		},
		// Empty map test
		{
			name:       "empty map array converts to empty object",
			descriptor: new(testdata.CreateItemRequest),
			input: map[string]any{
				"labels": []any{},
			},
			expected: map[string]any{
				"labels": map[string]any{},
			},
		},
		// Well-known types tests using WktTestMessage
		{
			name:       "struct: valid JSON string converts to struct",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"struct_field": `{"foo": "bar", "num": 42}`,
			},
			expected: map[string]any{
				"struct_field": map[string]any{
					"foo": "bar",
					"num": float64(42),
				},
			},
		},
		{
			name:       "struct: invalid JSON string remains unchanged",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"struct_field": `{invalid json}`,
			},
			expected: map[string]any{
				"struct_field": `{invalid json}`,
			},
			expectUnmarshalError: true,
		},
		{
			name:       "value: valid JSON string converts to value",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"value_field": `"hello world"`,
			},
			expected: map[string]any{
				"value_field": "hello world",
			},
		},
		{
			name:       "value: empty string remains unchanged",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"value_field": "",
			},
			expected: map[string]any{
				"value_field": "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Make a copy of input to avoid modifying the test data
			fixed := make(map[string]any)
			for k, v := range tt.input {
				fixed[k] = v
			}

			FixOpenAI(tt.descriptor.ProtoReflect().Descriptor(), fixed)

			if !tt.expectUnmarshalError {
				// Test that the fixed data can be unmarshaled into the proto message
				fixedJSON, err := json.Marshal(fixed)
				g.Expect(err).ToNot(HaveOccurred())

				err = protojson.Unmarshal(fixedJSON, tt.descriptor)
				g.Expect(err).ToNot(HaveOccurred(), "protojson.Unmarshal should succeed after FixOpenAI processing")
			}

			g.Expect(fixed).To(Equal(tt.expected))
		})
	}
}

func TestFixOpenAINonMapField(t *testing.T) {
	RegisterTestingT(t)
	g := NewWithT(t)

	descriptor := new(testdata.CreateItemRequest)

	// Test case for non-array map field remains unchanged
	input := map[string]any{
		"labels": map[string]any{"already": "object"},
	}
	expected := map[string]any{
		"labels": map[string]any{"already": "object"},
	}

	// Make a copy of input
	fixed := make(map[string]any)
	for k, v := range input {
		fixed[k] = v
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), fixed)
	g.Expect(fixed).To(Equal(expected))

	// Should be able to unmarshal successfully
	fixedJSON, err := json.Marshal(fixed)
	g.Expect(err).ToNot(HaveOccurred())

	err = protojson.Unmarshal(fixedJSON, descriptor)
	g.Expect(err).ToNot(HaveOccurred())
}
