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

// TestMessageSchema_Standard verifies basic standard-mode schema properties.
func TestMessageSchema_Standard(t *testing.T) {
	g := NewWithT(t)
	md := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor()
	schema := MessageSchema(md, SchemaOptions{})

	g.Expect(schema["type"]).To(Equal("object"))
	g.Expect(schema).To(HaveKey("properties"))
	g.Expect(schema).ToNot(HaveKey("additionalProperties"))

	// Standard mode must NOT use top-level union keywords; oneofs are
	// rendered as discriminated wrapper objects.
	g.Expect(schema).ToNot(HaveKey("anyOf"))
	g.Expect(schema).ToNot(HaveKey("oneOf"))
	g.Expect(schema).ToNot(HaveKey("allOf"))
}

func TestFieldSchema_AllKinds(t *testing.T) {
	msg := (&testdata.AllScalarTypesRequest{}).ProtoReflect().Descriptor()
	opts := SchemaOptions{}

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

	tool := ToolForMethod(method, "Create a new item")

	g.Expect(tool.Name).To(Equal("testdata_TestService_CreateItem"))
	g.Expect(tool.Description).To(Equal("Create a new item"))

	// Standard schema should NOT have additionalProperties: false
	var stdSchema map[string]any
	err := json.Unmarshal(tool.RawInputSchema, &stdSchema)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(stdSchema).ToNot(HaveKey("additionalProperties"))

	// No top-level union keywords.
	g.Expect(stdSchema).ToNot(HaveKey("anyOf"))
	g.Expect(stdSchema).ToNot(HaveKey("oneOf"))
	g.Expect(stdSchema).ToNot(HaveKey("allOf"))

	// Tool must include an output schema derived from method.Output().
	g.Expect(tool.RawOutputSchema).ToNot(BeEmpty())

	var stdOut map[string]any
	g.Expect(json.Unmarshal(tool.RawOutputSchema, &stdOut)).To(Succeed())
	g.Expect(stdOut["type"]).To(Equal("object"))
	stdOutProps := stdOut["properties"].(map[string]any)
	g.Expect(stdOutProps).To(HaveKey("id"))
	g.Expect(stdOutProps).To(HaveKey("created_at"))
}

func TestNewToolResultJSON(t *testing.T) {
	g := NewWithT(t)

	payload := []byte(`{"id":"42","name":"thing"}`)
	result := runtime.NewToolResultJSON(payload)

	g.Expect(result.IsError).To(BeFalse())
	g.Expect(result.Text).To(Equal(string(payload)))
	g.Expect(result.StructuredContent).To(Equal(json.RawMessage(payload)))
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
	opts := SchemaOptions{}

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

func TestEnumFieldSchema(t *testing.T) {
	md := (&testdata.EnumFieldsRequest{}).ProtoReflect().Descriptor()

	t.Run("single_enum_standard", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("priority")
		schema := FieldSchema(fd, SchemaOptions{})
		g.Expect(schema["type"]).To(Equal("string"))
		g.Expect(schema["enum"]).To(ConsistOf(
			"PRIORITY_UNSPECIFIED", "PRIORITY_LOW", "PRIORITY_MEDIUM", "PRIORITY_HIGH", "PRIORITY_CRITICAL",
		))
	})

	t.Run("repeated_enum", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("priorities")
		schema := FieldSchema(fd, SchemaOptions{})
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
		schema := FieldSchema(fd, SchemaOptions{})
		g.Expect(schema["type"]).To(Equal("object"))
		pn := schema["propertyNames"].(map[string]any)
		g.Expect(pn["type"]).To(Equal("string"))
		g.Expect(pn).ToNot(HaveKey("enum"))
		g.Expect(pn).ToNot(HaveKey("pattern"))
	})

	t.Run("bool_key_standard", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("bool_to_string")
		schema := FieldSchema(fd, SchemaOptions{})
		pn := schema["propertyNames"].(map[string]any)
		g.Expect(pn["enum"]).To(ConsistOf("true", "false"))
	})

	t.Run("int_key_standard", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("int_to_string")
		schema := FieldSchema(fd, SchemaOptions{})
		pn := schema["propertyNames"].(map[string]any)
		g.Expect(pn["pattern"]).To(Equal(`^-?(0|[1-9]\d*)$`))
	})

	t.Run("uint64_key_standard", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("uint64_to_string")
		schema := FieldSchema(fd, SchemaOptions{})
		pn := schema["propertyNames"].(map[string]any)
		g.Expect(pn["pattern"]).To(Equal(`^(0|[1-9]\d*)$`))
	})

	t.Run("string_to_message_standard", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("string_to_message")
		schema := FieldSchema(fd, SchemaOptions{})
		g.Expect(schema["type"]).To(Equal("object"))
		ap := schema["additionalProperties"].(map[string]any)
		g.Expect(ap["type"]).To(Equal("object"))
		g.Expect(ap).To(HaveKey("properties"))
	})

	t.Run("string_to_double_standard", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("string_to_double")
		schema := FieldSchema(fd, SchemaOptions{})
		ap := schema["additionalProperties"].(map[string]any)
		g.Expect(ap["type"]).To(Equal("number"))
	})

	t.Run("string_to_bool_standard", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("string_to_bool")
		schema := FieldSchema(fd, SchemaOptions{})
		ap := schema["additionalProperties"].(map[string]any)
		g.Expect(ap["type"]).To(Equal("boolean"))
	})
}

