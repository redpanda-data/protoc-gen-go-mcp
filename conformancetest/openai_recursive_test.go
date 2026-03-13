//go:build integration
// +build integration

package conformancetest

import (
	"testing"

	testdatamcp "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata/testdatamcp"
)

// TestOpenAIRecursiveSchemaAcceptance sends the recursive TreeNode schema to
// OpenAI. With depth=99 this should fail because the schema is enormous and
// exceeds OpenAI's nesting/size limits.
func TestOpenAIRecursiveSchemaAcceptance(t *testing.T) {
	client := newOpenAIClient(t)

	mcpTool := testdatamcp.EdgeCaseService_RecursiveTreeToolOpenAI
	tool := openaiToolFromSchema(mcpTool.Name, mcpTool.Description, mcpTool.RawInputSchema)

	t.Logf("Schema size: %d bytes", len(mcpTool.RawInputSchema))

	args := callOpenAITool(t, client, tool,
		"Create a tree with root value 'root', two children: 'left' (no children) and 'right' with one child 'right-left'.",
	)

	t.Logf("Got args: %v", args)
}
