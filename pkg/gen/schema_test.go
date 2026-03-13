package gen

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestMessageSchema_Standard(t *testing.T) {
	g := NewWithT(t)
	md := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor()
	schema := MessageSchema(md, SchemaOptions{OpenAICompat: false})

	g.Expect(schema["type"]).To(Equal("object"))
	g.Expect(schema).To(HaveKey("properties"))
	g.Expect(schema).ToNot(HaveKey("additionalProperties"))

	// Should have anyOf for oneof
	g.Expect(schema).To(HaveKey("anyOf"))
}

func TestMessageSchema_OpenAI(t *testing.T) {
	g := NewWithT(t)
	md := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor()
	schema := MessageSchema(md, SchemaOptions{OpenAICompat: true})

	g.Expect(schema["additionalProperties"]).To(Equal(false))
	g.Expect(schema).ToNot(HaveKey("anyOf"))
	// All properties must be in required
	props := schema["properties"].(map[string]any)
	required := schema["required"].([]string)
	for name := range props {
		g.Expect(required).To(ContainElement(name))
	}
}

func TestFieldSchema_AllKinds(t *testing.T) {
	msg := (&testdata.AllScalarTypesRequest{}).ProtoReflect().Descriptor()
	opts := SchemaOptions{OpenAICompat: false}

	tests := []struct {
		name     string
		expected string
	}{
		{"double_field", "number"},
		{"float_field", "number"},
		{"int32_field", "integer"},
		{"int64_field", "string"},
		{"uint32_field", "integer"},
		{"uint64_field", "string"},
		{"bool_field", "boolean"},
		{"string_field", "string"},
		{"bytes_field", "string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			fd := msg.Fields().ByName(protoreflect.Name(tt.name))
			schema := FieldSchema(fd, opts)
			g.Expect(schema["type"]).To(Equal(tt.expected))
		})
	}
}

func TestToolForMethod(t *testing.T) {
	g := NewWithT(t)
	// Use the TestService's CreateItem method descriptor
	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	svc := file.Services().ByName("TestService")
	g.Expect(svc).ToNot(BeNil())

	method := svc.Methods().ByName("CreateItem")
	g.Expect(method).ToNot(BeNil())

	standard, openAI := ToolForMethod(method, "Create a new item")

	g.Expect(standard.Name).To(Equal("testdata_TestService_CreateItem"))
	g.Expect(standard.Description).To(Equal("Create a new item"))
	g.Expect(openAI.Name).To(Equal(standard.Name))

	// Standard schema should NOT have additionalProperties: false
	var stdSchema map[string]any
	err := json.Unmarshal(standard.RawInputSchema, &stdSchema)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(stdSchema).ToNot(HaveKey("additionalProperties"))

	// OpenAI schema MUST have additionalProperties: false
	var oaiSchema map[string]any
	err = json.Unmarshal(openAI.RawInputSchema, &oaiSchema)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(oaiSchema["additionalProperties"]).To(Equal(false))
	g.Expect(oaiSchema["type"]).To(Equal("object")) // Not ["object","null"] at top level
}

func TestMangleHeadIfTooLong_Deterministic(t *testing.T) {
	g := NewWithT(t)
	name := "some.very.long.package.name.with.lots.of.nesting.ServiceName.MethodName"
	r1 := MangleHeadIfTooLong(name, 64)
	r2 := MangleHeadIfTooLong(name, 64)
	g.Expect(r1).To(Equal(r2))
	g.Expect(len(r1)).To(BeNumerically("<=", 64))
}

func TestCleanComment_Strips(t *testing.T) {
	g := NewWithT(t)
	g.Expect(CleanComment("buf:lint:FOO\nActual")).To(Equal("Actual"))
	g.Expect(CleanComment("@ignore-comment\nKeep")).To(Equal("Keep"))
	g.Expect(CleanComment("Normal comment")).To(Equal("Normal comment"))
}

func TestMessageFieldSchema_WellKnownTypes_Standard(t *testing.T) {
	md := (&testdata.WktTestMessage{}).ProtoReflect().Descriptor()
	opts := SchemaOptions{OpenAICompat: false}

	tests := []struct {
		fieldName string
		check     func(g Gomega, schema map[string]any)
	}{
		{"timestamp", func(g Gomega, s map[string]any) {
			g.Expect(s["type"]).To(Equal([]string{"string", "null"}))
			g.Expect(s["format"]).To(Equal("date-time"))
		}},
		{"duration", func(g Gomega, s map[string]any) {
			g.Expect(s["type"]).To(Equal([]string{"string", "null"}))
			g.Expect(s["pattern"]).To(Equal(`^-?[0-9]+(\.[0-9]+)?s$`))
		}},
		{"struct_field", func(g Gomega, s map[string]any) {
			g.Expect(s["type"]).To(Equal("object"))
			g.Expect(s["additionalProperties"]).To(Equal(true))
		}},
		{"value_field", func(g Gomega, s map[string]any) {
			g.Expect(s).To(HaveKey("description"))
			g.Expect(s).ToNot(HaveKey("type"))
		}},
		{"list_value", func(g Gomega, s map[string]any) {
			g.Expect(s["type"]).To(Equal("array"))
			g.Expect(s).To(HaveKey("items"))
			g.Expect(s).To(HaveKey("description"))
		}},
		{"field_mask", func(g Gomega, s map[string]any) {
			g.Expect(s["type"]).To(Equal("string"))
		}},
		{"any", func(g Gomega, s map[string]any) {
			g.Expect(s["type"]).To(Equal([]string{"object", "null"}))
			props := s["properties"].(map[string]any)
			g.Expect(props).To(HaveKey("@type"))
			required := s["required"].([]string)
			g.Expect(required).To(ContainElement("@type"))
		}},
		{"string_value", func(g Gomega, s map[string]any) {
			g.Expect(s["type"]).To(Equal([]string{"string", "null"}))
		}},
		{"int32_value", func(g Gomega, s map[string]any) {
			g.Expect(s["type"]).To(Equal([]string{"number", "null"}))
		}},
		{"int64_value", func(g Gomega, s map[string]any) {
			g.Expect(s["type"]).To(Equal([]string{"string", "null"}))
		}},
		{"bool_value", func(g Gomega, s map[string]any) {
			g.Expect(s["type"]).To(Equal([]string{"boolean", "null"}))
		}},
		{"bytes_value", func(g Gomega, s map[string]any) {
			g.Expect(s["type"]).To(Equal([]string{"string", "null"}))
			g.Expect(s["format"]).To(Equal("byte"))
		}},
	}

	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			g := NewWithT(t)
			fd := md.Fields().ByName(protoreflect.Name(tt.fieldName))
			g.Expect(fd).ToNot(BeNil(), "field %s not found", tt.fieldName)
			schema := FieldSchema(fd, opts)
			tt.check(g, schema)
		})
	}
}

