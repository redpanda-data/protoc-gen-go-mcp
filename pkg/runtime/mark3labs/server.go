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

// Package mark3labs provides an adapter from mark3labs/mcp-go to the
// runtime.MCPServer interface.
package mark3labs

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
)

type server struct {
	s *mcpserver.MCPServer
}

// Wrap returns a runtime.MCPServer backed by a mark3labs MCPServer.
func Wrap(s *mcpserver.MCPServer) runtime.MCPServer {
	return &server{s: s}
}

// NewServer creates a new mark3labs MCPServer and returns it alongside
// the runtime.MCPServer adapter. Callers use the raw *mcpserver.MCPServer
// for transport setup (e.g. server.ServeStdio) and the adapter for
// tool registration.
func NewServer(name, version string) (*mcpserver.MCPServer, runtime.MCPServer) {
	s := mcpserver.NewMCPServer(name, version)
	return s, Wrap(s)
}

func (w *server) AddTool(tool runtime.Tool, handler runtime.ToolHandler) {
	mcpTool := mcp.Tool{
		Name:           tool.Name,
		Description:    tool.Description,
		RawInputSchema: json.RawMessage(tool.RawInputSchema),
	}
	w.s.AddTool(mcpTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := handler(ctx, &runtime.CallToolRequest{
			Arguments: request.GetArguments(),
		})
		if err != nil {
			return nil, err
		}
		if result == nil {
			return nil, nil
		}
		if result.IsError {
			return mcp.NewToolResultError(result.Text), nil
		}
		return mcp.NewToolResultText(result.Text), nil
	})
}
