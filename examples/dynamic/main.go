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

// Package main demonstrates dynamic MCP tool registration using protoreflect.
// No code generation needed - tools are created at runtime from proto descriptors.
//
// This is the "proxy/gateway" mode: you have a proto service descriptor (from
// reflection, a file descriptor set, or the proto registry) and you want to
// expose it as MCP tools without generating any code.
package main

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/server"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/gen"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func main() {
	s := server.NewMCPServer(
		"Dynamic MCP server - no codegen needed",
		"1.0.0",
	)

	// Get the service descriptor from the proto registry.
	// In a real proxy, you'd get this from server reflection or a file descriptor set.
	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")

	// Register all RPCs as MCP tools dynamically.
	// No NewMessage needed - defaults to dynamicpb for zero-config operation.
	gen.RegisterService(s, sd, handler, gen.RegisterServiceOptions{
		Provider: runtime.LLMProviderStandard,
		// NewMessage is nil - uses dynamicpb automatically.
		// For concrete types, provide your own:
		//   NewMessage: func(md protoreflect.MessageDescriptor) proto.Message { ... },
	})

	fmt.Printf("Registered %d tools from %s\n", sd.Methods().Len(), sd.FullName())

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

// handler is called for every tool invocation. The request is a proto.Message
// (either a concrete type or a dynamicpb.Message depending on NewMessage).
func handler(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error) {
	reqJSON, _ := protojson.MarshalOptions{Indent: "  "}.Marshal(req)
	fmt.Printf("Tool called: %s\nRequest: %s\n", method.FullName(), string(reqJSON))

	// In a real proxy, you'd forward to the upstream gRPC/Connect server here.
	// For this example, return a default response.
	return gen.DynamicNewMessage(method.Output()), nil
}
