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
				target := obj
				for _, key := range currentPath[:len(currentPath)-1] {
					if nested, ok := target[key].(map[string]any); ok {
						target = nested
					} else {
						return
					}
				}
				if arr, ok := target[name].([]any); ok {
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
					target[name] = m
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
