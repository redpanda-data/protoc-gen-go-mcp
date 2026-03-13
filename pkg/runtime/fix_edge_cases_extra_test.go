package runtime

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestFixOpenAI_MapWithMultipleEntries(t *testing.T) {
	g := NewWithT(t)

	descriptor := new(testdata.MapVariantsRequest)
	input := map[string]any{
		"string_to_string": []any{
			map[string]any{"key": "a", "value": "1"},
			map[string]any{"key": "b", "value": "2"},
			map[string]any{"key": "c", "value": "3"},
		},
		"string_to_double": []any{
			map[string]any{"key": "pi", "value": 3.14},
			map[string]any{"key": "e", "value": 2.718},
		},
		"string_to_bool": []any{
			map[string]any{"key": "enabled", "value": true},
			map[string]any{"key": "debug", "value": false},
		},
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)

	g.Expect(input["string_to_string"]).To(Equal(map[string]any{
		"a": "1", "b": "2", "c": "3",
	}))
	g.Expect(input["string_to_double"]).To(Equal(map[string]any{
		"pi": 3.14, "e": 2.718,
	}))
	g.Expect(input["string_to_bool"]).To(Equal(map[string]any{
		"enabled": true, "debug": false,
	}))

	// Verify full round-trip
	fixedJSON, err := json.Marshal(input)
	g.Expect(err).ToNot(HaveOccurred())

	var result testdata.MapVariantsRequest
	err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(fixedJSON, &result)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.StringToString).To(Equal(map[string]string{"a": "1", "b": "2", "c": "3"}))
	g.Expect(result.StringToDouble).To(HaveKeyWithValue("pi", 3.14))
	g.Expect(result.StringToBool).To(HaveKeyWithValue("enabled", true))
}

func TestFixOpenAI_MapAlreadyObject(t *testing.T) {
	g := NewWithT(t)

	// If the map is already in object form (not array), it should be left alone
	descriptor := new(testdata.MapVariantsRequest)
	input := map[string]any{
		"string_to_string": map[string]any{"already": "object"},
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)
	g.Expect(input["string_to_string"]).To(Equal(map[string]any{"already": "object"}))
}

func TestFixOpenAI_MapWithDuplicateKeys(t *testing.T) {
	g := NewWithT(t)

	descriptor := new(testdata.CreateItemRequest)
	input := map[string]any{
		"labels": []any{
			map[string]any{"key": "dup", "value": "first"},
			map[string]any{"key": "dup", "value": "second"},
			map[string]any{"key": "unique", "value": "val"},
		},
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)

	labels := input["labels"].(map[string]any)
	g.Expect(labels).To(HaveLen(2))
	g.Expect(labels["unique"]).To(Equal("val"))
	// Last write wins for duplicates
	g.Expect(labels["dup"]).To(Equal("second"))
}

func TestFixOpenAI_MapWithNullValue(t *testing.T) {
	g := NewWithT(t)

	descriptor := new(testdata.CreateItemRequest)
	input := map[string]any{
		"labels": []any{
			map[string]any{"key": "k", "value": nil},
		},
	}

	// Null values are now skipped to avoid protojson unmarshal failures
	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)
	labels := input["labels"].(map[string]any)
	g.Expect(labels).To(BeEmpty())
}

func TestFixOpenAI_MapWithMixedMalformedEntries(t *testing.T) {
	g := NewWithT(t)

	descriptor := new(testdata.CreateItemRequest)
	input := map[string]any{
		"labels": []any{
			nil,                    // nil entry
			42,                     // number entry
			"string",               // plain string
			[]any{"nested", "arr"}, // nested array
			map[string]any{"key": "good", "value": "pair"}, // valid
			map[string]any{},       // empty map
			map[string]any{"key": "no-val"}, // missing value key
		},
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)

	labels := input["labels"].(map[string]any)
	g.Expect(labels).To(HaveLen(1))
	g.Expect(labels["good"]).To(Equal("pair"))
}

