// OpenAI conformance tests: validates that generated MCP tool schemas are
// accepted by the OpenAI API and that the model can correctly call tools.
package conformancetest

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"

	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	testdatamcp "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata/testdatamcp"
	"google.golang.org/protobuf/encoding/protojson"
)

const openaiModel = openai.ChatModelGPT4oMini

func skipIfNoOpenAIKey(t *testing.T) string {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Skip("OPENAI_API_KEY not set")
	}
	return key
}

func newOpenAIClient(t *testing.T) *openai.Client {
	key := skipIfNoOpenAIKey(t)
	client := openai.NewClient(option.WithAPIKey(key))
	return &client
}

// openaiToolFromSchema builds a ChatCompletionToolParam from raw schema bytes.
func openaiToolFromSchema(name, desc string, rawSchema json.RawMessage) openai.ChatCompletionToolParam {
	var params map[string]any
	if err := json.Unmarshal(rawSchema, &params); err != nil {
		panic(err)
	}
	return openai.ChatCompletionToolParam{
		Function: openai.FunctionDefinitionParam{
			Name:        name,
			Description: openai.String(desc),
			Parameters:  openai.FunctionParameters(params),
		},
	}
}

// callOpenAITool sends a prompt with a single tool and returns the parsed arguments.
func callOpenAITool(t *testing.T, client *openai.Client, tool openai.ChatCompletionToolParam, prompt string) map[string]any {
	t.Helper()
	g := NewWithT(t)
	ctx := context.Background()

	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openaiModel,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
		Tools: []openai.ChatCompletionToolParam{tool},
		ToolChoice: openai.ChatCompletionToolChoiceOptionUnionParam{
			OfAuto: param.NewOpt(string(openai.ChatCompletionToolChoiceOptionAutoRequired)),
		},
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp.Choices).ToNot(BeEmpty())
	g.Expect(resp.Choices[0].Message.ToolCalls).ToNot(BeEmpty(), "expected a tool call in response")

	toolCall := resp.Choices[0].Message.ToolCalls[0]
	var args map[string]any
	err = json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
	g.Expect(err).ToNot(HaveOccurred())
	return args
}

// TestOpenAIStandardSchemaAcceptance verifies that OpenAI accepts our standard
// (non-OpenAI-compat) schemas and produces valid tool calls.
func TestOpenAIStandardSchemaAcceptance(t *testing.T) {
	client := newOpenAIClient(t)

	tests := []struct {
		name       string
		toolName   string
		toolDesc   string
		toolSchema json.RawMessage
		prompt     string
		validateFn func(t *testing.T, args map[string]any)
	}{
		// NOTE: CreateItem standard schema has anyOf (oneofs) which OpenAI rejects.
		// This is expected - use OpenAI-compat schema instead. Tested in TestOpenAIOpenAISchemaAcceptance.
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
			tool := openaiToolFromSchema(tt.toolName, tt.toolDesc, tt.toolSchema)
			args := callOpenAITool(t, client, tool, tt.prompt)

			// Verify tool call args unmarshal to proto
			argsJSON, err := json.Marshal(args)
			NewWithT(t).Expect(err).ToNot(HaveOccurred())
			t.Logf("Standard schema args: %s", string(argsJSON))

			tt.validateFn(t, args)
		})
	}
}

// TestOpenAIOpenAISchemaAcceptance verifies OpenAI accepts our OpenAI-compatible
// schemas and that FixOpenAI produces valid proto JSON for round-trip.
func TestOpenAIOpenAISchemaAcceptance(t *testing.T) {
	client := newOpenAIClient(t)

	t.Run("CreateItem OpenAI round-trip", func(t *testing.T) {
		g := NewWithT(t)

		mcpTool := testdatamcp.TestService_CreateItemToolOpenAI
		tool := openaiToolFromSchema(mcpTool.Name, mcpTool.Description, mcpTool.RawInputSchema)

		args := callOpenAITool(t, client, tool,
			"Create a new item called 'Gadget' with description 'A cool gadget', "+
				"labels {env: staging, team: frontend}, and tags ['featured']. "+
				"Make it a product with price 49.99 and quantity 10. No thumbnail needed (set to null).",
		)

		// Apply FixOpenAI to convert OpenAI-compat format back to proto-compatible format
		runtime.FixOpenAI((&testdata.CreateItemRequest{}).ProtoReflect().Descriptor(), args)

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
		tool := openaiToolFromSchema(mcpTool.Name, mcpTool.Description, mcpTool.RawInputSchema)

		args := callOpenAITool(t, client, tool, "Get the item with ID 'abc-456'.")

		runtime.FixOpenAI((&testdata.GetItemRequest{}).ProtoReflect().Descriptor(), args)
		argsJSON, err := json.Marshal(args)
		g.Expect(err).ToNot(HaveOccurred())

		var req testdata.GetItemRequest
		err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(argsJSON, &req)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(req.Id).To(Equal("abc-456"))
	})
}

