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

package generator

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	anypb "google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func init() {
}

func TestCompat(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name  string
		input proto.Message
		// If rawJsonInput is set, it's preferred over input.
		// It can be used to simulate a wrong input, eg. not using base64 for byte fields.
		// input must still be provided, so we know the proto type.
		rawJsonInput  json.RawMessage
		errorExpected bool
		errorContains string
	}{
		{
			name: "any containing struct",
			input: func() proto.Message {
				val := &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"nested": structpb.NewStringValue("value"),
					},
				}
				any, err := anypb.New(val)
				g.Expect(err).ToNot(HaveOccurred())
				return &testdata.WktTestMessage{
					Any: any,
				}
			}(),
		},
		{
			name: "bytes value with weird base64",
			input: &testdata.WktTestMessage{
				BytesValue: wrapperspb.Bytes([]byte{0xde, 0xad, 0xbe, 0xef}),
			},
		},
		{
			name: "negative duration",
			input: &testdata.WktTestMessage{
				Duration: durationpb.New(-5 * time.Second),
			},
		},
		{
			name: "timestamp in the future",
			input: &testdata.WktTestMessage{
				Timestamp: timestamppb.New(time.Date(3000, 1, 1, 0, 0, 0, 0, time.UTC)),
			},
		},
		{
			name: "wrapper types with default values",
			input: &testdata.WktTestMessage{
				StringValue: wrapperspb.String(""),
				Int32Value:  wrapperspb.Int32(0),
				Int64Value:  wrapperspb.Int64(0),
				BoolValue:   wrapperspb.Bool(false),
				BytesValue:  wrapperspb.Bytes(nil),
			},
		},
		{
			name: "basic any test",
			input: func() proto.Message {
				any, err := anypb.New(wrapperspb.String("some-string-in-any"))
				g.Expect(err).ToNot(HaveOccurred())
				return &testdata.WktTestMessage{
					Any: any,
				}
			}(),
		},
		{
			name: "bytes as base64 works",
			input: &testdata.TestMessage{
				SomeBytes: []byte{1, 200, 125},
			},
		},
		{
			name:          "bytes must be base64 - fails if it's not",
			input:         &testdata.TestMessage{},
			rawJsonInput:  json.RawMessage(`{"some_bytes":"hello this is not base64"}`),
			errorExpected: true,
			errorContains: "/properties/some_bytes/contentEncoding",
		},
		{
			name: "a little bit of everything, required field is set",
			input: &testdata.WktTestMessage{
				Timestamp:   timestamppb.New(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)),
				Duration:    durationpb.New(3 * time.Second),
				StructField: &structpb.Struct{Fields: map[string]*structpb.Value{"foo": structpb.NewStringValue("bar")}},
				ValueField:  structpb.NewNumberValue(42),
				ListValue:   &structpb.ListValue{Values: []*structpb.Value{structpb.NewBoolValue(true)}},
				FieldMask:   &fieldmaskpb.FieldMask{Paths: []string{"foo", "bar"}},
				StringValue: wrapperspb.String("hello"),
				Int32Value:  wrapperspb.Int32(123),
				Int64Value:  wrapperspb.Int64(1234567890123),
				BoolValue:   wrapperspb.Bool(true),
				BytesValue:  wrapperspb.Bytes([]byte("hi")),
			},
		},
		{
			name:          "required field absent throws error",
			input:         &testdata.RequiredFieldTest{},
			errorExpected: true,
			errorContains: `missing properties: 'required_field'`,
		},
		{
			name:         "nullable timestamp as null",
			input:        &testdata.WktTestMessage{}, // Empty message, timestamp is nil
			rawJsonInput: json.RawMessage(`{"timestamp": null}`),
		},
		{
			name: "map as object",
			input: &testdata.MapTestMessage{
				StringMap: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			var input []byte
			if tt.rawJsonInput == nil {
				marshaled, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(tt.input)
				g.Expect(err).ToNot(HaveOccurred())
				input = marshaled
			} else {
				input = tt.rawJsonInput
			}

			// Create a generator instance to access messageSchema method
			fg := &FileGenerator{}
			schemaMap := fg.messageSchema(tt.input.ProtoReflect().Descriptor())
			schemaJSON, err := json.Marshal(schemaMap)
			g.Expect(err).ToNot(HaveOccurred())

			// Step 4: Validate the marshaled JSON against the schema
			compiler := jsonschema.NewCompiler()
			// This is required, so it can assert that strings are base64.
			compiler.AssertContent = true

			err = compiler.AddResource("schema.json", bytes.NewReader(schemaJSON))
			g.Expect(err).ToNot(HaveOccurred())

			schema, err := compiler.Compile("schema.json")
			g.Expect(err).ToNot(HaveOccurred())

			var jsonData interface{}
			err = json.Unmarshal(input, &jsonData)
			g.Expect(err).ToNot(HaveOccurred())

			err = schema.Validate(jsonData)
			if tt.errorExpected {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			if tt.errorContains != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errorContains)))
			}
		})
	}
}

