package gen

import (
	"context"
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime/mark3labs"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// newTestMessage creates proto messages from descriptors using the test types.
// In real usage, users would use dynamicpb or their own generated types.
func newTestMessage(md protoreflect.MessageDescriptor) proto.Message {
	switch string(md.FullName()) {
	case "testdata.CreateItemRequest":
		return &testdata.CreateItemRequest{}
	case "testdata.CreateItemResponse":
		return &testdata.CreateItemResponse{}
	case "testdata.GetItemRequest":
		return &testdata.GetItemRequest{}
	case "testdata.GetItemResponse":
		return &testdata.GetItemResponse{}
	case "testdata.ProcessWellKnownTypesRequest":
		return &testdata.ProcessWellKnownTypesRequest{}
	case "testdata.ProcessWellKnownTypesResponse":
		return &testdata.ProcessWellKnownTypesResponse{}
	case "testdata.TestValidationRequest":
		return &testdata.TestValidationRequest{}
	case "testdata.TestValidationResponse":
		return &testdata.TestValidationResponse{}
	default:
		// Fall back to dynamic messages for unknown types
		return dynamicpb.NewMessage(md)
	}
}

// recordingServer is a runtime.MCPServer that captures every registered tool
// and its handler so tests can inspect both the schemas attached to the tool
// and the result that flows back from the handler.
type recordingServer struct {
	tools    []runtime.Tool
	handlers map[string]runtime.ToolHandler
}

func (r *recordingServer) AddTool(tool runtime.Tool, handler runtime.ToolHandler) {
	if r.handlers == nil {
		r.handlers = map[string]runtime.ToolHandler{}
	}
	r.tools = append(r.tools, tool)
	r.handlers[tool.Name] = handler
}

func TestRegisterService_OutputSchemaAndStructuredContent(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")

	handler := func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		if string(method.Name()) != "GetItem" {
			return newTestMessage(method.Output()), nil
		}
		return &testdata.GetItemResponse{
			Item: &testdata.Item{Id: req.(*testdata.GetItemRequest).Id, Name: "found"},
		}, nil
	}

	rec := &recordingServer{}
	RegisterService(rec, sd, handler, RegisterServiceOptions{
		NewMessage: newTestMessage,
	})

	// Every registered tool should now carry a non-empty output schema.
	g.Expect(rec.tools).ToNot(BeEmpty())
	for _, tool := range rec.tools {
		g.Expect(tool.RawOutputSchema).ToNot(BeEmpty(), "tool %q missing output schema", tool.Name)

		var schema map[string]any
		g.Expect(json.Unmarshal(tool.RawOutputSchema, &schema)).To(Succeed())
		g.Expect(schema["type"]).To(Equal("object"), "tool %q output schema must be a JSON object", tool.Name)
	}

	// The result returned by the handler should populate StructuredContent
	// with the same JSON payload as Text, so MCP clients that prefer the
	// structured field don't have to re-parse the text.
	getItem := rec.handlers["testdata_TestService_GetItem"]
	g.Expect(getItem).ToNot(BeNil())

	result, err := getItem(context.Background(), &runtime.CallToolRequest{
		Arguments: map[string]any{"id": "abc"},
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.IsError).To(BeFalse())
	g.Expect(result.Text).ToNot(BeEmpty())
	g.Expect(result.StructuredContent).ToNot(BeNil())

	rawJSON, ok := result.StructuredContent.(json.RawMessage)
	g.Expect(ok).To(BeTrue(), "StructuredContent should carry the JSON bytes verbatim for the adapter layer")
	g.Expect(string(rawJSON)).To(Equal(result.Text))

	var decoded map[string]any
	g.Expect(json.Unmarshal(rawJSON, &decoded)).To(Succeed())
	g.Expect(decoded).To(HaveKey("item"))
}

func TestRegisterService_Standard(t *testing.T) {
	g := NewWithT(t)

	// Get the TestService descriptor from the proto registry
	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")
	g.Expect(sd).ToNot(BeNil())

	// Track which methods get called
	called := map[string]bool{}

	handler := func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		called[string(method.Name())] = true

		switch string(method.Name()) {
		case "GetItem":
			getReq := req.(*testdata.GetItemRequest)
			return &testdata.GetItemResponse{
				Item: &testdata.Item{
					Id:   getReq.Id,
					Name: "Found Item",
				},
			}, nil
		default:
			return newTestMessage(method.Output()), nil
		}
	}

	raw, adapter := mark3labs.NewServer("test", "1.0")
	RegisterService(adapter, sd, handler, RegisterServiceOptions{
		NewMessage: newTestMessage,
	})

	// Verify tools were registered by listing them
	ctx := context.Background()

	// Call GetItem tool
	result := raw.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "testdata_TestService_GetItem",
			"arguments": {"id": "test-123"}
		}
	}`))
	g.Expect(result).ToNot(BeNil())
	g.Expect(called["GetItem"]).To(BeTrue())

	// Parse the response to verify the handler output
	resultBytes, err := json.Marshal(result)
	g.Expect(err).ToNot(HaveOccurred())
	var resp map[string]any
	err = json.Unmarshal(resultBytes, &resp)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestRegisterService_OpenAI(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")

	handler := func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		if string(method.Name()) == "CreateItem" {
			createReq := req.(*testdata.CreateItemRequest)
			g.Expect(createReq.Name).To(Equal("Widget"))
			g.Expect(createReq.Labels).To(HaveKeyWithValue("env", "prod"))
		}
		return newTestMessage(method.Output()), nil
	}

	raw, adapter := mark3labs.NewServer("test", "1.0")
	RegisterService(adapter, sd, handler, RegisterServiceOptions{
		NewMessage: newTestMessage,
	})

	// Call CreateItem with OpenAI-format map (array of key-value pairs)
	ctx := context.Background()
	_ = raw.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "testdata_TestService_CreateItem",
			"arguments": {
				"name": "Widget",
				"labels": [{"key": "env", "value": "prod"}],
				"tags": ["sale"]
			}
		}
	}`))
}

