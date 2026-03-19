package gen

import (
	"context"
	"encoding/json"
	"testing"

	mcpserver "github.com/mark3labs/mcp-go/server"
	. "github.com/onsi/gomega"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// TestExtraPropBug_NameCollisionWithProtoField demonstrates that when an extra
// property has the same name as a real proto field, the extra property value
// leaks into the proto message. The handler receives a proto message where a
// real field is populated with the extra property's value, because extra
// properties are never removed from the arguments map before proto
// unmarshaling.
//
// In practice this means:
//  1. The caller cannot distinguish the extra property from the proto field -
//     they always share the same value since they occupy the same map key.
//  2. An extra property intended only for context metadata silently corrupts
//     the proto request message.
func TestExtraPropBug_NameCollisionWithProtoField(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.GetItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")
	g.Expect(sd).ToNot(BeNil())

	type idOverrideKey struct{}
	var capturedContextVal any
	var capturedProtoID string

	handler := func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		capturedContextVal = ctx.Value(idOverrideKey{})
		if getReq, ok := req.(*testdata.GetItemRequest); ok {
			capturedProtoID = getReq.Id
		}
		return newTestMessage(method.Output()), nil
	}

	server := mcpserver.NewMCPServer("test", "1.0")
	RegisterService(server, sd, handler, RegisterServiceOptions{
		Provider:   runtime.LLMProviderStandard,
		NewMessage: newTestMessage,
		ExtraProperties: []runtime.ExtraProperty{
			{
				// BUG: This name collides with the "id" field on
				// GetItemRequest. The extra property value will be
				// unmarshaled into the proto field because the handler
				// never strips extra properties from the arguments map.
				Name:        "id",
				Description: "Override ID for routing",
				Required:    true,
				ContextKey:  idOverrideKey{},
			},
		},
	})

	ctx := context.Background()
	_ = server.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "testdata_TestService_GetItem",
			"arguments": {"id": "route-to-shard-7"}
		}
	}`))

	// The extra property value ends up in context - this is expected.
	g.Expect(capturedContextVal).To(Equal("route-to-shard-7"))

	// BUG: The extra property value also leaks into the proto message's "id"
	// field. A user who intended "id" as a routing context key now finds
	// GetItemRequest.Id populated with "route-to-shard-7" instead of the
	// item ID the caller actually wanted to look up.
	//
	// The root cause: extra properties are read from the arguments map
	// (register.go:127) but never deleted. The full map - including extra
	// property keys - is then marshaled to JSON and fed to protojson.Unmarshal
	// (register.go:138-144). If a key matches a proto field, the extra
	// property value silently becomes the field value.
	//
	// This assertion documents the bug: we EXPECT the proto field to be empty
	// (extra properties shouldn't leak into the proto message), but it will
	// actually be populated. Flip the assertion to see the bug go green.
	g.Expect(capturedProtoID).To(Equal(""),
		"extra property 'id' should NOT leak into GetItemRequest.Id, "+
			"but it does: the proto field got value %q from the extra property", capturedProtoID)
}

// TestExtraPropBug_ExtraPropsNotStrippedBeforeUnmarshal demonstrates that
// extra property keys remain in the arguments map when it is fed to
// protojson.Unmarshal. This only works because DiscardUnknown is set to true.
// If extra properties used names that happen to be valid proto fields with
// incompatible types, protojson.Unmarshal would fail with a type error.
func TestExtraPropBug_ExtraPropsNotStrippedBeforeUnmarshal(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")
	g.Expect(sd).ToNot(BeNil())

	type tenantKey struct{}
	var capturedTenant any
	var handlerCalled bool

	handler := func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		handlerCalled = true
		capturedTenant = ctx.Value(tenantKey{})
		return newTestMessage(method.Output()), nil
	}

	server := mcpserver.NewMCPServer("test", "1.0")
	RegisterService(server, sd, handler, RegisterServiceOptions{
		Provider:   runtime.LLMProviderStandard,
		NewMessage: newTestMessage,
		ExtraProperties: []runtime.ExtraProperty{
			{
				// "tags" is a real repeated string field on CreateItemRequest.
				// The extra property schema declares it as type "string",
				// but the proto field is repeated string. Sending a string
				// value for a repeated field causes a protojson unmarshal error.
				Name:        "tags",
				Description: "Tenant identifier",
				Required:    true,
				ContextKey:  tenantKey{},
			},
		},
	})

	ctx := context.Background()
	result := server.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "testdata_TestService_CreateItem",
			"arguments": {"name": "Widget", "tags": "tenant-42"}
		}
	}`))

	// With "tags" as both an extra property (string) and a proto field
	// (repeated string), protojson.Unmarshal will fail because a plain
	// string is not valid JSON for a repeated field. The extra property
	// was never removed from the map before marshaling.
	resultBytes, err := json.Marshal(result)
	g.Expect(err).ToNot(HaveOccurred())

	// The error surfaces as an MCP error response rather than a successful call.
	var resp map[string]any
	g.Expect(json.Unmarshal(resultBytes, &resp)).To(Succeed())

	// BUG: The handler should have been called (the extra property should
	// have been stripped from the map before proto unmarshaling), but it
	// wasn't - protojson choked on the type mismatch.
	g.Expect(handlerCalled).To(BeTrue(),
		"handler was never called: the extra property 'tags' (string) collided "+
			"with proto field 'tags' (repeated string) and protojson.Unmarshal failed. "+
			"Extra properties must be removed from the arguments map before unmarshaling.")
	g.Expect(capturedTenant).To(Equal("tenant-42"))
}
