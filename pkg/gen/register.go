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

package gen

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Handler is a function that handles an MCP tool call for a specific RPC method.
// It receives the unmarshaled proto request and returns a proto response.
// This is the interface users implement to handle tool calls dynamically.
type Handler func(ctx context.Context, method protoreflect.MethodDescriptor, req proto.Message) (proto.Message, error)

// NewMessage creates a new empty proto message for the given descriptor.
// Users must provide this because protoreflect descriptors alone can't instantiate messages.
type NewMessage func(descriptor protoreflect.MessageDescriptor) proto.Message

// DynamicNewMessage creates proto messages using dynamicpb. This is the default
// NewMessage implementation when none is provided. It works with any descriptor
// but the resulting messages are dynamic (not concrete Go types).
func DynamicNewMessage(md protoreflect.MessageDescriptor) proto.Message {
	return dynamicpb.NewMessage(md)
}

// RegisterServiceOptions controls how a service is registered as MCP tools.
type RegisterServiceOptions struct {
	// Provider selects the schema mode (standard or OpenAI-compatible).
	Provider runtime.LLMProvider

	// ExtraProperties adds additional properties to all tool schemas.
	ExtraProperties []runtime.ExtraProperty

	// NewMessage creates new proto message instances from descriptors.
	// If nil, defaults to DynamicNewMessage (uses dynamicpb).
	NewMessage NewMessage

	// CommentProvider optionally returns the leading comment for an RPC method.
	// If nil, the tool description will be empty.
	CommentProvider func(method protoreflect.MethodDescriptor) string
}

// RegisterService dynamically registers all unary RPCs from a protobuf service
// descriptor as MCP tools. This is the reflection-based alternative to the
// static code generation approach.
//
// Unlike the generated code, this works at runtime with any service descriptor,
// making it suitable for proxy/gateway scenarios where you don't have the
// generated types at compile time.
func RegisterService(s runtime.MCPServer, sd protoreflect.ServiceDescriptor, handler Handler, opts RegisterServiceOptions) {
	if opts.NewMessage == nil {
		opts.NewMessage = DynamicNewMessage
	}
	openAI := opts.Provider == runtime.LLMProviderOpenAI
	schemaOpts := SchemaOptions{OpenAICompat: openAI}

	for i := 0; i < sd.Methods().Len(); i++ {
		method := sd.Methods().Get(i)

		// Skip streaming methods
		if method.IsStreamingClient() || method.IsStreamingServer() {
			continue
		}

		comment := ""
		if opts.CommentProvider != nil {
			comment = opts.CommentProvider(method)
		}

		// Generate tool schema
		toolName := MangleHeadIfTooLong(
			strings.ReplaceAll(string(method.FullName()), ".", "_"),
			64,
		)
		inputSchema := MessageSchema(method.Input(), schemaOpts)
		if openAI {
			// Top-level schema must be plain "object", not ["object","null"]
			inputSchema["type"] = "object"
		}

		marshaledSchema, err := json.Marshal(inputSchema)
		if err != nil {
			panic(err)
		}

		tool := runtime.Tool{
			Name:           toolName,
			Description:    CleanComment(comment),
			RawInputSchema: json.RawMessage(marshaledSchema),
		}

		// Apply extra properties
		if len(opts.ExtraProperties) > 0 {
			tool = runtime.AddExtraPropertiesToTool(tool, opts.ExtraProperties)
		}

		// Capture loop variable
		md := method
		newMsg := opts.NewMessage

		s.AddTool(tool, func(ctx context.Context, request *runtime.CallToolRequest) (*runtime.CallToolResult, error) {
			message := request.Arguments

			// Extract extra properties into context and remove them from
			// the arguments map so they don't leak into proto unmarshaling.
			for _, prop := range opts.ExtraProperties {
				if propVal, ok := message[prop.Name]; ok {
					ctx = context.WithValue(ctx, prop.ContextKey, propVal)
					delete(message, prop.Name)
				}
			}

			// Apply OpenAI fix if needed
			if openAI {
				runtime.FixOpenAI(md.Input(), message)
			}

			// Marshal to JSON, then unmarshal into proto
			marshaled, err := json.Marshal(message)
			if err != nil {
				return nil, err
			}

			req := newMsg(md.Input())
			if req == nil {
				return nil, fmt.Errorf("NewMessage returned nil for %s", md.Input().FullName())
			}
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(marshaled, req); err != nil {
				return nil, err
			}

			// Call handler
			resp, err := handler(ctx, md, req)
			if err != nil {
				return runtime.HandleError(err)
			}

			// Marshal response
			marshaled, err = (protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}).Marshal(resp)
			if err != nil {
				return nil, err
			}

			return runtime.NewToolResultText(string(marshaled)), nil
		})
	}
}