func TestBytesField(t *testing.T) {
	md := (&testdata.AllScalarTypesRequest{}).ProtoReflect().Descriptor()
	fd := md.Fields().ByName("bytes_field")

	t.Run("standard", func(t *testing.T) {
		g := NewWithT(t)
		schema := FieldSchema(fd, SchemaOptions{})
		g.Expect(schema["type"]).To(Equal("string"))
		g.Expect(schema["contentEncoding"]).To(Equal("base64"))
		g.Expect(schema["format"]).To(Equal("byte"))
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

	tool := ToolForMethod(method, "Test deep nesting")

	g.Expect(tool.Name).To(Equal("testdata_EdgeCaseService_DeepNesting"))
	g.Expect(len(tool.Name)).To(BeNumerically("<=", 64))

	// No top-level union keywords in schema.
	var schema map[string]any
	err := json.Unmarshal(tool.RawInputSchema, &schema)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(schema["type"]).To(Equal("object"))
	g.Expect(schema).ToNot(HaveKey("anyOf"))
	g.Expect(schema).ToNot(HaveKey("oneOf"))
	g.Expect(schema).ToNot(HaveKey("allOf"))

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

// TestMultipleOneofs_Standard verifies the discriminated-object rendering for oneofs.
func TestMultipleOneofs_Standard(t *testing.T) {
	g := NewWithT(t)
	md := (&testdata.MultipleOneofsRequest{}).ProtoReflect().Descriptor()
	// Marshal/unmarshal to normalize orderedMap values to plain map[string]any.
	raw, err := json.Marshal(MessageSchema(md, SchemaOptions{}))
	g.Expect(err).ToNot(HaveOccurred())
	var schema map[string]any
	g.Expect(json.Unmarshal(raw, &schema)).To(Succeed())

	// No top-level union keywords.
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

	// "name" should be a normal required field
	g.Expect(schema["required"]).To(ContainElement("name"))
}

func TestMessageSchema_NestedMessages(t *testing.T) {
	md := (&testdata.DeepNestingRequest{}).ProtoReflect().Descriptor()

	t.Run("standard", func(t *testing.T) {
		g := NewWithT(t)
		// Marshal/unmarshal to normalize orderedMap values.
		raw, err := json.Marshal(MessageSchema(md, SchemaOptions{}))
		g.Expect(err).ToNot(HaveOccurred())
		var schema map[string]any
		g.Expect(json.Unmarshal(raw, &schema)).To(Succeed())
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

func TestToolForMethod_WellKnownTypesRPC(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	svc := file.Services().ByName("TestService")
	method := svc.Methods().ByName("ProcessWellKnownTypes")

	tool := ToolForMethod(method, "Process well-known types")

	// Verify standard schema parses and has the WKT fields
	var stdSchema map[string]any
	err := json.Unmarshal(tool.RawInputSchema, &stdSchema)
	g.Expect(err).ToNot(HaveOccurred())
	props := stdSchema["properties"].(map[string]any)
	g.Expect(props).To(HaveKey("metadata"))
	g.Expect(props).To(HaveKey("config"))
	g.Expect(props).To(HaveKey("payload"))
	g.Expect(props).To(HaveKey("timestamp"))
}

func TestRepeatedMessageFields(t *testing.T) {
	md := (&testdata.RepeatedMessagesRequest{}).ProtoReflect().Descriptor()

	t.Run("standard", func(t *testing.T) {
		g := NewWithT(t)
		schema := MessageSchema(md, SchemaOptions{})
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
			schema := MessageSchema(msg.md, SchemaOptions{})
			_, err := compileJSONSchema(schema)
			if err != nil {
				t.Fatalf("standard schema for %s failed to compile: %v", msg.name, err)
			}
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
			schema := MessageSchema(tc.md, SchemaOptions{})
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
	}
}
