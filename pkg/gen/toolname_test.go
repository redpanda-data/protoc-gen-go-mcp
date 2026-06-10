package gen

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	mcpv1 "github.com/redpanda-data/protoc-gen-go-mcp/pkg/mcpv1"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
)

// testService builds a dynamic service descriptor with one method per
// entry; a non-empty toolName sets the (mcp.v1.tool_name) option.
func testService(t *testing.T, methods map[string]string) protoreflect.ServiceDescriptor {
	t.Helper()
	var mdps []*descriptorpb.MethodDescriptorProto
	// Deterministic order for the registration-order-sensitive tests.
	names := make([]string, 0, len(methods))
	for n := range methods {
		names = append(names, n)
	}
	for i := len(names) - 1; i > 0; i-- {
		for j := 0; j < i; j++ {
			if names[j] > names[j+1] {
				names[j], names[j+1] = names[j+1], names[j]
			}
		}
	}
	for _, name := range names {
		mdp := &descriptorpb.MethodDescriptorProto{
			Name:       proto.String(name),
			InputType:  proto.String(".test.v1.Empty"),
			OutputType: proto.String(".test.v1.Empty"),
		}
		if toolName := methods[name]; toolName != "" {
			opts := &descriptorpb.MethodOptions{}
			proto.SetExtension(opts, mcpv1.E_ToolName, toolName)
			mdp.Options = opts
		}
		mdps = append(mdps, mdp)
	}
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test/v1/test.proto"),
		Package: proto.String("test.v1"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Empty")},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{Name: proto.String("TestService"), Method: mdps},
		},
	}
	file, err := protodesc.NewFile(fdp, nil)
	if err != nil {
		t.Fatalf("protodesc.NewFile: %v", err)
	}
	return file.Services().Get(0)
}

func TestValidateToolName(t *testing.T) {
	valid := []string{"a", "_x", "get_equity_research_report", "A-b_c9", strings.Repeat("a", 64)}
	for _, n := range valid {
		if err := ValidateToolName(n); err != nil {
			t.Errorf("ValidateToolName(%q) = %v, want nil", n, err)
		}
	}
	invalid := []string{"", "9start", "-lead", "has space", "dot.ted", "co:lon", strings.Repeat("a", 65)}
	for _, n := range invalid {
		if err := ValidateToolName(n); err == nil {
			t.Errorf("ValidateToolName(%q) = nil, want error", n)
		}
	}
}

func TestToolNameForMethod(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		want       string
	}{
		{"unset uses derived", "", "test_v1_TestService_DoThing"},
		{"configured wins", "custom_name", "custom_name"},
		{"invalid configured falls back to derived", "9not-valid", "test_v1_TestService_DoThing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sd := testService(t, map[string]string{"DoThing": tt.configured})
			got := ToolNameForMethod(sd.Methods().Get(0))
			if got != tt.want {
				t.Errorf("ToolNameForMethod = %q, want %q", got, tt.want)
			}
		})
	}
}

type collectingServer struct {
	tools []runtime.Tool
}

func (c *collectingServer) AddTool(tool runtime.Tool, _ runtime.ToolHandler) {
	c.tools = append(c.tools, tool)
}

// Duplicate configured names within a service: the first registration
// keeps the configured name, the second falls back to its derived name
// (unique by construction) — nothing is silently dropped or
// overwritten.
func TestRegisterService_DuplicateConfiguredNamesFallBack(t *testing.T) {
	sd := testService(t, map[string]string{
		"AlphaThing": "same_name",
		"BetaThing":  "same_name",
	})
	srv := &collectingServer{}
	RegisterService(srv, sd, func(_ context.Context, _ protoreflect.MethodDescriptor, _ proto.Message) (proto.Message, error) {
		return nil, nil
	}, RegisterServiceOptions{})

	if len(srv.tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(srv.tools))
	}
	got := map[string]bool{}
	for _, tool := range srv.tools {
		got[tool.Name] = true
	}
	if !got["same_name"] {
		t.Errorf("first method should keep the configured name, got %v", got)
	}
	if !got["test_v1_TestService_BetaThing"] {
		t.Errorf("second method should fall back to its derived name, got %v", got)
	}
}

// Configured names flow through RegisterService for the normal case.
func TestRegisterService_ConfiguredName(t *testing.T) {
	sd := testService(t, map[string]string{"DoThing": "do_the_thing"})
	srv := &collectingServer{}
	RegisterService(srv, sd, func(_ context.Context, _ protoreflect.MethodDescriptor, _ proto.Message) (proto.Message, error) {
		return nil, nil
	}, RegisterServiceOptions{})
	if len(srv.tools) != 1 || srv.tools[0].Name != "do_the_thing" {
		t.Fatalf("expected tool do_the_thing, got %+v", srv.tools)
	}
}
