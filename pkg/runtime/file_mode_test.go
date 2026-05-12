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
	"testing"

	. "github.com/onsi/gomega"
)

func TestRewriteFileSchemas_PathMode_Input(t *testing.T) {
	g := NewWithT(t)

	tool := Tool{
		Name: "test_tool",
		RawInputSchema: mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"site_id": map[string]any{"type": "string"},
				"file": map[string]any{
					"type":             "object",
					fileSchemaMarkerKey: "input",
					"properties": map[string]any{
						"content":   map[string]any{"type": "string", "contentEncoding": "base64"},
						"file_path": map[string]any{"type": "string"},
						"filename":  map[string]any{"type": "string"},
						"mime_type": map[string]any{"type": "string"},
					},
					"required": []string{"filename"},
				},
			},
			"required": []string{"site_id"},
		}),
	}

	rewritten := rewriteFileSchemas(tool, FileModePath)

	var schema map[string]any
	g.Expect(json.Unmarshal(rewritten.RawInputSchema, &schema)).To(Succeed())

	fileProps := schema["properties"].(map[string]any)["file"].(map[string]any)
	props := fileProps["properties"].(map[string]any)

	g.Expect(props).ToNot(HaveKey("content"), "content should be stripped in path mode")
	g.Expect(props).To(HaveKey("file_path"))
	g.Expect(props).To(HaveKey("filename"))
	g.Expect(props).To(HaveKey("mime_type"))
	g.Expect(fileProps).ToNot(HaveKey(fileSchemaMarkerKey), "marker should be removed")

	required := toStringSlice(fileProps["required"])
	g.Expect(required).To(ContainElement("file_path"))
	g.Expect(required).To(ContainElement("filename"))
	g.Expect(required).ToNot(ContainElement("content"))
}

func TestRewriteFileSchemas_InlineMode_Input(t *testing.T) {
	g := NewWithT(t)

	tool := Tool{
		Name: "test_tool",
		RawInputSchema: mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file": map[string]any{
					"type":             "object",
					fileSchemaMarkerKey: "input",
					"properties": map[string]any{
						"content":   map[string]any{"type": "string", "contentEncoding": "base64"},
						"file_path": map[string]any{"type": "string"},
						"filename":  map[string]any{"type": "string"},
						"mime_type": map[string]any{"type": "string"},
					},
					"required": []string{"filename"},
				},
			},
			"required": []string{},
		}),
	}

	rewritten := rewriteFileSchemas(tool, FileModeInline)

	var schema map[string]any
	g.Expect(json.Unmarshal(rewritten.RawInputSchema, &schema)).To(Succeed())

	fileProps := schema["properties"].(map[string]any)["file"].(map[string]any)
	props := fileProps["properties"].(map[string]any)

	g.Expect(props).To(HaveKey("content"))
	g.Expect(props).ToNot(HaveKey("file_path"), "file_path should be stripped in inline mode")
	g.Expect(props).To(HaveKey("filename"))

	required := toStringSlice(fileProps["required"])
	g.Expect(required).To(ContainElement("content"))
	g.Expect(required).To(ContainElement("filename"))
	g.Expect(required).ToNot(ContainElement("file_path"))
}

func TestRewriteFileSchemas_PathMode_Output(t *testing.T) {
	g := NewWithT(t)

	tool := Tool{
		Name: "test_tool",
		RawOutputSchema: mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"result": map[string]any{
					"type":             "object",
					fileSchemaMarkerKey: "output",
					"properties": map[string]any{
						"content":    map[string]any{"type": "string"},
						"file_path":  map[string]any{"type": "string"},
						"filename":   map[string]any{"type": "string"},
						"mime_type":  map[string]any{"type": "string"},
						"size_bytes": map[string]any{"type": "string"},
					},
					"required": []string{"filename"},
				},
			},
		}),
	}

	rewritten := rewriteFileSchemas(tool, FileModePath)

	var schema map[string]any
	g.Expect(json.Unmarshal(rewritten.RawOutputSchema, &schema)).To(Succeed())

	resultProps := schema["properties"].(map[string]any)["result"].(map[string]any)
	props := resultProps["properties"].(map[string]any)

	g.Expect(props).ToNot(HaveKey("content"), "content should be stripped in path mode")
	g.Expect(props).To(HaveKey("file_path"))
	g.Expect(props).To(HaveKey("size_bytes"))

	required := toStringSlice(resultProps["required"])
	g.Expect(required).To(ContainElement("file_path"))
}

