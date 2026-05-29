package gen

import (
	"encoding/json"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// roundTripSchema marshals schema to JSON and back so that all internal orderedMap
// values become plain map[string]any, enabling direct type assertions in tests.
func roundTripSchema(schema map[string]any) map[string]any {
	b, err := json.Marshal(schema)
	if err != nil {
		panic(err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		panic(err)
	}
	return out
}

func TestMangleHeadIfTooLong_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		maxLen  int
		checkFn func(g Gomega, result string)
	}{
		{
			name:   "negative maxLen returns empty",
			input:  "anything",
			maxLen: -1,
			checkFn: func(g Gomega, result string) {
				g.Expect(result).To(Equal(""))
			},
		},
		{
			name:   "maxLen 1 returns single char of hash",
			input:  strings.Repeat("x", 100),
			maxLen: 1,
			checkFn: func(g Gomega, result string) {
				g.Expect(result).To(HaveLen(1))
			},
		},
		{
			name:   "maxLen 10 returns exactly hash prefix",
			input:  strings.Repeat("x", 100),
			maxLen: 10,
			checkFn: func(g Gomega, result string) {
				g.Expect(result).To(HaveLen(10))
			},
		},
		{
			name:   "maxLen 11 returns hash prefix only (no room for separator + tail)",
			input:  strings.Repeat("x", 100),
			maxLen: 11,
			checkFn: func(g Gomega, result string) {
				// maxLen=11: hashPrefix=10, available = 11-10-1 = 0, so returns just hashPrefix
				g.Expect(result).To(HaveLen(10))
			},
		},
		{
			name:   "maxLen 12 returns hash_X format",
			input:  strings.Repeat("x", 100),
			maxLen: 12,
			checkFn: func(g Gomega, result string) {
				g.Expect(result).To(HaveLen(12))
				g.Expect(result).To(ContainSubstring("_"))
			},
		},
		{
			name:   "exact length is not mangled",
			input:  strings.Repeat("a", 64),
			maxLen: 64,
			checkFn: func(g Gomega, result string) {
				g.Expect(result).To(Equal(strings.Repeat("a", 64)))
			},
		},
		{
			name:   "one over is mangled",
			input:  strings.Repeat("a", 65),
			maxLen: 64,
			checkFn: func(g Gomega, result string) {
				g.Expect(len(result)).To(BeNumerically("<=", 64))
				g.Expect(result).ToNot(Equal(strings.Repeat("a", 65)))
			},
		},
		{
			name:   "different inputs produce different mangles",
			input:  "first_long_name_" + strings.Repeat("a", 100),
			maxLen: 20,
			checkFn: func(g Gomega, result string) {
				other := MangleHeadIfTooLong("second_long_name_"+strings.Repeat("b", 100), 20)
				g.Expect(result).ToNot(Equal(other))
			},
		},
		{
			name:   "empty input returns empty",
			input:  "",
			maxLen: 64,
			checkFn: func(g Gomega, result string) {
				g.Expect(result).To(Equal(""))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			result := MangleHeadIfTooLong(tt.input, tt.maxLen)
			tt.checkFn(g, result)
		})
	}
}

func TestCleanComment_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"only whitespace", "   \n   ", "\n"},
		{"all lines stripped", "buf:lint:FOO\n@ignore-comment bar", ""},
		{"multiple buf:lint lines", "buf:lint:A\nbuf:lint:B\nKeep this", "Keep this"},
		{"prefix in middle of line is kept", "This is not buf:lint:FOO", "This is not buf:lint:FOO"},
		{"trailing newlines preserved", "Hello\n\n", "Hello\n\n"},
		{"mixed content and stripped", "buf:lint:X\nFirst line\n@ignore-comment Y\nSecond line", "First line\nSecond line"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(CleanComment(tt.input)).To(Equal(tt.expected))
		})
	}
}

func TestBase36String_Deterministic(t *testing.T) {
	g := NewWithT(t)

	// Same input always produces same output
	input := []byte("test input")
	g.Expect(Base36String(input)).To(Equal(Base36String(input)))

	// Different inputs produce different output
	g.Expect(Base36String([]byte("a"))).ToNot(Equal(Base36String([]byte("b"))))

	// Empty input
	g.Expect(Base36String([]byte{})).To(Equal("0"))

	// Single zero byte
	g.Expect(Base36String([]byte{0})).To(Equal("0"))
}

func TestKindToType_AllKinds(t *testing.T) {
	tests := []struct {
		kind     protoreflect.Kind
		expected string
	}{
		{protoreflect.BoolKind, "boolean"},
		{protoreflect.StringKind, "string"},
		{protoreflect.Int32Kind, "integer"},
		{protoreflect.Sint32Kind, "integer"},
		{protoreflect.Sfixed32Kind, "integer"},
		{protoreflect.Uint32Kind, "integer"},
		{protoreflect.Fixed32Kind, "integer"},
		{protoreflect.Int64Kind, "string"},
		{protoreflect.Sint64Kind, "string"},
		{protoreflect.Sfixed64Kind, "string"},
		{protoreflect.Uint64Kind, "string"},
		{protoreflect.Fixed64Kind, "string"},
		{protoreflect.FloatKind, "number"},
		{protoreflect.DoubleKind, "number"},
		{protoreflect.BytesKind, "string"},
		{protoreflect.EnumKind, "string"},
	}

	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(KindToType(tt.kind)).To(Equal(tt.expected))
		})
	}
}

