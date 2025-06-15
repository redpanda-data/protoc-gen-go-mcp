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

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata/testdataconnect"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata/testdatamcp"
)

// Ensure our interface and the official gRPC interface are grpcClient
var (
	grpcClient testdata.TestServiceClient
	_          = testdatamcp.TestServiceClient(grpcClient)
)

// Ensure our interface and the official connect-go interface are compatible
var (
	connectClient testdataconnect.TestServiceClient
	_             = testdatamcp.ConnectTestServiceClient(connectClient)
)

func main() {
	// Create MCP server
	s := server.NewMCPServer(
		"Example auto-generated gRPC-MCP with runtime LLM provider selection",
		"1.0.0",
	)

	srv := testServer{}

	// Get LLM provider from environment variable, default to standard
	providerStr := os.Getenv("LLM_PROVIDER")
	var provider runtime.LLMProvider
	switch providerStr {
	case "openai":
		provider = runtime.LLMProviderOpenAI
		fmt.Printf("Using OpenAI-compatible MCP handlers\n")
	case "standard":
		fallthrough
	default:
		provider = runtime.LLMProviderStandard
		fmt.Printf("Using standard MCP handlers\n")
	}

	// Register handlers for the selected provider
	testdatamcp.RegisterTestServiceHandlerWithProvider(s, &srv, provider)

	// Alternative: Register specific handlers directly
	// testdatamcp.RegisterTestServiceHandler(s, &srv)        // Standard
	// testdatamcp.RegisterTestServiceHandlerOpenAI(s, &srv)  // OpenAI

	// Alternative: Register both for different tool names
	// testdatamcp.RegisterTestServiceHandler(s, &srv)
	// testdatamcp.RegisterTestServiceHandlerOpenAI(s, &srv)

	testdatamcp.ForwardToConnectTestServiceClient(s, connectClient)
	testdatamcp.ForwardToTestServiceClient(s, grpcClient)

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

type testServer struct{}

func (t *testServer) CreateItem(ctx context.Context, in *testdata.CreateItemRequest) (*testdata.CreateItemResponse, error) {
	return &testdata.CreateItemResponse{
		Id: "item-123",
	}, nil
}

func (t *testServer) GetItem(ctx context.Context, in *testdata.GetItemRequest) (*testdata.GetItemResponse, error) {
	return &testdata.GetItemResponse{
		Item: &testdata.Item{
			Id:   in.GetId(),
			Name: "Retrieved item",
		},
	}, nil
}

func (t *testServer) ProcessWellKnownTypes(ctx context.Context, in *testdata.ProcessWellKnownTypesRequest) (*testdata.ProcessWellKnownTypesResponse, error) {
	return &testdata.ProcessWellKnownTypesResponse{
		Message: "Processed well-known types",
	}, nil
}
