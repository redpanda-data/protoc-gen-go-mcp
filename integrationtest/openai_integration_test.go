//go:build integration
// +build integration

package integrationtest

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	// Generated code (will be created by buf generate)
	integrationtestv1mcp "github.com/redpanda-data/protoc-gen-go-mcp/integrationtest/gen/go/integrationtest/v1/integrationtestv1mcp"
)

func TestOpenAIIntegration(t *testing.T) {
	// Skip if no API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	g := NewWithT(t)
	ctx := context.Background()

	// 1. Initialize OpenAI client
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
	)

	t.Run("CreateItem with OpenAI compatibility", func(t *testing.T) {
		// 2. Get the generated OpenAI-compatible tool
		tool := integrationtestv1mcp.TestService_CreateItemToolOpenAI

		// 3. Create a function tool from our MCP tool
		var params map[string]interface{}
		err := json.Unmarshal(tool.RawInputSchema, &params)
		g.Expect(err).ToNot(HaveOccurred())

		functionTool := openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description),
				Parameters:  openai.FunctionParameters(params),
			},
		}

		// 4. Make a completion request that should trigger the tool
		resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model: openai.ChatModelGPT4oMini, // Use cheaper model for testing
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage("Create a new item with name 'Test Item', description 'A test product', labels {env: production, team: backend}, and tags ['urgent', 'important']. Make it a product with price 29.99 and quantity 100."),
			},
			Tools: []openai.ChatCompletionToolParam{functionTool},
		})
		g.Expect(err).ToNot(HaveOccurred())

		// 5. Verify we got a tool call
		g.Expect(resp.Choices).To(HaveLen(1))
		g.Expect(resp.Choices[0].Message.ToolCalls).To(HaveLen(1))

		toolCall := resp.Choices[0].Message.ToolCalls[0]
		g.Expect(toolCall.Function.Name).To(Equal(tool.Name))

		// 6. Verify the arguments can be unmarshaled
		var args map[string]any
		err = json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
		g.Expect(err).ToNot(HaveOccurred())

		// 7. Verify the structure matches what we expect
		g.Expect(args).To(HaveKey("name"))
		g.Expect(args["name"]).To(Equal("Test Item"))

		g.Expect(args).To(HaveKey("description"))
		g.Expect(args["description"]).To(Equal("A test product"))

		// In OpenAI mode, maps become arrays of key-value pairs
		g.Expect(args).To(HaveKey("labels"))
		labels := args["labels"].([]interface{})
		g.Expect(labels).To(HaveLen(2))

		// Verify the map was converted correctly
		labelMap := make(map[string]string)
		for _, item := range labels {
			kv := item.(map[string]interface{})
			labelMap[kv["key"].(string)] = kv["value"].(string)
		}
		g.Expect(labelMap).To(Equal(map[string]string{
			"env":  "production",
			"team": "backend",
		}))

		// Verify tags
		g.Expect(args).To(HaveKey("tags"))
		tags := args["tags"].([]interface{})
		g.Expect(tags).To(ConsistOf("urgent", "important"))

		// Verify oneof (product details)
		g.Expect(args).To(HaveKey("product"))
		product := args["product"].(map[string]interface{})
		g.Expect(product["price"]).To(BeNumerically("~", 29.99))
		g.Expect(product["quantity"]).To(BeNumerically("==", 100))
	})

	t.Run("ProcessWellKnownTypes with OpenAI compatibility", func(t *testing.T) {
		// Test well-known types handling
		tool := integrationtestv1mcp.TestService_ProcessWellKnownTypesToolOpenAI

		var params map[string]interface{}
		err := json.Unmarshal(tool.RawInputSchema, &params)
		g.Expect(err).ToNot(HaveOccurred())

		functionTool := openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description),
				Parameters:  openai.FunctionParameters(params),
			},
		}

		resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model: openai.ChatModelGPT4oMini,
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(`Process data with:
- metadata: {"environment": "production", "version": "1.2.3"}
- config: {"debug": true}
- timestamp: "2024-01-15T10:30:00Z"
- payload type: "example.v1.Item" with value: {"id": "123"}`),
			},
			Tools: []openai.ChatCompletionToolParam{functionTool},
		})
		g.Expect(err).ToNot(HaveOccurred())

		// Verify tool was called
		g.Expect(resp.Choices).To(HaveLen(1))
		g.Expect(resp.Choices[0].Message.ToolCalls).To(HaveLen(1))

		var args map[string]any
		err = json.Unmarshal([]byte(resp.Choices[0].Message.ToolCalls[0].Function.Arguments), &args)
		g.Expect(err).ToNot(HaveOccurred())

		// In OpenAI mode:
		// - google.protobuf.Struct should be a JSON string
		// - google.protobuf.Value should be a JSON string
		// - google.protobuf.Any should be a regular object (we fixed this)

		// Verify metadata (Struct) - should be JSON string in OpenAI mode
		g.Expect(args).To(HaveKey("metadata"))
		g.Expect(args["metadata"]).To(BeAssignableToTypeOf("string"))

		// Verify config (Value) - should be JSON string in OpenAI mode
		g.Expect(args).To(HaveKey("config"))
		g.Expect(args["config"]).To(BeAssignableToTypeOf("string"))

		// Verify timestamp
		g.Expect(args).To(HaveKey("timestamp"))
		g.Expect(args["timestamp"]).To(Equal("2024-01-15T10:30:00Z"))

		// Verify payload (Any) - should be object with @type
		g.Expect(args).To(HaveKey("payload"))
		payload := args["payload"].(map[string]interface{})
		g.Expect(payload).To(HaveKey("@type"))
		g.Expect(payload["@type"]).To(Equal("example.v1.Item"))
	})
}