func TestExtractValidateConstraints_UUIDAndEmail(t *testing.T) {
	md := (&testdata.TestValidationRequest{}).ProtoReflect().Descriptor()

	t.Run("uuid_format", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("resource_group_id")
		g.Expect(fd).ToNot(BeNil())
		constraints := ExtractValidateConstraints(fd)
		g.Expect(constraints["format"]).To(Equal("uuid"))
	})

	t.Run("email_format", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("email")
		g.Expect(fd).ToNot(BeNil())
		constraints := ExtractValidateConstraints(fd)
		g.Expect(constraints["format"]).To(Equal("email"))
	})

	t.Run("pattern", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("username")
		g.Expect(fd).ToNot(BeNil())
		constraints := ExtractValidateConstraints(fd)
		g.Expect(constraints["pattern"]).To(Equal("^[a-zA-Z][a-zA-Z0-9_]{2,19}$"))
	})

	t.Run("min_max_len", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("name")
		g.Expect(fd).ToNot(BeNil())
		constraints := ExtractValidateConstraints(fd)
		g.Expect(constraints["minLength"]).To(Equal(3))
		g.Expect(constraints["maxLength"]).To(Equal(50))
	})

	t.Run("int64_gt", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("timestamp")
		g.Expect(fd).ToNot(BeNil())
		constraints := ExtractValidateConstraints(fd)
		g.Expect(constraints["minimum"]).To(Equal(1)) // gt=0 -> minimum=1
	})
}

func TestMessageSchema_EmptyRequiredList(t *testing.T) {
	g := NewWithT(t)

	// GetItemRequest has a single field "id" with no REQUIRED annotation
	md := (&testdata.GetItemRequest{}).ProtoReflect().Descriptor()

	t.Run("standard_no_required", func(t *testing.T) {
		schema := MessageSchema(md, SchemaOptions{})
		required := schema["required"].([]string)
		// No fields are required in standard mode (no REQUIRED annotation)
		g.Expect(required).To(BeEmpty())
	})
}

func TestFieldSchema_RepeatedEnum(t *testing.T) {
	g := NewWithT(t)

	md := (&testdata.EnumFieldsRequest{}).ProtoReflect().Descriptor()
	fd := md.Fields().ByName("priorities")

	t.Run("standard", func(t *testing.T) {
		schema := FieldSchema(fd, SchemaOptions{})
		g.Expect(schema["type"]).To(Equal("array"))
		items := schema["items"].(map[string]any)
		g.Expect(items["type"]).To(Equal("string"))
		g.Expect(items["enum"]).To(ConsistOf(
			"PRIORITY_UNSPECIFIED", "PRIORITY_LOW", "PRIORITY_MEDIUM", "PRIORITY_HIGH", "PRIORITY_CRITICAL",
		))
	})
}

func TestFieldSchema_RepeatedMessage_HasArrayOfObjectSchemas(t *testing.T) {
	g := NewWithT(t)

	md := (&testdata.DeepNestingRequest{}).ProtoReflect().Descriptor()

	t.Run("repeated_middles_standard", func(t *testing.T) {
		fd := md.Fields().ByName("middles")
		schema := FieldSchema(fd, SchemaOptions{})
		g.Expect(schema["type"]).To(Equal("array"))
		items := schema["items"].(map[string]any)
		g.Expect(items["type"]).To(Equal("object"))
		g.Expect(items["properties"]).ToNot(BeNil())
	})
}

func TestFieldSchema_ConstraintsMergedIntoSchema(t *testing.T) {
	g := NewWithT(t)

	// Verify that validation constraints are merged into the field schema, not returned separately
	md := (&testdata.NumericValidationRequest{}).ProtoReflect().Descriptor()

	fd := md.Fields().ByName("age")
	schema := FieldSchema(fd, SchemaOptions{})
	g.Expect(schema["type"]).To(Equal("integer"))
	g.Expect(schema["minimum"]).To(Equal(0))
	g.Expect(schema["maximum"]).To(Equal(150))

	fd = md.Fields().ByName("temperature")
	schema = FieldSchema(fd, SchemaOptions{})
	g.Expect(schema["type"]).To(Equal("number"))
	g.Expect(schema["exclusiveMinimum"]).To(BeNumerically("~", -273.15))
	g.Expect(schema["exclusiveMaximum"]).To(BeNumerically("~", 1000000.0))
}

// TestMessageSchema_NoUnionKeywords asserts that no top-level anyOf/oneOf/allOf
// keywords appear in generated schemas — oneofs are rendered as discriminated
// wrapper objects instead.
func TestMessageSchema_NoUnionKeywords(t *testing.T) {
	g := NewWithT(t)

	md := (&testdata.MultipleOneofsRequest{}).ProtoReflect().Descriptor()
	// roundTripSchema converts internal orderedMap values to plain map[string]any.
	schema := roundTripSchema(MessageSchema(md, SchemaOptions{}))

	g.Expect(schema).ToNot(HaveKey("anyOf"))
	g.Expect(schema).ToNot(HaveKey("oneOf"))
	g.Expect(schema).ToNot(HaveKey("allOf"))

	// The two oneof groups should appear as discriminated wrapper objects
	// under properties keyed by the oneof name.
	props := schema["properties"].(map[string]any)
	g.Expect(props).To(HaveKey("source"))
	g.Expect(props).To(HaveKey("output_format"))

	sourceWrapper := props["source"].(map[string]any)
	g.Expect(sourceWrapper["type"]).To(Equal("object"))
	sourceProps := sourceWrapper["properties"].(map[string]any)
	g.Expect(sourceProps).To(HaveKey("which"))

	// Regular non-oneof field is still in properties.
	g.Expect(props).To(HaveKey("name"))
}
