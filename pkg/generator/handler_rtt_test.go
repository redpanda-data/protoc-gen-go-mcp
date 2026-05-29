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

// Handler-level round-trip tests: a model-shaped tools/call (discriminated oneof
// wrapper, OpenAI-strict null siblings, malformed oneof) goes through the actual
// generated/registered handler -> DecodeArguments -> protojson -> handler ->
// EncodeMessage -> tool result, on both MCP adapters (mark3labs, go-sdk) and the
// dynamic RegisterService path (dynamicpb). This is the full schema->read->proto
// round trip without a live model.
package generator

import (
	"context"
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/gen"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime/gosdk"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime/mark3labs"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	testdatamcp "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata/testdatamcp"

	"github.com/mark3labs/mcp-go/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// callMark3labs issues a tools/call over the JSON-RPC surface and returns the
// parsed "result" object (content / structuredContent / isError).
func callMark3labs(t *testing.T, raw *server.MCPServer, name string, args map[string]any) map[string]any {
	t.Helper()
	req, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	})
	resp := raw.HandleMessage(context.Background(), req)
	rb, _ := json.Marshal(resp)
	var parsed struct {
		Result map[string]any `json:"result"`
		Error  any            `json:"error"`
	}
	if err := json.Unmarshal(rb, &parsed); err != nil {
		t.Fatalf("unmarshal response: %v\n%s", err, rb)
	}
	if parsed.Error != nil {
		t.Fatalf("transport-level JSON-RPC error (should be a tool result instead): %v", parsed.Error)
	}
	if parsed.Result == nil {
		t.Fatalf("no result in response: %s", rb)
	}
	return parsed.Result
}

// firstTextContent returns the text of the first content block of a tool result.
func firstTextContent(t *testing.T, result map[string]any) string {
	t.Helper()
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("no content in result: %v", result)
	}
	block, _ := content[0].(map[string]any)
	text, _ := block["text"].(string)
	return text
}

func TestRTT_Mark3labs_OneofWrapper(t *testing.T) {
	g := NewWithT(t)
	srv := &fullTestServer{}
	raw, adapter := mark3labs.NewServer("t", "1")
	testdatamcp.RegisterTestServiceHandler(adapter, srv)

	t.Run("happy discriminated wrapper", func(t *testing.T) {
		callMark3labs(t, raw, "testdata_TestService_CreateItem", map[string]any{
			"name":      "Widget",
			"item_type": map[string]any{"which": "product", "product": map[string]any{"price": 9.99, "quantity": 5}},
		})
		g.Expect(srv.lastCreateReq.GetProduct()).ToNot(BeNil())
		g.Expect(srv.lastCreateReq.GetProduct().GetPrice()).To(Equal(9.99))
	})

	t.Run("OpenAI-strict shape with null siblings", func(t *testing.T) {
		// OpenAI strict sends every member; unused ones null.
		callMark3labs(t, raw, "testdata_TestService_CreateItem", map[string]any{
			"name":        "W2",
			"description": nil,
			"item_type":   map[string]any{"which": "service", "product": nil, "service": map[string]any{"duration": "1h", "recurring": true}},
		})
		g.Expect(srv.lastCreateReq.GetService()).ToNot(BeNil())
		g.Expect(srv.lastCreateReq.GetService().GetDuration()).To(Equal("1h"))
	})

	t.Run("malformed oneof returns IsError result, not transport error", func(t *testing.T) {
		result := callMark3labs(t, raw, "testdata_TestService_CreateItem", map[string]any{
			"name":      "bad",
			"item_type": map[string]any{"which": "product", "product": nil, "service": map[string]any{"duration": "1h"}},
		})
		g.Expect(result["isError"]).To(Equal(true))
		// The message must name the fix so the model can self-correct.
		content, _ := json.Marshal(result["content"])
		g.Expect(string(content)).To(ContainSubstring("product"))
	})
}

