package gen

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	mcpserver "github.com/mark3labs/mcp-go/server"
	. "github.com/onsi/gomega"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestRegisterService_HandlerReturnsError(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")

	handler := func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		return nil, errors.New("handler exploded")
	}

	server := mcpserver.NewMCPServer("test", "1.0")
	RegisterService(server, sd, handler, RegisterServiceOptions{
		Provider:   runtime.LLMProviderStandard,
		NewMessage: newTestMessage,
	})

	ctx := context.Background()
	result := server.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "testdata_TestService_GetItem",
			"arguments": {"id": "test-1"}
		}
	}`))

	resultBytes, err := json.Marshal(result)
	g.Expect(err).ToNot(HaveOccurred())

	var resp map[string]any
	err = json.Unmarshal(resultBytes, &resp)
	g.Expect(err).ToNot(HaveOccurred())

	// The error should be converted to MCP error result, not returned as a JSON-RPC error
	toolResult := resp["result"].(map[string]any)
	g.Expect(toolResult["isError"]).To(BeTrue())
}

func TestRegisterService_CommentProvider(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")

	handler := func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		return newTestMessage(method.Output()), nil
	}

	comments := map[string]string{
		"CreateItem":           "Create a new item in the store",
		"GetItem":              "Retrieve an item by its identifier",
		"ProcessWellKnownTypes": "Process messages with well-known types",
		"TestValidation":       "Validate input with protovalidate",
	}

	server := mcpserver.NewMCPServer("test", "1.0")
	RegisterService(server, sd, handler, RegisterServiceOptions{
		Provider:   runtime.LLMProviderStandard,
		NewMessage: newTestMessage,
		CommentProvider: func(method protoreflect.MethodDescriptor) string {
			return comments[string(method.Name())]
		},
	})

	ctx := context.Background()
	_ = server.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 0,
		"method": "initialize",
		"params": {
			"protocolVersion": "2024-11-05",
			"clientInfo": {"name": "test", "version": "1.0"},
			"capabilities": {}
		}
	}`))

	result := server.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/list"
	}`))

	resultBytes, err := json.Marshal(result)
	g.Expect(err).ToNot(HaveOccurred())

	var resp struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"tools"`
		} `json:"result"`
	}
	err = json.Unmarshal(resultBytes, &resp)
	g.Expect(err).ToNot(HaveOccurred())

	// Verify each tool has the correct description
	for _, tool := range resp.Result.Tools {
		switch tool.Name {
		case "testdata_TestService_CreateItem":
			g.Expect(tool.Description).To(Equal("Create a new item in the store"))
		case "testdata_TestService_GetItem":
			g.Expect(tool.Description).To(Equal("Retrieve an item by its identifier"))
		}
	}
}

func TestRegisterService_NilCommentProvider(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")

	handler := func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		return newTestMessage(method.Output()), nil
	}

	server := mcpserver.NewMCPServer("test", "1.0")
	RegisterService(server, sd, handler, RegisterServiceOptions{
		Provider:   runtime.LLMProviderStandard,
		NewMessage: newTestMessage,
		// CommentProvider is nil - should result in empty descriptions
	})

	ctx := context.Background()
	_ = server.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 0,
		"method": "initialize",
		"params": {
			"protocolVersion": "2024-11-05",
			"clientInfo": {"name": "test", "version": "1.0"},
			"capabilities": {}
		}
	}`))

	result := server.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/list"
	}`))

	resultBytes, err := json.Marshal(result)
	g.Expect(err).ToNot(HaveOccurred())

	var resp struct {
		Result struct {
			Tools []struct {
				Description string `json:"description"`
			} `json:"tools"`
		} `json:"result"`
	}
	err = json.Unmarshal(resultBytes, &resp)
	g.Expect(err).ToNot(HaveOccurred())

	for _, tool := range resp.Result.Tools {
		g.Expect(tool.Description).To(BeEmpty())
	}
}

