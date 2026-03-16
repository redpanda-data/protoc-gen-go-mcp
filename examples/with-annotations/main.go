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

// This example demonstrates how to add MCP tool annotations to generated tools.
// Tool annotations provide semantic hints to LLMs and MCP clients about tool behavior.
package main

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata/testdatamcp"
)

// Helper function to create bool pointers
func boolPtr(b bool) *bool {
	return &b
}

func main() {
	// Create MCP server
	s := server.NewMCPServer(
		"Example with tool annotations",
		"1.0.0",
	)

	srv := testServer{}

	// Register tools with annotations applied manually
	// This shows how to add semantic hints to generated tools

	// GetItem is a read-only operation - mark it accordingly
	getItemTool := runtime.ApplyToolAnnotations(
		testdatamcp.TestService_GetItemTool,
		runtime.WithToolAnnotations(runtime.ToolAnnotationConfig{
			Title:        "Get Item",
			ReadOnlyHint: boolPtr(true),
		}),
	)
	s.AddTool(getItemTool, makeGetItemHandler(&srv))

	// CreateItem modifies state - mark as destructive
	createItemTool := runtime.ApplyToolAnnotations(
		testdatamcp.TestService_CreateItemTool,
		runtime.WithToolAnnotations(runtime.ToolAnnotationConfig{
			Title:           "Create Item",
			DestructiveHint: boolPtr(true),
		}),
	)
	s.AddTool(createItemTool, makeCreateItemHandler(&srv))

	// ProcessWellKnownTypes - read-only processing
	processTypesTool := runtime.ApplyToolAnnotations(
		testdatamcp.TestService_ProcessWellKnownTypesTool,
		runtime.WithToolAnnotations(runtime.ToolAnnotationConfig{
			Title:        "Process Well-Known Types",
			ReadOnlyHint: boolPtr(true),
		}),
	)
	s.AddTool(processTypesTool, makeProcessTypesHandler(&srv))

	// TestValidation - read-only validation
	validationTool := runtime.ApplyToolAnnotations(
		testdatamcp.TestService_TestValidationTool,
		runtime.WithToolAnnotations(runtime.ToolAnnotationConfig{
			Title:        "Test Validation",
			ReadOnlyHint: boolPtr(true),
		}),
	)
	s.AddTool(validationTool, makeValidationHandler(&srv))

	fmt.Println("Starting server with annotated tools...")
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

// Handler functions that wrap the service methods

func makeGetItemHandler(srv *testServer) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, _ := request.GetArguments()["id"].(string)
		resp, err := srv.GetItem(ctx, &testdata.GetItemRequest{Id: id})
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultText(fmt.Sprintf("Item: %s - %s", resp.Item.Id, resp.Item.Name)), nil
	}
}

func makeCreateItemHandler(srv *testServer) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, _ := request.GetArguments()["name"].(string)
		resp, err := srv.CreateItem(ctx, &testdata.CreateItemRequest{Name: name})
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultText(fmt.Sprintf("Created item: %s", resp.Id)), nil
	}
}

func makeProcessTypesHandler(srv *testServer) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		resp, err := srv.ProcessWellKnownTypes(ctx, &testdata.ProcessWellKnownTypesRequest{})
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultText(resp.Message), nil
	}
}

func makeValidationHandler(srv *testServer) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		resp, err := srv.TestValidation(ctx, &testdata.TestValidationRequest{})
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultText(fmt.Sprintf("Success: %v - %s", resp.Success, resp.Message)), nil
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

func (t *testServer) TestValidation(ctx context.Context, in *testdata.TestValidationRequest) (*testdata.TestValidationResponse, error) {
	return &testdata.TestValidationResponse{
		Success: true,
		Message: "Validation test completed",
	}, nil
}
