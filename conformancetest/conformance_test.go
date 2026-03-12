//go:build integration
// +build integration

// Package conformancetest validates that generated MCP tool schemas are accepted
// by real LLM providers and that the providers can correctly call the tools.
// This is the "does it actually work end-to-end" test suite.
package conformancetest

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	. "github.com/onsi/gomega"
	"google.golang.org/genai"

	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	testdatamcp "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata/testdatamcp"
	"google.golang.org/protobuf/encoding/protojson"
)

const geminiModel = "gemini-2.5-flash"

func skipIfNoGoogleKey(t *testing.T) string {
	key := os.Getenv("GOOGLE_API_KEY")
	if key == "" {
		t.Skip("GOOGLE_API_KEY not set")
	}
	return key
}

func newGeminiClient(t *testing.T) *genai.Client {
	key := skipIfNoGoogleKey(t)
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: key,
	})
	if err != nil {
		t.Fatalf("Failed to create Gemini client: %v", err)
	}
	return client
}

// TestGeminiStandardSchemaAcceptance verifies that Gemini accepts our standard
// (non-OpenAI) schemas and can produce valid tool calls.
func TestGeminiStandardSchemaAcceptance(t *testing.T) {
	client := newGeminiClient(t)
	ctx := context.Background()

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

			funcDecl := &genai.FunctionDeclaration{
				Name:        tt.toolName,
				Description: tt.toolDesc,
			}
			var params map[string]any
			err := json.Unmarshal(tt.toolSchema, &params)
			g.Expect(err).ToNot(HaveOccurred())
			funcDecl.ParametersJsonSchema = params

			tools := []*genai.Tool{
				{FunctionDeclarations: []*genai.FunctionDeclaration{funcDecl}},
			}

			contents := []*genai.Content{
				{
					Role:  genai.RoleUser,
					Parts: []*genai.Part{genai.NewPartFromText(tt.prompt)},
				},
			}

			resp, err := client.Models.GenerateContent(ctx, geminiModel, contents, &genai.GenerateContentConfig{
				Tools: tools,
				ToolConfig: &genai.ToolConfig{
					FunctionCallingConfig: &genai.FunctionCallingConfig{
						Mode: genai.FunctionCallingConfigModeAny,
					},
				},
			})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(resp.Candidates).ToNot(BeEmpty())
			g.Expect(resp.Candidates[0].Content).ToNot(BeNil())

			// Find the tool call
			var toolCall *genai.FunctionCall
			for _, part := range resp.Candidates[0].Content.Parts {
				if part.FunctionCall != nil {
					toolCall = part.FunctionCall
					break
				}
			}
			g.Expect(toolCall).ToNot(BeNil(), "expected a tool call in response")
			g.Expect(toolCall.Name).To(Equal(tt.toolName))

			tt.validateFn(t, toolCall.Args)
		})
	}
}

