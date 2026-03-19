package gen

import (
	"testing"

	. "github.com/onsi/gomega"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// TestMapSchema_Bug_OpenAIDropsKeyConstraints demonstrates that the OpenAI
// map schema silently drops all key-type constraints that the standard schema
// correctly enforces. The mapFieldSchema function computes keyConstraints
// (enum for bool, pattern for int/uint) but the OpenAI branch ignores them
// entirely, hardcoding {"type": "string"} for the key field.
//
// This means an LLM using the OpenAI schema can send arbitrary strings as
// map keys for map<bool, ...> or map<int32, ...> fields, with no schema-level
// indication that only "true"/"false" or numeric strings are valid.
func TestMapSchema_Bug_OpenAIDropsKeyConstraints(t *testing.T) {
	md := (&testdata.MapVariantsRequest{}).ProtoReflect().Descriptor()

	t.Run("bool_key_constraints_lost_in_openai_mode", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("bool_to_string")

		// Standard mode correctly constrains bool map keys to "true"/"false".
		stdSchema := FieldSchema(fd, SchemaOptions{OpenAICompat: false})
		stdKeyConstraints := stdSchema["propertyNames"].(map[string]any)
		g.Expect(stdKeyConstraints).To(HaveKey("enum"),
			"standard schema has enum constraint on bool map keys")
		g.Expect(stdKeyConstraints["enum"]).To(ConsistOf("true", "false"))

		// OpenAI mode should carry over the same constraint to the "key" field.
		// BUG: It does not. The key field is just {"type": "string"} with no
		// enum restriction, so the LLM can send "yes", "1", "maybe", etc.
		oaiSchema := FieldSchema(fd, SchemaOptions{OpenAICompat: true})
		items := oaiSchema["items"].(map[string]any)
		props := items["properties"].(map[string]any)
		keySchema := props["key"].(map[string]any)

		// This assertion demonstrates the bug: the key schema has NO enum constraint.
		g.Expect(keySchema).To(HaveKey("enum"),
			"BUG: OpenAI map schema for map<bool, string> drops the enum constraint on keys; "+
				"LLM can send any string instead of only \"true\"/\"false\"")
	})

	t.Run("int32_key_constraints_lost_in_openai_mode", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("int_to_string")

		// Standard mode constrains int map keys with a regex pattern.
		stdSchema := FieldSchema(fd, SchemaOptions{OpenAICompat: false})
		stdKeyConstraints := stdSchema["propertyNames"].(map[string]any)
		g.Expect(stdKeyConstraints).To(HaveKey("pattern"),
			"standard schema has pattern constraint on int32 map keys")
		g.Expect(stdKeyConstraints["pattern"]).To(Equal(`^-?(0|[1-9]\d*)$`))

		// OpenAI mode should carry over the same pattern to the "key" field.
		// BUG: It does not.
		oaiSchema := FieldSchema(fd, SchemaOptions{OpenAICompat: true})
		items := oaiSchema["items"].(map[string]any)
		props := items["properties"].(map[string]any)
		keySchema := props["key"].(map[string]any)

		g.Expect(keySchema).To(HaveKey("pattern"),
			"BUG: OpenAI map schema for map<int32, string> drops the pattern constraint on keys; "+
				"LLM can send \"hello\" as a key instead of only integer strings")
	})

	t.Run("uint64_key_constraints_lost_in_openai_mode", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("uint64_to_string")

		// Standard mode constrains unsigned int map keys.
		stdSchema := FieldSchema(fd, SchemaOptions{OpenAICompat: false})
		stdKeyConstraints := stdSchema["propertyNames"].(map[string]any)
		g.Expect(stdKeyConstraints).To(HaveKey("pattern"))
		g.Expect(stdKeyConstraints["pattern"]).To(Equal(`^(0|[1-9]\d*)$`))

		// OpenAI mode drops this.
		oaiSchema := FieldSchema(fd, SchemaOptions{OpenAICompat: true})
		items := oaiSchema["items"].(map[string]any)
		props := items["properties"].(map[string]any)
		keySchema := props["key"].(map[string]any)

		g.Expect(keySchema).To(HaveKey("pattern"),
			"BUG: OpenAI map schema for map<uint64, string> drops the pattern constraint on keys; "+
				"LLM can send negative numbers or non-numeric strings")
	})

	// Verify that even string keys work, as a sanity check that the test
	// structure itself is correct (string keys have no extra constraints).
	t.Run("string_key_no_extra_constraints_sanity_check", func(t *testing.T) {
		g := NewWithT(t)
		fd := md.Fields().ByName("string_to_string")

		stdSchema := FieldSchema(fd, SchemaOptions{OpenAICompat: false})
		stdKeyConstraints := stdSchema["propertyNames"].(map[string]any)
		g.Expect(stdKeyConstraints).ToNot(HaveKey("enum"))
		g.Expect(stdKeyConstraints).ToNot(HaveKey("pattern"))

		oaiSchema := FieldSchema(fd, SchemaOptions{OpenAICompat: true})
		items := oaiSchema["items"].(map[string]any)
		props := items["properties"].(map[string]any)
		keySchema := props["key"].(map[string]any)
		// String keys should have no extra constraints in either mode -- this passes.
		g.Expect(keySchema).ToNot(HaveKey("enum"))
		g.Expect(keySchema).ToNot(HaveKey("pattern"))
	})
}

// TestMapSchema_Bug_KeyConstraintParity is a data-driven test that checks
// every non-string map key type for constraint parity between standard and
// OpenAI modes. For each map field with a non-string key, the standard schema
// adds constraints (enum or pattern) to propertyNames. The OpenAI schema must
// carry equivalent constraints to the "key" field in the KV-pair items schema.
func TestMapSchema_Bug_KeyConstraintParity(t *testing.T) {
	md := (&testdata.MapVariantsRequest{}).ProtoReflect().Descriptor()

	// Fields with non-string keys and their expected constraint key
	tests := []struct {
		fieldName      string
		constraintKey  string // "enum" or "pattern"
		constraintDesc string
	}{
		{"bool_to_string", "enum", "bool keys must be constrained to true/false"},
		{"int_to_string", "pattern", "int32 keys must match integer pattern"},
		{"uint64_to_string", "pattern", "uint64 keys must match unsigned integer pattern"},
	}

	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			g := NewWithT(t)
			fd := md.Fields().ByName(protoreflect.Name(tt.fieldName))
			g.Expect(fd).ToNot(BeNil())

			stdSchema := FieldSchema(fd, SchemaOptions{OpenAICompat: false})
			stdConstraint := stdSchema["propertyNames"].(map[string]any)[tt.constraintKey]
			g.Expect(stdConstraint).ToNot(BeNil(),
				"precondition: standard schema has %s constraint", tt.constraintKey)

			oaiSchema := FieldSchema(fd, SchemaOptions{OpenAICompat: true})
			keySchema := oaiSchema["items"].(map[string]any)["properties"].(map[string]any)["key"].(map[string]any)

			// This is the bug: the OpenAI key schema is missing the constraint
			// that the standard schema correctly provides.
			g.Expect(keySchema).To(HaveKey(tt.constraintKey),
				"BUG: OpenAI map key schema is missing %s constraint -- %s",
				tt.constraintKey, tt.constraintDesc)

			// If the constraint were present, it should match the standard value.
			if oaiConstraint, ok := keySchema[tt.constraintKey]; ok {
				g.Expect(oaiConstraint).To(Equal(stdConstraint),
					"constraint values should match between standard and OpenAI modes")
			}
		})
	}
}
