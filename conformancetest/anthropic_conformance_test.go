//go:build integration
// +build integration

// Package conformancetest validates that generated MCP tool schemas are accepted
// by real LLM providers and that the providers can correctly call the tools.
package conformancetest

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	. "github.com/onsi/gomega"

	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	testdatamcp "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata/testdatamcp"
	"google.golang.org/protobuf/encoding/protojson"
)

const anthropicModel = anthropic.ModelClaudeHaiku4_5

func skipIfNoAnthropicKey(t *testing.T) string {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}
	return key
}

func newAnthropicClient(t *testing.T) *anthropic.Client {
	key := skipIfNoAnthropicKey(t)
	client := anthropic.NewClient(option.WithAPIKey(key))
	return &client
}

// anthropicToolFromSchema builds a ToolUnionParam from our raw JSON schema.
func anthropicToolFromSchema(name, description string, rawSchema json.RawMessage) anthropic.ToolUnionParam {
	var schema anthropic.ToolInputSchemaParam
	if err := json.Unmarshal(rawSchema, &schema); err != nil {
		panic("failed to unmarshal schema: " + err.Error())
	}
	tool := anthropic.ToolUnionParamOfTool(schema, name)
	tool.OfTool.Description = param.NewOpt(description)
	return tool
}

// extractAnthropicToolUse finds the first tool_use block in the response.
func extractAnthropicToolUse(t *testing.T, msg *anthropic.Message) (name string, input map[string]any) {
	t.Helper()
	g := NewWithT(t)
	g.Expect(msg.Content).ToNot(BeEmpty(), "expected content blocks in response")

	for _, block := range msg.Content {
		if block.Type == "tool_use" {
			toolUse := block.AsToolUse()
			var args map[string]any
			err := json.Unmarshal(toolUse.Input, &args)
			g.Expect(err).ToNot(HaveOccurred())
			return toolUse.Name, args
		}
	}
	t.Fatal("no tool_use block found in response")
	return "", nil
}

// callAnthropicWithTool sends a message with a single tool and forces tool use.
func callAnthropicWithTool(t *testing.T, client *anthropic.Client, tool anthropic.ToolUnionParam, prompt string) *anthropic.Message {
	t.Helper()
	g := NewWithT(t)
	ctx := context.Background()

	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropicModel,
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
		Tools: []anthropic.ToolUnionParam{tool},
		ToolChoice: anthropic.ToolChoiceUnionParam{
			OfAny: &anthropic.ToolChoiceAnyParam{},
		},
	})
	g.Expect(err).ToNot(HaveOccurred())
	return resp
}