func TestMangleHeadIfTooLong(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		check  func(*WithT, string)
	}{
		{
			name:   "short name unchanged",
			input:  "foo_bar",
			maxLen: 64,
			check: func(g *WithT, result string) {
				g.Expect(result).To(Equal("foo_bar"))
			},
		},
		{
			name:   "exactly at limit",
			input:  "a_very_long_name_that_is_exactly_at_the_limit_0123456789_abcdefg",
			maxLen: 64,
			check: func(g *WithT, result string) {
				g.Expect(result).To(Equal("a_very_long_name_that_is_exactly_at_the_limit_0123456789_abcdefg"))
			},
		},
		{
			name:   "over limit gets mangled",
			input:  "a_very_long_name_that_exceeds_the_limit_0123456789_abcdefghijklmnop",
			maxLen: 64,
			check: func(g *WithT, result string) {
				g.Expect(len(result)).To(BeNumerically("<=", 64))
				g.Expect(result).To(ContainSubstring("_"))
			},
		},
		{
			name:   "deterministic mangling",
			input:  "some.very.long.package.name.with.many.dots.ServiceName.MethodName",
			maxLen: 40,
			check: func(g *WithT, result string) {
				result2 := MangleHeadIfTooLong("some.very.long.package.name.with.many.dots.ServiceName.MethodName", 40)
				g.Expect(result).To(Equal(result2))
			},
		},
		{
			name:   "preserves tail (most specific part)",
			input:  "some_package_ServiceName_MethodName",
			maxLen: 24,
			check: func(g *WithT, result string) {
				g.Expect(result).To(HaveSuffix("MethodName"))
			},
		},
		{
			name:   "very small maxLen",
			input:  "abcdefghij",
			maxLen: 6,
			check: func(g *WithT, result string) {
				// Should just return hash prefix
				g.Expect(len(result)).To(Equal(6))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			result := MangleHeadIfTooLong(tt.input, tt.maxLen)
			tt.check(g, result)
		})
	}
}

func TestCleanComment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple comment", "This is a comment", "This is a comment"},
		{"strips buf lint prefix", "buf:lint:FIELD_LOWER_SNAKE_CASE\nActual comment", "Actual comment"},
		{"strips ignore prefix", "@ignore-comment\nKeep this", "Keep this"},
		{"multiple lines", "Line 1\nLine 2\nLine 3", "Line 1\nLine 2\nLine 3"},
		{"empty string", "", ""},
		{"only stripped lines", "buf:lint:FOO\n@ignore-comment", ""},
		{"whitespace handling", "  Indented  \n  Also indented  ", "Indented\nAlso indented"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(cleanComment(tt.input)).To(Equal(tt.expected))
		})
	}
}