func TestRegisterService_WithExtraProperties(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")

	type apiKeyCtx struct{}
	var capturedAPIKey any

	handler := func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		capturedAPIKey = ctx.Value(apiKeyCtx{})
		return newTestMessage(method.Output()), nil
	}

	raw, adapter := mark3labs.NewServer("test", "1.0")
	RegisterService(adapter, sd, handler, RegisterServiceOptions{
		NewMessage: newTestMessage,
		ExtraProperties: []runtime.ExtraProperty{
			{
				Name:        "api_key",
				Description: "API key",
				Required:    true,
				ContextKey:  apiKeyCtx{},
			},
		},
	})

	ctx := context.Background()
	_ = raw.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "testdata_TestService_GetItem",
			"arguments": {"id": "test-1", "api_key": "sk-secret"}
		}
	}`))
	g.Expect(capturedAPIKey).To(Equal("sk-secret"))
}

func TestRegisterService_ToolList(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")

	handler := func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		return newTestMessage(method.Output()), nil
	}

	raw, adapter := mark3labs.NewServer("test", "1.0")
	RegisterService(adapter, sd, handler, RegisterServiceOptions{
		NewMessage: newTestMessage,
		CommentProvider: func(method protoreflect.MethodDescriptor) string {
			return "Description for " + string(method.Name())
		},
	})

	// List tools - need to initialize first
	ctx := context.Background()

	// Initialize the server (required before listing tools)
	initResult := raw.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 0,
		"method": "initialize",
		"params": {
			"protocolVersion": "2024-11-05",
			"clientInfo": {"name": "test", "version": "1.0"},
			"capabilities": {}
		}
	}`))
	g.Expect(initResult).ToNot(BeNil())

	result := raw.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/list"
	}`))

	resultBytes, err := json.Marshal(result)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(len(resultBytes)).To(BeNumerically(">", 2), "result should not be empty: %s", string(resultBytes))

	// Parse the response
	var resp struct {
		Result struct {
			Tools []struct {
				Name           string          `json:"name"`
				Description    string          `json:"description"`
				RawInputSchema json.RawMessage `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	err = json.Unmarshal(resultBytes, &resp)
	g.Expect(err).ToNot(HaveOccurred())

	// TestService has 4 unary RPCs
	g.Expect(resp.Result.Tools).To(HaveLen(4))

	// Verify tool names
	names := make([]string, 0, len(resp.Result.Tools))
	for _, tool := range resp.Result.Tools {
		names = append(names, tool.Name)
	}
	g.Expect(names).To(ConsistOf(
		"testdata_TestService_CreateItem",
		"testdata_TestService_GetItem",
		"testdata_TestService_ProcessWellKnownTypes",
		"testdata_TestService_TestValidation",
	))

	// Verify descriptions
	for _, tool := range resp.Result.Tools {
		g.Expect(tool.Description).To(HavePrefix("Description for "))
	}
}

func TestRegisterService_ZeroConfigDynamicPB(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")

	var calledMethod string

	handler := func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		calledMethod = string(method.Name())
		// With dynamicpb, req is a *dynamicpb.Message, not a concrete type.
		// Verify we can read field values from it.
		idField := req.ProtoReflect().Descriptor().Fields().ByName("id")
		if idField != nil {
			g.Expect(req.ProtoReflect().Get(idField).String()).To(Equal("dynamic-456"))
		}
		// Return a dynamic response
		return DynamicNewMessage(method.Output()), nil
	}

	// Zero-config: no NewMessage provided, should default to DynamicNewMessage
	raw, adapter := mark3labs.NewServer("test", "1.0")
	RegisterService(adapter, sd, handler, RegisterServiceOptions{
		// NewMessage intentionally omitted - should default to dynamicpb
	})

	ctx := context.Background()
	_ = raw.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "testdata_TestService_GetItem",
			"arguments": {"id": "dynamic-456"}
		}
	}`))
	g.Expect(calledMethod).To(Equal("GetItem"))
}

func TestDynamicNewMessage(t *testing.T) {
	g := NewWithT(t)
	md := (&testdata.GetItemRequest{}).ProtoReflect().Descriptor()
	msg := DynamicNewMessage(md)
	g.Expect(msg).ToNot(BeNil())
	g.Expect(string(msg.ProtoReflect().Descriptor().FullName())).To(Equal("testdata.GetItemRequest"))
}
