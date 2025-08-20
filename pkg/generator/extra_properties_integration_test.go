// Copyright 2025 Redpanda Data, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package generator

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/mrproliu/protoc-gen-go-mcp/pkg/runtime"
	testdata "github.com/mrproliu/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	. "github.com/onsi/gomega"
)

func TestExtraPropertiesSchemaModification(t *testing.T) {
	g := NewWithT(t)

	// Create a test tool with a basic schema (simulate what would be generated)
	originalSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
			"description": map[string]interface{}{
				"type": "string",
			},
			"labels": map[string]interface{}{
				"type": "object",
				"additionalProperties": map[string]interface{}{
					"type": "string",
				},
			},
			"tags": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"type": "string"},
			},
		},
		"required": []string{"name"},
	}

	schemaBytes, err := json.Marshal(originalSchema)
	g.Expect(err).ToNot(HaveOccurred())

	originalTool := mcp.Tool{
		Name:           "test_CreateItem",
		Description:    "Creates a new item",
		RawInputSchema: json.RawMessage(schemaBytes),
	}

	// Verify it doesn't have base_url field initially
	originalProperties := originalSchema["properties"].(map[string]interface{})
	g.Expect(originalProperties).ToNot(HaveKey("base_url"))

	// Add base_url field to the tool
	extraProps := []runtime.ExtraProperty{
		{
			Name:        "base_url",
			Description: "Base URL for the API",
			Required:    true,
			ContextKey:  "base_url_key",
		},
	}
	modifiedTool := runtime.AddExtraPropertiesToTool(originalTool, extraProps)

	// Parse the modified schema
	var modifiedSchema map[string]interface{}
	err = json.Unmarshal(modifiedTool.RawInputSchema, &modifiedSchema)
	g.Expect(err).ToNot(HaveOccurred())

	// Verify the base_url field was added
	modifiedProperties := modifiedSchema["properties"].(map[string]interface{})
	g.Expect(modifiedProperties).To(HaveKey("base_url"))

	urlField := modifiedProperties["base_url"].(map[string]interface{})
	g.Expect(urlField["type"]).To(Equal("string"))
	g.Expect(urlField).ToNot(HaveKey("format")) // No special format handling
	g.Expect(urlField["description"]).To(Equal("Base URL for the API"))

	// Verify original fields are still there
	g.Expect(modifiedProperties).To(HaveKey("name"))
	g.Expect(modifiedProperties).To(HaveKey("description"))
	g.Expect(modifiedProperties).To(HaveKey("labels"))
	g.Expect(modifiedProperties).To(HaveKey("tags"))

	// Verify the base_url field was added to required fields
	originalRequired := originalSchema["required"].([]string)
	modifiedRequired := modifiedSchema["required"].([]interface{})
	g.Expect(len(modifiedRequired)).To(Equal(len(originalRequired) + 1))
	g.Expect(modifiedRequired).To(ContainElement("base_url"))
}

