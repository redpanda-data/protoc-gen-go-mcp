// Code generated by protoc-gen-mcp-go. DO NOT EDIT.
// source: proto/example/v1/example.proto

package examplev1mcp

import (
	v1 "github.com/redpanda-data/protoc-gen-go-mcp/example/gen/go/proto/example/v1"
)

import (
	"context"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"encoding/json"
	"google.golang.org/protobuf/encoding/protojson"
	"connectrpc.com/connect"
	grpc "google.golang.org/grpc"
)

var (
	ExampleService_CreateExampleTool = mcp.Tool{Name: "example_v1_ExampleService_CreateExample", Description: "CreateCluster create a Redpanda cluster. The input contains the spec, that describes the cluster.\nA Operation is returned. This task allows the caller to find out when the long-running operation of creating a cluster has finished.\n", InputSchema: mcp.ToolInputSchema{Type: "", Properties: map[string]interface{}(nil), Required: []string(nil)}, RawInputSchema: json.RawMessage{0x7b, 0x22, 0x61, 0x6e, 0x79, 0x4f, 0x66, 0x22, 0x3a, 0x5b, 0x7b, 0x22, 0x24, 0x63, 0x6f, 0x6d, 0x6d, 0x65, 0x6e, 0x74, 0x22, 0x3a, 0x22, 0x49, 0x6e, 0x20, 0x74, 0x68, 0x69, 0x73, 0x20, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2c, 0x20, 0x74, 0x68, 0x65, 0x72, 0x65, 0x20, 0x69, 0x73, 0x20, 0x61, 0x20, 0x6f, 0x6e, 0x65, 0x4f, 0x66, 0x20, 0x67, 0x72, 0x6f, 0x75, 0x70, 0x20, 0x66, 0x6f, 0x72, 0x20, 0x65, 0x76, 0x65, 0x72, 0x79, 0x20, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x20, 0x6f, 0x6e, 0x65, 0x4f, 0x66, 0x20, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x20, 0x69, 0x6e, 0x20, 0x74, 0x68, 0x65, 0x20, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x22, 0x2c, 0x22, 0x6f, 0x6e, 0x65, 0x4f, 0x66, 0x22, 0x3a, 0x5b, 0x7b, 0x22, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x69, 0x65, 0x73, 0x22, 0x3a, 0x7b, 0x22, 0x66, 0x69, 0x72, 0x73, 0x74, 0x5f, 0x69, 0x74, 0x65, 0x6d, 0x22, 0x3a, 0x7b, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x7d, 0x7d, 0x2c, 0x22, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64, 0x22, 0x3a, 0x5b, 0x22, 0x66, 0x69, 0x72, 0x73, 0x74, 0x5f, 0x69, 0x74, 0x65, 0x6d, 0x22, 0x5d, 0x7d, 0x2c, 0x7b, 0x22, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x69, 0x65, 0x73, 0x22, 0x3a, 0x7b, 0x22, 0x73, 0x65, 0x63, 0x6f, 0x6e, 0x64, 0x5f, 0x69, 0x74, 0x65, 0x6d, 0x22, 0x3a, 0x7b, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x69, 0x6e, 0x74, 0x65, 0x67, 0x65, 0x72, 0x22, 0x7d, 0x7d, 0x2c, 0x22, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64, 0x22, 0x3a, 0x5b, 0x22, 0x73, 0x65, 0x63, 0x6f, 0x6e, 0x64, 0x5f, 0x69, 0x74, 0x65, 0x6d, 0x22, 0x5d, 0x7d, 0x5d, 0x7d, 0x5d, 0x2c, 0x22, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x69, 0x65, 0x73, 0x22, 0x3a, 0x7b, 0x22, 0x6d, 0x61, 0x70, 0x5f, 0x77, 0x69, 0x74, 0x68, 0x5f, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x5f, 0x76, 0x61, 0x6c, 0x22, 0x3a, 0x7b, 0x22, 0x61, 0x64, 0x64, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x6c, 0x50, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x69, 0x65, 0x73, 0x22, 0x3a, 0x7b, 0x22, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x69, 0x65, 0x73, 0x22, 0x3a, 0x7b, 0x22, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x32, 0x22, 0x3a, 0x7b, 0x22, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x69, 0x65, 0x73, 0x22, 0x3a, 0x7b, 0x22, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x33, 0x22, 0x3a, 0x7b, 0x22, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x69, 0x65, 0x73, 0x22, 0x3a, 0x7b, 0x22, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x6c, 0x5f, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x3a, 0x7b, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x7d, 0x2c, 0x22, 0x73, 0x6f, 0x6d, 0x65, 0x5f, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x5f, 0x69, 0x6e, 0x5f, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x5f, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x22, 0x3a, 0x7b, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x7d, 0x7d, 0x2c, 0x22, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64, 0x22, 0x3a, 0x5b, 0x5d, 0x2c, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x22, 0x7d, 0x2c, 0x22, 0x73, 0x6f, 0x6d, 0x65, 0x5f, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x5f, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x22, 0x3a, 0x7b, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x7d, 0x7d, 0x2c, 0x22, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64, 0x22, 0x3a, 0x5b, 0x5d, 0x2c, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x22, 0x7d, 0x2c, 0x22, 0x73, 0x6f, 0x6d, 0x65, 0x5f, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x22, 0x3a, 0x7b, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x7d, 0x7d, 0x2c, 0x22, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64, 0x22, 0x3a, 0x5b, 0x5d, 0x2c, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x22, 0x7d, 0x2c, 0x22, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x79, 0x4e, 0x61, 0x6d, 0x65, 0x73, 0x22, 0x3a, 0x7b, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x7d, 0x2c, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x22, 0x7d, 0x2c, 0x22, 0x6d, 0x61, 0x70, 0x5f, 0x77, 0x69, 0x74, 0x68, 0x5f, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x5f, 0x76, 0x61, 0x6c, 0x5f, 0x6e, 0x6f, 0x5f, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x5f, 0x6b, 0x65, 0x79, 0x22, 0x3a, 0x7b, 0x22, 0x61, 0x64, 0x64, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x6c, 0x50, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x69, 0x65, 0x73, 0x22, 0x3a, 0x7b, 0x22, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x69, 0x65, 0x73, 0x22, 0x3a, 0x7b, 0x22, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x32, 0x22, 0x3a, 0x7b, 0x22, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x69, 0x65, 0x73, 0x22, 0x3a, 0x7b, 0x22, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x33, 0x22, 0x3a, 0x7b, 0x22, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x69, 0x65, 0x73, 0x22, 0x3a, 0x7b, 0x22, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x6c, 0x5f, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x3a, 0x7b, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x7d, 0x2c, 0x22, 0x73, 0x6f, 0x6d, 0x65, 0x5f, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x5f, 0x69, 0x6e, 0x5f, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x5f, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x22, 0x3a, 0x7b, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x7d, 0x7d, 0x2c, 0x22, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64, 0x22, 0x3a, 0x5b, 0x5d, 0x2c, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x22, 0x7d, 0x2c, 0x22, 0x73, 0x6f, 0x6d, 0x65, 0x5f, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x5f, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x22, 0x3a, 0x7b, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x7d, 0x7d, 0x2c, 0x22, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64, 0x22, 0x3a, 0x5b, 0x5d, 0x2c, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x22, 0x7d, 0x2c, 0x22, 0x73, 0x6f, 0x6d, 0x65, 0x5f, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x22, 0x3a, 0x7b, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x7d, 0x7d, 0x2c, 0x22, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64, 0x22, 0x3a, 0x5b, 0x5d, 0x2c, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x22, 0x7d, 0x2c, 0x22, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x79, 0x4e, 0x61, 0x6d, 0x65, 0x73, 0x22, 0x3a, 0x7b, 0x22, 0x70, 0x61, 0x74, 0x74, 0x65, 0x72, 0x6e, 0x22, 0x3a, 0x22, 0x5e, 0x2d, 0x3f, 0x28, 0x30, 0x7c, 0x5b, 0x31, 0x2d, 0x39, 0x5d, 0x5c, 0x5c, 0x64, 0x2a, 0x29, 0x24, 0x22, 0x2c, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x7d, 0x2c, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x22, 0x7d, 0x2c, 0x22, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x22, 0x3a, 0x7b, 0x22, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x69, 0x65, 0x73, 0x22, 0x3a, 0x7b, 0x22, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x32, 0x22, 0x3a, 0x7b, 0x22, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x69, 0x65, 0x73, 0x22, 0x3a, 0x7b, 0x22, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x33, 0x22, 0x3a, 0x7b, 0x22, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x69, 0x65, 0x73, 0x22, 0x3a, 0x7b, 0x22, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x6c, 0x5f, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x3a, 0x7b, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x7d, 0x2c, 0x22, 0x73, 0x6f, 0x6d, 0x65, 0x5f, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x5f, 0x69, 0x6e, 0x5f, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x5f, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x22, 0x3a, 0x7b, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x7d, 0x7d, 0x2c, 0x22, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64, 0x22, 0x3a, 0x5b, 0x5d, 0x2c, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x22, 0x7d, 0x2c, 0x22, 0x73, 0x6f, 0x6d, 0x65, 0x5f, 0x6e, 0x65, 0x73, 0x74, 0x65, 0x64, 0x5f, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x22, 0x3a, 0x7b, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x7d, 0x7d, 0x2c, 0x22, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64, 0x22, 0x3a, 0x5b, 0x5d, 0x2c, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x22, 0x7d, 0x2c, 0x22, 0x73, 0x6f, 0x6d, 0x65, 0x5f, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x22, 0x3a, 0x7b, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x7d, 0x7d, 0x2c, 0x22, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64, 0x22, 0x3a, 0x5b, 0x5d, 0x2c, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x22, 0x7d, 0x2c, 0x22, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x6c, 0x5f, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x3a, 0x7b, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x7d, 0x2c, 0x22, 0x72, 0x65, 0x70, 0x65, 0x61, 0x74, 0x65, 0x64, 0x5f, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x3a, 0x7b, 0x22, 0x69, 0x74, 0x65, 0x6d, 0x73, 0x22, 0x3a, 0x7b, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x7d, 0x2c, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x61, 0x72, 0x72, 0x61, 0x79, 0x22, 0x7d, 0x2c, 0x22, 0x73, 0x6f, 0x6d, 0x65, 0x5f, 0x65, 0x6e, 0x75, 0x6d, 0x22, 0x3a, 0x7b, 0x22, 0x65, 0x6e, 0x75, 0x6d, 0x22, 0x3a, 0x5b, 0x22, 0x53, 0x4f, 0x4d, 0x45, 0x5f, 0x45, 0x4e, 0x55, 0x4d, 0x5f, 0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x22, 0x2c, 0x22, 0x53, 0x4f, 0x4d, 0x45, 0x5f, 0x45, 0x4e, 0x55, 0x4d, 0x5f, 0x46, 0x49, 0x52, 0x53, 0x54, 0x22, 0x2c, 0x22, 0x53, 0x4f, 0x4d, 0x45, 0x5f, 0x45, 0x4e, 0x55, 0x4d, 0x5f, 0x53, 0x45, 0x43, 0x4f, 0x4e, 0x44, 0x22, 0x5d, 0x2c, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x7d, 0x2c, 0x22, 0x73, 0x6f, 0x6d, 0x65, 0x5f, 0x69, 0x6e, 0x74, 0x33, 0x32, 0x22, 0x3a, 0x7b, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x69, 0x6e, 0x74, 0x65, 0x67, 0x65, 0x72, 0x22, 0x7d, 0x2c, 0x22, 0x73, 0x6f, 0x6d, 0x65, 0x5f, 0x69, 0x6e, 0x74, 0x36, 0x34, 0x22, 0x3a, 0x7b, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x7d, 0x2c, 0x22, 0x73, 0x6f, 0x6d, 0x65, 0x5f, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x3a, 0x7b, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x22, 0x7d, 0x7d, 0x2c, 0x22, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64, 0x22, 0x3a, 0x5b, 0x5d, 0x2c, 0x22, 0x74, 0x79, 0x70, 0x65, 0x22, 0x3a, 0x22, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x22, 0x7d}}
)