// TestOpenAIValidationSchemaAcceptance tests that validation constraints
// in the schema help OpenAI produce conformant output.
func TestOpenAIValidationSchemaAcceptance(t *testing.T) {
	client := newOpenAIClient(t)
	g := NewWithT(t)

	mcpTool := testdatamcp.TestService_TestValidationTool
	tool := openaiToolFromSchema(mcpTool.Name, mcpTool.Description, mcpTool.RawInputSchema)

	args := callOpenAITool(t, client, tool,
		"Validate a user: resource_group_id '550e8400-e29b-41d4-a716-446655440000', "+
			"email 'test@example.com', username 'JohnDoe', name 'John', age 30, timestamp 1700000000.",
	)

	argsJSON, err := json.Marshal(args)
	g.Expect(err).ToNot(HaveOccurred())

	var req testdata.TestValidationRequest
	err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(argsJSON, &req)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(req.Email).To(Equal("test@example.com"))
	g.Expect(req.Username).To(Equal("JohnDoe"))
	t.Logf("Validation round-trip successful: email=%s username=%s", req.Email, req.Username)
}

// TestOpenAIEdgeCaseSchemaAcceptance verifies that OpenAI accepts edge case
// schemas (all scalar types, enum fields, map variants) and produces valid
// tool calls that unmarshal back to proto.
func TestOpenAIEdgeCaseSchemaAcceptance(t *testing.T) {
	client := newOpenAIClient(t)

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
			// NOTE: MapVariants uses OpenAI-compat schema because standard maps with
			// additionalProperties don't round-trip through OpenAI's structured output.
			name:       "MapVariants",
			toolName:   testdatamcp.EdgeCaseService_MapVariantsToolOpenAI.Name,
			toolDesc:   testdatamcp.EdgeCaseService_MapVariantsToolOpenAI.Description,
			toolSchema: testdatamcp.EdgeCaseService_MapVariantsToolOpenAI.RawInputSchema,
			prompt:     "Create map variants with string_to_string {\"color\": \"red\", \"size\": \"large\"}, string_to_double {\"price\": 9.99}, and string_to_bool {\"active\": true}. Set all other map fields to empty arrays.",
			unmarshal: func(t *testing.T, args map[string]any) {
				g := NewWithT(t)
				// Apply FixOpenAI since we're using OpenAI-compat schema
				runtime.FixOpenAI((&testdata.MapVariantsRequest{}).ProtoReflect().Descriptor(), args)
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
			tool := openaiToolFromSchema(tt.toolName, tt.toolDesc, tt.toolSchema)
			args := callOpenAITool(t, client, tool, tt.prompt)
			tt.unmarshal(t, args)
		})
	}
}

// TestOpenAIDeepNestingAcceptance verifies that OpenAI can handle deeply nested
// message schemas and produce valid tool calls that unmarshal to proto.
func TestOpenAIDeepNestingAcceptance(t *testing.T) {
	client := newOpenAIClient(t)
	g := NewWithT(t)

	mcpTool := testdatamcp.EdgeCaseService_DeepNestingTool
	tool := openaiToolFromSchema(mcpTool.Name, mcpTool.Description, mcpTool.RawInputSchema)

	args := callOpenAITool(t, client, tool,
		"Create a deeply nested structure. The middle layer should have an inner message with id 'inner-1' and tags {\"env\": \"prod\"}. "+
			"Also include a list of middles with one entry that has an inner with id 'inner-2'. "+
			"Add named_items with key 'primary' mapping to an inner with id 'inner-3'.",
	)

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
