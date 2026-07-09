package generator

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	testdatamcp "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata/testdatamcp"
)

// recordingServer captures registered tools so tests can assert which tools
// made it past the filter.
type recordingServer struct {
	tools []runtime.Tool
}

func (r *recordingServer) AddTool(tool runtime.Tool, _ runtime.ToolHandler) {
	r.tools = append(r.tools, tool)
}

func (r *recordingServer) names() []string {
	names := make([]string, 0, len(r.tools))
	for _, tool := range r.tools {
		names = append(names, tool.Name)
	}
	return names
}

// TestGeneratedHandlerToolFilter verifies that WithToolFilter drops filtered
// tools at registration time and that the filter is matched against the
// generated tool descriptors' names.
func TestGeneratedHandlerToolFilter(t *testing.T) {
	g := NewWithT(t)

	excluded := map[string]bool{
		testdatamcp.TestService_CreateItemTool.Name: true,
	}

	rec := &recordingServer{}
	testdatamcp.RegisterTestServiceHandler(rec, &fullTestServer{}, runtime.WithToolFilter(func(toolName string) bool {
		return !excluded[toolName]
	}))

	g.Expect(rec.names()).To(ConsistOf(
		testdatamcp.TestService_GetItemTool.Name,
		testdatamcp.TestService_ProcessWellKnownTypesTool.Name,
		testdatamcp.TestService_TestValidationTool.Name,
	))
}

// TestGeneratedHandlerToolFilter_AfterNamePrefix verifies that the filter
// observes the final tool name, i.e. after WithNamePrefix is applied.
func TestGeneratedHandlerToolFilter_AfterNamePrefix(t *testing.T) {
	g := NewWithT(t)

	var seen []string
	rec := &recordingServer{}
	testdatamcp.RegisterTestServiceHandler(rec, &fullTestServer{},
		runtime.WithNamePrefix("primary"),
		runtime.WithToolFilter(func(toolName string) bool {
			seen = append(seen, toolName)
			return toolName == "primary_"+testdatamcp.TestService_GetItemTool.Name
		}),
	)

	g.Expect(seen).To(ConsistOf(
		"primary_"+testdatamcp.TestService_CreateItemTool.Name,
		"primary_"+testdatamcp.TestService_GetItemTool.Name,
		"primary_"+testdatamcp.TestService_ProcessWellKnownTypesTool.Name,
		"primary_"+testdatamcp.TestService_TestValidationTool.Name,
	))
	g.Expect(rec.names()).To(ConsistOf("primary_" + testdatamcp.TestService_GetItemTool.Name))
}

// TestGeneratedHandlerNoFilter verifies that omitting WithToolFilter keeps
// the previous behavior: every tool is registered.
func TestGeneratedHandlerNoFilter(t *testing.T) {
	g := NewWithT(t)

	rec := &recordingServer{}
	testdatamcp.RegisterTestServiceHandler(rec, &fullTestServer{})

	g.Expect(rec.names()).To(ConsistOf(
		testdatamcp.TestService_CreateItemTool.Name,
		testdatamcp.TestService_GetItemTool.Name,
		testdatamcp.TestService_ProcessWellKnownTypesTool.Name,
		testdatamcp.TestService_TestValidationTool.Name,
	))
}
