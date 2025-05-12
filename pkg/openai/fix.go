// Copyright 2025 Redpanda Data, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package openai

import "google.golang.org/protobuf/reflect/protoreflect"

// ReplaceArrayWithMap replaces an array of key/value pairs with a proper map.
// The generator can output a JSON schema, that describes protobuf maps as an array of key/value pairs,
// because OpenAI does not support dynamic maps (additionalProperties).
// This function here translates it back, before the generated code uses protojson.Unmarshal.
func FixMap(descriptor protoreflect.MessageDescriptor, args map[string]any) {
	var rewrite func(msg protoreflect.MessageDescriptor, path []string, obj map[string]any)

	rewrite = func(msg protoreflect.MessageDescriptor, path []string, obj map[string]any) {
		for i := 0; i < msg.Fields().Len(); i++ {
			field := msg.Fields().Get(i)
			name := string(field.Name())
			currentPath := append(path, name)

			if field.IsMap() {
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
				if nested, ok := obj[name].(map[string]any); ok {
					rewrite(field.Message(), currentPath, nested)
				}
			}
		}
	}

	rewrite(descriptor, nil, args)
}