func TestMessageFieldSchema_WellKnownTypes_OpenAI(t *testing.T) {
	md := (&testdata.WktTestMessage{}).ProtoReflect().Descriptor()
	opts := SchemaOptions{OpenAICompat: true}

	tests := []struct {
		fieldName string
		check     func(g Gomega, schema map[string]any)
	}{
		{"timestamp", func(g Gomega, s map[string]any) {
			g.Expect(s["type"]).To(Equal([]string{"string", "null"}))
			g.Expect(s["format"]).To(Equal("date-time"))
		}},
		{"duration", func(g Gomega, s map[string]any) {
			g.Expect(s["type"]).To(Equal([]string{"string", "null"}))
		}},
		{"struct_field", func(g Gomega, s map[string]any) {
			g.Expect(s["type"]).To(Equal("string"))
			g.Expect(s["description"]).To(ContainSubstring("JSON object"))
		}},
		{"value_field", func(g Gomega, s map[string]any) {
			g.Expect(s["type"]).To(Equal("string"))
			g.Expect(s["description"]).To(ContainSubstring("JSON value"))
		}},
		{"list_value", func(g Gomega, s map[string]any) {
			g.Expect(s["type"]).To(Equal("string"))
			g.Expect(s["description"]).To(ContainSubstring("JSON array"))
		}},
		{"field_mask", func(g Gomega, s map[string]any) {
			g.Expect(s["type"]).To(Equal([]string{"string", "null"}))
		}},
		{"any", func(g Gomega, s map[string]any) {
			g.Expect(s["type"]).To(Equal("object"))
			g.Expect(s["additionalProperties"]).To(Equal(false))
			required := s["required"].([]string)
			g.Expect(required).To(ConsistOf("@type", "value"))
		}},
		{"bytes_value", func(g Gomega, s map[string]any) {
			g.Expect(s["type"]).To(Equal([]string{"string", "null"}))
			g.Expect(s).ToNot(HaveKey("format"))
		}},
	}

	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			g := NewWithT(t)
			fd := md.Fields().ByName(protoreflect.Name(tt.fieldName))
			g.Expect(fd).ToNot(BeNil(), "field %s not found", tt.fieldName)
			schema := FieldSchema(fd, opts)
			tt.check(g, schema)
		})
	}
}

func TestEnumFieldSchema(t *testing.T) {
	md := (&testdata.EnumFieldsRequest{}).ProtoReflect().Descriptor()

	t.Run("single_enum_standard", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("priority")
		schema := FieldSchema(fd, SchemaOptions{OpenAICompat: false})
		g.Expect(schema["type"]).To(Equal("string"))
		g.Expect(schema["enum"]).To(ConsistOf(
			"PRIORITY_UNSPECIFIED", "PRIORITY_LOW", "PRIORITY_MEDIUM", "PRIORITY_HIGH", "PRIORITY_CRITICAL",
		))
	})

	t.Run("single_enum_openai", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("priority")
		schema := FieldSchema(fd, SchemaOptions{OpenAICompat: true})
		g.Expect(schema["type"]).To(Equal("string"))
		g.Expect(schema["enum"]).To(HaveLen(5))
	})

	t.Run("repeated_enum", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("priorities")
		schema := FieldSchema(fd, SchemaOptions{OpenAICompat: false})
		g.Expect(schema["type"]).To(Equal("array"))
		items := schema["items"].(map[string]any)
		g.Expect(items["type"]).To(Equal("string"))
		g.Expect(items["enum"]).To(HaveLen(5))
	})
}