func TestBase36String(t *testing.T) {
	g := NewWithT(t)
	result := Base36String([]byte{0xde, 0xad, 0xbe, 0xef})
	g.Expect(result).ToNot(BeEmpty())
	// Different inputs should produce different outputs
	result2 := Base36String([]byte{0xca, 0xfe, 0xba, 0xbe})
	g.Expect(result).ToNot(Equal(result2))
}

func TestAllScalarTypes(t *testing.T) {
	msg := &testdata.AllScalarTypesRequest{
		DoubleField:   3.14,
		FloatField:    2.71,
		Int32Field:    42,
		Int64Field:    9999999999,
		Uint32Field:   100,
		Uint64Field:   200,
		Sint32Field:   -50,
		Sint64Field:   -99999,
		Fixed32Field:  1000,
		Fixed64Field:  2000,
		Sfixed32Field: -1000,
		Sfixed64Field: -2000,
		BoolField:     true,
		StringField:   "hello",
		BytesField:    []byte{0xde, 0xad},
	}

	t.Run("standard schema validates", func(t *testing.T) {
		g := NewWithT(t)
		fg := &FileGenerator{}
		md := msg.ProtoReflect().Descriptor()
		schema := fg.messageSchema(md)
		b, err := json.Marshal(schema)
		g.Expect(err).ToNot(HaveOccurred())
		compiler := jsonschema.NewCompiler()
		g.Expect(compiler.AddResource("schema.json", bytes.NewReader(b))).To(Succeed())
		compiled, err := compiler.Compile("schema.json")
		g.Expect(err).ToNot(HaveOccurred())
		jsonBytes, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(msg)
		g.Expect(err).ToNot(HaveOccurred())
		var doc any
		g.Expect(json.Unmarshal(jsonBytes, &doc)).To(Succeed())
		g.Expect(compiled.Validate(doc)).To(Succeed())
	})
}

func TestDeepNestingRoundTrip(t *testing.T) {
	inner := &testdata.InnerMessage{
		Id:   "inner-1",
		Tags: map[string]string{"env": "prod", "team": "backend"},
		Metadata: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"key": structpb.NewStringValue("value"),
			},
		},
		DynamicConfig: structpb.NewNumberValue(42),
	}

	msg := &testdata.DeepNestingRequest{
		Middle: &testdata.MiddleMessage{
			Inner: inner,
			Items: []*testdata.InnerMessage{inner, inner},
			NamedItems: map[string]*testdata.InnerMessage{
				"first": inner,
			},
		},
	}

	t.Run("standard round-trip", func(t *testing.T) {
		g := NewWithT(t)
		fg := &FileGenerator{}
		md := msg.ProtoReflect().Descriptor()
		schema := fg.messageSchema(md)
		b, err := json.Marshal(schema)
		g.Expect(err).ToNot(HaveOccurred())
		compiler := jsonschema.NewCompiler()
		g.Expect(compiler.AddResource("schema.json", bytes.NewReader(b))).To(Succeed())
		compiled, err := compiler.Compile("schema.json")
		g.Expect(err).ToNot(HaveOccurred())
		jsonBytes, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(msg)
		g.Expect(err).ToNot(HaveOccurred())
		var doc any
		g.Expect(json.Unmarshal(jsonBytes, &doc)).To(Succeed())
		g.Expect(compiled.Validate(doc)).To(Succeed())
	})
}