func TestRegisterService_ExtraPropertyNotInArgs(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")

	type tokenKey struct{}
	var capturedToken any

	handler := func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		capturedToken = ctx.Value(tokenKey{})
		return newTestMessage(method.Output()), nil
	}

	server := mcpserver.NewMCPServer("test", "1.0")
	RegisterService(server, sd, handler, RegisterServiceOptions{
		Provider:   runtime.LLMProviderStandard,
		NewMessage: newTestMessage,
		ExtraProperties: []runtime.ExtraProperty{
			{
				Name:        "auth_token",
				Description: "Auth token",
				Required:    false, // optional
				ContextKey:  tokenKey{},
			},
		},
	})

	ctx := context.Background()
	// Call WITHOUT the extra property in arguments
	_ = server.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "testdata_TestService_GetItem",
			"arguments": {"id": "test-1"}
		}
	}`))

	// Token should be nil since it wasn't provided
	g.Expect(capturedToken).To(BeNil())
}

func TestRegisterService_OpenAI_SchemaHasAdditionalPropertiesFalse(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")

	handler := func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		return newTestMessage(method.Output()), nil
	}

	server := mcpserver.NewMCPServer("test", "1.0")
	RegisterService(server, sd, handler, RegisterServiceOptions{
		Provider:   runtime.LLMProviderOpenAI,
		NewMessage: newTestMessage,
	})

	ctx := context.Background()
	_ = server.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 0,
		"method": "initialize",
		"params": {
			"protocolVersion": "2024-11-05",
			"clientInfo": {"name": "test", "version": "1.0"},
			"capabilities": {}
		}
	}`))

	result := server.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/list"
	}`))

	resultBytes, err := json.Marshal(result)
	g.Expect(err).ToNot(HaveOccurred())

	var resp struct {
		Result struct {
			Tools []struct {
				Name        string          `json:"name"`
				InputSchema json.RawMessage `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	err = json.Unmarshal(resultBytes, &resp)
	g.Expect(err).ToNot(HaveOccurred())

	for _, tool := range resp.Result.Tools {
		var schema map[string]any
		err = json.Unmarshal(tool.InputSchema, &schema)
		g.Expect(err).ToNot(HaveOccurred(), "tool %s", tool.Name)

		// Top-level type must be plain "object" (not ["object","null"])
		g.Expect(schema["type"]).To(Equal("object"), "tool %s", tool.Name)
		g.Expect(schema["additionalProperties"]).To(Equal(false), "tool %s", tool.Name)
	}
}

func TestRegisterService_OpenAI_FixAppliedToMapArgs(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")

	var capturedReq *testdata.CreateItemRequest

	handler := func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		if string(method.Name()) == "CreateItem" {
			capturedReq = req.(*testdata.CreateItemRequest)
		}
		return newTestMessage(method.Output()), nil
	}

	server := mcpserver.NewMCPServer("test", "1.0")
	RegisterService(server, sd, handler, RegisterServiceOptions{
		Provider:   runtime.LLMProviderOpenAI,
		NewMessage: newTestMessage,
	})

	ctx := context.Background()
	// Send map as array of KV pairs (OpenAI format) + well-known type data
	_ = server.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "testdata_TestService_CreateItem",
			"arguments": {
				"name": "TestWidget",
				"labels": [
					{"key": "env", "value": "staging"},
					{"key": "team", "value": "platform"}
				],
				"tags": ["go", "grpc"]
			}
		}
	}`))

	g.Expect(capturedReq).ToNot(BeNil())
	g.Expect(capturedReq.Name).To(Equal("TestWidget"))
	g.Expect(capturedReq.Labels).To(HaveLen(2))
	g.Expect(capturedReq.Labels["env"]).To(Equal("staging"))
	g.Expect(capturedReq.Labels["team"]).To(Equal("platform"))
	g.Expect(capturedReq.Tags).To(ConsistOf("go", "grpc"))
}

func TestRegisterService_EdgeCaseService_AllMethods(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.DeepNestingRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("EdgeCaseService")
	g.Expect(sd).ToNot(BeNil())

	calledMethods := map[string]bool{}

	handler := func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		calledMethods[string(method.Name())] = true
		return DynamicNewMessage(method.Output()), nil
	}

	server := mcpserver.NewMCPServer("test", "1.0")
	RegisterService(server, sd, handler, RegisterServiceOptions{
		Provider: runtime.LLMProviderStandard,
	})

	ctx := context.Background()
	_ = server.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 0,
		"method": "initialize",
		"params": {
			"protocolVersion": "2024-11-05",
			"clientInfo": {"name": "test", "version": "1.0"},
			"capabilities": {}
		}
	}`))

	result := server.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/list"
	}`))

	resultBytes, err := json.Marshal(result)
	g.Expect(err).ToNot(HaveOccurred())

	var resp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	err = json.Unmarshal(resultBytes, &resp)
	g.Expect(err).ToNot(HaveOccurred())

	// EdgeCaseService has 8 unary RPCs
	g.Expect(resp.Result.Tools).To(HaveLen(8))

	expectedTools := []string{
		"testdata_EdgeCaseService_DeepNesting",
		"testdata_EdgeCaseService_AllScalarTypes",
		"testdata_EdgeCaseService_RepeatedMessages",
		"testdata_EdgeCaseService_MapVariants",
		"testdata_EdgeCaseService_EnumFields",
		"testdata_EdgeCaseService_MultipleOneofs",
		"testdata_EdgeCaseService_NumericValidation",
		"testdata_EdgeCaseService_RecursiveTree",
	}
	toolNames := make([]string, 0)
	for _, tool := range resp.Result.Tools {
		toolNames = append(toolNames, tool.Name)
	}
	g.Expect(toolNames).To(ConsistOf(expectedTools))
}

func TestRegisterService_DiscardUnknownFields(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")

	var handlerCalled bool

	handler := func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		handlerCalled = true
		return newTestMessage(method.Output()), nil
	}

	server := mcpserver.NewMCPServer("test", "1.0")
	RegisterService(server, sd, handler, RegisterServiceOptions{
		Provider:   runtime.LLMProviderStandard,
		NewMessage: newTestMessage,
	})

	ctx := context.Background()
	// Send request with unknown fields - DiscardUnknown should handle this
	result := server.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "testdata_TestService_GetItem",
			"arguments": {"id": "test-1", "unknown_field": "should be ignored"}
		}
	}`))

	g.Expect(result).ToNot(BeNil())
	g.Expect(handlerCalled).To(BeTrue())
}

