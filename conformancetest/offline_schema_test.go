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

// This file is intentionally NOT behind the external_llm build tag: it makes no
// network calls. It runs every generated tool schema through the ai-sdk-go
// per-provider schema adapters and asserts the result satisfies that provider's
// tool-input-schema constraints. ai-sdk-go stands in for the whole class of MCP
// clients (LangChain, Vercel AI SDK, the OpenAI Agents SDK, ...) that downgrade
// rich MCP schemas per provider. This is the offline gate that would have caught
// the original top-level-anyOf break, the OpenAI format/propertyNames rejects,
// and the typeless-WKT rejects, all without spending a token.
package conformancetest

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/redpanda-data/ai-sdk-go/providers/anthropic"
	"github.com/redpanda-data/ai-sdk-go/providers/google"
	"github.com/redpanda-data/ai-sdk-go/providers/openai"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/gen"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// allSchemas returns every generated input and output tool schema across the
// testdata services, keyed by a readable name.
func allSchemas(t *testing.T) map[string]map[string]any {
	t.Helper()
	files := []protoreflect.FileDescriptor{
		(&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile(),
		(&testdata.DeepNestingRequest{}).ProtoReflect().Descriptor().ParentFile(),
	}
	out := map[string]map[string]any{}
	for _, f := range files {
		for i := 0; i < f.Services().Len(); i++ {
			sd := f.Services().Get(i)
			for j := 0; j < sd.Methods().Len(); j++ {
				method := sd.Methods().Get(j)
				tool := gen.ToolForMethod(method, "")
				for kind, raw := range map[string]json.RawMessage{
					"in":  tool.RawInputSchema,
					"out": tool.RawOutputSchema,
				} {
					var m map[string]any
					if err := json.Unmarshal(raw, &m); err != nil {
						t.Fatalf("%s: %v", method.FullName(), err)
					}
					out[fmt.Sprintf("%s_%s.%s", sd.Name(), method.Name(), kind)] = m
				}
			}
		}
	}
	return out
}

func mustCompile(t *testing.T, name string, schema map[string]any) {
	t.Helper()
	raw, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("%s: marshal: %v", name, err)
	}
	if _, err := jsonschema.CompileString(name+".json", string(raw)); err != nil {
		t.Fatalf("%s: adapted schema does not compile as JSON Schema: %v\n%s", name, err, raw)
	}
}

// walkNodes visits every schema-object node, calling fn with a path. It is
// schema-aware: it recurses only into subschema positions (properties values,
// items, additionalProperties, combinators, $defs) so container maps like
// "properties" are not themselves mistaken for schema nodes.
func walkNodes(node map[string]any, path string, fn func(node map[string]any, path string)) {
	fn(node, path)
	if props, ok := node["properties"].(map[string]any); ok {
		for name, child := range props {
			if c, ok := child.(map[string]any); ok {
				walkNodes(c, path+".properties."+name, fn)
			}
		}
	}
	for _, k := range []string{"$defs", "definitions", "patternProperties"} {
		if defs, ok := node[k].(map[string]any); ok {
			for name, child := range defs {
				if c, ok := child.(map[string]any); ok {
					walkNodes(c, fmt.Sprintf("%s.%s.%s", path, k, name), fn)
				}
			}
		}
	}
	for _, k := range []string{"anyOf", "oneOf", "allOf", "prefixItems"} {
		if arr, ok := node[k].([]any); ok {
			for i, child := range arr {
				if c, ok := child.(map[string]any); ok {
					walkNodes(c, fmt.Sprintf("%s.%s[%d]", path, k, i), fn)
				}
			}
		}
	}
	switch it := node["items"].(type) {
	case map[string]any:
		walkNodes(it, path+".items", fn)
	case []any:
		for i, child := range it {
			if c, ok := child.(map[string]any); ok {
				walkNodes(c, fmt.Sprintf("%s.items[%d]", path, i), fn)
			}
		}
	}
	if ap, ok := node["additionalProperties"].(map[string]any); ok {
		walkNodes(ap, path+".additionalProperties", fn)
	}
}

