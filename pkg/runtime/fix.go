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

	"google.golang.org/protobuf/reflect/protoreflect"
)

// LLMProvider represents different LLM providers for runtime selection
type LLMProvider string

const (
	LLMProviderStandard LLMProvider = "standard"
	LLMProviderOpenAI   LLMProvider = "openai"
)

// FixOpenAI applies all OpenAI compatibility transformations to convert OpenAI-formatted JSON
// back to standard protobuf-compatible JSON. This includes:
// - Converting map arrays back to objects
// - Converting string representations back to proper JSON for google.protobuf.Value/ListValue/Struct
// - Recursing into repeated message fields and map message values
//
// Handles both proto field names (snake_case) and JSON field names (camelCase)
// since different LLMs may use either naming convention.
func FixOpenAI(descriptor protoreflect.MessageDescriptor, args map[string]any) {
	var rewrite func(msg protoreflect.MessageDescriptor, obj map[string]any)

	rewrite = func(msg protoreflect.MessageDescriptor, obj map[string]any) {
		for i := 0; i < msg.Fields().Len(); i++ {
			field := msg.Fields().Get(i)

			// Resolve the actual key used in the JSON object.
			// LLMs might use proto name (snake_case) or JSON name (camelCase).
			name := resolveFieldName(field, obj)
			if name == "" {
				continue // field not present in object
			}

			if field.IsMap() {
				// Handle map conversion (from array-of-key-value-pairs to object)
				if arr, ok := obj[name].([]any); ok {
					m := make(map[string]any)
					for _, e := range arr {
						if pair, ok := e.(map[string]any); ok {
							k, kOk := pair["key"].(string)
							v, vOk := pair["value"]
							if kOk && vOk {
								m[k] = v
							}
						}
					}
					obj[name] = m
				}
				// After conversion, recurse into map values if they are messages
				if field.MapValue().Kind() == protoreflect.MessageKind {
					if m, ok := obj[name].(map[string]any); ok {
						for k, v := range m {
							if nested, ok := v.(map[string]any); ok {
								rewrite(field.MapValue().Message(), nested)
								m[k] = nested
							}
						}
					}
				}
			} else if field.Kind() == protoreflect.MessageKind {
				fullName := string(field.Message().FullName())

				if field.IsList() {
					// Handle repeated message fields, including well-known types
					if arr, ok := obj[name].([]any); ok {
						for i, elem := range arr {
							switch fullName {
							case "google.protobuf.Value":
								if str, ok := elem.(string); ok {
									var value any
									if err := json.Unmarshal([]byte(str), &value); err == nil {
										arr[i] = value
									}
								}
							case "google.protobuf.ListValue":
								if str, ok := elem.(string); ok {
									var listValue []any
									if err := json.Unmarshal([]byte(str), &listValue); err == nil {
										arr[i] = listValue
									}
								}
							case "google.protobuf.Struct":
								if str, ok := elem.(string); ok {
									var structValue map[string]any
									if err := json.Unmarshal([]byte(str), &structValue); err == nil {
										arr[i] = structValue
									}
								}
							default:
								if nested, ok := elem.(map[string]any); ok {
									rewrite(field.Message(), nested)
									arr[i] = nested
								}
							}
						}
					}
					continue
				}

				// Handle OpenAI string representations of special protobuf types
				switch fullName {
				case "google.protobuf.Value":
					if str, ok := obj[name].(string); ok {
						var value any
						if err := json.Unmarshal([]byte(str), &value); err == nil {
							obj[name] = value
						}
					}
				case "google.protobuf.ListValue":
					if str, ok := obj[name].(string); ok {
						var listValue []any
						if err := json.Unmarshal([]byte(str), &listValue); err == nil {
							obj[name] = listValue
						}
					}
				case "google.protobuf.Struct":
					if str, ok := obj[name].(string); ok {
						var structValue map[string]any
						if err := json.Unmarshal([]byte(str), &structValue); err == nil {
							obj[name] = structValue
						}
					}
				default:
					// Recursively process nested messages
					if nested, ok := obj[name].(map[string]any); ok {
						rewrite(field.Message(), nested)
					}
				}
			}
		}
	}

	rewrite(descriptor, args)
}

// resolveFieldName returns the key actually present in the JSON object for the given field.
// Checks proto name first (snake_case), then JSON name (camelCase).
// Returns empty string if neither is present.
func resolveFieldName(fd protoreflect.FieldDescriptor, obj map[string]any) string {
	protoName := string(fd.Name())
	if _, ok := obj[protoName]; ok {
		return protoName
	}
	jsonName := fd.JSONName()
	if _, ok := obj[jsonName]; ok {
		return jsonName
	}
	return ""
}