func TestRepeatedMessagesRoundTrip(t *testing.T) {
	msg := &testdata.RepeatedMessagesRequest{
		Items: []*testdata.ItemWithMap{
			{
				Name:   "item1",
				Labels: map[string]string{"a": "1"},
				Config: structpb.NewStringValue("hello"),
				Extra: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"nested": structpb.NewBoolValue(true),
					},
				},
			},
			{
				Name:   "item2",
				Labels: map[string]string{"b": "2", "c": "3"},
				Config: structpb.NewBoolValue(false),
				Extra: &structpb.Struct{
					Fields: map[string]*structpb.Value{},
				},
			},
		},
	}

	t.Run("standard", func(t *testing.T) {
		g := NewWithT(t)
		fg := &FileGenerator{}
		md := msg.ProtoReflect().Descriptor()
		schema := fg.messageSchema(md)
		b, err := json.Marshal(schema)
		g.Expect(err).ToNot(HaveOccurred())
		compiler := jsonschema.NewCompiler()
		g.Expect(compiler.AddResource("schema.json", bytes.NewReader(b))).To(Succeed())
		compiled, err := compiler.Compile("schema.json")
		g.Expect(err).ToNot(HaveOccurred())
		jsonBytes, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(msg)
		g.Expect(err).ToNot(HaveOccurred())
		var doc any
		g.Expect(json.Unmarshal(jsonBytes, &doc)).To(Succeed())
		g.Expect(compiled.Validate(doc)).To(Succeed())
	})
}

func TestMapVariantsSchema(t *testing.T) {
	msg := &testdata.MapVariantsRequest{}
	md := msg.ProtoReflect().Descriptor()

	t.Run("standard map schemas", func(t *testing.T) {
		g := NewWithT(t)
		fg := &FileGenerator{}
		schema := fg.messageSchema(md)

		props := schema["properties"].(map[string]any)

		// string-to-string: object with string additionalProperties
		sts := props["string_to_string"].(map[string]any)
		g.Expect(sts["type"]).To(Equal("object"))

		// int-to-string: object with integer pattern on propertyNames
		its := props["int_to_string"].(map[string]any)
		g.Expect(its["type"]).To(Equal("object"))
		pn := its["propertyNames"].(map[string]any)
		g.Expect(pn).To(HaveKey("pattern"))

		// bool-to-string: object with enum on propertyNames
		bts := props["bool_to_string"].(map[string]any)
		g.Expect(bts["type"]).To(Equal("object"))
		bpn := bts["propertyNames"].(map[string]any)
		g.Expect(bpn["enum"]).To(Equal([]string{"true", "false"}))

		// uint64-to-string: unsigned int pattern
		u64s := props["uint64_to_string"].(map[string]any)
		u64pn := u64s["propertyNames"].(map[string]any)
		g.Expect(u64pn["pattern"]).To(Equal("^(0|[1-9]\\d*)$"))

		// string-to-message: object with nested message additionalProperties
		stm := props["string_to_message"].(map[string]any)
		g.Expect(stm["type"]).To(Equal("object"))
		ap := stm["additionalProperties"].(map[string]any)
		g.Expect(ap["type"]).To(Equal("object"))
		g.Expect(ap["properties"]).ToNot(BeNil())

		// string-to-double
		std := props["string_to_double"].(map[string]any)
		ap2 := std["additionalProperties"].(map[string]any)
		g.Expect(ap2["type"]).To(Equal("number"))

		// string-to-bool
		stb := props["string_to_bool"].(map[string]any)
		ap3 := stb["additionalProperties"].(map[string]any)
		g.Expect(ap3["type"]).To(Equal("boolean"))
	})
}

func TestEnumFieldSchema(t *testing.T) {
	g := NewWithT(t)
	msg := &testdata.EnumFieldsRequest{}
	md := msg.ProtoReflect().Descriptor()

	fg := &FileGenerator{}
	schema := fg.messageSchema(md)
	props := schema["properties"].(map[string]any)

	// Single enum
	priority := props["priority"].(map[string]any)
	g.Expect(priority["type"]).To(Equal("string"))
	g.Expect(priority["enum"]).To(ConsistOf(
		"PRIORITY_UNSPECIFIED", "PRIORITY_LOW", "PRIORITY_MEDIUM", "PRIORITY_HIGH", "PRIORITY_CRITICAL",
	))

	// Repeated enum
	priorities := props["priorities"].(map[string]any)
	g.Expect(priorities["type"]).To(Equal("array"))
	items := priorities["items"].(map[string]any)
	g.Expect(items["enum"]).To(ConsistOf(
		"PRIORITY_UNSPECIFIED", "PRIORITY_LOW", "PRIORITY_MEDIUM", "PRIORITY_HIGH", "PRIORITY_CRITICAL",
	))
}