func TestRegisterService_MultipleExtraProperties(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")

	type tokenKey struct{}
	type regionKey struct{}

	var capturedToken, capturedRegion any

	handler := func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		capturedToken = ctx.Value(tokenKey{})
		capturedRegion = ctx.Value(regionKey{})
		return newTestMessage(method.Output()), nil
	}

	server := mcpserver.NewMCPServer("test", "1.0")
	RegisterService(server, sd, handler, RegisterServiceOptions{
		Provider:   runtime.LLMProviderStandard,
		NewMessage: newTestMessage,
		ExtraProperties: []runtime.ExtraProperty{
			{Name: "auth_token", Description: "Auth token", Required: true, ContextKey: tokenKey{}},
			{Name: "region", Description: "AWS region", Required: false, ContextKey: regionKey{}},
		},
	})

	ctx := context.Background()
	_ = server.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "testdata_TestService_GetItem",
			"arguments": {"id": "test-1", "auth_token": "sk-123", "region": "us-east-1"}
		}
	}`))

	g.Expect(capturedToken).To(Equal("sk-123"))
	g.Expect(capturedRegion).To(Equal("us-east-1"))
}

func TestRegisterService_ToolCallReturnsResponse(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")

	handler := func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		if string(method.Name()) == "GetItem" {
			getReq := req.(*testdata.GetItemRequest)
			return &testdata.GetItemResponse{
				Item: &testdata.Item{
					Id:   getReq.Id,
					Name: "Widget",
				},
			}, nil
		}
		return newTestMessage(method.Output()), nil
	}

	server := mcpserver.NewMCPServer("test", "1.0")
	RegisterService(server, sd, handler, RegisterServiceOptions{
		Provider:   runtime.LLMProviderStandard,
		NewMessage: newTestMessage,
	})

	ctx := context.Background()
	result := server.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "testdata_TestService_GetItem",
			"arguments": {"id": "item-42"}
		}
	}`))

	resultBytes, err := json.Marshal(result)
	g.Expect(err).ToNot(HaveOccurred())

	var resp struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	err = json.Unmarshal(resultBytes, &resp)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(resp.Result.Content).To(HaveLen(1))
	g.Expect(resp.Result.Content[0].Type).To(Equal("text"))

	// Parse the response text as JSON and verify the proto was marshaled correctly
	var itemResp map[string]any
	err = json.Unmarshal([]byte(resp.Result.Content[0].Text), &itemResp)
	g.Expect(err).ToNot(HaveOccurred())

	item := itemResp["item"].(map[string]any)
	g.Expect(item["id"]).To(Equal("item-42"))
	g.Expect(item["name"]).To(Equal("Widget"))
}

