package gen

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// TestRegisterService_ToolFilter verifies that ToolFilter drops filtered
// tools at registration time and observes the final (prefixed) tool name.
func TestRegisterService_ToolFilter(t *testing.T) {
	g := NewWithT(t)

	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	sd := file.Services().ByName("TestService")
	g.Expect(sd).ToNot(BeNil())

	handler := func(_ context.Context, method protoreflect.MethodDescriptor, _ proto.Message) (proto.Message, error) {
		return newTestMessage(method.Output()), nil
	}

	var seen []string
	rec := &recordingServer{}
	RegisterService(rec, sd, handler, RegisterServiceOptions{
		NewMessage: newTestMessage,
		NamePrefix: "primary",
		ToolFilter: func(toolName string) bool {
			seen = append(seen, toolName)
			return toolName != "primary_testdata_TestService_CreateItem"
		},
	})

	// The filter sees every tool's final (prefixed) name.
	g.Expect(seen).To(ContainElement("primary_testdata_TestService_CreateItem"))
	g.Expect(seen).To(ContainElement("primary_testdata_TestService_GetItem"))

	registered := make([]string, 0, len(rec.tools))
	for _, tool := range rec.tools {
		registered = append(registered, tool.Name)
	}
	g.Expect(registered).ToNot(ContainElement("primary_testdata_TestService_CreateItem"))
	g.Expect(registered).To(ContainElement("primary_testdata_TestService_GetItem"))
}