func TestMapFieldSchema_KeyTypes(t *testing.T) {
	md := (&testdata.MapVariantsRequest{}).ProtoReflect().Descriptor()

	t.Run("string_key_standard", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("string_to_string")
		schema := FieldSchema(fd, SchemaOptions{OpenAICompat: false})
		g.Expect(schema["type"]).To(Equal("object"))
		pn := schema["propertyNames"].(map[string]any)
		g.Expect(pn["type"]).To(Equal("string"))
		g.Expect(pn).ToNot(HaveKey("enum"))
		g.Expect(pn).ToNot(HaveKey("pattern"))
	})

	t.Run("bool_key_standard", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("bool_to_string")
		schema := FieldSchema(fd, SchemaOptions{OpenAICompat: false})
		pn := schema["propertyNames"].(map[string]any)
		g.Expect(pn["enum"]).To(ConsistOf("true", "false"))
	})

	t.Run("int_key_standard", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("int_to_string")
		schema := FieldSchema(fd, SchemaOptions{OpenAICompat: false})
		pn := schema["propertyNames"].(map[string]any)
		g.Expect(pn["pattern"]).To(Equal(`^-?(0|[1-9]\d*)$`))
	})

	t.Run("uint64_key_standard", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("uint64_to_string")
		schema := FieldSchema(fd, SchemaOptions{OpenAICompat: false})
		pn := schema["propertyNames"].(map[string]any)
		g.Expect(pn["pattern"]).To(Equal(`^(0|[1-9]\d*)$`))
	})

	t.Run("string_to_message_standard", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("string_to_message")
		schema := FieldSchema(fd, SchemaOptions{OpenAICompat: false})
		g.Expect(schema["type"]).To(Equal("object"))
		ap := schema["additionalProperties"].(map[string]any)
		g.Expect(ap["type"]).To(Equal("object"))
		g.Expect(ap).To(HaveKey("properties"))
	})

	t.Run("string_to_double_standard", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("string_to_double")
		schema := FieldSchema(fd, SchemaOptions{OpenAICompat: false})
		ap := schema["additionalProperties"].(map[string]any)
		g.Expect(ap["type"]).To(Equal("number"))
	})

	t.Run("string_to_bool_standard", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("string_to_bool")
		schema := FieldSchema(fd, SchemaOptions{OpenAICompat: false})
		ap := schema["additionalProperties"].(map[string]any)
		g.Expect(ap["type"]).To(Equal("boolean"))
	})

	t.Run("string_key_openai", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("string_to_string")
		schema := FieldSchema(fd, SchemaOptions{OpenAICompat: true})
		g.Expect(schema["type"]).To(Equal("array"))
		g.Expect(schema["description"]).To(Equal("List of key value pairs"))
		items := schema["items"].(map[string]any)
		g.Expect(items["type"]).To(Equal("object"))
		g.Expect(items["additionalProperties"]).To(Equal(false))
		props := items["properties"].(map[string]any)
		g.Expect(props).To(HaveKey("key"))
		g.Expect(props).To(HaveKey("value"))
	})

	t.Run("bool_key_openai", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("bool_to_string")
		schema := FieldSchema(fd, SchemaOptions{OpenAICompat: true})
		g.Expect(schema["type"]).To(Equal("array"))
	})
}

func TestBytesField(t *testing.T) {
	md := (&testdata.AllScalarTypesRequest{}).ProtoReflect().Descriptor()
	fd := md.Fields().ByName("bytes_field")

	t.Run("standard", func(t *testing.T) {
		g := NewWithT(t)
		schema := FieldSchema(fd, SchemaOptions{OpenAICompat: false})
		g.Expect(schema["type"]).To(Equal("string"))
		g.Expect(schema["contentEncoding"]).To(Equal("base64"))
		g.Expect(schema["format"]).To(Equal("byte"))
	})

	t.Run("openai", func(t *testing.T) {
		g := NewWithT(t)
		schema := FieldSchema(fd, SchemaOptions{OpenAICompat: true})
		g.Expect(schema["type"]).To(Equal("string"))
		g.Expect(schema["contentEncoding"]).To(Equal("base64"))
		g.Expect(schema).ToNot(HaveKey("format"))
	})
}

func TestExtractValidateConstraints_NumericTypes(t *testing.T) {
	md := (&testdata.NumericValidationRequest{}).ProtoReflect().Descriptor()

	tests := []struct {
		fieldName string
		check     func(g Gomega, c map[string]any)
	}{
		{"age", func(g Gomega, c map[string]any) {
			// int32 gte=0, lte=150
			g.Expect(c["minimum"]).To(Equal(0))
			g.Expect(c["maximum"]).To(Equal(150))
		}},
		{"score", func(g Gomega, c map[string]any) {
			// int32 gt=0, lt=100
			g.Expect(c["minimum"]).To(Equal(1))
			g.Expect(c["maximum"]).To(Equal(99))
		}},
		{"count", func(g Gomega, c map[string]any) {
			// uint32 gte=1, lte=1000
			g.Expect(c["minimum"]).To(Equal(1))
			g.Expect(c["maximum"]).To(Equal(1000))
		}},
		{"big_count", func(g Gomega, c map[string]any) {
			// uint64 gt=0
			g.Expect(c["minimum"]).To(Equal(1))
			g.Expect(c).ToNot(HaveKey("maximum"))
		}},
		{"percentage", func(g Gomega, c map[string]any) {
			// float gte=0.0, lte=100.0
			g.Expect(c["minimum"]).To(BeNumerically("~", 0.0))
			g.Expect(c["maximum"]).To(BeNumerically("~", 100.0))
		}},
		{"temperature", func(g Gomega, c map[string]any) {
			// double gt=-273.15, lt=1000000.0
			g.Expect(c["exclusiveMinimum"]).To(BeNumerically("~", -273.15))
			g.Expect(c["exclusiveMaximum"]).To(BeNumerically("~", 1000000.0))
		}},
		{"timestamp_nanos", func(g Gomega, c map[string]any) {
			// int64 gte=0
			g.Expect(c["minimum"]).To(Equal(0))
			g.Expect(c).ToNot(HaveKey("maximum"))
		}},
		{"code", func(g Gomega, c map[string]any) {
			// string min_len=2, max_len=10, pattern
			g.Expect(c["minLength"]).To(Equal(2))
			g.Expect(c["maxLength"]).To(Equal(10))
			g.Expect(c["pattern"]).To(Equal("^[A-Z0-9]+$"))
		}},
	}

	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			g := NewWithT(t)
			fd := md.Fields().ByName(protoreflect.Name(tt.fieldName))
			g.Expect(fd).ToNot(BeNil(), "field %s not found", tt.fieldName)
			constraints := ExtractValidateConstraints(fd)
			tt.check(g, constraints)
		})
	}
}

