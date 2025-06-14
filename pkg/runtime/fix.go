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

// FixOpenAI applies all OpenAI compatibility transformations to convert OpenAI-formatted JSON
// back to standard protobuf-compatible JSON. This includes:
// - Converting map arrays back to objects
// - Converting string representations back to proper JSON for google.protobuf.Value/ListValue/Struct
func FixOpenAI(descriptor protoreflect.MessageDescriptor, args map[string]any) {
	var rewrite func(msg protoreflect.MessageDescriptor, path []string, obj map[string]any)

	rewrite = func(msg protoreflect.MessageDescriptor, path []string, obj map[string]any) {
		for i := 0; i < msg.Fields().Len(); i++ {
			field := msg.Fields().Get(i)
			name := string(field.Name())
			currentPath := append(path, name)

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
			} else if field.Kind() == protoreflect.MessageKind {
				fullName := string(field.Message().FullName())

				// Handle OpenAI string representations of special protobuf types
				switch fullName {
				case "google.protobuf.Value":
					if str, ok := obj[name].(string); ok {
						var value interface{}
						if err := json.Unmarshal([]byte(str), &value); err == nil {
							obj[name] = value
						}
					}
				case "google.protobuf.ListValue":
					if str, ok := obj[name].(string); ok {
						var listValue []interface{}
						if err := json.Unmarshal([]byte(str), &listValue); err == nil {
							obj[name] = listValue
						}
					}
				case "google.protobuf.Struct":
					if str, ok := obj[name].(string); ok {
						var structValue map[string]interface{}
						if err := json.Unmarshal([]byte(str), &structValue); err == nil {
							obj[name] = structValue
						}
					}
				default:
					// Recursively process nested messages
					if nested, ok := obj[name].(map[string]any); ok {
						rewrite(field.Message(), currentPath, nested)
					}
				}
			}
		}
	}

	rewrite(descriptor, nil, args)
}