func TestFixOpenAI_ValueWithDifferentJSONTypes(t *testing.T) {
	descriptor := new(testdata.WktTestMessage)

	tests := []struct {
		name     string
		input    string
		expected any
	}{
		{"number", "42.5", 42.5},
		{"boolean_true", "true", true},
		{"boolean_false", "false", false},
		{"array", `[1, "two"]`, []any{float64(1), "two"}},
		{"object", `{"k": "v"}`, map[string]any{"k": "v"}},
		{"string", `"hello"`, "hello"},
		{"negative_number", "-99", float64(-99)},
		{"zero", "0", float64(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			input := map[string]any{"value_field": tt.input}
			FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)
			g.Expect(input["value_field"]).To(Equal(tt.expected))
		})
	}

	t.Run("null", func(t *testing.T) {
		g := NewWithT(t)
		input := map[string]any{"value_field": "null"}
		FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)
		g.Expect(input["value_field"]).To(BeNil())
	})

	t.Run("invalid_json", func(t *testing.T) {
		g := NewWithT(t)
		input := map[string]any{"value_field": "not valid json {"}
		FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)
		g.Expect(input["value_field"]).To(Equal("not valid json {"))
	})
}

func TestFixOpenAI_StructOnlyAcceptsObjects(t *testing.T) {
	g := NewWithT(t)

	descriptor := new(testdata.WktTestMessage)

	// Valid object string
	input := map[string]any{"struct_field": `{"key": "val"}`}
	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)
	g.Expect(input["struct_field"]).To(Equal(map[string]any{"key": "val"}))

	// Array JSON string - NOT a valid Struct, should remain string
	// (because json.Unmarshal into map[string]any will fail for arrays)
	input = map[string]any{"struct_field": `[1, 2, 3]`}
	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)
	g.Expect(input["struct_field"]).To(Equal(`[1, 2, 3]`))

	// Number JSON string - NOT a valid Struct
	input = map[string]any{"struct_field": `42`}
	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)
	g.Expect(input["struct_field"]).To(Equal(`42`))

	// Boolean JSON string - NOT a valid Struct
	input = map[string]any{"struct_field": `true`}
	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)
	g.Expect(input["struct_field"]).To(Equal(`true`))
}

func TestFixOpenAI_ListValueOnlyAcceptsArrays(t *testing.T) {
	g := NewWithT(t)

	descriptor := new(testdata.WktTestMessage)

	// Valid array
	input := map[string]any{"list_value": `[1, "two", true]`}
	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)
	g.Expect(input["list_value"]).To(Equal([]any{float64(1), "two", true}))

	// Object JSON - NOT a valid ListValue array
	input = map[string]any{"list_value": `{"not": "array"}`}
	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)
	g.Expect(input["list_value"]).To(Equal(`{"not": "array"}`))

	// Number JSON - NOT a valid ListValue
	input = map[string]any{"list_value": `42`}
	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)
	g.Expect(input["list_value"]).To(Equal(`42`))

	// Empty array is valid
	input = map[string]any{"list_value": `[]`}
	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)
	g.Expect(input["list_value"]).To(Equal([]any{}))
}

func TestFixOpenAI_NestedMapInRepeatedMessage(t *testing.T) {
	g := NewWithT(t)

	// RepeatedMessagesRequest.items[].labels is a map
	descriptor := new(testdata.RepeatedMessagesRequest)
	input := map[string]any{
		"items": []any{
			map[string]any{
				"name": "first",
				"labels": []any{
					map[string]any{"key": "a", "value": "1"},
				},
			},
			map[string]any{
				"name": "second",
				"labels": []any{}, // empty map
			},
			map[string]any{
				"name":   "third",
				"labels": map[string]any{"already": "obj"}, // already object
			},
		},
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)

	items := input["items"].([]any)
	g.Expect(items[0].(map[string]any)["labels"]).To(Equal(map[string]any{"a": "1"}))
	g.Expect(items[1].(map[string]any)["labels"]).To(Equal(map[string]any{}))
	g.Expect(items[2].(map[string]any)["labels"]).To(Equal(map[string]any{"already": "obj"}))
}