func TestExtractValidateConstraints_NoConstraints(t *testing.T) {
	g := NewWithT(t)
	// AllScalarTypesRequest fields have no validation constraints
	md := (&testdata.AllScalarTypesRequest{}).ProtoReflect().Descriptor()
	fd := md.Fields().ByName("double_field")
	constraints := ExtractValidateConstraints(fd)
	g.Expect(constraints).To(BeEmpty())
}

func TestToolForMethod_LongNameMangling(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.DeepNestingRequest{}).ProtoReflect().Descriptor().ParentFile()
	svc := file.Services().ByName("EdgeCaseService")
	g.Expect(svc).ToNot(BeNil())

	method := svc.Methods().ByName("DeepNesting")
	g.Expect(method).ToNot(BeNil())

	standard, openAI := ToolForMethod(method, "Test deep nesting")

	g.Expect(standard.Name).To(Equal("testdata_EdgeCaseService_DeepNesting"))
	g.Expect(len(standard.Name)).To(BeNumerically("<=", 64))
	g.Expect(openAI.Name).To(Equal(standard.Name))

	// Verify OpenAI top-level type is plain "object", not ["object","null"]
	var oaiSchema map[string]any
	err := json.Unmarshal(openAI.RawInputSchema, &oaiSchema)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(oaiSchema["type"]).To(Equal("object"))
	g.Expect(oaiSchema["additionalProperties"]).To(Equal(false))

	// Test MangleHeadIfTooLong with a name that actually exceeds 64 chars
	longName := strings.Repeat("a", 100)
	mangled := MangleHeadIfTooLong(longName, 64)
	g.Expect(len(mangled)).To(BeNumerically("<=", 64))
	g.Expect(len(mangled)).To(BeNumerically(">", 0))
	g.Expect(MangleHeadIfTooLong(longName, 64)).To(Equal(mangled))

	// Edge cases for MangleHeadIfTooLong
	g.Expect(MangleHeadIfTooLong("short", 64)).To(Equal("short"))
	g.Expect(MangleHeadIfTooLong("anything", 0)).To(Equal(""))
}

func TestMultipleOneofs_Standard(t *testing.T) {
	g := NewWithT(t)
	md := (&testdata.MultipleOneofsRequest{}).ProtoReflect().Descriptor()
	schema := MessageSchema(md, SchemaOptions{OpenAICompat: false})

	// Should have anyOf for the two oneof groups
	g.Expect(schema).To(HaveKey("anyOf"))
	anyOf := schema["anyOf"].([]map[string]any)
	g.Expect(anyOf).To(HaveLen(2))

	// "name" should be a normal required field
	required := schema["required"].([]string)
	g.Expect(required).To(ContainElement("name"))
}

func TestMultipleOneofs_OpenAI(t *testing.T) {
	g := NewWithT(t)
	md := (&testdata.MultipleOneofsRequest{}).ProtoReflect().Descriptor()
	schema := MessageSchema(md, SchemaOptions{OpenAICompat: true})

	// OpenAI mode flattens oneofs into nullable fields
	g.Expect(schema).ToNot(HaveKey("anyOf"))
	g.Expect(schema["additionalProperties"]).To(Equal(false))

	props := schema["properties"].(map[string]any)
	urlSchema := props["url"].(map[string]any)
	g.Expect(urlSchema["type"]).To(Equal([]string{"string", "null"}))
	g.Expect(urlSchema["description"]).To(ContainSubstring("oneof"))

	// All fields should be required in OpenAI mode
	required := schema["required"].([]string)
	g.Expect(required).To(ContainElement("name"))
	g.Expect(required).To(ContainElement("url"))
	g.Expect(required).To(ContainElement("raw_data"))
	g.Expect(required).To(ContainElement("as_json"))
}

func TestMessageSchema_NestedMessages(t *testing.T) {
	md := (&testdata.DeepNestingRequest{}).ProtoReflect().Descriptor()

	for _, openAI := range []bool{false, true} {
		name := "standard"
		if openAI {
			name = "openai"
		}
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			schema := MessageSchema(md, SchemaOptions{OpenAICompat: openAI})
			g.Expect(schema["type"]).ToNot(BeNil())
			props := schema["properties"].(map[string]any)
			g.Expect(props).To(HaveKey("middle"))

			middleSchema := props["middle"].(map[string]any)
			middleProps := middleSchema["properties"].(map[string]any)
			g.Expect(middleProps).To(HaveKey("inner"))
			g.Expect(middleProps).To(HaveKey("items"))
			g.Expect(middleProps).To(HaveKey("named_items"))
		})
	}
}