// TestOffline_OpenAIAdaptedSchemasAreStrictValid asserts every generated schema,
// after ai-sdk-go's OpenAI adapter, satisfies OpenAI Structured Outputs strict
// rules: object root, no unions/$ref, every node typed, every object closed
// (additionalProperties:false) with all properties required, and none of the
// rejected keywords (propertyNames, contentEncoding, format:byte).
func TestOffline_OpenAIAdaptedSchemasAreStrictValid(t *testing.T) {
	mapper := openai.NewSchemaMapper()
	for name, schema := range allSchemas(t) {
		t.Run(name, func(t *testing.T) {
			adapted := mapper.AdaptSchemaForOpenAI(schema)
			mustCompile(t, name, adapted)

			if adapted["type"] != "object" {
				t.Errorf("root type = %v, want object", adapted["type"])
			}
			walkNodes(adapted, name, func(node map[string]any, path string) {
				for _, bad := range []string{"anyOf", "oneOf", "allOf", "$ref", "$defs", "propertyNames", "patternProperties", "contentEncoding", "contentMediaType"} {
					if _, ok := node[bad]; ok {
						t.Errorf("%s: forbidden keyword %q", path, bad)
					}
				}
				if f, ok := node["format"].(string); ok && !openAISupportedFormat(f) {
					t.Errorf("%s: unsupported format %q", path, f)
				}
				// Every node must carry a type (OpenAI rejects typeless schemas).
				if _, hasType := node["type"]; !hasType {
					// Tolerate pure container wrappers that only hold properties via
					// a typed parent; in our generated+adapted output there are none.
					t.Errorf("%s: node has no \"type\" key: %v", path, node)
				}
				// Objects must be closed and fully required.
				if isObjectNode(node) {
					if node["additionalProperties"] != false {
						t.Errorf("%s: object additionalProperties = %v, want false", path, node["additionalProperties"])
					}
					if props, ok := node["properties"].(map[string]any); ok {
						req := map[string]bool{}
						for _, r := range toAnySlice(node["required"]) {
							if s, ok := r.(string); ok {
								req[s] = true
							}
						}
						for p := range props {
							if !req[p] {
								t.Errorf("%s: property %q not in required (OpenAI strict requires all)", path, p)
							}
						}
					}
				}
			})
		})
	}
}

// TestOffline_AnthropicAdaptedSchemasValid asserts the Anthropic adapter output
// compiles as JSON Schema draft 2020-12 and contains no typeless nodes (which
// Anthropic rejects as an invalid input_schema).
func TestOffline_AnthropicAdaptedSchemasValid(t *testing.T) {
	mapper := anthropic.NewSchemaMapper()
	for name, schema := range allSchemas(t) {
		t.Run(name, func(t *testing.T) {
			adapted := mapper.AdaptSchemaForAnthropic(schema)
			mustCompile(t, name, adapted)
			if adapted["type"] != "object" {
				t.Errorf("root type = %v, want object", adapted["type"])
			}
			walkNodes(adapted, name, func(node map[string]any, path string) {
				if hasNoTypeLeaf(node) {
					t.Errorf("%s: typeless node rejected by Anthropic: %v", path, node)
				}
			})
		})
	}
}

// TestOffline_GoogleAdaptedSchemasValid asserts the Google adapter output still
// compiles. Gemini is the most permissive, so this is a basic sanity gate.
func TestOffline_GoogleAdaptedSchemasValid(t *testing.T) {
	mapper := google.NewSchemaMapper()
	for name, schema := range allSchemas(t) {
		t.Run(name, func(t *testing.T) {
			adapted := mapper.AdaptSchemaForGoogle(schema)
			mustCompile(t, name, adapted)
		})
	}
}

func openAISupportedFormat(f string) bool {
	switch f {
	case "date-time", "time", "date", "duration", "email", "hostname", "ipv4", "ipv6", "uuid":
		return true
	}
	return false
}

func isObjectNode(node map[string]any) bool {
	if t, ok := node["type"].(string); ok && t == "object" {
		return true
	}
	if ts, ok := node["type"].([]any); ok {
		for _, e := range ts {
			if e == "object" {
				return true
			}
		}
	}
	_, hasProps := node["properties"]
	return hasProps
}

// hasNoTypeLeaf reports whether node lacks a type and is not a combinator/$ref
// wrapper (i.e. a typeless leaf that strict validators reject).
func hasNoTypeLeaf(node map[string]any) bool {
	if _, ok := node["type"]; ok {
		return false
	}
	for _, k := range []string{"$ref", "anyOf", "oneOf", "allOf"} {
		if _, ok := node[k]; ok {
			return false
		}
	}
	return true
}

func toAnySlice(v any) []any {
	switch s := v.(type) {
	case []any:
		return s
	case []string:
		out := make([]any, len(s))
		for i, e := range s {
			out[i] = e
		}
		return out
	}
	return nil
}
