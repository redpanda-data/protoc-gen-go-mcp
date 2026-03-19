package runtime

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestFixOpenAI_RepeatedMessages(t *testing.T) {
	g := NewWithT(t)

	// RepeatedMessagesRequest has repeated ItemWithMap which has maps and WKTs inside
	descriptor := new(testdata.RepeatedMessagesRequest)

	input := map[string]any{
		"items": []any{
			map[string]any{
				"name": "item1",
				"labels": []any{
					map[string]any{"key": "env", "value": "prod"},
					map[string]any{"key": "team", "value": "backend"},
				},
				"config": `"hello"`,
				"extra":  `{"nested": true}`,
			},
			map[string]any{
				"name": "item2",
				"labels": []any{
					map[string]any{"key": "a", "value": "1"},
				},
			},
		},
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)

	// Verify maps were converted
	items := input["items"].([]any)
	item1 := items[0].(map[string]any)
	g.Expect(item1["labels"]).To(Equal(map[string]any{"env": "prod", "team": "backend"}))
	// Verify WKT Value was converted
	g.Expect(item1["config"]).To(Equal("hello"))
	// Verify WKT Struct was converted
	g.Expect(item1["extra"]).To(Equal(map[string]any{"nested": true}))

	item2 := items[1].(map[string]any)
	g.Expect(item2["labels"]).To(Equal(map[string]any{"a": "1"}))

	// Verify full round-trip: fixed data should unmarshal into proto
	fixedJSON, err := json.Marshal(input)
	g.Expect(err).ToNot(HaveOccurred())

	var result testdata.RepeatedMessagesRequest
	err = protojson.Unmarshal(fixedJSON, &result)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(result.Items).To(HaveLen(2))
	g.Expect(result.Items[0].Name).To(Equal("item1"))
	g.Expect(result.Items[0].Labels).To(Equal(map[string]string{"env": "prod", "team": "backend"}))
}

func TestFixOpenAI_MapWithMessageValues(t *testing.T) {
	g := NewWithT(t)

	// DeepNestingRequest -> MiddleMessage -> named_items is map<string, InnerMessage>
	// InnerMessage has tags (map<string,string>) and metadata (Struct) and dynamic_config (Value)
	descriptor := new(testdata.DeepNestingRequest)

	input := map[string]any{
		"middle": map[string]any{
			"inner": map[string]any{
				"id": "inner-1",
				"tags": []any{
					map[string]any{"key": "env", "value": "prod"},
				},
				"metadata":       `{"key": "value"}`,
				"dynamic_config": `42`,
			},
			"items": []any{
				map[string]any{
					"id": "inner-2",
					"tags": []any{
						map[string]any{"key": "k", "value": "v"},
					},
				},
			},
			"named_items": []any{
				map[string]any{
					"key": "first",
					"value": map[string]any{
						"id": "inner-3",
						"tags": []any{
							map[string]any{"key": "x", "value": "y"},
						},
						"metadata": `{"deep": "nest"}`,
					},
				},
			},
		},
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)

	middle := input["middle"].(map[string]any)

	// Verify inner message
	inner := middle["inner"].(map[string]any)
	g.Expect(inner["tags"]).To(Equal(map[string]any{"env": "prod"}))
	g.Expect(inner["metadata"]).To(Equal(map[string]any{"key": "value"}))
	g.Expect(inner["dynamic_config"]).To(BeNumerically("==", 42))

	// Verify repeated inner messages
	items := middle["items"].([]any)
	item := items[0].(map[string]any)
	g.Expect(item["tags"]).To(Equal(map[string]any{"k": "v"}))

	// Verify map with message values was converted
	namedItems := middle["named_items"].(map[string]any)
	first := namedItems["first"].(map[string]any)
	g.Expect(first["tags"]).To(Equal(map[string]any{"x": "y"}))
	g.Expect(first["metadata"]).To(Equal(map[string]any{"deep": "nest"}))
}

func TestFixOpenAI_ListValueConversion(t *testing.T) {
	g := NewWithT(t)
	descriptor := new(testdata.WktTestMessage)

	input := map[string]any{
		"list_value": `[1, "two", true, null]`,
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)

	g.Expect(input["list_value"]).To(Equal([]any{float64(1), "two", true, nil}))
}

func TestFixOpenAI_EmptyInput(t *testing.T) {
	g := NewWithT(t)
	descriptor := new(testdata.CreateItemRequest)

	input := map[string]any{}
	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)
	g.Expect(input).To(BeEmpty())
}

func TestFixOpenAI_NilValues(t *testing.T) {
	g := NewWithT(t)
	descriptor := new(testdata.WktTestMessage)

	input := map[string]any{
		"struct_field": nil,
		"value_field":  nil,
		"list_value":   nil,
	}

	// Should not panic
	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)
	g.Expect(input["struct_field"]).To(BeNil())
}

func TestFixOpenAI_MapKeyValueMissingFields(t *testing.T) {
	g := NewWithT(t)
	descriptor := new(testdata.CreateItemRequest)

	// Malformed key-value pairs - missing value or key
	input := map[string]any{
		"labels": []any{
			map[string]any{"key": "only-key"},              // missing value
			map[string]any{"value": "only-value"},          // missing key
			map[string]any{"key": "good", "value": "pair"}, // valid
			"not-a-map", // not even a map
			map[string]any{"key": 42, "value": "non-string-key"}, // non-string key
		},
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)

	// Valid pair + non-string key (now coerced to "42") should survive
	labels := input["labels"].(map[string]any)
	g.Expect(labels).To(HaveLen(2))
	g.Expect(labels["good"]).To(Equal("pair"))
	g.Expect(labels["42"]).To(Equal("non-string-key"))
}

func TestFixOpenAI_CamelCaseFieldNames(t *testing.T) {
	g := NewWithT(t)

	// WktTestMessage has field "struct_field" with JSON name "structField",
	// "value_field" -> "valueField", "list_value" -> "listValue"
	descriptor := new(testdata.WktTestMessage)

	// Use camelCase JSON names (as some LLMs might return)
	input := map[string]any{
		"structField": `{"foo": "bar"}`,
		"valueField":  `42`,
		"listValue":   `[1, 2, 3]`,
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)

	// Should have converted the string representations using camelCase keys
	g.Expect(input["structField"]).To(Equal(map[string]any{"foo": "bar"}))
	g.Expect(input["valueField"]).To(BeNumerically("==", 42))
	g.Expect(input["listValue"]).To(Equal([]any{float64(1), float64(2), float64(3)}))
}

func TestFixOpenAI_MixedCaseFieldNames(t *testing.T) {
	g := NewWithT(t)

	// Mix of proto names and JSON names in same object
	descriptor := new(testdata.WktTestMessage)

	input := map[string]any{
		"struct_field": `{"a": 1}`, // proto name
		"valueField":   `"hello"`,  // JSON name
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)

	g.Expect(input["struct_field"]).To(Equal(map[string]any{"a": float64(1)}))
	g.Expect(input["valueField"]).To(Equal("hello"))
}