func TestToolForMethod_EdgeCaseService(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.DeepNestingRequest{}).ProtoReflect().Descriptor().ParentFile()
	svc := file.Services().ByName("EdgeCaseService")
	g.Expect(svc).ToNot(BeNil())

	// Verify all edge case methods produce valid tools
	for i := 0; i < svc.Methods().Len(); i++ {
		method := svc.Methods().Get(i)
		t.Run(string(method.Name()), func(t *testing.T) {
			g := NewWithT(t)
			standard, openAI := ToolForMethod(method, "Test "+string(method.Name()))

			g.Expect(len(standard.Name)).To(BeNumerically("<=", 64))
			g.Expect(standard.Name).To(Equal(openAI.Name))
			g.Expect(standard.Description).To(HavePrefix("Test "))

			// Both schemas should be valid JSON
			var stdSchema, oaiSchema map[string]any
			g.Expect(json.Unmarshal(standard.RawInputSchema, &stdSchema)).To(Succeed())
			g.Expect(json.Unmarshal(openAI.RawInputSchema, &oaiSchema)).To(Succeed())

			// OpenAI schema: top-level "object", additionalProperties: false
			g.Expect(oaiSchema["type"]).To(Equal("object"))
			g.Expect(oaiSchema["additionalProperties"]).To(Equal(false))

			// Standard schema: "object", no additionalProperties
			g.Expect(stdSchema["type"]).To(Equal("object"))
			g.Expect(stdSchema).ToNot(HaveKey("additionalProperties"))

			// Validate using the raw schema (not JSON round-tripped) since
			// validateOpenAISchema expects []string for required, not []interface{}
			oaiRawSchema := MessageSchema(method.Input(), SchemaOptions{OpenAICompat: true})
			oaiRawSchema["type"] = "object"
			validateOpenAISchema(t, oaiRawSchema, string(method.Name()))
		})
	}
}

func TestRegisterService_EmptyArguments(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")

	var capturedReq *testdata.GetItemRequest

	handler := func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		if string(method.Name()) == "GetItem" {
			capturedReq = req.(*testdata.GetItemRequest)
		}
		return newTestMessage(method.Output()), nil
	}

	server := mcpserver.NewMCPServer("test", "1.0")
	RegisterService(server, sd, handler, RegisterServiceOptions{
		Provider:   runtime.LLMProviderStandard,
		NewMessage: newTestMessage,
	})

	ctx := context.Background()
	_ = server.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "testdata_TestService_GetItem",
			"arguments": {}
		}
	}`))

	g.Expect(capturedReq).ToNot(BeNil())
	g.Expect(capturedReq.Id).To(Equal("")) // zero value
}

func TestRegisterService_ExtraPropertiesInSchema(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")

	handler := func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		return newTestMessage(method.Output()), nil
	}

	server := mcpserver.NewMCPServer("test", "1.0")
	RegisterService(server, sd, handler, RegisterServiceOptions{
		Provider:   runtime.LLMProviderStandard,
		NewMessage: newTestMessage,
		ExtraProperties: []runtime.ExtraProperty{
			{Name: "session_id", Description: "Session identifier", Required: true, ContextKey: "session"},
		},
	})

	ctx := context.Background()
	_ = server.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 0,
		"method": "initialize",
		"params": {
			"protocolVersion": "2024-11-05",
			"clientInfo": {"name": "test", "version": "1.0"},
			"capabilities": {}
		}
	}`))

	result := server.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/list"
	}`))

	resultBytes, err := json.Marshal(result)
	g.Expect(err).ToNot(HaveOccurred())

	var resp struct {
		Result struct {
			Tools []struct {
				Name        string          `json:"name"`
				InputSchema json.RawMessage `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	err = json.Unmarshal(resultBytes, &resp)
	g.Expect(err).ToNot(HaveOccurred())

	// Every tool should have session_id in its schema
	for _, tool := range resp.Result.Tools {
		var schema map[string]any
		err = json.Unmarshal(tool.InputSchema, &schema)
		g.Expect(err).ToNot(HaveOccurred(), "tool %s", tool.Name)

		props := schema["properties"].(map[string]any)
		g.Expect(props).To(HaveKey("session_id"), "tool %s missing session_id", tool.Name)

		sessionProp := props["session_id"].(map[string]any)
		g.Expect(sessionProp["type"]).To(Equal("string"))
		g.Expect(sessionProp["description"]).To(Equal("Session identifier"))

		// session_id should be required
		required := schema["required"]
		switch r := required.(type) {
		case []any:
			g.Expect(r).To(ContainElement("session_id"), "tool %s", tool.Name)
		case []string:
			g.Expect(r).To(ContainElement("session_id"), "tool %s", tool.Name)
		}
	}
}
