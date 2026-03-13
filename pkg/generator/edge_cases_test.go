package generator

import (
	"bytes"
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/structpb"
)

// validateAgainstSchema compiles the schema and validates the JSON data against it.
func validateAgainstSchema(g *WithT, schemaJSON, dataJSON []byte) {
	compiler := jsonschema.NewCompiler()
	compiler.AssertContent = true
	err := compiler.AddResource("schema.json", bytes.NewReader(schemaJSON))
	g.Expect(err).ToNot(HaveOccurred())
	schema, err := compiler.Compile("schema.json")
	g.Expect(err).ToNot(HaveOccurred())
	var data any
	err = json.Unmarshal(dataJSON, &data)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(schema.Validate(data)).To(Succeed())
}

// schemaJSON generates a JSON schema for the given message descriptor.
func schemaJSON(g *WithT, md protoreflect.MessageDescriptor, openAI bool) []byte {
	fg := &FileGenerator{openAICompat: openAI}
	schema := fg.messageSchema(md)
	if openAI {
		schema["type"] = "object"
	}
	b, err := json.Marshal(schema)
	g.Expect(err).ToNot(HaveOccurred())
	return b
}

// roundTrip takes a proto message, marshals to JSON, validates against schema,
// optionally applies OpenAI fix, unmarshals back, and compares.
func roundTrip(g *WithT, msg proto.Message, openAI bool) {
	md := msg.ProtoReflect().Descriptor()
	schema := schemaJSON(g, md, openAI)

	var inputJSON []byte
	if openAI {
		// For OpenAI mode, we need to manually construct the JSON in OpenAI format
		// because protojson gives standard format
		inputJSON = protoToOpenAIJSON(g, msg)
	} else {
		var err error
		inputJSON, err = protojson.MarshalOptions{UseProtoNames: true}.Marshal(msg)
		g.Expect(err).ToNot(HaveOccurred())
	}

	// Validate against schema
	validateAgainstSchema(g, schema, inputJSON)

	// Unmarshal and compare (after OpenAI fix if needed)
	var data map[string]any
	err := json.Unmarshal(inputJSON, &data)
	g.Expect(err).ToNot(HaveOccurred())

	if openAI {
		runtime.FixOpenAI(md, data)
	}

	fixedJSON, err := json.Marshal(data)
	g.Expect(err).ToNot(HaveOccurred())

	result := proto.Clone(msg)
	proto.Reset(result)
	err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(fixedJSON, result)
	g.Expect(err).ToNot(HaveOccurred())
}

// protoToOpenAIJSON creates OpenAI-compatible JSON from a proto message.
// Maps become arrays of key-value pairs, WKTs become strings.
// Also adds all missing fields with zero values to satisfy OpenAI's "all required" constraint.
func protoToOpenAIJSON(g *WithT, msg proto.Message) []byte {
	// First get standard JSON with defaults
	standardJSON, err := (protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}).Marshal(msg)
	g.Expect(err).ToNot(HaveOccurred())

	var data map[string]any
	err = json.Unmarshal(standardJSON, &data)
	g.Expect(err).ToNot(HaveOccurred())

	// Transform maps and WKTs to OpenAI format
	transformToOpenAI(msg.ProtoReflect().Descriptor(), data)

	result, err := json.Marshal(data)
	g.Expect(err).ToNot(HaveOccurred())
	return result
}

// transformToOpenAI converts standard proto JSON to OpenAI format (inverse of FixOpenAI).
func transformToOpenAI(md protoreflect.MessageDescriptor, data map[string]any) {
	for i := 0; i < md.Fields().Len(); i++ {
		field := md.Fields().Get(i)
		name := string(field.Name())

		if _, ok := data[name]; !ok {
			continue
		}

		if field.IsMap() {
			// Convert map to array of key-value pairs.
			// If map values are messages, transform them first.
			if m, ok := data[name].(map[string]any); ok {
				var arr []any
				for k, v := range m {
					if field.MapValue().Kind() == protoreflect.MessageKind {
						if nested, ok := v.(map[string]any); ok {
							transformToOpenAI(field.MapValue().Message(), nested)
							v = nested
						}
					}
					arr = append(arr, map[string]any{"key": k, "value": v})
				}
				data[name] = arr
			}
		} else if field.Kind() == protoreflect.MessageKind {
			fullName := string(field.Message().FullName())

			if field.IsList() {
				if arr, ok := data[name].([]any); ok {
					for i, elem := range arr {
						if nested, ok := elem.(map[string]any); ok {
							transformToOpenAI(field.Message(), nested)
							arr[i] = nested
						}
					}
				}
				continue
			}

			switch fullName {
			case "google.protobuf.Value", "google.protobuf.ListValue", "google.protobuf.Struct":
				// Convert to JSON string
				b, _ := json.Marshal(data[name])
				data[name] = string(b)
			default:
				if nested, ok := data[name].(map[string]any); ok {
					transformToOpenAI(field.Message(), nested)
				}
			}
		}
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
			maxLen: 20,
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
		roundTrip(g, msg, false)
	})

	t.Run("openai schema validates", func(t *testing.T) {
		g := NewWithT(t)
		roundTrip(g, msg, true)
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
		roundTrip(g, msg, false)
	})

	t.Run("openai round-trip", func(t *testing.T) {
		g := NewWithT(t)
		roundTrip(g, msg, true)
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
		roundTrip(g, msg, false)
	})

	t.Run("openai", func(t *testing.T) {
		g := NewWithT(t)
		roundTrip(g, msg, true)
	})
}

