package runtime

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// ToolAnnotationConfig defines annotation hints for a tool.
// These annotations help LLMs and MCP clients understand tool behavior
// and make better decisions about when and how to use tools.
type ToolAnnotationConfig struct {
	// Title is a human-readable title for the tool
	Title string

	// ReadOnlyHint indicates that the tool does not modify its environment.
	// If true, the tool only reads data without making changes.
	ReadOnlyHint *bool

	// DestructiveHint indicates that the tool may perform destructive updates
	// to its environment. If false, the tool performs only additive updates.
	DestructiveHint *bool

	// IdempotentHint indicates that calling the tool repeatedly with the same
	// arguments will have no additional effect on its environment.
	IdempotentHint *bool

	// OpenWorldHint indicates that the tool may interact with an "open world"
	// of external entities. If false, the tool's domain of interaction is closed.
	OpenWorldHint *bool
}

// WithToolAnnotations adds tool annotations that provide semantic hints
// about tool behavior to MCP clients.
func WithToolAnnotations(annotations ToolAnnotationConfig) Option {
	return func(c *config) {
		c.Annotations = &annotations
	}
}

// ApplyToolAnnotations applies annotation options to a tool, returning
// a modified tool with the annotations set.
func ApplyToolAnnotations(tool mcp.Tool, opts ...Option) mcp.Tool {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.Annotations == nil {
		return tool
	}

	// Create a copy of the tool to avoid modifying the original
	modifiedTool := tool

	// Build the ToolAnnotation from config
	modifiedTool.Annotations = mcp.ToolAnnotation{
		Title:           cfg.Annotations.Title,
		ReadOnlyHint:    cfg.Annotations.ReadOnlyHint,
		DestructiveHint: cfg.Annotations.DestructiveHint,
		IdempotentHint:  cfg.Annotations.IdempotentHint,
		OpenWorldHint:   cfg.Annotations.OpenWorldHint,
	}

	return modifiedTool
}

// ApplyOptions applies all options (including annotations and extra properties)
// to a tool in a single call.
func ApplyOptions(tool mcp.Tool, opts ...Option) mcp.Tool {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Apply annotations if present
	if cfg.Annotations != nil {
		tool.Annotations = mcp.ToolAnnotation{
			Title:           cfg.Annotations.Title,
			ReadOnlyHint:    cfg.Annotations.ReadOnlyHint,
			DestructiveHint: cfg.Annotations.DestructiveHint,
			IdempotentHint:  cfg.Annotations.IdempotentHint,
			OpenWorldHint:   cfg.Annotations.OpenWorldHint,
		}
	}

	// Apply extra properties if present
	if len(cfg.ExtraProperties) > 0 {
		tool = AddExtraPropertiesToTool(tool, cfg.ExtraProperties)
	}

	return tool
}