// TestGeminiOpenAISchemaAcceptance verifies Gemini accepts our OpenAI-compatible schemas
// and that the FixOpenAI transform produces valid proto JSON.
func TestGeminiOpenAISchemaAcceptance(t *testing.T) {
	client := newGeminiClient(t)
	ctx := context.Background()
	g := NewWithT(t)

	t.Run("CreateItem OpenAI round-trip", func(t *testing.T) {
		g := NewWithT(t)

		tool := testdatamcp.TestService_CreateItemToolOpenAI

		var params map[string]any
		err := json.Unmarshal(tool.RawInputSchema, &params)
		g.Expect(err).ToNot(HaveOccurred())

		funcDecl := &genai.FunctionDeclaration{
			Name:                 tool.Name,
			Description:          tool.Description,
			ParametersJsonSchema: params,
		}

		tools := []*genai.Tool{
			{FunctionDeclarations: []*genai.FunctionDeclaration{funcDecl}},
		}

		contents := []*genai.Content{
			{
				Role:  genai.RoleUser,
				Parts: []*genai.Part{genai.NewPartFromText("Create a new item called 'Gadget' with description 'A cool gadget', labels {env: staging, team: frontend}, and tags ['featured']. Make it a product with price 49.99 and quantity 10. No thumbnail needed (set to null).")},
			},
		}

		resp, err := client.Models.GenerateContent(ctx, geminiModel, contents, &genai.GenerateContentConfig{
			Tools: tools,
			ToolConfig: &genai.ToolConfig{
				FunctionCallingConfig: &genai.FunctionCallingConfig{
					Mode: genai.FunctionCallingConfigModeAny,
				},
			},
		})
		g.Expect(err).ToNot(HaveOccurred())

		// Extract tool call
		var toolCall *genai.FunctionCall
		for _, part := range resp.Candidates[0].Content.Parts {
			if part.FunctionCall != nil {
				toolCall = part.FunctionCall
				break
			}
		}
		g.Expect(toolCall).ToNot(BeNil())
		g.Expect(toolCall.Name).To(Equal(tool.Name))

		// Apply FixOpenAI
		args := toolCall.Args
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
		tool := testdatamcp.TestService_GetItemToolOpenAI

		var params map[string]any
		err := json.Unmarshal(tool.RawInputSchema, &params)
		g.Expect(err).ToNot(HaveOccurred())

		funcDecl := &genai.FunctionDeclaration{
			Name:                 tool.Name,
			Description:          tool.Description,
			ParametersJsonSchema: params,
		}

		tools := []*genai.Tool{
			{FunctionDeclarations: []*genai.FunctionDeclaration{funcDecl}},
		}

		contents := []*genai.Content{
			{
				Role:  genai.RoleUser,
				Parts: []*genai.Part{genai.NewPartFromText("Get the item with ID 'abc-456'.")},
			},
		}

		resp, err := client.Models.GenerateContent(ctx, geminiModel, contents, &genai.GenerateContentConfig{
			Tools: tools,
			ToolConfig: &genai.ToolConfig{
				FunctionCallingConfig: &genai.FunctionCallingConfig{
					Mode: genai.FunctionCallingConfigModeAny,
				},
			},
		})
		g.Expect(err).ToNot(HaveOccurred())

		var toolCall *genai.FunctionCall
		for _, part := range resp.Candidates[0].Content.Parts {
			if part.FunctionCall != nil {
				toolCall = part.FunctionCall
				break
			}
		}
		g.Expect(toolCall).ToNot(BeNil())

		// Apply fix and unmarshal
		args := toolCall.Args
		runtime.FixOpenAI((&testdata.GetItemRequest{}).ProtoReflect().Descriptor(), args)
		argsJSON, err := json.Marshal(args)
		g.Expect(err).ToNot(HaveOccurred())

		var req testdata.GetItemRequest
		err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(argsJSON, &req)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(req.Id).To(Equal("abc-456"))
	})
}

// TestGeminiValidationSchemaAcceptance tests that validation constraints
// in the schema help Gemini produce conformant output.
func TestGeminiValidationSchemaAcceptance(t *testing.T) {
	client := newGeminiClient(t)
	ctx := context.Background()
	g := NewWithT(t)

	tool := testdatamcp.TestService_TestValidationTool

	var params map[string]any
	err := json.Unmarshal(tool.RawInputSchema, &params)
	g.Expect(err).ToNot(HaveOccurred())

	funcDecl := &genai.FunctionDeclaration{
		Name:                 tool.Name,
		Description:          tool.Description,
		ParametersJsonSchema: params,
	}

	tools := []*genai.Tool{
		{FunctionDeclarations: []*genai.FunctionDeclaration{funcDecl}},
	}

	contents := []*genai.Content{
		{
			Role: genai.RoleUser,
			Parts: []*genai.Part{genai.NewPartFromText(
				"Validate a user: resource_group_id '550e8400-e29b-41d4-a716-446655440000', " +
					"email 'test@example.com', username 'JohnDoe', name 'John', age 30, timestamp 1700000000.",
			)},
		},
	}

	resp, err := client.Models.GenerateContent(ctx, geminiModel, contents, &genai.GenerateContentConfig{
		Tools: tools,
		ToolConfig: &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: genai.FunctionCallingConfigModeAny,
			},
		},
	})
	g.Expect(err).ToNot(HaveOccurred())

	var toolCall *genai.FunctionCall
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.FunctionCall != nil {
			toolCall = part.FunctionCall
			break
		}
	}
	g.Expect(toolCall).ToNot(BeNil())

	argsJSON, err := json.Marshal(toolCall.Args)
	g.Expect(err).ToNot(HaveOccurred())

	var req testdata.TestValidationRequest
	err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(argsJSON, &req)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(req.Email).To(Equal("test@example.com"))
	g.Expect(req.Username).To(Equal("JohnDoe"))
	t.Logf("Validation round-trip successful: email=%s username=%s", req.Email, req.Username)
}