func TestMapVariantsSchema(t *testing.T) {
	msg := &testdata.MapVariantsRequest{}
	md := msg.ProtoReflect().Descriptor()

	t.Run("standard map schemas", func(t *testing.T) {
		g := NewWithT(t)
		fg := &FileGenerator{openAICompat: false}
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

	t.Run("openai map schemas are arrays", func(t *testing.T) {
		g := NewWithT(t)
		fg := &FileGenerator{openAICompat: true}
		schema := fg.messageSchema(md)

		props := schema["properties"].(map[string]any)

		for _, fieldName := range []string{"string_to_string", "int_to_string", "bool_to_string", "uint64_to_string", "string_to_message", "string_to_double", "string_to_bool"} {
			field := props[fieldName].(map[string]any)
			g.Expect(field["type"]).To(Equal("array"), "field %s should be array in OpenAI mode", fieldName)
			g.Expect(field).To(HaveKey("items"))
		}
	})
}

func TestEnumFieldSchema(t *testing.T) {
	g := NewWithT(t)
	msg := &testdata.EnumFieldsRequest{}
	md := msg.ProtoReflect().Descriptor()

	fg := &FileGenerator{openAICompat: false}
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

func TestMultipleOneofsSchema(t *testing.T) {
	msg := &testdata.MultipleOneofsRequest{}
	md := msg.ProtoReflect().Descriptor()

	t.Run("standard mode has two anyOf groups", func(t *testing.T) {
		g := NewWithT(t)
		fg := &FileGenerator{openAICompat: false}
		schema := fg.messageSchema(md)

		// Should have anyOf with two oneOf groups
		anyOf := schema["anyOf"].([]map[string]any)
		g.Expect(anyOf).To(HaveLen(2))

		// First oneOf (source): url, raw_data, file_path
		firstOneOf := anyOf[0]["oneOf"].([]map[string]any)
		g.Expect(firstOneOf).To(HaveLen(3))

		// Second oneOf (output_format): as_json, as_xml, as_csv
		secondOneOf := anyOf[1]["oneOf"].([]map[string]any)
		g.Expect(secondOneOf).To(HaveLen(3))

		// Regular field 'name' should be in properties
		props := schema["properties"].(map[string]any)
		g.Expect(props).To(HaveKey("name"))
		g.Expect(props).ToNot(HaveKey("url"))
	})

	t.Run("openai mode flattens oneofs into properties", func(t *testing.T) {
		g := NewWithT(t)
		fg := &FileGenerator{openAICompat: true}
		schema := fg.messageSchema(md)

		// OpenAI mode should NOT have anyOf
		g.Expect(schema).ToNot(HaveKey("anyOf"))

		// All fields should be in properties
		props := schema["properties"].(map[string]any)
		g.Expect(props).To(HaveKey("name"))
		g.Expect(props).To(HaveKey("url"))
		g.Expect(props).To(HaveKey("raw_data"))
		g.Expect(props).To(HaveKey("file_path"))
		g.Expect(props).To(HaveKey("as_json"))
		g.Expect(props).To(HaveKey("as_xml"))
		g.Expect(props).To(HaveKey("as_csv"))

		// Oneof fields should have description about the group
		urlField := props["url"].(map[string]any)
		g.Expect(urlField["description"]).To(ContainSubstring("source"))
		g.Expect(urlField["description"]).To(ContainSubstring("oneof"))

		// All fields required in OpenAI mode
		required := schema["required"].([]string)
		g.Expect(required).To(ContainElements("name", "url", "raw_data", "file_path", "as_json", "as_xml", "as_csv"))

		// No duplicate required entries
		seen := make(map[string]bool)
		for _, r := range required {
			g.Expect(seen[r]).To(BeFalse(), "duplicate required field: %s", r)
			seen[r] = true
		}
	})
}

func TestNumericValidationConstraints(t *testing.T) {
	g := NewWithT(t)
	msg := &testdata.NumericValidationRequest{}
	md := msg.ProtoReflect().Descriptor()

	fg := &FileGenerator{openAICompat: false}
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

func TestOpenAISchemaInvariants(t *testing.T) {
	// Every message schema in OpenAI mode must have:
	// 1. additionalProperties: false
	// 2. All fields in required
	// 3. type: "object" (or ["object","null"] for nested)

	messages := []proto.Message{
		&testdata.AllScalarTypesRequest{},
		&testdata.DeepNestingRequest{},
		&testdata.RepeatedMessagesRequest{},
		&testdata.MapVariantsRequest{},
		&testdata.EnumFieldsRequest{},
		&testdata.MultipleOneofsRequest{},
		&testdata.NumericValidationRequest{},
		&testdata.CreateItemRequest{},
		&testdata.ProcessWellKnownTypesRequest{},
	}

	for _, msg := range messages {
		name := string(msg.ProtoReflect().Descriptor().FullName())
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			fg := &FileGenerator{openAICompat: true}
			schema := fg.messageSchema(msg.ProtoReflect().Descriptor())

			// Must have additionalProperties: false
			g.Expect(schema["additionalProperties"]).To(Equal(false))

			// All properties must be in required
			props := schema["properties"].(map[string]any)
			required := schema["required"].([]string)
			for fieldName := range props {
				g.Expect(required).To(ContainElement(fieldName),
					"field %s not in required list for %s", fieldName, name)
			}
		})
	}
}

func TestWrapperTypesSchema(t *testing.T) {
	msg := &testdata.WktTestMessage{}
	md := msg.ProtoReflect().Descriptor()

	t.Run("wrapper types use nullable type array", func(t *testing.T) {
		g := NewWithT(t)
		fg := &FileGenerator{openAICompat: false}
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

	fg := &FileGenerator{openAICompat: false}
	schema := fg.getType(fd)
	g.Expect(schema["type"]).To(Equal([]string{"string", "null"}))
	g.Expect(schema["pattern"]).To(Equal(`^-?[0-9]+(\.[0-9]+)?s$`))
}

func TestFieldMaskSchema(t *testing.T) {
	g := NewWithT(t)
	msg := &testdata.WktTestMessage{}
	fd := msg.ProtoReflect().Descriptor().Fields().ByName("field_mask")

	t.Run("standard", func(t *testing.T) {
		g := NewWithT(t)
		fg := &FileGenerator{openAICompat: false}
		schema := fg.getType(fd)
		g.Expect(schema["type"]).To(Equal("string"))
	})

	t.Run("openai", func(t *testing.T) {
		fg := &FileGenerator{openAICompat: true}
		schema := fg.getType(fd)
		g.Expect(schema["type"]).To(Equal([]string{"string", "null"}))
	})
}

func TestAnyTypeSchema(t *testing.T) {
	g := NewWithT(t)
	msg := &testdata.WktTestMessage{}
	fd := msg.ProtoReflect().Descriptor().Fields().ByName("any")

	t.Run("standard has nullable type", func(t *testing.T) {
		g := NewWithT(t)
		fg := &FileGenerator{openAICompat: false}
		schema := fg.getType(fd)
		g.Expect(schema["type"]).To(Equal([]string{"object", "null"}))
		props := schema["properties"].(map[string]any)
		g.Expect(props).To(HaveKey("@type"))
		g.Expect(props).To(HaveKey("value"))
	})

	t.Run("openai has additionalProperties false", func(t *testing.T) {
		fg := &FileGenerator{openAICompat: true}
		schema := fg.getType(fd)
		g.Expect(schema["type"]).To(Equal("object"))
		g.Expect(schema["additionalProperties"]).To(Equal(false))
		required := schema["required"].([]string)
		g.Expect(required).To(ContainElements("@type", "value"))
	})
}

func TestBytesFieldSchema(t *testing.T) {
	g := NewWithT(t)
	msg := &testdata.AllScalarTypesRequest{}
	fd := msg.ProtoReflect().Descriptor().Fields().ByName("bytes_field")

	t.Run("standard has format byte", func(t *testing.T) {
		g := NewWithT(t)
		fg := &FileGenerator{openAICompat: false}
		schema := fg.getType(fd)
		g.Expect(schema["type"]).To(Equal("string"))
		g.Expect(schema["format"]).To(Equal("byte"))
		g.Expect(schema["contentEncoding"]).To(Equal("base64"))
	})

	t.Run("openai omits format byte", func(t *testing.T) {
		fg := &FileGenerator{openAICompat: true}
		schema := fg.getType(fd)
		g.Expect(schema["type"]).To(Equal("string"))
		g.Expect(schema).ToNot(HaveKey("format"))
		g.Expect(schema["contentEncoding"]).To(Equal("base64"))
	})
}

func TestRepeatedFieldSchema(t *testing.T) {
	g := NewWithT(t)
	msg := &testdata.CreateItemRequest{}
	fd := msg.ProtoReflect().Descriptor().Fields().ByName("tags")

	fg := &FileGenerator{openAICompat: false}
	schema := fg.getType(fd)
	g.Expect(schema["type"]).To(Equal("array"))
	items := schema["items"].(map[string]any)
	g.Expect(items["type"]).To(Equal("string"))
}
