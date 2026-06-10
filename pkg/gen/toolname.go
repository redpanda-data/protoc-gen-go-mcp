package gen

import (
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	mcpv1 "github.com/redpanda-data/protoc-gen-go-mcp/pkg/mcpv1"
)

// validToolName is the intersection of every LLM provider's
// function-name rules: Gemini requires a leading letter or underscore,
// OpenAI restricts the charset to [a-zA-Z0-9_-], and 64 is the
// strictest length limit.
var validToolName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]{0,63}$`)

// ValidateToolName reports whether a configured tool name is usable on
// every LLM provider.
func ValidateToolName(name string) error {
	if !validToolName.MatchString(name) {
		return fmt.Errorf("invalid tool name %q: must match %s (letter or underscore first, alphanumerics/underscores/dashes, max 64 chars)", name, validToolName)
	}
	return nil
}

// ConfiguredToolName returns the (mcp.v1.tool_name) option on the
// method, or "" when unset. The value is returned as authored; callers
// decide how to handle an invalid one (the generator fails the build,
// the runtime registration path falls back to the derived name).
func ConfiguredToolName(method protoreflect.MethodDescriptor) string {
	opts, ok := method.Options().(*descriptorpb.MethodOptions)
	if !ok || opts == nil {
		return ""
	}
	name, _ := proto.GetExtension(opts, mcpv1.E_ToolName).(string)
	return name
}

// ToolNameForMethod resolves the MCP tool name for a method: the
// (mcp.v1.tool_name) option when set and valid, otherwise the derived
// full-name-based form, hash-truncated to 64 chars. Invalid configured
// names fall back to the derived form here so dynamic registration
// never publishes a provider-rejected name; the generator additionally
// fails code generation on them so they are caught when authored.
func ToolNameForMethod(method protoreflect.MethodDescriptor) string {
	if name := ConfiguredToolName(method); name != "" && ValidateToolName(name) == nil {
		return name
	}
	return MangleHeadIfTooLong(strings.ReplaceAll(string(method.FullName()), ".", "_"), 64)
}