// TestMultipleOneofsSchema verifies the discriminated-object rendering for oneofs.
func TestMultipleOneofsSchema(t *testing.T) {
	msg := &testdata.MultipleOneofsRequest{}
	md := msg.ProtoReflect().Descriptor()

	t.Run("no top-level union keywords", func(t *testing.T) {
		g := NewWithT(t)
		fg := &FileGenerator{}
		// Marshal/unmarshal to normalize orderedMap values to plain map[string]any.
		raw, err := json.Marshal(fg.messageSchema(md))
		g.Expect(err).ToNot(HaveOccurred())
		var schema map[string]any
		g.Expect(json.Unmarshal(raw, &schema)).To(Succeed())

		// No anyOf/oneOf/allOf at top level.
		g.Expect(schema).ToNot(HaveKey("anyOf"))
		g.Expect(schema).ToNot(HaveKey("oneOf"))
		g.Expect(schema).ToNot(HaveKey("allOf"))

		// Oneof groups render as discriminated wrapper objects in properties.
		props := schema["properties"].(map[string]any)
		g.Expect(props).To(HaveKey("source"))
		g.Expect(props).To(HaveKey("output_format"))

		sourceWrapper := props["source"].(map[string]any)
		g.Expect(sourceWrapper["type"]).To(Equal("object"))
		sourceProps := sourceWrapper["properties"].(map[string]any)
		g.Expect(sourceProps).To(HaveKey("which"))

		// Regular field 'name' should be in properties
		g.Expect(props).To(HaveKey("name"))
	})
}

func TestNumericValidationConstraints(t *testing.T) {
	g := NewWithT(t)
	msg := &testdata.NumericValidationRequest{}
	md := msg.ProtoReflect().Descriptor()

	fg := &FileGenerator{}
	schema := fg.messageSchema(md)
	props := schema["properties"].(map[string]any)

	// int32 with gte/lte
	age := props["age"].(map[string]any)
	g.Expect(age["minimum"]).To(Equal(0))
	g.Expect(age["maximum"]).To(Equal(150))

	// int32 with gt/lt (gt:0 -> minimum:1, lt:100 -> maximum:99)
	score := props["score"].(map[string]any)
	g.Expect(score["minimum"]).To(Equal(1))
	g.Expect(score["maximum"]).To(Equal(99))

	// uint32 with gte/lte
	count := props["count"].(map[string]any)
	g.Expect(count["minimum"]).To(Equal(1))
	g.Expect(count["maximum"]).To(Equal(1000))

	// uint64 with gt (gt:0 -> minimum:1)
	bigCount := props["big_count"].(map[string]any)
	g.Expect(bigCount["minimum"]).To(Equal(1))

	// float with gte/lte
	pct := props["percentage"].(map[string]any)
	g.Expect(pct["minimum"]).To(BeNumerically("==", 0.0))
	g.Expect(pct["maximum"]).To(BeNumerically("==", 100.0))

	// double with gt/lt (exclusive)
	temp := props["temperature"].(map[string]any)
	g.Expect(temp["exclusiveMinimum"]).To(BeNumerically("==", -273.15))
	g.Expect(temp["exclusiveMaximum"]).To(BeNumerically("==", 1000000.0))

	// int64 with gte
	ts := props["timestamp_nanos"].(map[string]any)
	g.Expect(ts["minimum"]).To(Equal(0))

	// string with multiple constraints
	code := props["code"].(map[string]any)
	g.Expect(code["minLength"]).To(Equal(2))
	g.Expect(code["maxLength"]).To(Equal(10))
	g.Expect(code["pattern"]).To(Equal("^[A-Z0-9]+$"))
}