// ExampleServiceServer is compatible with the grpc-go server interface.
type ExampleServiceServer interface {
	CreateExample(ctx context.Context, req *v1.CreateExampleRequest) (*v1.CreateExampleResponse, error)
}

func RegisterExampleServiceHandler(s *mcpserver.MCPServer, srv ExampleServiceServer) {
	s.AddTool(ExampleService_CreateExampleTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var req v1.CreateExampleRequest

		message := request.Params.Arguments

		marshaled, err := json.Marshal(message)
		if err != nil {
			return nil, err
		}

		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(marshaled, &req); err != nil {
			return nil, err
		}

		resp, err := srv.CreateExample(ctx, &req)
		if err != nil {
			return nil, err
		}

		marshaled, err = (protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}).Marshal(resp)
		if err != nil {
			return nil, err
		}

		return mcp.NewToolResultText(string(marshaled)), nil
	})
}

// ExampleServiceClient is compatible with the grpc-go client interface.
type ExampleServiceClient interface {
	CreateExample(ctx context.Context, req *v1.CreateExampleRequest, opts ...grpc.CallOption) (*v1.CreateExampleResponse, error)
}

// ConnectExampleServiceClient is compatible with the connectrpc-go client interface.
type ConnectExampleServiceClient interface {
	CreateExample(ctx context.Context, req *connect.Request[v1.CreateExampleRequest]) (*connect.Response[v1.CreateExampleResponse], error)
}

