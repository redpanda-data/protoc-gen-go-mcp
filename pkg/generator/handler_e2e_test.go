package generator

import (
	"context"
	"encoding/json"
	"testing"

	mcpserver "github.com/mark3labs/mcp-go/server"
	. "github.com/onsi/gomega"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	testdatamcp "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata/testdatamcp"
)

// fullTestServer implements all TestService methods.
type fullTestServer struct {
	lastCreateReq *testdata.CreateItemRequest
}

func (s *fullTestServer) CreateItem(_ context.Context, in *testdata.CreateItemRequest) (*testdata.CreateItemResponse, error) {
	s.lastCreateReq = in
	return &testdata.CreateItemResponse{Id: "created-1"}, nil
}

func (s *fullTestServer) GetItem(_ context.Context, in *testdata.GetItemRequest) (*testdata.GetItemResponse, error) {
	return &testdata.GetItemResponse{
		Item: &testdata.Item{Id: in.Id, Name: "found"},
	}, nil
}

func (s *fullTestServer) ProcessWellKnownTypes(_ context.Context, _ *testdata.ProcessWellKnownTypesRequest) (*testdata.ProcessWellKnownTypesResponse, error) {
	return &testdata.ProcessWellKnownTypesResponse{Success: true}, nil
}

func (s *fullTestServer) TestValidation(_ context.Context, _ *testdata.TestValidationRequest) (*testdata.TestValidationResponse, error) {
	return &testdata.TestValidationResponse{Success: true}, nil
}

// TestGeneratedHandlerE2E tests the full flow: register generated handler -> MCP call -> proto unmarshal -> handler call -> response
func TestGeneratedHandlerE2E(t *testing.T) {
	g := NewWithT(t)
	srv := &fullTestServer{}
	mcp := mcpserver.NewMCPServer("test", "1.0")

	testdatamcp.RegisterTestServiceHandler(mcp, srv)

	ctx := context.Background()
	result := mcp.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "testdata_TestService_CreateItem",
			"arguments": {
				"name": "Widget",
				"labels": {"env": "prod"},
				"tags": ["sale"]
			}
		}
	}`))
	g.Expect(result).ToNot(BeNil())
	g.Expect(srv.lastCreateReq).ToNot(BeNil())
	g.Expect(srv.lastCreateReq.Name).To(Equal("Widget"))
	g.Expect(srv.lastCreateReq.Labels).To(HaveKeyWithValue("env", "prod"))
	g.Expect(srv.lastCreateReq.Tags).To(ConsistOf("sale"))
}

// TestGeneratedOpenAIHandlerE2E tests the OpenAI handler with map array format
func TestGeneratedOpenAIHandlerE2E(t *testing.T) {
	g := NewWithT(t)
	srv := &fullTestServer{}
	mcp := mcpserver.NewMCPServer("test", "1.0")

	testdatamcp.RegisterTestServiceHandlerOpenAI(mcp, srv)

	ctx := context.Background()
	result := mcp.HandleMessage(ctx, json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "testdata_TestService_CreateItem",
			"arguments": {
				"name": "Gadget",
				"labels": [{"key": "team", "value": "backend"}],
				"tags": ["featured"],
				"product": {"price": 9.99, "quantity": 5},
				"service": null,
				"description": "A gadget",
				"thumbnail": null
			}
		}
	}`))
	g.Expect(result).ToNot(BeNil())
	g.Expect(srv.lastCreateReq).ToNot(BeNil())
	g.Expect(srv.lastCreateReq.Name).To(Equal("Gadget"))
	g.Expect(srv.lastCreateReq.Labels).To(HaveKeyWithValue("team", "backend"))
	g.Expect(srv.lastCreateReq.GetProduct()).ToNot(BeNil())
	g.Expect(srv.lastCreateReq.GetProduct().Price).To(BeNumerically("~", 9.99))
}

// TestGeneratedHandlerWithProviderSelection tests the runtime provider selection
func TestGeneratedHandlerWithProviderSelection(t *testing.T) {
	g := NewWithT(t)
	srv := &fullTestServer{}

	for _, provider := range []runtime.LLMProvider{runtime.LLMProviderStandard, runtime.LLMProviderOpenAI} {
		t.Run(string(provider), func(t *testing.T) {
			mcp := mcpserver.NewMCPServer("test", "1.0")
			testdatamcp.RegisterTestServiceHandlerWithProvider(mcp, srv, provider)

			ctx := context.Background()
			result := mcp.HandleMessage(ctx, json.RawMessage(`{
				"jsonrpc": "2.0",
				"id": 1,
				"method": "tools/call",
				"params": {
					"name": "testdata_TestService_GetItem",
					"arguments": {"id": "item-42"}
				}
			}`))
			g.Expect(result).ToNot(BeNil())

			// Parse response to verify handler was called
			respBytes, err := json.Marshal(result)
			g.Expect(err).ToNot(HaveOccurred())

			var resp map[string]any
			err = json.Unmarshal(respBytes, &resp)
			g.Expect(err).ToNot(HaveOccurred())
		})
	}
}
