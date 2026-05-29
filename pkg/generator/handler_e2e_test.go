package generator

import (
	"context"
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime/mark3labs"
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
	raw, adapter := mark3labs.NewServer("test", "1.0")

	testdatamcp.RegisterTestServiceHandler(adapter, srv)

	ctx := context.Background()
	result := raw.HandleMessage(ctx, json.RawMessage(`{
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
