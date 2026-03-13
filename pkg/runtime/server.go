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

package runtime

import (
	"context"
	"encoding/json"
)

// MCPServer is the abstraction that both mark3labs/mcp-go and
// modelcontextprotocol/go-sdk satisfy through thin adapter packages.
// Generated code and the dynamic registration path (gen.RegisterService)
// program against this interface so users can swap MCP libraries without
// regenerating.
type MCPServer interface {
	AddTool(tool Tool, handler ToolHandler)
}

// Tool describes an MCP tool independent of any MCP library.
type Tool struct {
	Name           string
	Description    string
	RawInputSchema json.RawMessage
}

// ToolHandler is the callback invoked when an MCP client calls a tool.
type ToolHandler func(ctx context.Context, request *CallToolRequest) (*CallToolResult, error)

// CallToolRequest carries the decoded arguments from an MCP tool call.
type CallToolRequest struct {
	Arguments map[string]any
}

// CallToolResult is the response from a tool handler.
type CallToolResult struct {
	Text    string
	IsError bool
}

// NewToolResultText creates a successful text result.
func NewToolResultText(text string) *CallToolResult {
	return &CallToolResult{Text: text}
}

// NewToolResultError creates an error text result.
func NewToolResultError(text string) *CallToolResult {
	return &CallToolResult{Text: text, IsError: true}
}