// ForwardToConnectExampleServiceClient registers a connectrpc client, to forward MCP calls to it.
func ForwardToConnectExampleServiceClient(s *mcpserver.MCPServer, client ConnectExampleServiceClient) {
	s.AddTool(ExampleService_CreateExampleTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var req v1.CreateExampleRequest

		message := request.Params.Arguments

		marshaled, err := json.Marshal(message)
		if err != nil {
			return nil, err
		}

		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(marshaled, &req); err != nil {
			return nil, err
		}

		resp, err := client.CreateExample(ctx, connect.NewRequest(&req))
		if err != nil {
			return nil, err
		}

		marshaled, err = (protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}).Marshal(resp.Msg)
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultText(string(marshaled)), nil
	})
}

// ForwardToExampleServiceClient registers a gRPC client, to forward MCP calls to it.
func ForwardToExampleServiceClient(s *mcpserver.MCPServer, client ExampleServiceClient) {
	s.AddTool(ExampleService_CreateExampleTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var req v1.CreateExampleRequest

		message := request.Params.Arguments

		marshaled, err := json.Marshal(message)
		if err != nil {
			return nil, err
		}

		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(marshaled, &req); err != nil {
			return nil, err
		}

		resp, err := client.CreateExample(ctx, &req)
		if err != nil {
			return nil, err
		}

		marshaled, err = (protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}).Marshal(resp)
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResultText(string(marshaled)), nil
	})
}