func TestFixOpenAI_DeepNesting_FullRoundTrip(t *testing.T) {
	g := NewWithT(t)

	descriptor := new(testdata.DeepNestingRequest)

	// A deeply nested structure exercising maps, WKTs, and repeated messages
	input := map[string]any{
		"middle": map[string]any{
			"inner": map[string]any{
				"id": "root-inner",
				"tags": []any{
					map[string]any{"key": "level", "value": "deep"},
				},
				"metadata":       `{"structured": true}`,
				"dynamic_config": `{"nested": {"deeply": true}}`,
			},
			"items": []any{
				map[string]any{
					"id": "item-in-middle",
					"tags": []any{
						map[string]any{"key": "idx", "value": "0"},
					},
					"metadata":       `{}`,
					"dynamic_config": `null`,
				},
			},
			"named_items": []any{
				map[string]any{
					"key": "alpha",
					"value": map[string]any{
						"id": "named-alpha",
						"tags": []any{
							map[string]any{"key": "name", "value": "alpha"},
						},
						"metadata":       `{"named": true}`,
						"dynamic_config": `42`,
					},
				},
			},
		},
		"middles": []any{
			map[string]any{
				"inner": map[string]any{
					"id":   "repeated-inner",
					"tags": []any{},
				},
				"items":       []any{},
				"named_items": []any{},
			},
		},
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)

	// Verify the deep nesting was fixed properly
	middle := input["middle"].(map[string]any)
	inner := middle["inner"].(map[string]any)
	g.Expect(inner["tags"]).To(Equal(map[string]any{"level": "deep"}))
	g.Expect(inner["metadata"]).To(Equal(map[string]any{"structured": true}))
	g.Expect(inner["dynamic_config"]).To(Equal(map[string]any{"nested": map[string]any{"deeply": true}}))

	items := middle["items"].([]any)
	item := items[0].(map[string]any)
	g.Expect(item["tags"]).To(Equal(map[string]any{"idx": "0"}))
	g.Expect(item["metadata"]).To(Equal(map[string]any{}))
	g.Expect(item["dynamic_config"]).To(BeNil())

	namedItems := middle["named_items"].(map[string]any)
	alpha := namedItems["alpha"].(map[string]any)
	g.Expect(alpha["tags"]).To(Equal(map[string]any{"name": "alpha"}))
	g.Expect(alpha["metadata"]).To(Equal(map[string]any{"named": true}))
	g.Expect(alpha["dynamic_config"]).To(BeNumerically("==", 42))

	// Verify repeated middles
	middles := input["middles"].([]any)
	m := middles[0].(map[string]any)
	mInner := m["inner"].(map[string]any)
	g.Expect(mInner["tags"]).To(Equal(map[string]any{}))

	// Full round-trip: marshal and unmarshal into proto
	fixedJSON, err := json.Marshal(input)
	g.Expect(err).ToNot(HaveOccurred())

	var result testdata.DeepNestingRequest
	err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(fixedJSON, &result)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(result.Middle.Inner.Id).To(Equal("root-inner"))
	g.Expect(result.Middle.Inner.Tags).To(Equal(map[string]string{"level": "deep"}))
	g.Expect(result.Middle.NamedItems).To(HaveKey("alpha"))
	g.Expect(result.Middle.NamedItems["alpha"].Id).To(Equal("named-alpha"))
}

func TestFixOpenAI_NonMessageFieldsUntouched(t *testing.T) {
	g := NewWithT(t)

	descriptor := new(testdata.AllScalarTypesRequest)
	input := map[string]any{
		"double_field":   1.5,
		"float_field":    2.5,
		"int32_field":    float64(42), // JSON numbers are float64
		"int64_field":    "1000",      // int64 as string in protojson
		"uint32_field":   float64(100),
		"uint64_field":   "200",
		"bool_field":     true,
		"string_field":   "hello",
		"bytes_field":    "d29ybGQ=", // base64
		"sint32_field":   float64(-10),
		"sint64_field":   "-20",
		"fixed32_field":  float64(300),
		"fixed64_field":  "400",
		"sfixed32_field": float64(-30),
		"sfixed64_field": "-40",
	}

	original := make(map[string]any)
	for k, v := range input {
		original[k] = v
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)

	// All scalar fields should remain unchanged
	for k, v := range original {
		g.Expect(input[k]).To(Equal(v), "field %s was modified", k)
	}
}

func TestFixOpenAI_FieldNotPresent(t *testing.T) {
	g := NewWithT(t)

	// DeepNestingRequest has "middle" and "middles" fields
	// If we pass in a completely unrelated map, nothing should happen
	descriptor := new(testdata.DeepNestingRequest)
	input := map[string]any{
		"unrelated_key": "some value",
		"another":       42,
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)

	// Unrelated fields are left alone (resolveFieldName returns "")
	g.Expect(input["unrelated_key"]).To(Equal("some value"))
	g.Expect(input["another"]).To(Equal(42))
}