// TestSchemaValidation verifies the generated schemas meet OpenAI requirements
func TestSchemaValidation(t *testing.T) {
	g := NewWithT(t)

	t.Run("CreateItem schema validation", func(t *testing.T) {
		tool := integrationtestv1mcp.TestService_CreateItemToolOpenAI

		var schema map[string]any
		err := json.Unmarshal(tool.RawInputSchema, &schema)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify root type is "object" not ["object", "null"]
		g.Expect(schema["type"]).To(Equal("object"))

		// Verify additionalProperties is false
		g.Expect(schema["additionalProperties"]).To(Equal(false))

		// Verify all fields are required (OpenAI requirement)
		required := schema["required"].([]interface{})
		g.Expect(required).To(ContainElements("name", "labels", "tags"))

		// Verify no format fields exist (OpenAI restriction)
		props := schema["properties"].(map[string]interface{})
		for _, prop := range props {
			propMap, ok := prop.(map[string]interface{})
			if ok {
				g.Expect(propMap).ToNot(HaveKey("format"))
			}
		}

		// Verify maps are converted to arrays
		labelsSchema := props["labels"].(map[string]interface{})
		g.Expect(labelsSchema["type"]).To(Equal("array"))
		g.Expect(labelsSchema).To(HaveKey("items"))
	})

	t.Run("ProcessWellKnownTypes schema validation", func(t *testing.T) {
		tool := integrationtestv1mcp.TestService_ProcessWellKnownTypesToolOpenAI

		var schema map[string]any
		err := json.Unmarshal(tool.RawInputSchema, &schema)
		g.Expect(err).ToNot(HaveOccurred())

		props := schema["properties"].(map[string]interface{})

		// Verify google.protobuf.Any has type "object" not ["object", "null"]
		payloadSchema := props["payload"].(map[string]interface{})
		g.Expect(payloadSchema["type"]).To(Equal("object"))

		// Verify Struct is string in OpenAI mode
		metadataSchema := props["metadata"].(map[string]interface{})
		g.Expect(metadataSchema["type"]).To(Equal("string"))

		// Verify Value is string in OpenAI mode
		configSchema := props["config"].(map[string]interface{})
		g.Expect(configSchema["type"]).To(Equal("string"))
	})
}
