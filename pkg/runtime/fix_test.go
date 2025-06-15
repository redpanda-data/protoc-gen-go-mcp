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

	examplev1 "github.com/redpanda-data/protoc-gen-go-mcp/examples/openai-compat/gen/go/proto/example/v1"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go"
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
		// Basic map transformation test
		{
			name:       "basic map transformation",
			descriptor: new(examplev1.CreateExampleRequest),
			input: map[string]any{
				"nested": map[string]any{
					"labels": []any{
						map[string]any{
							"key":   "my-key",
							"value": "my-value",
						},
					},
				},
			},
			expected: map[string]any{
				"nested": map[string]any{
					"labels": map[string]any{
						"my-key": "my-value",
					},
				},
			},
		},
		// Well-known types tests
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
			name:       "struct: non-string value remains unchanged",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"struct_field": map[string]any{"already": "object"},
			},
			expected: map[string]any{
				"struct_field": map[string]any{"already": "object"},
			},
		},
		{
			name:       "struct: empty string remains unchanged",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"struct_field": "",
			},
			expected: map[string]any{
				"struct_field": "",
			},
			expectUnmarshalError: true,
		},
		{
			name:       "struct: empty JSON object string converts correctly",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"struct_field": `{}`,
			},
			expected: map[string]any{
				"struct_field": map[string]any{},
			},
		},
		{
			name:       "struct: string with array JSON remains unchanged",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"struct_field": `[1, 2, 3]`,
			},
			expected: map[string]any{
				"struct_field": `[1, 2, 3]`,
			},
			expectUnmarshalError: true,
		},
		{
			name:       "value: valid JSON string converts to value",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"value_field": `{"foo": "bar"}`,
			},
			expected: map[string]any{
				"value_field": map[string]any{
					"foo": "bar",
				},
			},
		},
		{
			name:       "value: JSON number string converts to value",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"value_field": `42`,
			},
			expected: map[string]any{
				"value_field": float64(42),
			},
		},
		{
			name:       "value: JSON boolean string converts to value",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"value_field": `true`,
			},
			expected: map[string]any{
				"value_field": true,
			},
		},
		{
			name:       "value: JSON null string converts to value",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"value_field": `null`,
			},
			expected: map[string]any{
				"value_field": nil,
			},
		},
		{
			name:       "value: JSON array string converts to value",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"value_field": `[1, "two", true]`,
			},
			expected: map[string]any{
				"value_field": []any{float64(1), "two", true},
			},
		},
		{
			name:       "value: invalid JSON string remains unchanged",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"value_field": `{invalid json}`,
			},
			expected: map[string]any{
				"value_field": `{invalid json}`,
			},
			expectUnmarshalError: false,
		},
		{
			name:       "value: non-string value remains unchanged",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"value_field": 42,
			},
			expected: map[string]any{
				"value_field": 42,
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
			expectUnmarshalError: false,
		},
		{
			name:       "list: valid JSON array string converts to list",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"list_value": `[1, "two", true, null]`,
			},
			expected: map[string]any{
				"list_value": []any{float64(1), "two", true, nil},
			},
		},
		{
			name:       "list: empty JSON array string converts to empty list",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"list_value": `[]`,
			},
			expected: map[string]any{
				"list_value": []any{},
			},
		},
		{
			name:       "list: nested array string converts correctly",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"list_value": `[[1, 2], {"foo": "bar"}]`,
			},
			expected: map[string]any{
				"list_value": []any{
					[]any{float64(1), float64(2)},
					map[string]any{"foo": "bar"},
				},
			},
		},
		{
			name:       "list: invalid JSON string remains unchanged",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"list_value": `[invalid json}`,
			},
			expected: map[string]any{
				"list_value": `[invalid json}`,
			},
			expectUnmarshalError: true,
		},
		{
			name:       "list: JSON object string remains unchanged",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"list_value": `{"not": "array"}`,
			},
			expected: map[string]any{
				"list_value": `{"not": "array"}`,
			},
			expectUnmarshalError: true,
		},
		{
			name:       "list: non-string value remains unchanged",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"list_value": []any{"already", "array"},
			},
			expected: map[string]any{
				"list_value": []any{"already", "array"},
			},
		},
		{
			name:       "list: empty string remains unchanged",
			descriptor: new(testdata.WktTestMessage),
			input: map[string]any{
				"list_value": "",
			},
			expected: map[string]any{
				"list_value": "",
			},
			expectUnmarshalError: true,
		},
		// Map edge cases
		{
			name:       "empty map array converts to empty object",
			descriptor: new(examplev1.CreateExampleRequest),
			input: map[string]any{
				"nested": map[string]any{
					"labels": []any{},
				},
			},
			expected: map[string]any{
				"nested": map[string]any{
					"labels": map[string]any{},
				},
			},
		},
		{
			name:       "malformed map entry without key",
			descriptor: new(examplev1.CreateExampleRequest),
			input: map[string]any{
				"nested": map[string]any{
					"labels": []any{
						map[string]any{
							"value": "orphaned-value",
						},
					},
				},
			},
			expected: map[string]any{
				"nested": map[string]any{
					"labels": map[string]any{},
				},
			},
		},
		{
			name:       "malformed map entry without value",
			descriptor: new(examplev1.CreateExampleRequest),
			input: map[string]any{
				"nested": map[string]any{
					"labels": []any{
						map[string]any{
							"key": "orphaned-key",
						},
					},
				},
			},
			expected: map[string]any{
				"nested": map[string]any{
					"labels": map[string]any{},
				},
			},
		},
		{
			name:       "non-string key ignored",
			descriptor: new(examplev1.CreateExampleRequest),
			input: map[string]any{
				"nested": map[string]any{
					"labels": []any{
						map[string]any{
							"key":   42,
							"value": "number-key",
						},
					},
				},
			},
			expected: map[string]any{
				"nested": map[string]any{
					"labels": map[string]any{},
				},
			},
		},
		{
			name:       "non-map entry in array ignored",
			descriptor: new(examplev1.CreateExampleRequest),
			input: map[string]any{
				"nested": map[string]any{
					"labels": []any{
						"not-a-map",
						map[string]any{
							"key":   "valid-key",
							"value": "valid-value",
						},
					},
				},
			},
			expected: map[string]any{
				"nested": map[string]any{
					"labels": map[string]any{
						"valid-key": "valid-value",
					},
				},
			},
		},
		{
			name:       "nil value in map preserved",
			descriptor: new(examplev1.CreateExampleRequest),
			input: map[string]any{
				"nested": map[string]any{
					"labels": []any{
						map[string]any{
							"key":   "null-key",
							"value": nil,
						},
					},
				},
			},
			expected: map[string]any{
				"nested": map[string]any{
					"labels": map[string]any{
						"null-key": nil,
					},
				},
			},
			expectUnmarshalError: true,
		},
		{
			name:       "duplicate keys overwrite",
			descriptor: new(examplev1.CreateExampleRequest),
			input: map[string]any{
				"nested": map[string]any{
					"labels": []any{
						map[string]any{
							"key":   "duplicate",
							"value": "first",
						},
						map[string]any{
							"key":   "duplicate",
							"value": "second",
						},
					},
				},
			},
			expected: map[string]any{
				"nested": map[string]any{
					"labels": map[string]any{
						"duplicate": "second",
					},
				},
			},
		},
		{
			name:       "non-array map field remains unchanged",
			descriptor: new(examplev1.CreateExampleRequest),
			input: map[string]any{
				"nested": map[string]any{
					"labels": map[string]any{
						"already": "object",
					},
				},
			},
			expected: map[string]any{
				"nested": map[string]any{
					"labels": map[string]any{
						"already": "object",
					},
				},
			},
		},
		// Realistic JSON payload tests - simulating actual OpenAI requests
		{
			name:       "realistic: OpenAI sends invalid JSON in struct field",
			descriptor: new(testdata.WktTestMessage),
			input: func() map[string]any {
				// Simulate unmarshaling a real OpenAI JSON payload
				jsonPayload := `{"struct_field": "{invalid json}"}`
				var args map[string]any
				err := json.Unmarshal([]byte(jsonPayload), &args)
				Expect(err).ToNot(HaveOccurred())
				return args
			}(),
			expected: map[string]any{
				"struct_field": "{invalid json}", // Should remain unchanged - not FixOpenAI's job to fix broken input
			},
			expectUnmarshalError: true, // protojson.Unmarshal should fail with invalid JSON - this is correct behavior
		},
		{
			name:       "realistic: OpenAI sends valid JSON in struct field",
			descriptor: new(testdata.WktTestMessage),
			input: func() map[string]any {
				jsonPayload := `{"struct_field": "{\"foo\": \"bar\", \"num\": 42}"}`
				var args map[string]any
				err := json.Unmarshal([]byte(jsonPayload), &args)
				Expect(err).ToNot(HaveOccurred())
				return args
			}(),
			expected: map[string]any{
				"struct_field": map[string]any{
					"foo": "bar",
					"num": float64(42),
				},
			},
			expectUnmarshalError: false,
		},
		{
			name:       "realistic: OpenAI sends empty string in value field",
			descriptor: new(testdata.WktTestMessage),
			input: func() map[string]any {
				jsonPayload := `{"value_field": ""}`
				var args map[string]any
				err := json.Unmarshal([]byte(jsonPayload), &args)
				Expect(err).ToNot(HaveOccurred())
				return args
			}(),
			expected: map[string]any{
				"value_field": "", // Should remain unchanged
			},
			expectUnmarshalError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			FixOpenAI(tt.descriptor.ProtoReflect().Descriptor(), tt.input)
			g.Expect(tt.input).To(Equal(tt.expected))

			// Test protojson unmarshaling with the fixed data
			jzon, err := json.Marshal(tt.input)
			g.Expect(err).ToNot(HaveOccurred())

			testMsg := tt.descriptor.ProtoReflect().New().Interface()
			err = protojson.Unmarshal(jzon, testMsg)
			if tt.expectUnmarshalError {
				g.Expect(err).To(HaveOccurred(), "protojson.Unmarshal should fail for invalid input")
			} else {
				g.Expect(err).ToNot(HaveOccurred(), "protojson.Unmarshal should succeed after FixOpenAI processing")
			}
		})
	}
}