func TestRewriteFileSchemas_InlineMode_Output(t *testing.T) {
	g := NewWithT(t)

	tool := Tool{
		Name: "test_tool",
		RawOutputSchema: mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"result": map[string]any{
					"type":             "object",
					fileSchemaMarkerKey: "output",
					"properties": map[string]any{
						"content":    map[string]any{"type": "string"},
						"file_path":  map[string]any{"type": "string"},
						"filename":   map[string]any{"type": "string"},
						"mime_type":  map[string]any{"type": "string"},
						"size_bytes": map[string]any{"type": "string"},
					},
					"required": []string{"filename"},
				},
			},
		}),
	}

	rewritten := rewriteFileSchemas(tool, FileModeInline)

	var schema map[string]any
	g.Expect(json.Unmarshal(rewritten.RawOutputSchema, &schema)).To(Succeed())

	resultProps := schema["properties"].(map[string]any)["result"].(map[string]any)
	props := resultProps["properties"].(map[string]any)

	g.Expect(props).To(HaveKey("content"))
	g.Expect(props).ToNot(HaveKey("file_path"), "file_path should be stripped in inline mode")
}

func TestRewriteFileSchemas_AllMode_KeepsAllFields(t *testing.T) {
	g := NewWithT(t)

	tool := Tool{
		Name: "test_tool",
		RawInputSchema: mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file": map[string]any{
					"type":             "object",
					fileSchemaMarkerKey: "input",
					"properties": map[string]any{
						"content":   map[string]any{"type": "string", "contentEncoding": "base64"},
						"file_path": map[string]any{"type": "string"},
						"filename":  map[string]any{"type": "string"},
						"mime_type": map[string]any{"type": "string"},
					},
					"required": []string{"filename"},
				},
			},
		}),
	}

	rewritten := rewriteFileSchemas(tool, FileModeAll)

	var schema map[string]any
	g.Expect(json.Unmarshal(rewritten.RawInputSchema, &schema)).To(Succeed())

	fileProps := schema["properties"].(map[string]any)["file"].(map[string]any)
	props := fileProps["properties"].(map[string]any)

	g.Expect(props).To(HaveKey("content"), "content should be kept in all mode")
	g.Expect(props).To(HaveKey("file_path"), "file_path should be kept in all mode")
	g.Expect(props).To(HaveKey("filename"))
	g.Expect(props).To(HaveKey("mime_type"))
	g.Expect(fileProps).ToNot(HaveKey(fileSchemaMarkerKey), "marker should still be removed")
}

func TestRewriteFileSchemas_NoMarker_Unchanged(t *testing.T) {
	g := NewWithT(t)

	original := mustJSON(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
		"required": []string{"name"},
	})

	tool := Tool{
		Name:           "plain_tool",
		RawInputSchema: original,
	}

	rewritten := rewriteFileSchemas(tool, FileModePath)
	g.Expect(rewritten.RawInputSchema).To(Equal(original))
}

func TestRewriteFileSchemas_EmptySchema(t *testing.T) {
	g := NewWithT(t)

	tool := Tool{Name: "empty"}
	rewritten := rewriteFileSchemas(tool, FileModePath)
	g.Expect(rewritten.RawInputSchema).To(BeNil())
	g.Expect(rewritten.RawOutputSchema).To(BeNil())
}

func TestApplyConfig_WithFileMode(t *testing.T) {
	g := NewWithT(t)

	cfg := NewConfig()
	WithFileMode(FileModePath)(cfg)

	tool := Tool{
		Name: "upload_tool",
		RawInputSchema: mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file": map[string]any{
					"type":             "object",
					fileSchemaMarkerKey: "input",
					"properties": map[string]any{
						"content":   map[string]any{"type": "string"},
						"file_path": map[string]any{"type": "string"},
						"filename":  map[string]any{"type": "string"},
					},
					"required": []string{"filename"},
				},
			},
		}),
	}

	result := ApplyConfig(tool, cfg)

	var schema map[string]any
	g.Expect(json.Unmarshal(result.RawInputSchema, &schema)).To(Succeed())

	fileProps := schema["properties"].(map[string]any)["file"].(map[string]any)
	props := fileProps["properties"].(map[string]any)
	g.Expect(props).ToNot(HaveKey("content"))
	g.Expect(props).To(HaveKey("file_path"))
}

func TestApplyConfig_WithoutFileMode_PreservesMarker(t *testing.T) {
	g := NewWithT(t)

	cfg := NewConfig()

	tool := Tool{
		Name: "tool",
		RawInputSchema: mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file": map[string]any{
					"type":             "object",
					fileSchemaMarkerKey: "input",
					"properties": map[string]any{
						"content":   map[string]any{"type": "string"},
						"file_path": map[string]any{"type": "string"},
						"filename":  map[string]any{"type": "string"},
					},
					"required": []string{"filename"},
				},
			},
		}),
	}

	result := ApplyConfig(tool, cfg)

	var schema map[string]any
	g.Expect(json.Unmarshal(result.RawInputSchema, &schema)).To(Succeed())

	fileProps := schema["properties"].(map[string]any)["file"].(map[string]any)
	g.Expect(fileProps).To(HaveKey(fileSchemaMarkerKey), "marker should be preserved when FileModeSet is false")
	props := fileProps["properties"].(map[string]any)
	g.Expect(props).To(HaveKey("content"))
	g.Expect(props).To(HaveKey("file_path"))
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return json.RawMessage(b)
}

func toStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, elem := range arr {
		if s, ok := elem.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