func TestFixOpenAI_StructAlreadyParsed(t *testing.T) {
	g := NewWithT(t)

	// If struct_field is already a map (not a string), it should be left as-is
	descriptor := new(testdata.WktTestMessage)
	input := map[string]any{
		"struct_field": map[string]any{"already": "parsed"},
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)
	g.Expect(input["struct_field"]).To(Equal(map[string]any{"already": "parsed"}))
}

func TestFixOpenAI_ValueAlreadyParsed(t *testing.T) {
	descriptor := new(testdata.WktTestMessage)

	// If value_field is already a non-string type, leave it alone
	tests := []struct {
		name  string
		value any
	}{
		{"number", float64(42)},
		{"bool", true},
		{"array", []any{1, 2}},
		{"object", map[string]any{"k": "v"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			input := map[string]any{"value_field": tt.value}
			FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)
			g.Expect(input["value_field"]).To(Equal(tt.value))
		})
	}

	t.Run("nil", func(t *testing.T) {
		g := NewWithT(t)
		input := map[string]any{"value_field": nil}
		FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)
		g.Expect(input["value_field"]).To(BeNil())
	})
}

func TestFixOpenAI_EnumFieldUntouched(t *testing.T) {
	descriptor := new(testdata.EnumFieldsRequest)
	input := map[string]any{
		"priority":   "PRIORITY_HIGH",
		"priorities": []any{"PRIORITY_LOW", "PRIORITY_MEDIUM"},
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)

	// Enum fields should be untouched
	g := NewWithT(t)
	g.Expect(input["priority"]).To(Equal("PRIORITY_HIGH"))
	g.Expect(input["priorities"]).To(Equal([]any{"PRIORITY_LOW", "PRIORITY_MEDIUM"}))
}

func TestFixOpenAI_RepeatedNonMessageUntouched(t *testing.T) {
	g := NewWithT(t)

	descriptor := new(testdata.CreateItemRequest)
	input := map[string]any{
		"tags": []any{"a", "b", "c"},
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)
	g.Expect(input["tags"]).To(Equal([]any{"a", "b", "c"}))
}

func TestFixOpenAI_RepeatedTimestamps(t *testing.T) {
	descriptor := new(testdata.RepeatedMessagesRequest)
	input := map[string]any{
		"timestamps": []any{
			"2024-01-01T00:00:00Z",
			"2024-06-15T12:30:00Z",
		},
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)

	// Repeated timestamps are string WKTs - no map conversion needed
	// The IsList() branch for messages calls continue, so nested rewrite happens on each element
	// But Timestamp is not Struct/Value/ListValue so the default case runs
	// Since timestamps are strings, the nested type assertion to map[string]any fails, which is fine
	g := NewWithT(t)
	timestamps := input["timestamps"].([]any)
	g.Expect(timestamps).To(HaveLen(2))
	g.Expect(timestamps[0]).To(Equal("2024-01-01T00:00:00Z"))
}

func TestFixOpenAI_MapWithMessageValues_NestedWKTs(t *testing.T) {
	g := NewWithT(t)

	// MapVariantsRequest.string_to_message is map<string, InnerMessage>
	// InnerMessage has: tags (map<string,string>), metadata (Struct), dynamic_config (Value)
	descriptor := new(testdata.MapVariantsRequest)
	input := map[string]any{
		"string_to_message": []any{
			map[string]any{
				"key": "entry1",
				"value": map[string]any{
					"id": "inner-1",
					"tags": []any{
						map[string]any{"key": "env", "value": "prod"},
					},
					"metadata":       `{"nested": true}`,
					"dynamic_config": `[1, 2, 3]`,
				},
			},
		},
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)

	// The outer map should be converted from array to object
	msgMap := input["string_to_message"].(map[string]any)
	entry := msgMap["entry1"].(map[string]any)

	// Inner tags map should be converted
	g.Expect(entry["tags"]).To(Equal(map[string]any{"env": "prod"}))
	// Struct should be parsed
	g.Expect(entry["metadata"]).To(Equal(map[string]any{"nested": true}))
	// Value should be parsed
	g.Expect(entry["dynamic_config"]).To(Equal([]any{float64(1), float64(2), float64(3)}))
}
