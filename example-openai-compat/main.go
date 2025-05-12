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

	"github.com/mark3labs/mcp-go/server"
	examplev1 "github.com/redpanda-data/protoc-gen-go-mcp/example-openai-compat/gen/go/proto/example/v1"
	"github.com/redpanda-data/protoc-gen-go-mcp/example-openai-compat/gen/go/proto/example/v1/examplev1connect"
	"github.com/redpanda-data/protoc-gen-go-mcp/example-openai-compat/gen/go/proto/example/v1/examplev1mcp"
)

// Ensure our interface and the official gRPC interface are grpcClient
var (
	grpcClient examplev1.ExampleServiceClient
	mcpClient  = examplev1mcp.ExampleServiceClient(grpcClient)
)

// Ensure our interface and the official connect-go interface are compatible
var (
	connectClient    examplev1connect.ExampleServiceClient
	connectMcpClient = examplev1mcp.ConnectExampleServiceClient(connectClient)
)

func main() {
	// Create MCP server
	s := server.NewMCPServer(
		"Example auto-generated gRPC-MCP",
		"1.0.0",
	)

	srv := exampleServer{}

	examplev1mcp.RegisterExampleServiceHandler(s, &srv)

	examplev1mcp.ForwardToConnectExampleServiceClient(s, connectClient)
	examplev1mcp.ForwardToExampleServiceClient(s, grpcClient)

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}

}

type exampleServer struct {
}

func (t *exampleServer) CreateExample(ctx context.Context, in *examplev1.CreateExampleRequest) (*examplev1.CreateExampleResponse, error) {
	return &examplev1.CreateExampleResponse{
		SomeString: "HAHA " + in.GetNested().GetNested2().GetNested3().GetOptionalString(),
	}, nil
}