func TestFieldSchema_AllKinds_OpenAI(t *testing.T) {
	msg := (&testdata.AllScalarTypesRequest{}).ProtoReflect().Descriptor()
	opts := SchemaOptions{OpenAICompat: true}

	tests := []struct {
		name     string
		expected string
	}{
		{"sint32_field", "integer"},
		{"sint64_field", "string"},
		{"fixed32_field", "integer"},
		{"fixed64_field", "string"},
		{"sfixed32_field", "integer"},
		{"sfixed64_field", "string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			fd := msg.Fields().ByName(protoreflect.Name(tt.name))
			schema := FieldSchema(fd, opts)
			g.Expect(schema["type"]).To(Equal(tt.expected))
		})
	}
}

func TestToolForMethod_WellKnownTypesRPC(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	svc := file.Services().ByName("TestService")
	method := svc.Methods().ByName("ProcessWellKnownTypes")

	standard, openAI := ToolForMethod(method, "Process well-known types")

	// Verify standard schema parses and has the WKT fields
	var stdSchema map[string]any
	err := json.Unmarshal(standard.RawInputSchema, &stdSchema)
	g.Expect(err).ToNot(HaveOccurred())
	props := stdSchema["properties"].(map[string]any)
	g.Expect(props).To(HaveKey("metadata"))
	g.Expect(props).To(HaveKey("config"))
	g.Expect(props).To(HaveKey("payload"))
	g.Expect(props).To(HaveKey("timestamp"))

	// Verify OpenAI schema parses
	var oaiSchema map[string]any
	err = json.Unmarshal(openAI.RawInputSchema, &oaiSchema)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(oaiSchema["additionalProperties"]).To(Equal(false))
}

// validateOpenAISchema recursively validates that a JSON schema conforms to
// OpenAI's strict structured output requirements. It reports the JSON path
// of any violation.
func validateOpenAISchema(t *testing.T, schema map[string]any, path string) {
	t.Helper()

	// Rule: no anyOf, oneOf, allOf at this level
	for _, keyword := range []string{"anyOf", "oneOf", "allOf"} {
		if _, ok := schema[keyword]; ok {
			t.Errorf("%s: found disallowed keyword %q", path, keyword)
		}
	}

	// Rule: no "format": "byte" anywhere (OpenAI rejects this)
	if fmt, ok := schema["format"]; ok {
		if fmt == "byte" {
			t.Errorf("%s: found disallowed format \"byte\"", path)
		}
	}

	// If this schema has type "object" (either plain string or array containing "object"),
	// it must conform to additional rules.
	isObject := false
	switch typ := schema["type"].(type) {
	case string:
		isObject = typ == "object"
	case []string:
		for _, v := range typ {
			if v == "object" {
				isObject = true
				break
			}
		}
	case []any:
		for _, v := range typ {
			if s, ok := v.(string); ok && s == "object" {
				isObject = true
				break
			}
		}
	}

	if isObject {
		// Rule: must have additionalProperties: false
		ap, hasAP := schema["additionalProperties"]
		if !hasAP || ap != false {
			t.Errorf("%s: object schema missing additionalProperties: false (got %v)", path, ap)
		}

		// Rule: all property names must be in required
		if props, ok := schema["properties"].(map[string]any); ok {
			required := map[string]bool{}
			if reqList, ok := schema["required"].([]string); ok {
				for _, r := range reqList {
					required[r] = true
				}
			}
			for name := range props {
				if !required[name] {
					t.Errorf("%s: property %q not in required list", path, name)
				}
			}

			// Recurse into each property
			for name, propRaw := range props {
				if propSchema, ok := propRaw.(map[string]any); ok {
					validateOpenAISchema(t, propSchema, path+"."+name)
				}
			}
		}
	}

	// Recurse into "items" (for arrays)
	if items, ok := schema["items"].(map[string]any); ok {
		validateOpenAISchema(t, items, path+".items")
	}
}

