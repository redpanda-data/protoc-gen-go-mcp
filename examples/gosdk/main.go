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

// Package main demonstrates using protoc-gen-go-mcp with the official
// modelcontextprotocol/go-sdk MCP library instead of mark3labs/mcp-go.
//
// The generated code is MCP-library-agnostic. You pick the library at
// server creation time by choosing the right adapter package.
package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime/gosdk"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata/testdatamcp"
)

func main() {
	// Create MCP server using the official go-sdk adapter.
	// raw is the *mcp.Server for transport setup,
	// s is the runtime.MCPServer for tool registration.
	raw, s := gosdk.NewServer(
		"Example gRPC-MCP server (official go-sdk)",
		"1.0.0",
	)

	srv := testServer{}

	// Register handlers - same generated code, different MCP library.
	testdatamcp.RegisterTestServiceHandlerWithProvider(s, &srv, runtime.LLMProviderStandard)

	fmt.Println("Serving over stdio with modelcontextprotocol/go-sdk")

	// Run the server over stdio using the go-sdk transport.
	if err := raw.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

type testServer struct{}

func (t *testServer) CreateItem(_ context.Context, in *testdata.CreateItemRequest) (*testdata.CreateItemResponse, error) {
	return &testdata.CreateItemResponse{Id: "item-123"}, nil
}

func (t *testServer) GetItem(_ context.Context, in *testdata.GetItemRequest) (*testdata.GetItemResponse, error) {
	return &testdata.GetItemResponse{
		Item: &testdata.Item{Id: in.GetId(), Name: "Retrieved item"},
	}, nil
}

func (t *testServer) ProcessWellKnownTypes(_ context.Context, _ *testdata.ProcessWellKnownTypesRequest) (*testdata.ProcessWellKnownTypesResponse, error) {
	return &testdata.ProcessWellKnownTypesResponse{Message: "Processed well-known types"}, nil
}

func (t *testServer) TestValidation(_ context.Context, _ *testdata.TestValidationRequest) (*testdata.TestValidationResponse, error) {
	return &testdata.TestValidationResponse{Success: true, Message: "Validation test completed"}, nil
}
