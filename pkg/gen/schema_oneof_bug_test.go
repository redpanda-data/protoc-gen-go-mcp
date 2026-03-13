package gen

import (
	"testing"

	. "github.com/onsi/gomega"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// TestOneofBug_MessageFieldInOpenAIMode tests the oneof handling in
// MessageSchema when a oneof contains message-typed fields and OpenAI
// mode is enabled.
//
// The bug: In OpenAI mode, oneof fields get flattened into normal
// properties with nullable types. The null-wrapping code (line 69) does:
//
//	if v, ok := schema["type"].(string); ok {
//	    schema["type"] = []string{v, "null"}
//	}
//
// For message fields, FieldSchema -> messageFieldSchema -> MessageSchema
// recursively returns a schema where "type" is already []string{"object", "null"}
// (applied by MessageSchema's own OpenAI postprocessing at lines 110-112).
// The type assertion .(string) silently fails because the type is []string,
// so the null-wrapping is skipped. The result is accidentally correct because
// the recursive call already added null -- but the oneof description annotation
// is what actually reveals the divergence.
func TestOneofBug_MessageFieldInOpenAIMode(t *testing.T) {
	g := NewWithT(t)

	// CreateItemRequest has a oneof "item_type" with two message fields:
	// ProductDetails and ServiceDetails.
	md := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor()
	schema := MessageSchema(md, SchemaOptions{OpenAICompat: true})

	props := schema["properties"].(map[string]any)
	g.Expect(props).To(HaveKey("product"))
	g.Expect(props).To(HaveKey("service"))

	productSchema := props["product"].(map[string]any)
	serviceSchema := props["service"].(map[string]any)

	// Verify that oneof message fields have nullable object type.
	productType := productSchema["type"].([]string)
	g.Expect(productType).To(ContainElement("null"))
	g.Expect(productType).To(ContainElement("object"))

	serviceType := serviceSchema["type"].([]string)
	g.Expect(serviceType).To(ContainElement("null"))
	g.Expect(serviceType).To(ContainElement("object"))

	// Verify the oneof description is applied.
	g.Expect(productSchema["description"]).To(ContainSubstring("oneof"))
	g.Expect(serviceSchema["description"]).To(ContainSubstring("oneof"))

	// Both should be required (OpenAI mode).
	required := schema["required"].([]string)
	g.Expect(required).To(ContainElement("product"))
	g.Expect(required).To(ContainElement("service"))
}

// TestOneofBug_MessageFieldNullAppliedByWrongLayer demonstrates the
// actual bug: the null type for oneof message fields is applied by
// the WRONG layer of code.
//
// In OpenAI mode, MessageSchema has two places that add null:
// 1. Lines 69-71: oneof-specific null wrapping (for oneof members)
// 2. Lines 110-112: general OpenAI null wrapping (for ALL messages)
//
// For oneof message fields, #1 silently fails (type assertion mismatch)
// and #2 provides the null. This means the oneof field's nested object
// schema is marked nullable because ALL objects are nullable in OpenAI
// mode, NOT because it's a oneof member.
//
// This is wrong: non-oneof message fields should NOT be nullable in
// OpenAI mode. Only oneof members need null because only one can be set.
// The fact that MessageSchema makes ALL objects nullable is itself a bug
// that masks the oneof null-wrapping bug.
func TestOneofBug_MessageFieldNullAppliedByWrongLayer(t *testing.T) {
	g := NewWithT(t)

	md := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor()
	schema := MessageSchema(md, SchemaOptions{OpenAICompat: true})

	props := schema["properties"].(map[string]any)

	// "name" is a regular string field -- its type should NOT be nullable.
	nameSchema := props["name"].(map[string]any)
	g.Expect(nameSchema["type"]).To(Equal("string"),
		"non-oneof scalar fields should not be nullable")

	// Now the interesting part: look at what happens when we generate
	// the schema for ProductDetails directly (as if it were a standalone
	// top-level message, not a field).
	productMd := md.Fields().ByName("product").Message()
	standaloneSchema := MessageSchema(productMd, SchemaOptions{OpenAICompat: true})

	// BUG: MessageSchema in OpenAI mode adds null to the top-level type
	// of EVERY message. This means even a top-level request message is
	// typed as ["object", "null"], which makes no sense -- you can't send
	// null as a top-level request.
	//
	// This test FAILS because standaloneSchema["type"] is ["object", "null"]
	// but it SHOULD be just "object" for a top-level message schema.
	g.Expect(standaloneSchema["type"]).To(Equal("object"),
		"top-level message schema should have type 'object', not nullable -- "+
			"only fields (especially oneof members) should be nullable. "+
			"Got: %v", standaloneSchema["type"])
}

// TestOneofBug_TypeAssertionSilentFailure directly demonstrates that
// the type assertion on line 69 fails for message fields in oneofs.
//
// We call FieldSchema for a oneof message field and verify that its
// type is []string (not string), proving the type assertion would fail.
func TestOneofBug_TypeAssertionSilentFailure(t *testing.T) {
	g := NewWithT(t)

	md := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor()
	productFd := md.Fields().ByName(protoreflect.Name("product"))

	// Get the schema that would be returned by FieldSchema for this
	// oneof message field in OpenAI mode.
	schema := FieldSchema(productFd, SchemaOptions{OpenAICompat: true})

	// The oneof code on line 69 does: schema["type"].(string)
	// For message fields, this will be []string, not string.
	_, isString := schema["type"].(string)

	// BUG: This assertion FAILS because FieldSchema returns a schema
	// with type []string{"object", "null"} for message fields in OpenAI
	// mode. The oneof null-wrapping code assumes type is always a string,
	// which is wrong for message fields.
	g.Expect(isString).To(BeTrue(),
		"FieldSchema for a oneof message field returns type as %T (%v), "+
			"but the oneof null-wrapping code on line 69 assumes it's a string. "+
			"This type assertion silently fails, and null is only present because "+
			"MessageSchema unconditionally adds it in OpenAI mode.",
		schema["type"], schema["type"])
}