// TestAnthropicStandardSchemaAcceptance verifies that Anthropic accepts our standard
// (non-OpenAI) schemas and can produce valid tool calls.
func TestAnthropicStandardSchemaAcceptance(t *testing.T) {
	client := newAnthropicClient(t)

	tests := []struct {
		name       string
		toolName   string
		toolDesc   string
		toolSchema json.RawMessage
		prompt     string
		validateFn func(t *testing.T, args map[string]any)
	}{
		{
			name:       "CreateItem standard",
			toolName:   testdatamcp.TestService_CreateItemTool.Name,
			toolDesc:   testdatamcp.TestService_CreateItemTool.Description,
			toolSchema: testdatamcp.TestService_CreateItemTool.RawInputSchema,
			prompt:     "Create a new item called 'Widget' with description 'A test widget', labels {color: blue}, and tags ['sale']. Make it a product with price 9.99 and quantity 5.",
			validateFn: func(t *testing.T, args map[string]any) {
				g := NewWithT(t)
				g.Expect(args).To(HaveKey("name"))
			},
		},
		{
			name:       "GetItem standard",
			toolName:   testdatamcp.TestService_GetItemTool.Name,
			toolDesc:   testdatamcp.TestService_GetItemTool.Description,
			toolSchema: testdatamcp.TestService_GetItemTool.RawInputSchema,
			prompt:     "Get the item with ID 'item-123'.",
			validateFn: func(t *testing.T, args map[string]any) {
				g := NewWithT(t)
				g.Expect(args).To(HaveKey("id"))
				g.Expect(args["id"]).To(Equal("item-123"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			tool := anthropicToolFromSchema(tt.toolName, tt.toolDesc, tt.toolSchema)
			resp := callAnthropicWithTool(t, client, tool, tt.prompt)

			name, args := extractAnthropicToolUse(t, resp)
			g.Expect(name).To(Equal(tt.toolName))

			tt.validateFn(t, args)
		})
	}
}

// TestAnthropicOpenAISchemaAcceptance verifies Anthropic accepts our OpenAI-compatible schemas
// and that the FixOpenAI transform produces valid proto JSON.
func TestAnthropicOpenAISchemaAcceptance(t *testing.T) {
	client := newAnthropicClient(t)

	t.Run("CreateItem OpenAI round-trip", func(t *testing.T) {
		g := NewWithT(t)

		mcpTool := testdatamcp.TestService_CreateItemToolOpenAI
		tool := anthropicToolFromSchema(mcpTool.Name, mcpTool.Description, mcpTool.RawInputSchema)
		resp := callAnthropicWithTool(t, client, tool,
			"Create a new item called 'Gadget' with description 'A cool gadget', labels {env: staging, team: frontend}, and tags ['featured']. Make it a product with price 49.99 and quantity 10. No thumbnail needed (set to null).")

		name, args := extractAnthropicToolUse(t, resp)
		g.Expect(name).To(Equal(mcpTool.Name))

		// Apply FixOpenAI
		runtime.FixOpenAI((&testdata.CreateItemRequest{}).ProtoReflect().Descriptor(), args)

		// Unmarshal into proto
		argsJSON, err := json.Marshal(args)
		g.Expect(err).ToNot(HaveOccurred())

		var req testdata.CreateItemRequest
		err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(argsJSON, &req)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(req.Name).To(Equal("Gadget"))
		t.Logf("Round-trip successful: name=%s", req.Name)
	})

	t.Run("GetItem OpenAI simple", func(t *testing.T) {
		g := NewWithT(t)

		mcpTool := testdatamcp.TestService_GetItemToolOpenAI
		tool := anthropicToolFromSchema(mcpTool.Name, mcpTool.Description, mcpTool.RawInputSchema)
		resp := callAnthropicWithTool(t, client, tool, "Get the item with ID 'abc-456'.")

		_, args := extractAnthropicToolUse(t, resp)

		// Apply fix and unmarshal
		runtime.FixOpenAI((&testdata.GetItemRequest{}).ProtoReflect().Descriptor(), args)
		argsJSON, err := json.Marshal(args)
		g.Expect(err).ToNot(HaveOccurred())

		var req testdata.GetItemRequest
		err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(argsJSON, &req)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(req.Id).To(Equal("abc-456"))
	})
}

// TestAnthropicValidationSchemaAcceptance tests that validation constraints
// in the schema help Anthropic produce conformant output.
func TestAnthropicValidationSchemaAcceptance(t *testing.T) {
	client := newAnthropicClient(t)
	g := NewWithT(t)

	mcpTool := testdatamcp.TestService_TestValidationTool
	tool := anthropicToolFromSchema(mcpTool.Name, mcpTool.Description, mcpTool.RawInputSchema)
	resp := callAnthropicWithTool(t, client, tool,
		"Validate a user: resource_group_id '550e8400-e29b-41d4-a716-446655440000', "+
			"email 'test@example.com', username 'JohnDoe', name 'John', age 30, timestamp 1700000000.")

	_, args := extractAnthropicToolUse(t, resp)

	argsJSON, err := json.Marshal(args)
	g.Expect(err).ToNot(HaveOccurred())

	var req testdata.TestValidationRequest
	err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(argsJSON, &req)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(req.Email).To(Equal("test@example.com"))
	g.Expect(req.Username).To(Equal("JohnDoe"))
	t.Logf("Validation round-trip successful: email=%s username=%s", req.Email, req.Username)
}

// TestAnthropicEdgeCaseSchemaAcceptance verifies that Anthropic accepts edge case
// schemas (all scalar types, enum fields, map variants) and produces valid
// tool calls that unmarshal back to proto.
func TestAnthropicEdgeCaseSchemaAcceptance(t *testing.T) {
	client := newAnthropicClient(t)

	tests := []struct {
		name       string
		toolName   string
		toolDesc   string
		toolSchema json.RawMessage
		prompt     string
		unmarshal  func(t *testing.T, args map[string]any)
	}{
		{
			name:       "AllScalarTypes",
			toolName:   testdatamcp.EdgeCaseService_AllScalarTypesTool.Name,
			toolDesc:   testdatamcp.EdgeCaseService_AllScalarTypesTool.Description,
			toolSchema: testdatamcp.EdgeCaseService_AllScalarTypesTool.RawInputSchema,
			prompt:     "Set all scalar fields: double 3.14, float 2.72, int32 42, int64 100, uint32 7, uint64 8, sint32 -5, sint64 -10, fixed32 11, fixed64 12, sfixed32 -13, sfixed64 -14, bool true, string 'hello', bytes 'dGVzdA==' (base64 for 'test').",
			unmarshal: func(t *testing.T, args map[string]any) {
				g := NewWithT(t)
				argsJSON, err := json.Marshal(args)
				g.Expect(err).ToNot(HaveOccurred())

				var req testdata.AllScalarTypesRequest
				err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(argsJSON, &req)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(req.StringField).To(Equal("hello"))
				g.Expect(req.BoolField).To(BeTrue())
				t.Logf("AllScalarTypes round-trip: string=%s bool=%v int32=%d", req.StringField, req.BoolField, req.Int32Field)
			},
		},
		{
			name:       "EnumFields",
			toolName:   testdatamcp.EdgeCaseService_EnumFieldsTool.Name,
			toolDesc:   testdatamcp.EdgeCaseService_EnumFieldsTool.Description,
			toolSchema: testdatamcp.EdgeCaseService_EnumFieldsTool.RawInputSchema,
			prompt:     "Set priority to PRIORITY_HIGH and priorities to [PRIORITY_LOW, PRIORITY_MEDIUM].",
			unmarshal: func(t *testing.T, args map[string]any) {
				g := NewWithT(t)
				argsJSON, err := json.Marshal(args)
				g.Expect(err).ToNot(HaveOccurred())

				var req testdata.EnumFieldsRequest
				err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(argsJSON, &req)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(req.Priority).To(Equal(testdata.Priority_PRIORITY_HIGH))
				t.Logf("EnumFields round-trip: priority=%s priorities=%v", req.Priority, req.Priorities)
			},
		},
		{
			name:       "MapVariants",
			toolName:   testdatamcp.EdgeCaseService_MapVariantsTool.Name,
			toolDesc:   testdatamcp.EdgeCaseService_MapVariantsTool.Description,
			toolSchema: testdatamcp.EdgeCaseService_MapVariantsTool.RawInputSchema,
			prompt:     "Create map variants with string_to_string {\"color\": \"red\", \"size\": \"large\"}, string_to_double {\"price\": 9.99}, and string_to_bool {\"active\": true}.",
			unmarshal: func(t *testing.T, args map[string]any) {
				g := NewWithT(t)
				argsJSON, err := json.Marshal(args)
				g.Expect(err).ToNot(HaveOccurred())

				var req testdata.MapVariantsRequest
				err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(argsJSON, &req)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(req.StringToString).To(HaveKeyWithValue("color", "red"))
				t.Logf("MapVariants round-trip: string_to_string=%v", req.StringToString)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			tool := anthropicToolFromSchema(tt.toolName, tt.toolDesc, tt.toolSchema)
			resp := callAnthropicWithTool(t, client, tool, tt.prompt)

			name, args := extractAnthropicToolUse(t, resp)
			g.Expect(name).To(Equal(tt.toolName))

			tt.unmarshal(t, args)
		})
	}
}

// TestAnthropicDeepNestingAcceptance verifies that Anthropic can handle deeply nested
// message schemas and produce valid tool calls that unmarshal to proto.
func TestAnthropicDeepNestingAcceptance(t *testing.T) {
	client := newAnthropicClient(t)
	g := NewWithT(t)

	mcpTool := testdatamcp.EdgeCaseService_DeepNestingTool
	tool := anthropicToolFromSchema(mcpTool.Name, mcpTool.Description, mcpTool.RawInputSchema)
	resp := callAnthropicWithTool(t, client, tool,
		"Create a deeply nested structure. The middle layer should have an inner message with id 'inner-1' and tags {\"env\": \"prod\"}. "+
			"Also include a list of middles with one entry that has an inner with id 'inner-2'. "+
			"Add named_items with key 'primary' mapping to an inner with id 'inner-3'.")

	name, args := extractAnthropicToolUse(t, resp)
	g.Expect(name).To(Equal(mcpTool.Name))

	argsJSON, err := json.Marshal(args)
	g.Expect(err).ToNot(HaveOccurred())

	var req testdata.DeepNestingRequest
	err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(argsJSON, &req)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(req.Middle).ToNot(BeNil(), "expected middle to be populated")
	g.Expect(req.Middle.Inner).ToNot(BeNil(), "expected middle.inner to be populated")
	g.Expect(req.Middle.Inner.Id).To(Equal("inner-1"))
	t.Logf("DeepNesting round-trip: middle.inner.id=%s middles=%d", req.Middle.Inner.Id, len(req.Middles))
}
