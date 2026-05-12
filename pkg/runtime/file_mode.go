// Copyright 2025 Redpanda Data, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runtime

import (
	"encoding/json"
)

// FileMode controls how FileInput/FileOutput schemas are exposed to the LLM.
type FileMode int

const (
	// FileModeInline is the default: file content flows inline through the
	// JSON-RPC payload (bytes/base64). The file_path field is stripped from
	// the schema. Suitable for hosted deployments.
	FileModeInline FileMode = iota
	// FileModePath exposes file_path instead of content. The agent
	// provides/receives filesystem paths on a shared volume. Suitable for
	// sandbox deployments where aigw and the agent share a filesystem.
	FileModePath
	// FileModeAll exposes both content and file_path. The LLM picks
	// whichever is appropriate; the handler checks which field is populated.
	FileModeAll
)

// fileSchemaMarkerKey must match gen.FileSchemaMarkerKey. Duplicated here to
// avoid a circular import between runtime and gen.
const fileSchemaMarkerKey = "x-mcp-file-mode"

// WithFileMode sets the file transfer mode for tool schema generation.
// When set, FileInput/FileOutput schemas are rewritten at registration
// time to expose only the fields relevant to the selected mode.
func WithFileMode(mode FileMode) Option {
	return func(c *config) {
		c.FileMode = mode
		c.FileModeSet = true
	}
}

// rewriteFileSchemas walks a tool's input and output schemas, finds any
// properties marked with x-mcp-file-mode, and strips the fields that
// don't apply to the given mode.
func rewriteFileSchemas(tool Tool, mode FileMode) Tool {
	tool.RawInputSchema = rewriteSchema(tool.RawInputSchema, mode)
	tool.RawOutputSchema = rewriteSchema(tool.RawOutputSchema, mode)
	return tool
}

func rewriteSchema(raw json.RawMessage, mode FileMode) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return raw
	}
	if rewriteSchemaNode(schema, mode) {
		out, err := json.Marshal(schema)
		if err != nil {
			return raw
		}
		return json.RawMessage(out)
	}
	return raw
}

// rewriteSchemaNode recursively walks a JSON Schema node. Returns true if
// any modification was made.
func rewriteSchemaNode(node map[string]any, mode FileMode) bool {
	modified := false

	if marker, ok := node[fileSchemaMarkerKey]; ok {
		markerStr, _ := marker.(string)
		applyFileModeToNode(node, markerStr, mode)
		delete(node, fileSchemaMarkerKey)
		modified = true
	}

	if props, ok := node["properties"].(map[string]any); ok {
		for _, v := range props {
			if sub, ok := v.(map[string]any); ok {
				if rewriteSchemaNode(sub, mode) {
					modified = true
				}
			}
		}
	}

	if items, ok := node["items"].(map[string]any); ok {
		if rewriteSchemaNode(items, mode) {
			modified = true
		}
	}

	if anyOf, ok := node["anyOf"].([]any); ok {
		for _, entry := range anyOf {
			if sub, ok := entry.(map[string]any); ok {
				if rewriteSchemaNode(sub, mode) {
					modified = true
				}
			}
		}
	}

	if oneOf, ok := node["oneOf"].([]any); ok {
		for _, entry := range oneOf {
			if sub, ok := entry.(map[string]any); ok {
				if rewriteSchemaNode(sub, mode) {
					modified = true
				}
			}
		}
	}

	return modified
}

func applyFileModeToNode(node map[string]any, markerType string, mode FileMode) {
	props, _ := node["properties"].(map[string]any)
	if props == nil {
		return
	}

	required, _ := node["required"].([]any)

	switch mode {
	case FileModePath:
		delete(props, "content")
		required = addToRequired(removeFromRequired(required, "content"), "file_path")
	case FileModeInline:
		delete(props, "file_path")
		if markerType == "input" {
			required = addToRequired(removeFromRequired(required, "file_path"), "content")
		} else {
			required = removeFromRequired(required, "file_path")
		}
	case FileModeAll:
		// Keep all fields, just strip the marker (done by caller).
	}

	node["required"] = required
}

func removeFromRequired(required []any, field string) []any {
	out := make([]any, 0, len(required))
	for _, r := range required {
		if s, ok := r.(string); ok && s == field {
			continue
		}
		out = append(out, r)
	}
	return out
}

func addToRequired(required []any, field string) []any {
	for _, r := range required {
		if s, ok := r.(string); ok && s == field {
			return required
		}
	}
	return append(required, field)
}
