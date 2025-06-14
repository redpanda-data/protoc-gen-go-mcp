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

	examplev1 "github.com/redpanda-data/protoc-gen-go-mcp/examples/openai-compat/gen/go/proto/example/v1"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/generator"
)

func TestFix(t *testing.T) {
	g := NewWithT(t)
	in := `{
  "nested": {
    "labels": [
      {
        "key": "my-key",
        "value": "my-value"
      }
    ]
  }
}`
	var inMap map[string]any
	err := json.Unmarshal([]byte(in), &inMap)
	g.Expect(err).ToNot(HaveOccurred())

	FixOpenAI(new(examplev1.CreateExampleRequest).ProtoReflect().Descriptor(), inMap)

	jzon, err := json.Marshal(inMap)
	g.Expect(err).ToNot(HaveOccurred())
	expected := `{"nested":{"labels":{"my-key":"my-value"}}}`
	g.Expect(jzon).To(MatchJSON([]byte(expected)))
}

func TestFixWellKnownTypes(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		input     any
		expected  any
	}{
		// Struct field tests
		{
			name:      "struct: valid JSON string converts to struct",
			fieldName: "struct_field",
			input:     `{"foo": "bar", "num": 42}`,
			expected: map[string]any{
				"foo": "bar",
				"num": float64(42),
			},
		},
		{
			name:      "struct: invalid JSON string remains unchanged",
			fieldName: "struct_field",
			input:     `{invalid json}`,
			expected:  `{invalid json}`,
		},
		{
			name:      "struct: non-string value remains unchanged",
			fieldName: "struct_field",
			input:     map[string]any{"already": "object"},
			expected:  map[string]any{"already": "object"},
		},
		{
			name:      "struct: empty string remains unchanged",
			fieldName: "struct_field",
			input:     "",
			expected:  "",
		},
		{
			name:      "struct: empty JSON object string converts correctly",
			fieldName: "struct_field",
			input:     `{}`,
			expected:  map[string]any{},
		},
		{
			name:      "struct: string with array JSON remains unchanged",
			fieldName: "struct_field",
			input:     `[1, 2, 3]`,
			expected:  `[1, 2, 3]`,
		},
		// Value field tests
		{
			name:      "value: valid JSON string converts to value",
			fieldName: "value_field",
			input:     `{"foo": "bar"}`,
			expected: map[string]any{
				"foo": "bar",
			},
		},
		{
			name:      "value: JSON number string converts to value",
			fieldName: "value_field",
			input:     `42`,
			expected:  float64(42),
		},
		{
			name:      "value: JSON boolean string converts to value",
			fieldName: "value_field",
			input:     `true`,
			expected:  true,
		},
		{
			name:      "value: JSON null string converts to value",
			fieldName: "value_field",
			input:     `null`,
			expected:  nil,
		},
		{
			name:      "value: JSON array string converts to value",
			fieldName: "value_field",
			input:     `[1, "two", true]`,
			expected:  []any{float64(1), "two", true},
		},
		{
			name:      "value: invalid JSON string remains unchanged",
			fieldName: "value_field",
			input:     `{invalid json}`,
			expected:  `{invalid json}`,
		},
		{
			name:      "value: non-string value remains unchanged",
			fieldName: "value_field",
			input:     42,
			expected:  42,
		},
		{
			name:      "value: empty string remains unchanged",
			fieldName: "value_field",
			input:     "",
			expected:  "",
		},
		// ListValue field tests
		{
			name:      "list: valid JSON array string converts to list",
			fieldName: "list_value",
			input:     `[1, "two", true, null]`,
			expected:  []any{float64(1), "two", true, nil},
		},
		{
			name:      "list: empty JSON array string converts to empty list",
			fieldName: "list_value",
			input:     `[]`,
			expected:  []any{},
		},
		{
			name:      "list: nested array string converts correctly",
			fieldName: "list_value",
			input:     `[[1, 2], {"foo": "bar"}]`,
			expected: []any{
				[]any{float64(1), float64(2)},
				map[string]any{"foo": "bar"},
			},
		},
		{
			name:      "list: invalid JSON string remains unchanged",
			fieldName: "list_value",
			input:     `[invalid json}`,
			expected:  `[invalid json}`,
		},
		{
			name:      "list: JSON object string remains unchanged",
			fieldName: "list_value",
			input:     `{"not": "array"}`,
			expected:  `{"not": "array"}`,
		},
		{
			name:      "list: non-string value remains unchanged",
			fieldName: "list_value",
			input:     []any{"already", "array"},
			expected:  []any{"already", "array"},
		},
		{
			name:      "list: empty string remains unchanged",
			fieldName: "list_value",
			input:     "",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			input := map[string]any{
				tt.fieldName: tt.input,
			}

			FixOpenAI(new(generator.WktTestMessage).ProtoReflect().Descriptor(), input)

			expected := map[string]any{
				tt.fieldName: tt.expected,
			}
			g.Expect(input).To(Equal(expected))
		})
	}
}

func TestFixMapEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected map[string]any
	}{
		{
			name: "empty map array converts to empty object",
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
			name: "malformed map entry without key",
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
			name: "malformed map entry without value",
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
			name: "non-string key ignored",
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
			name: "non-map entry in array ignored",
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
			name: "nil value in map preserved",
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
		},
		{
			name: "duplicate keys overwrite",
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
			name: "non-array map field remains unchanged",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			FixOpenAI(new(examplev1.CreateExampleRequest).ProtoReflect().Descriptor(), tt.input)

			g.Expect(tt.input).To(Equal(tt.expected))
		})
	}
}