func TestWrapperTypesSchema(t *testing.T) {
	msg := &testdata.WktTestMessage{}
	md := msg.ProtoReflect().Descriptor()

	t.Run("wrapper types use nullable type array", func(t *testing.T) {
		g := NewWithT(t)
		fg := &FileGenerator{}
		schema := fg.messageSchema(md)
		props := schema["properties"].(map[string]any)

		// StringValue
		sv := props["string_value"].(map[string]any)
		g.Expect(sv["type"]).To(Equal([]string{"string", "null"}))

		// Int32Value
		i32v := props["int32_value"].(map[string]any)
		g.Expect(i32v["type"]).To(Equal([]string{"number", "null"}))

		// Int64Value
		i64v := props["int64_value"].(map[string]any)
		g.Expect(i64v["type"]).To(Equal([]string{"string", "null"}))

		// BoolValue
		bv := props["bool_value"].(map[string]any)
		g.Expect(bv["type"]).To(Equal([]string{"boolean", "null"}))

		// BytesValue
		byv := props["bytes_value"].(map[string]any)
		g.Expect(byv["type"]).To(Equal([]string{"string", "null"}))
		g.Expect(byv["format"]).To(Equal("byte"))

		// No "nullable" key anywhere
		for fieldName, prop := range props {
			propMap, ok := prop.(map[string]any)
			if ok {
				g.Expect(propMap).ToNot(HaveKey("nullable"),
					"field %s should not have nullable key", fieldName)
			}
		}
	})
}

func TestDurationFieldSchema(t *testing.T) {
	g := NewWithT(t)
	msg := &testdata.WktTestMessage{}
	fd := msg.ProtoReflect().Descriptor().Fields().ByName("duration")

	fg := &FileGenerator{}
	schema := fg.getType(fd)
	g.Expect(schema["type"]).To(Equal([]string{"string", "null"}))
	g.Expect(schema["pattern"]).To(Equal(`^-?[0-9]+(\.[0-9]+)?s$`))
}

func TestFieldMaskSchema(t *testing.T) {
	msg := &testdata.WktTestMessage{}
	fd := msg.ProtoReflect().Descriptor().Fields().ByName("field_mask")

	t.Run("standard", func(t *testing.T) {
		g := NewWithT(t)
		fg := &FileGenerator{}
		schema := fg.getType(fd)
		g.Expect(schema["type"]).To(Equal("string"))
	})
}

func TestAnyTypeSchema(t *testing.T) {
	msg := &testdata.WktTestMessage{}
	fd := msg.ProtoReflect().Descriptor().Fields().ByName("any")

	t.Run("standard has nullable type", func(t *testing.T) {
		g := NewWithT(t)
		fg := &FileGenerator{}
		schema := fg.getType(fd)
		g.Expect(schema["type"]).To(Equal([]string{"object", "null"}))
		props := schema["properties"].(map[string]any)
		g.Expect(props).To(HaveKey("@type"))
		g.Expect(props).To(HaveKey("value"))
	})
}

func TestBytesFieldSchema(t *testing.T) {
	msg := &testdata.AllScalarTypesRequest{}
	fd := msg.ProtoReflect().Descriptor().Fields().ByName("bytes_field")

	t.Run("standard has format byte", func(t *testing.T) {
		g := NewWithT(t)
		fg := &FileGenerator{}
		schema := fg.getType(fd)
		g.Expect(schema["type"]).To(Equal("string"))
		g.Expect(schema["format"]).To(Equal("byte"))
		g.Expect(schema["contentEncoding"]).To(Equal("base64"))
	})
}

func TestRepeatedFieldSchema(t *testing.T) {
	g := NewWithT(t)
	msg := &testdata.CreateItemRequest{}
	fd := msg.ProtoReflect().Descriptor().Fields().ByName("tags")

	fg := &FileGenerator{}
	schema := fg.getType(fd)
	g.Expect(schema["type"]).To(Equal("array"))
	items := schema["items"].(map[string]any)
	g.Expect(items["type"]).To(Equal("string"))
}