func TestExtraPropertiesSchemaModificationWithCustomField(t *testing.T) {
	g := NewWithT(t)

	// Create a test tool with a basic schema (simulate what would be generated)
	originalSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
			"description": map[string]interface{}{
				"type": "string",
			},
			"labels": map[string]interface{}{
				"type": "object",
				"additionalProperties": map[string]interface{}{
					"type": "string",
				},
			},
			"tags": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"type": "string"},
			},
		},
		"required": []string{"name"},
	}

	schemaBytes, err := json.Marshal(originalSchema)
	g.Expect(err).ToNot(HaveOccurred())

	originalTool := mcp.Tool{
		Name:           "test_CreateItem",
		Description:    "Creates a new item",
		RawInputSchema: json.RawMessage(schemaBytes),
	}

	// Parse the original schema to verify it doesn't have custom field
	originalProperties := originalSchema["properties"].(map[string]interface{})
	g.Expect(originalProperties).ToNot(HaveKey("api_url"))

	// Add custom field to the tool
	extraProps := []runtime.ExtraProperty{
		{
			Name:        "api_url",
			Description: "Custom API endpoint URL",
			Required:    true,
			ContextKey:  "base_url_key",
		},
	}
	modifiedTool := runtime.AddExtraPropertiesToTool(originalTool, extraProps)

	// Parse the modified schema
	var modifiedSchema map[string]interface{}
	err = json.Unmarshal(modifiedTool.RawInputSchema, &modifiedSchema)
	g.Expect(err).ToNot(HaveOccurred())

	// Verify the custom field was added
	modifiedProperties := modifiedSchema["properties"].(map[string]interface{})
	g.Expect(modifiedProperties).To(HaveKey("api_url"))
	g.Expect(modifiedProperties).ToNot(HaveKey("base_url")) // Should not have default base_url field

	customField := modifiedProperties["api_url"].(map[string]interface{})
	g.Expect(customField["type"]).To(Equal("string"))
	g.Expect(customField).ToNot(HaveKey("format")) // No special format handling
	g.Expect(customField["description"]).To(Equal("Custom API endpoint URL"))

	// Verify original fields are still there
	g.Expect(modifiedProperties).To(HaveKey("name"))
	g.Expect(modifiedProperties).To(HaveKey("description"))
	g.Expect(modifiedProperties).To(HaveKey("labels"))
	g.Expect(modifiedProperties).To(HaveKey("tags"))

	// Verify the api_url field was added to required fields
	originalRequired := originalSchema["required"].([]string)
	modifiedRequired := modifiedSchema["required"].([]interface{})
	g.Expect(len(modifiedRequired)).To(Equal(len(originalRequired) + 1))
	g.Expect(modifiedRequired).To(ContainElement("api_url"))
}

// testServer implements TestServiceServer and tracks context values
type testServer struct {
	lastURLString string
}

func (t *testServer) CreateItem(ctx context.Context, in *testdata.CreateItemRequest) (*testdata.CreateItemResponse, error) {
	// Check if API URL is set in context (using the custom context key)
	if urlVal := ctx.Value("base_url_key"); urlVal != nil {
		if urlStr, ok := urlVal.(string); ok {
			t.lastURLString = urlStr
		}
	}

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

func TestExtraPropertiesContextIntegration(t *testing.T) {
	g := NewWithT(t)

	server := &testServer{}

	// Create an MCP server
	mcpServer := mcpserver.NewMCPServer("test-server", "1.0.0")

	// Create a mock tool with extra properties (simulating what the generated code would do)
	originalSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []string{"name"},
	}

	schemaBytes, err := json.Marshal(originalSchema)
	g.Expect(err).ToNot(HaveOccurred())

	baseTool := mcp.Tool{
		Name:           "testdata_TestService_CreateItem",
		Description:    "Creates a new item",
		RawInputSchema: json.RawMessage(schemaBytes),
	}

	// Add extra properties to the tool (simulating the generated registration code)
	extraProps := []runtime.ExtraProperty{
		{
			Name:        "api_url",
			Description: "API base URL",
			Required:    true,
			ContextKey:  "base_url_key",
		},
	}
	modifiedTool := runtime.AddExtraPropertiesToTool(baseTool, extraProps)

	// Register the tool with a handler that simulates the generated handler logic
	mcpServer.AddTool(modifiedTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract extra properties (simulating generated code)
		message := request.GetArguments()
		for _, prop := range extraProps {
			if propVal, ok := message[prop.Name]; ok {
				ctx = context.WithValue(ctx, prop.ContextKey, propVal)
			}
		}

		// Create a mock request from the arguments
		req := &testdata.CreateItemRequest{
			Name: message["name"].(string),
		}

		// Call the server implementation
		resp, err := server.CreateItem(ctx, req)
		if err != nil {
			return nil, err
		}

		// Return mock response
		return mcp.NewToolResultText(`{"id": "` + resp.Id + `"}`), nil
	})

	// Simulate an MCP call_tool request message
	callToolMessage := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "testdata_TestService_CreateItem",
			"arguments": map[string]interface{}{
				"name":    "test item",
				"api_url": "https://api.example.com:8080/v1",
			},
		},
	}

	// Marshal the message to JSON
	messageBytes, err := json.Marshal(callToolMessage)
	g.Expect(err).ToNot(HaveOccurred())

	// Handle the message through the MCP server
	response := mcpServer.HandleMessage(context.Background(), json.RawMessage(messageBytes))
	g.Expect(response).ToNot(BeNil())

	// Verify the URL string was set in context and received by server
	g.Expect(server.lastURLString).To(Equal("https://api.example.com:8080/v1"))
}