func TestOpenAISchemaStrictValidation(t *testing.T) {
	messages := []struct {
		name string
		msg  protoreflect.MessageDescriptor
	}{
		{"CreateItemRequest", (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor()},
		{"GetItemRequest", (&testdata.GetItemRequest{}).ProtoReflect().Descriptor()},
		{"ProcessWellKnownTypesRequest", (&testdata.ProcessWellKnownTypesRequest{}).ProtoReflect().Descriptor()},
		{"TestValidationRequest", (&testdata.TestValidationRequest{}).ProtoReflect().Descriptor()},
		{"AllScalarTypesRequest", (&testdata.AllScalarTypesRequest{}).ProtoReflect().Descriptor()},
		{"DeepNestingRequest", (&testdata.DeepNestingRequest{}).ProtoReflect().Descriptor()},
		{"RepeatedMessagesRequest", (&testdata.RepeatedMessagesRequest{}).ProtoReflect().Descriptor()},
		{"MapVariantsRequest", (&testdata.MapVariantsRequest{}).ProtoReflect().Descriptor()},
		{"EnumFieldsRequest", (&testdata.EnumFieldsRequest{}).ProtoReflect().Descriptor()},
		{"MultipleOneofsRequest", (&testdata.MultipleOneofsRequest{}).ProtoReflect().Descriptor()},
		{"NumericValidationRequest", (&testdata.NumericValidationRequest{}).ProtoReflect().Descriptor()},
	}

	for _, msg := range messages {
		t.Run(msg.name, func(t *testing.T) {
			schema := MessageSchema(msg.msg, SchemaOptions{OpenAICompat: true})

			// Simulate what ToolForMethod does: fix top-level type to plain "object"
			schema["type"] = "object"

			// Root type must be plain "object", not array type
			if typ, ok := schema["type"].(string); !ok || typ != "object" {
				t.Errorf("root type must be \"object\", got %v", schema["type"])
			}

			validateOpenAISchema(t, schema, msg.name)
		})
	}
}

func TestRepeatedMessageFields(t *testing.T) {
	md := (&testdata.RepeatedMessagesRequest{}).ProtoReflect().Descriptor()

	t.Run("standard", func(t *testing.T) {
		g := NewWithT(t)
		schema := MessageSchema(md, SchemaOptions{OpenAICompat: false})
		props := schema["properties"].(map[string]any)

		// items is repeated ItemWithMap
		itemsSchema := props["items"].(map[string]any)
		g.Expect(itemsSchema["type"]).To(Equal("array"))
		items := itemsSchema["items"].(map[string]any)
		g.Expect(items["type"]).To(Equal("object"))

		// timestamps is repeated Timestamp
		tsSchema := props["timestamps"].(map[string]any)
		g.Expect(tsSchema["type"]).To(Equal("array"))
		tsItems := tsSchema["items"].(map[string]any)
		g.Expect(tsItems["type"]).To(Equal([]string{"string", "null"}))
		g.Expect(tsItems["format"]).To(Equal("date-time"))
	})

	t.Run("openai", func(t *testing.T) {
		g := NewWithT(t)
		schema := MessageSchema(md, SchemaOptions{OpenAICompat: true})
		props := schema["properties"].(map[string]any)

		itemsSchema := props["items"].(map[string]any)
		g.Expect(itemsSchema["type"]).To(Equal("array"))
	})
}

// allTestMessages returns the message descriptors used across schema tests.
func allTestMessages() []struct {
	name string
	md   protoreflect.MessageDescriptor
} {
	return []struct {
		name string
		md   protoreflect.MessageDescriptor
	}{
		{"CreateItemRequest", (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor()},
		{"GetItemRequest", (&testdata.GetItemRequest{}).ProtoReflect().Descriptor()},
		{"ProcessWellKnownTypesRequest", (&testdata.ProcessWellKnownTypesRequest{}).ProtoReflect().Descriptor()},
		{"TestValidationRequest", (&testdata.TestValidationRequest{}).ProtoReflect().Descriptor()},
		{"AllScalarTypesRequest", (&testdata.AllScalarTypesRequest{}).ProtoReflect().Descriptor()},
		{"DeepNestingRequest", (&testdata.DeepNestingRequest{}).ProtoReflect().Descriptor()},
		{"RepeatedMessagesRequest", (&testdata.RepeatedMessagesRequest{}).ProtoReflect().Descriptor()},
		{"MapVariantsRequest", (&testdata.MapVariantsRequest{}).ProtoReflect().Descriptor()},
		{"EnumFieldsRequest", (&testdata.EnumFieldsRequest{}).ProtoReflect().Descriptor()},
		{"MultipleOneofsRequest", (&testdata.MultipleOneofsRequest{}).ProtoReflect().Descriptor()},
		{"NumericValidationRequest", (&testdata.NumericValidationRequest{}).ProtoReflect().Descriptor()},
	}
}

// compileJSONSchema compiles a schema map using the jsonschema library.
// Returns an error if the schema is not a valid JSON Schema document.
func compileJSONSchema(schema map[string]any) (*jsonschema.Schema, error) {
	b, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("marshal schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", strings.NewReader(string(b))); err != nil {
		return nil, fmt.Errorf("add resource: %w", err)
	}
	compiled, err := compiler.Compile("schema.json")
	if err != nil {
		return nil, fmt.Errorf("compile: %w", err)
	}
	return compiled, nil
}

func TestGeneratedSchemasAreValidJSONSchema(t *testing.T) {
	for _, msg := range allTestMessages() {
		t.Run(msg.name, func(t *testing.T) {
			t.Run("standard", func(t *testing.T) {
				schema := MessageSchema(msg.md, SchemaOptions{OpenAICompat: false})
				_, err := compileJSONSchema(schema)
				if err != nil {
					t.Fatalf("standard schema for %s failed to compile: %v", msg.name, err)
				}
			})

			t.Run("openai", func(t *testing.T) {
				schema := MessageSchema(msg.md, SchemaOptions{OpenAICompat: true})
				// ToolForMethod sets top-level type to plain "object"
				schema["type"] = "object"
				_, err := compileJSONSchema(schema)
				if err != nil {
					t.Fatalf("openai schema for %s failed to compile: %v", msg.name, err)
				}
			})
		})
	}
}

func TestSchemaRoundTripAllMessages(t *testing.T) {
	type testCase struct {
		name string
		md   protoreflect.MessageDescriptor
		msg  proto.Message
	}

	metadata, _ := structpb.NewStruct(map[string]any{"key": "value"})
	configVal, _ := structpb.NewValue(map[string]any{"setting": true})
	anyPayload, _ := anypb.New(wrapperspb.String("test-payload"))

	cases := []testCase{
		{
			name: "CreateItemRequest",
			md:   (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor(),
			msg: &testdata.CreateItemRequest{
				Name:        "test-item",
				Description: proto.String("a description"),
				Labels:      map[string]string{"env": "prod"},
				Tags:        []string{"go", "proto"},
				ItemType:    &testdata.CreateItemRequest_Product{Product: &testdata.ProductDetails{Price: 9.99, Quantity: 5}},
				Thumbnail:   []byte("thumb"),
			},
		},
		{
			name: "GetItemRequest",
			md:   (&testdata.GetItemRequest{}).ProtoReflect().Descriptor(),
			msg:  &testdata.GetItemRequest{Id: "item-123"},
		},
		{
			name: "ProcessWellKnownTypesRequest",
			md:   (&testdata.ProcessWellKnownTypesRequest{}).ProtoReflect().Descriptor(),
			msg: &testdata.ProcessWellKnownTypesRequest{
				Metadata:  metadata,
				Config:    configVal,
				Payload:   anyPayload,
				Timestamp: timestamppb.Now(),
			},
		},
		{
			name: "TestValidationRequest",
			md:   (&testdata.TestValidationRequest{}).ProtoReflect().Descriptor(),
			msg: &testdata.TestValidationRequest{
				ResourceGroupId: "550e8400-e29b-41d4-a716-446655440000",
				Email:           "test@example.com",
				Username:        "testuser",
				Name:            "Test User",
				Age:             30,
				Timestamp:       1000,
			},
		},
		{
			name: "AllScalarTypesRequest",
			md:   (&testdata.AllScalarTypesRequest{}).ProtoReflect().Descriptor(),
			msg: &testdata.AllScalarTypesRequest{
				DoubleField:   1.5,
				FloatField:    2.5,
				Int32Field:    42,
				Int64Field:    1000,
				Uint32Field:   100,
				Uint64Field:   200,
				Sint32Field:   -10,
				Sint64Field:   -20,
				Fixed32Field:  300,
				Fixed64Field:  400,
				Sfixed32Field: -30,
				Sfixed64Field: -40,
				BoolField:     true,
				StringField:   "hello",
				BytesField:    []byte("world"),
			},
		},
		{
			name: "DeepNestingRequest",
			md:   (&testdata.DeepNestingRequest{}).ProtoReflect().Descriptor(),
			msg: &testdata.DeepNestingRequest{
				Middle: &testdata.MiddleMessage{
					Inner: &testdata.InnerMessage{
						Id:            "inner-1",
						Tags:          map[string]string{"a": "b"},
						Metadata:      &structpb.Struct{Fields: map[string]*structpb.Value{"k": structpb.NewStringValue("v")}},
						DynamicConfig: structpb.NewStringValue("cfg"),
					},
				},
			},
		},
		{
			name: "RepeatedMessagesRequest",
			md:   (&testdata.RepeatedMessagesRequest{}).ProtoReflect().Descriptor(),
			msg: &testdata.RepeatedMessagesRequest{
				Items: []*testdata.ItemWithMap{
					{
						Name:   "item1",
						Labels: map[string]string{"k": "v"},
						Config: structpb.NewStringValue("c"),
						Extra:  &structpb.Struct{Fields: map[string]*structpb.Value{}},
					},
				},
				Timestamps: []*timestamppb.Timestamp{timestamppb.Now()},
			},
		},
		{
			name: "MapVariantsRequest",
			md:   (&testdata.MapVariantsRequest{}).ProtoReflect().Descriptor(),
			msg: &testdata.MapVariantsRequest{
				StringToString: map[string]string{"a": "b"},
				IntToString:    map[int32]string{1: "one"},
				BoolToString:   map[bool]string{true: "yes"},
				Uint64ToString: map[uint64]string{99: "nn"},
				StringToDouble: map[string]float64{"pi": 3.14},
				StringToBool:   map[string]bool{"on": true},
				StringToMessage: map[string]*testdata.InnerMessage{
					"x": {
						Id:            "nested",
						Tags:          map[string]string{"t": "v"},
						Metadata:      &structpb.Struct{Fields: map[string]*structpb.Value{"m": structpb.NewBoolValue(true)}},
						DynamicConfig: structpb.NewNumberValue(42),
					},
				},
			},
		},
		{
			name: "EnumFieldsRequest",
			md:   (&testdata.EnumFieldsRequest{}).ProtoReflect().Descriptor(),
			msg: &testdata.EnumFieldsRequest{
				Priority:   testdata.Priority_PRIORITY_HIGH,
				Priorities: []testdata.Priority{testdata.Priority_PRIORITY_LOW, testdata.Priority_PRIORITY_MEDIUM},
			},
		},
		{
			name: "MultipleOneofsRequest",
			md:   (&testdata.MultipleOneofsRequest{}).ProtoReflect().Descriptor(),
			msg: &testdata.MultipleOneofsRequest{
				Name:         "test",
				Source:       &testdata.MultipleOneofsRequest_Url{Url: "https://example.com"},
				OutputFormat: &testdata.MultipleOneofsRequest_AsJson{AsJson: true},
			},
		},
		{
			name: "NumericValidationRequest",
			md:   (&testdata.NumericValidationRequest{}).ProtoReflect().Descriptor(),
			msg: &testdata.NumericValidationRequest{
				Age:            25,
				Score:          50,
				Count:          100,
				BigCount:       999,
				Percentage:     42.5,
				Temperature:    20.0,
				TimestampNanos: 1000000,
				Code:           "ABC123",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Marshal proto to JSON
			jsonBytes, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(tc.msg)
			if err != nil {
				t.Fatalf("protojson.Marshal: %v", err)
			}

			// Validate against standard schema
			t.Run("standard", func(t *testing.T) {
				schema := MessageSchema(tc.md, SchemaOptions{OpenAICompat: false})
				compiled, err := compileJSONSchema(schema)
				if err != nil {
					t.Fatalf("compile standard schema: %v", err)
				}
				var doc any
				if err := json.Unmarshal(jsonBytes, &doc); err != nil {
					t.Fatalf("unmarshal JSON: %v", err)
				}
				if err := compiled.Validate(doc); err != nil {
					t.Errorf("standard schema validation failed for %s:\n  json: %s\n  error: %v", tc.name, string(jsonBytes), err)
				}
			})

			// Validate OpenAI round-trip
			t.Run("openai", func(t *testing.T) {
				schema := MessageSchema(tc.md, SchemaOptions{OpenAICompat: true})
				schema["type"] = "object"
				compiled, err := compileJSONSchema(schema)
				if err != nil {
					t.Fatalf("compile openai schema: %v", err)
				}

				// Transform JSON to OpenAI format by re-marshaling through a map
				// and then applying the FixOpenAI reverse transform to verify round-trip.
				var standardDoc map[string]any
				if err := json.Unmarshal(jsonBytes, &standardDoc); err != nil {
					t.Fatalf("unmarshal JSON: %v", err)
				}

				// Convert standard JSON to OpenAI format:
				// maps -> arrays of KV pairs, WKTs -> strings
				openAIDoc := toOpenAIFormat(tc.md, standardDoc)

				if err := compiled.Validate(openAIDoc); err != nil {
					openAIBytes, _ := json.MarshalIndent(openAIDoc, "", "  ")
					t.Errorf("openai schema validation failed for %s:\n  json: %s\n  error: %v", tc.name, string(openAIBytes), err)
				}

				// Apply FixOpenAI to convert back to standard format,
				// then verify it can be unmarshaled back to proto
				runtime.FixOpenAI(tc.md, openAIDoc)
				fixedBytes, err := json.Marshal(openAIDoc)
				if err != nil {
					t.Fatalf("marshal fixed doc: %v", err)
				}
				roundTripped := tc.msg.ProtoReflect().New().Interface()
				if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(fixedBytes, roundTripped); err != nil {
					t.Errorf("unmarshal after FixOpenAI failed for %s:\n  json: %s\n  error: %v", tc.name, string(fixedBytes), err)
				}
			})
		})
	}
}

// toOpenAIFormat converts a standard protojson document to OpenAI format.
// Maps become arrays of {key, value} pairs. WKTs (Struct, Value, ListValue)
// become JSON strings. Missing fields are set to null (OpenAI requires all
// fields to be present).
func toOpenAIFormat(md protoreflect.MessageDescriptor, doc map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range doc {
		result[k] = v
	}

	for i := 0; i < md.Fields().Len(); i++ {
		field := md.Fields().Get(i)
		name := string(field.Name())
		jsonName := field.JSONName()

		// Resolve key used in the document
		key := ""
		if _, ok := result[name]; ok {
			key = name
		} else if _, ok := result[jsonName]; ok {
			key = jsonName
		}

		// For OpenAI mode, missing fields must be present.
		// Use appropriate zero values based on field type.
		if key == "" {
			if field.IsMap() {
				// Maps are arrays of KV pairs in OpenAI mode
				result[name] = []any{}
				continue
			}
			if field.IsList() {
				result[name] = []any{}
				continue
			}
			if field.Kind() == protoreflect.MessageKind {
				fullName := string(field.Message().FullName())
				switch fullName {
				case "google.protobuf.Struct":
					result[name] = "{}"
					continue
				case "google.protobuf.Value":
					result[name] = "null"
					continue
				case "google.protobuf.ListValue":
					result[name] = "[]"
					continue
				case "google.protobuf.Any":
					result[name] = map[string]any{"@type": "", "value": nil}
					continue
				}
			}
			result[name] = nil
			continue
		}

		if field.IsMap() {
			// Convert map object to array of KV pairs
			if m, ok := result[key].(map[string]any); ok {
				var arr []any
				for mk, mv := range m {
					// Recurse into message values
					if field.MapValue().Kind() == protoreflect.MessageKind {
						if nested, ok := mv.(map[string]any); ok {
							mv = toOpenAIFormat(field.MapValue().Message(), nested)
						}
					}
					arr = append(arr, map[string]any{"key": mk, "value": mv})
				}
				result[key] = arr
			}
		} else if field.Kind() == protoreflect.MessageKind {
			fullName := string(field.Message().FullName())

			if field.IsList() {
				if arr, ok := result[key].([]any); ok {
					for i, elem := range arr {
						if nested, ok := elem.(map[string]any); ok {
							arr[i] = toOpenAIFormat(field.Message(), nested)
						}
					}
				}
				continue
			}

			switch fullName {
			case "google.protobuf.Struct", "google.protobuf.Value", "google.protobuf.ListValue":
				b, _ := json.Marshal(result[key])
				result[key] = string(b)
			case "google.protobuf.Any":
				// Any is serialized specially by protojson with @type + content.
				// Don't recurse into it.
			default:
				if nested, ok := result[key].(map[string]any); ok {
					result[key] = toOpenAIFormat(field.Message(), nested)
				}
			}
		}
	}
	return result
}