func TestRTT_GoSDK_OneofWrapper(t *testing.T) {
	g := NewWithT(t)
	srv := &fullTestServer{}
	rawSrv, adapter := gosdk.NewServer("t", "1")
	testdatamcp.RegisterTestServiceHandler(adapter, srv)

	ctx := context.Background()
	clientT, serverT := mcp.NewInMemoryTransports()
	go func() { _ = rawSrv.Run(ctx, serverT) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "1"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	g.Expect(err).ToNot(HaveOccurred())
	defer session.Close()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "testdata_TestService_CreateItem",
		Arguments: map[string]any{
			"name":      "Gadget",
			"item_type": map[string]any{"which": "product", "product": map[string]any{"price": 1.5, "quantity": 2}},
		},
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res.IsError).To(BeFalse())
	g.Expect(srv.lastCreateReq.GetProduct()).ToNot(BeNil())
	g.Expect(srv.lastCreateReq.GetProduct().GetQuantity()).To(Equal(int32(2)))

	// Malformed -> IsError result (one-turn self-correction), not a transport error.
	res2, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "testdata_TestService_CreateItem",
		Arguments: map[string]any{
			"name":      "bad",
			"item_type": map[string]any{"which": "service", "product": map[string]any{"price": 1}},
		},
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res2.IsError).To(BeTrue())
}

// TestRTT_DynamicPath drives gen.RegisterService (dynamicpb) end to end: an input
// oneof wrapper decodes onto a dynamic message, and a response whose oneof is a
// false bool is re-wrapped (which first) in the structured result.
func TestRTT_DynamicPath(t *testing.T) {
	g := NewWithT(t)
	sd := (&testdata.OneofRecursiveRequest{}).ProtoReflect().Descriptor().
		ParentFile().Services().ByName("EdgeCaseService")
	g.Expect(sd).ToNot(BeNil())

	var gotReq proto.Message
	handler := func(_ context.Context, m protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
		gotReq = req
		// Respond with a false-bool oneof member; encode must use WhichOneof, not
		// JSON scanning, to know it is set.
		return &testdata.OneofRecursiveResponse{
			Result: &testdata.OneofRecursiveResponse_Ok{Ok: false},
		}, nil
	}

	raw, adapter := mark3labs.NewServer("t", "1")
	gen.RegisterService(adapter, sd, handler, gen.RegisterServiceOptions{})

	t.Run("input oneof decodes onto dynamicpb", func(t *testing.T) {
		result := callMark3labs(t, raw, "testdata_EdgeCaseService_OneofRecursive", map[string]any{
			"node": map[string]any{"which": "leaf", "leaf": "hello"},
		})
		g.Expect(result["isError"]).To(BeNil())
		// The request the handler saw must have the leaf oneof member set.
		nodeOneof := gotReq.ProtoReflect().Descriptor().Oneofs().ByName("node")
		set := gotReq.ProtoReflect().WhichOneof(nodeOneof)
		g.Expect(set).ToNot(BeNil())
		g.Expect(string(set.Name())).To(Equal("leaf"))
	})

	t.Run("response false-bool oneof re-wrapped which-first", func(t *testing.T) {
		result := callMark3labs(t, raw, "testdata_EdgeCaseService_OneofRecursive", map[string]any{
			"node": map[string]any{"which": "leaf", "leaf": "x"},
		})
		text := firstTextContent(t, result)
		// "which" must serialize before the value (ordering is a real invariant).
		g.Expect(text).To(ContainSubstring(`"result":{"which":"ok"`))
		var encoded map[string]any
		g.Expect(json.Unmarshal([]byte(text), &encoded)).To(Succeed())
		resultOneof, ok := encoded["result"].(map[string]any)
		g.Expect(ok).To(BeTrue(), "result oneof should be a discriminated object: %s", text)
		g.Expect(resultOneof["which"]).To(Equal("ok"))
		// false-bool member is found via WhichOneof, not JSON scanning.
		g.Expect(resultOneof["ok"]).To(Equal(false))
	})

	t.Run("malformed oneof -> IsError", func(t *testing.T) {
		result := callMark3labs(t, raw, "testdata_EdgeCaseService_OneofRecursive", map[string]any{
			"node": map[string]any{"which": "tree"}, // no value supplied
		})
		g.Expect(result["isError"]).To(Equal(true))
	})
}