// TestGeminiEdgeCaseSchemaAcceptance verifies that Gemini accepts edge case
// schemas (all scalar types, enum fields, map variants) and produces valid
// tool calls that unmarshal back to proto.
func TestGeminiEdgeCaseSchemaAcceptance(t *testing.T) {
	client := newGeminiClient(t)
	ctx := context.Background()

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

			var params map[string]any
			err := json.Unmarshal(tt.toolSchema, &params)
			g.Expect(err).ToNot(HaveOccurred())

			funcDecl := &genai.FunctionDeclaration{
				Name:                 tt.toolName,
				Description:          tt.toolDesc,
				ParametersJsonSchema: params,
			}

			tools := []*genai.Tool{
				{FunctionDeclarations: []*genai.FunctionDeclaration{funcDecl}},
			}

			contents := []*genai.Content{
				{
					Role:  genai.RoleUser,
					Parts: []*genai.Part{genai.NewPartFromText(tt.prompt)},
				},
			}

			resp, err := client.Models.GenerateContent(ctx, geminiModel, contents, &genai.GenerateContentConfig{
				Tools: tools,
				ToolConfig: &genai.ToolConfig{
					FunctionCallingConfig: &genai.FunctionCallingConfig{
						Mode: genai.FunctionCallingConfigModeAny,
					},
				},
			})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(resp.Candidates).ToNot(BeEmpty())
			g.Expect(resp.Candidates[0].Content).ToNot(BeNil())

			var toolCall *genai.FunctionCall
			for _, part := range resp.Candidates[0].Content.Parts {
				if part.FunctionCall != nil {
					toolCall = part.FunctionCall
					break
				}
			}
			g.Expect(toolCall).ToNot(BeNil(), "expected a tool call in response")
			g.Expect(toolCall.Name).To(Equal(tt.toolName))

			tt.unmarshal(t, toolCall.Args)
		})
	}
}

// TestGeminiDeepNestingAcceptance verifies that Gemini can handle deeply nested
// message schemas and produce valid tool calls that unmarshal to proto.
func TestGeminiDeepNestingAcceptance(t *testing.T) {
	client := newGeminiClient(t)
	ctx := context.Background()
	g := NewWithT(t)

	tool := testdatamcp.EdgeCaseService_DeepNestingTool

	var params map[string]any
	err := json.Unmarshal(tool.RawInputSchema, &params)
	g.Expect(err).ToNot(HaveOccurred())

	funcDecl := &genai.FunctionDeclaration{
		Name:                 tool.Name,
		Description:          tool.Description,
		ParametersJsonSchema: params,
	}

	tools := []*genai.Tool{
		{FunctionDeclarations: []*genai.FunctionDeclaration{funcDecl}},
	}

	contents := []*genai.Content{
		{
			Role: genai.RoleUser,
			Parts: []*genai.Part{genai.NewPartFromText(
				"Create a deeply nested structure. The middle layer should have an inner message with id 'inner-1' and tags {\"env\": \"prod\"}. " +
					"Also include a list of middles with one entry that has an inner with id 'inner-2'. " +
					"Add named_items with key 'primary' mapping to an inner with id 'inner-3'.",
			)},
		},
	}

	resp, err := client.Models.GenerateContent(ctx, geminiModel, contents, &genai.GenerateContentConfig{
		Tools: tools,
		ToolConfig: &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: genai.FunctionCallingConfigModeAny,
			},
		},
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resp.Candidates).ToNot(BeEmpty())
	g.Expect(resp.Candidates[0].Content).ToNot(BeNil())

	var toolCall *genai.FunctionCall
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.FunctionCall != nil {
			toolCall = part.FunctionCall
			break
		}
	}
	g.Expect(toolCall).ToNot(BeNil(), "expected a tool call in response")
	g.Expect(toolCall.Name).To(Equal(tool.Name))

	argsJSON, err := json.Marshal(toolCall.Args)
	g.Expect(err).ToNot(HaveOccurred())

	var req testdata.DeepNestingRequest
	err = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(argsJSON, &req)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(req.Middle).ToNot(BeNil(), "expected middle to be populated")
	g.Expect(req.Middle.Inner).ToNot(BeNil(), "expected middle.inner to be populated")
	g.Expect(req.Middle.Inner.Id).To(Equal("inner-1"))
	t.Logf("DeepNesting round-trip: middle.inner.id=%s middles=%d", req.Middle.Inner.Id, len(req.Middles))
}
