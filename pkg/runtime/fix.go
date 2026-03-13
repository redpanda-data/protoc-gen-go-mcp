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
	"fmt"
	"math"
	"strconv"

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
							rawKey, hasKey := pair["key"]
							v, hasVal := pair["value"]
							if !hasKey || !hasVal {
								continue
							}
							// Coerce non-string keys (LLMs may send int/bool map keys as native JSON types)
							var k string
							switch kt := rawKey.(type) {
							case string:
								k = kt
							case float64:
								// JSON numbers are float64. Format as integer if possible
								// to match protojson's expected map key format.
								if kt == math.Trunc(kt) {
									k = strconv.FormatInt(int64(kt), 10)
								} else {
									k = strconv.FormatFloat(kt, 'f', -1, 64)
								}
							case bool:
								k = strconv.FormatBool(kt)
							default:
								k = fmt.Sprintf("%v", kt)
							}
							// Skip null values -- protojson rejects null for map values
							if v == nil {
								continue
							}
							m[k] = v
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
				case "google.protobuf.DoubleValue", "google.protobuf.FloatValue",
					"google.protobuf.Int32Value", "google.protobuf.UInt32Value",
					"google.protobuf.Int64Value", "google.protobuf.UInt64Value",
					"google.protobuf.StringValue", "google.protobuf.BoolValue",
					"google.protobuf.BytesValue":
					// Unwrap {"value": X} -> X for wrapper types.
					// LLMs sometimes send the protobuf message form instead of
					// the plain scalar that protojson expects.
					if wrapped, ok := obj[name].(map[string]any); ok {
						if val, hasVal := wrapped["value"]; hasVal {
							obj[name] = val
						}
					}
				default:
					// Recursively process nested messages.
					// The value may be a JSON object (normal case) or a JSON string
					// (from depth-limited recursive schema expansion in OpenAI mode,
					// where deep recursion levels are encoded as string placeholders).
					switch v := obj[name].(type) {
					case map[string]any:
						rewrite(field.Message(), v)
					case string:
						var parsed map[string]any
						if err := json.Unmarshal([]byte(v), &parsed); err == nil {
							rewrite(field.Message(), parsed)
							obj[name] = parsed
						}
					}
				}
			}
		}

		// Strip null oneof alternatives. In OpenAI strict mode, all fields are
		// required so LLMs send all oneof members. protojson rejects messages
		// with multiple oneof fields set. Keep only the first non-null alternative.
		for i := 0; i < msg.Oneofs().Len(); i++ {
			oo := msg.Oneofs().Get(i)
			if oo.IsSynthetic() {
				continue
			}
			foundNonNull := false
			for j := 0; j < oo.Fields().Len(); j++ {
				fd := oo.Fields().Get(j)
				name := resolveFieldName(fd, obj)
				if name == "" {
					continue
				}
				if obj[name] == nil {
					delete(obj, name)
				} else if foundNonNull {
					delete(obj, name)
				} else {
					foundNonNull = true
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
