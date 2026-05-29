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

//go:build external_llm

// Package conformancetest submits the generated MCP tool schemas to real LLM
// providers through ai-sdk-go (the production calling path) and asserts that
// the schema is accepted (no 400) and that the model fills it correctly, which
// round-trips back into the proto through the runtime decode transform.
//
// These tests are nightly and non-blocking. They are gated behind the
// `external_llm` build tag AND per-provider API-key presence, so a normal
// `go test ./...` never runs them.
package conformancetest

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/redpanda-data/ai-sdk-go/llm"
	"github.com/redpanda-data/ai-sdk-go/providers/anthropic"
	"github.com/redpanda-data/ai-sdk-go/providers/google"
	"github.com/redpanda-data/ai-sdk-go/providers/openai"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	testdatamcp "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata/testdatamcp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// provider names a live provider plus a factory that builds an ai-sdk-go model,
// skipping the (sub)test when the provider's API key is absent.
type provider struct {
	name     string
	envKey   string
	newModel func(ctx context.Context, t *testing.T) llm.Model
}

func providersUnderTest() []provider {
	return []provider{
		{
			name:   "openai",
			envKey: "OPENAI_API_KEY",
			newModel: func(_ context.Context, t *testing.T) llm.Model {
				p, err := openai.NewProvider(os.Getenv("OPENAI_API_KEY"))
				if err != nil {
					t.Fatalf("openai provider: %v", err)
				}
				m, err := p.NewModel("gpt-5.5")
				if err != nil {
					t.Fatalf("openai model: %v", err)
				}
				return m
			},
		},
		{
			// Anthropic doubles as Bedrock coverage: Bedrock validates the same
			// Anthropic tool input_schema.
			name:   "anthropic",
			envKey: "ANTHROPIC_API_KEY",
			newModel: func(_ context.Context, t *testing.T) llm.Model {
				p, err := anthropic.NewProvider(os.Getenv("ANTHROPIC_API_KEY"))
				if err != nil {
					t.Fatalf("anthropic provider: %v", err)
				}
				m, err := p.NewModel("claude-sonnet-4-6")
				if err != nil {
					t.Fatalf("anthropic model: %v", err)
				}
				return m
			},
		},
		{
			name:   "gemini",
			envKey: "GOOGLE_API_KEY",
			newModel: func(ctx context.Context, t *testing.T) llm.Model {
				p, err := google.NewProvider(ctx, os.Getenv("GOOGLE_API_KEY"))
				if err != nil {
					t.Fatalf("google provider: %v", err)
				}
				m, err := p.NewModel("gemini-2.5-flash")
				if err != nil {
					t.Fatalf("google model: %v", err)
				}
				return m
			},
		},
	}
}

// runOnAllProviders runs fn as a subtest per provider, skipping providers whose
// key is unset.
func runOnAllProviders(t *testing.T, fn func(t *testing.T, m llm.Model)) {
	t.Helper()
	ctx := context.Background()
	for _, p := range providersUnderTest() {
		p := p
		t.Run(p.name, func(t *testing.T) {
			if os.Getenv(p.envKey) == "" {
				t.Skipf("%s not set", p.envKey)
			}
			fn(t, p.newModel(ctx, t))
		})
	}
}

// callTool submits the tool to the model with ToolChoice=required. A nil error
// proves the provider accepted the input_schema (the original bug surfaced as a
// 400 here). It returns the raw arguments of the first tool call.
func callTool(t *testing.T, m llm.Model, tool runtime.Tool, prompt string) json.RawMessage {
	t.Helper()
	resp, err := m.Generate(context.Background(), &llm.Request{
		Messages: []llm.Message{llm.NewMessage(llm.RoleUser, llm.NewTextPart(prompt))},
		Tools: []llm.ToolDefinition{{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.RawInputSchema,
		}},
		ToolChoice: &llm.ToolChoice{Type: llm.ToolChoiceRequired},
	})
	if err != nil {
		t.Fatalf("provider rejected schema or call failed: %v", err)
	}
	reqs := resp.ToolRequests()
	if len(reqs) == 0 {
		t.Fatalf("no tool call returned")
	}
	if reqs[0].Name != tool.Name {
		t.Fatalf("called %q, want %q", reqs[0].Name, tool.Name)
	}
	return reqs[0].Arguments
}

// decodeInto runs the production decode pipeline on the model's arguments.
func decodeInto(t *testing.T, args json.RawMessage, msg proto.Message) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(args, &m); err != nil {
		t.Fatalf("unmarshal args: %v", err)
	}
	if err := runtime.DecodeArguments(msg.ProtoReflect().Descriptor(), m); err != nil {
		t.Fatalf("DecodeArguments: %v (args: %s)", err, args)
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(b, msg); err != nil {
		t.Fatalf("protojson: %v (decoded: %s)", err, b)
	}
}

// TestConformance_Basic is a sanity check on a tool with no oneof.
func TestConformance_Basic(t *testing.T) {
	runOnAllProviders(t, func(t *testing.T, m llm.Model) {
		args := callTool(t, m, testdatamcp.TestService_GetItemTool, "Get the item with ID 'item-123'.")
		var req testdata.GetItemRequest
		decodeInto(t, args, &req)
		if req.GetId() != "item-123" {
			t.Fatalf("id = %q, want item-123", req.GetId())
		}
	})
}

// TestConformance_Oneof is the core regression: a tool whose input has a oneof.
// Before the fix this schema was rejected (HTTP 400) by OpenAI/Anthropic. The
// discriminated-object rendering must be accepted and correctly filled.
func TestConformance_Oneof(t *testing.T) {
	runOnAllProviders(t, func(t *testing.T, m llm.Model) {
		args := callTool(t, m, testdatamcp.EdgeCaseService_MultipleOneofsTool,
			"Create a job named 'export'. Its source should be the URL https://example.com/data, and the output format should be JSON.")
		var req testdata.MultipleOneofsRequest
		decodeInto(t, args, &req)
		if req.GetName() == "" {
			t.Fatalf("name not filled: %+v", &req)
		}
		if _, ok := req.GetSource().(*testdata.MultipleOneofsRequest_Url); !ok {
			t.Fatalf("source oneof not set to url: %#v", req.GetSource())
		}
	})
}

// TestConformance_MessageOneof exercises a oneof whose member is a message.
func TestConformance_MessageOneof(t *testing.T) {
	runOnAllProviders(t, func(t *testing.T, m llm.Model) {
		args := callTool(t, m, testdatamcp.TestService_CreateItemTool,
			"Create an item named 'Widget'. It is a product priced at 9.99 with quantity 5.")
		var req testdata.CreateItemRequest
		decodeInto(t, args, &req)
		if req.GetName() == "" {
			t.Fatalf("name not filled")
		}
		if req.GetProduct() == nil {
			t.Fatalf("item_type oneof not set to product: %#v", req.GetItemType())
		}
	})
}

// TestConformance_RecursiveOneof exercises a recursive message nested inside a
// oneof (the discriminated wrapper plus recursion-depth placeholders).
func TestConformance_RecursiveOneof(t *testing.T) {
	runOnAllProviders(t, func(t *testing.T, m llm.Model) {
		args := callTool(t, m, testdatamcp.EdgeCaseService_OneofRecursiveTool,
			"Build a tree node whose value is 'root' with a single child whose value is 'child'.")
		var req testdata.OneofRecursiveRequest
		decodeInto(t, args, &req)
		if req.GetNode() == nil {
			t.Fatalf("node oneof not set: %+v", &req)
		}
	})
}
