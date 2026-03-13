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

// Package gen provides JSON schema generation from protobuf message descriptors.
// It can be used at code generation time (by the protoc plugin) or at runtime
// (via protoreflect) for dynamic schema generation.
package gen

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// SchemaOptions controls JSON schema generation behavior.
type SchemaOptions struct {
	// OpenAICompat enables OpenAI-compatible schema generation:
	// - additionalProperties: false on all objects
	// - all fields required
	// - maps become arrays of key-value pairs
	// - well-known types (Struct, Value, ListValue) become JSON strings
	OpenAICompat bool
}

// MessageSchema generates a JSON schema for a protobuf message descriptor.
// This is the main entry point for schema generation and can be used both
// at codegen time and at runtime with protoreflect.
func MessageSchema(md protoreflect.MessageDescriptor, opts SchemaOptions) map[string]any {
	required := []string{}
	normalFields := map[string]any{}
	oneOf := map[string][]map[string]any{}

	for i := 0; i < md.Fields().Len(); i++ {
		nestedFd := md.Fields().Get(i)
		name := string(nestedFd.Name())

		if oneof := nestedFd.ContainingOneof(); oneof != nil && !oneof.IsSynthetic() {
			if !opts.OpenAICompat {
				if _, ok := oneOf[string(oneof.Name())]; !ok {
					oneOf[string(oneof.Name())] = []map[string]any{}
				}
				oneOf[string(oneof.Name())] = append(oneOf[string(oneof.Name())], map[string]any{
					"properties": map[string]any{
						name: FieldSchema(nestedFd, opts),
					},
					"required": []string{name},
				})
			} else {
				schema := FieldSchema(nestedFd, opts)
				if v, ok := schema["type"].(string); ok {
					schema["type"] = []string{v, "null"}
				}
				normalFields[name] = schema
				schema["description"] = fmt.Sprintf("Note: This field is part of the '%s' oneof group. Only one field in this group can be set at a time. Setting multiple fields in the group WILL result in an error. Protobuf oneOf semantics apply.", oneof.Name())
				required = append(required, name)
			}
		} else {
			normalFields[name] = FieldSchema(nestedFd, opts)
			if IsFieldRequired(nestedFd) || opts.OpenAICompat {
				required = append(required, name)
			}
		}
	}

	// Build anyOf deterministically by iterating over the message's oneofs
	// in declaration order (not over the map which is non-deterministic).
	var anyOf []map[string]any
	for i := 0; i < md.Oneofs().Len(); i++ {
		oo := md.Oneofs().Get(i)
		if oo.IsSynthetic() {
			continue
		}
		if entries, ok := oneOf[string(oo.Name())]; ok {
			anyOf = append(anyOf, map[string]any{
				"oneOf":    entries,
				"$comment": "In this schema, there is a oneOf group for every protobuf oneOf block in the message.",
			})
		}
	}

	result := map[string]any{
		"type":       "object",
		"properties": normalFields,
		"required":   required,
	}
	if anyOf != nil {
		result["anyOf"] = anyOf
	}
	if opts.OpenAICompat {
		result["additionalProperties"] = false
		if v, ok := result["type"].(string); ok {
			result["type"] = []string{v, "null"}
		}
	}

	return result
}

// FieldSchema generates a JSON schema for a single protobuf field descriptor.
func FieldSchema(fd protoreflect.FieldDescriptor, opts SchemaOptions) map[string]any {
	if fd.IsMap() {
		return mapFieldSchema(fd, opts)
	}

	var schema map[string]any

	switch fd.Kind() {
	case protoreflect.MessageKind:
		schema = messageFieldSchema(fd, opts)
	case protoreflect.EnumKind:
		schema = enumFieldSchema(fd)
	default:
		schema = scalarFieldSchema(fd, opts)
	}

	constraints := ExtractValidateConstraints(fd)
	for key, value := range constraints {
		schema[key] = value
	}

	if fd.IsList() {
		return map[string]any{
			"type":  "array",
			"items": schema,
		}
	}
	return schema
}

func mapFieldSchema(fd protoreflect.FieldDescriptor, opts SchemaOptions) map[string]any {
	keyType := fd.MapKey().Kind()
	keyConstraints := map[string]any{"type": "string"}

	switch keyType {
	case protoreflect.BoolKind:
		keyConstraints["enum"] = []string{"true", "false"}
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind, protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		keyConstraints["pattern"] = "^(0|[1-9]\\d*)$"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind, protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		keyConstraints["pattern"] = "^-?(0|[1-9]\\d*)$"
	}

	if opts.OpenAICompat {
		return map[string]any{
			"type":        "array",
			"description": "List of key value pairs",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key":   map[string]any{"type": "string"},
					"value": FieldSchema(fd.MapValue(), opts),
				},
				"required":             []string{"key", "value"},
				"additionalProperties": false,
			},
		}
	}

	return map[string]any{
		"type":                 "object",
		"propertyNames":        keyConstraints,
		"additionalProperties": FieldSchema(fd.MapValue(), opts),
	}
}

func messageFieldSchema(fd protoreflect.FieldDescriptor, opts SchemaOptions) map[string]any {
	fullName := string(fd.Message().FullName())
	switch fullName {
	case "google.protobuf.Timestamp":
		return map[string]any{"type": []string{"string", "null"}, "format": "date-time"}
	case "google.protobuf.Duration":
		return map[string]any{"type": []string{"string", "null"}, "pattern": `^-?[0-9]+(\.[0-9]+)?s$`}
	case "google.protobuf.Struct":
		if opts.OpenAICompat {
			return map[string]any{
				"type":        "string",
				"description": "string representation of any JSON object. represents a google.protobuf.Struct, a dynamic JSON object.",
			}
		}
		return map[string]any{
			"type":                 "object",
			"additionalProperties": true,
		}
	case "google.protobuf.Value":
		if opts.OpenAICompat {
			return map[string]any{
				"type":        "string",
				"description": "string representation of any JSON value. represents a google.protobuf.Value, a dynamic JSON value (string, number, boolean, array, object).",
			}
		}
		return map[string]any{
			"description": "represents a google.protobuf.Value, a dynamic JSON value (string, number, boolean, array, object).",
		}
	case "google.protobuf.ListValue":
		if opts.OpenAICompat {
			return map[string]any{
				"type":        "string",
				"description": "string representation of a JSON array. represents a google.protobuf.ListValue, a JSON array of values.",
			}
		}
		return map[string]any{
			"type":        "array",
			"description": "represents a google.protobuf.ListValue, a JSON array of values.",
			"items":       map[string]any{},
		}
	case "google.protobuf.FieldMask":
		if opts.OpenAICompat {
			return map[string]any{"type": []string{"string", "null"}}
		}
		return map[string]any{"type": "string"}
	case "google.protobuf.Any":
		if opts.OpenAICompat {
			return map[string]any{
				"type": "object",
				"properties": map[string]any{
					"@type": map[string]any{"type": "string"},
					"value": map[string]any{},
				},
				"required":             []string{"@type", "value"},
				"additionalProperties": false,
			}
		}
		return map[string]any{
			"type": []string{"object", "null"},
			"properties": map[string]any{
				"@type": map[string]any{"type": "string"},
				"value": map[string]any{},
			},
			"required": []string{"@type"},
		}
	case "google.protobuf.DoubleValue", "google.protobuf.FloatValue",
		"google.protobuf.Int32Value", "google.protobuf.UInt32Value":
		return map[string]any{"type": []string{"number", "null"}}
	case "google.protobuf.Int64Value", "google.protobuf.UInt64Value":
		return map[string]any{"type": []string{"string", "null"}}
	case "google.protobuf.StringValue":
		return map[string]any{"type": []string{"string", "null"}}
	case "google.protobuf.BoolValue":
		return map[string]any{"type": []string{"boolean", "null"}}
	case "google.protobuf.BytesValue":
		if opts.OpenAICompat {
			return map[string]any{"type": []string{"string", "null"}}
		}
		return map[string]any{"type": []string{"string", "null"}, "format": "byte"}
	default:
		return MessageSchema(fd.Message(), opts)
	}
}

func enumFieldSchema(fd protoreflect.FieldDescriptor) map[string]any {
	var values []string
	for i := 0; i < fd.Enum().Values().Len(); i++ {
		values = append(values, string(fd.Enum().Values().Get(i).Name()))
	}
	return map[string]any{
		"type": "string",
		"enum": values,
	}
}

func scalarFieldSchema(fd protoreflect.FieldDescriptor, opts SchemaOptions) map[string]any {
	schema := map[string]any{
		"type": KindToType(fd.Kind()),
	}
	if fd.Kind() == protoreflect.BytesKind {
		schema["contentEncoding"] = "base64"
		if !opts.OpenAICompat {
			schema["format"] = "byte"
		}
	}
	return schema
}

// KindToType converts a protobuf field kind to a JSON Schema type string.
func KindToType(kind protoreflect.Kind) string {
	switch kind {
	case protoreflect.BoolKind:
		return "boolean"
	case protoreflect.StringKind:
		return "string"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "integer"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "string"
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return "number"
	case protoreflect.BytesKind:
		return "string"
	case protoreflect.EnumKind:
		return "string"
	default:
		return "string"
	}
}

// IsFieldRequired checks if a field has the REQUIRED field behavior annotation.
func IsFieldRequired(fd protoreflect.FieldDescriptor) bool {
	if proto.HasExtension(fd.Options(), annotations.E_FieldBehavior) {
		behaviors := proto.GetExtension(fd.Options(), annotations.E_FieldBehavior).([]annotations.FieldBehavior)
		for _, behavior := range behaviors {
			if behavior == annotations.FieldBehavior_REQUIRED {
				return true
			}
		}
	}
	return false
}

// ExtractValidateConstraints reads buf.validate constraints from a field
// and returns corresponding JSON Schema constraint keywords.
func ExtractValidateConstraints(fd protoreflect.FieldDescriptor) map[string]any {
	constraints := make(map[string]any)

	if !proto.HasExtension(fd.Options(), validate.E_Field) {
		return constraints
	}

	fieldConstraints := proto.GetExtension(fd.Options(), validate.E_Field).(*validate.FieldRules)
	if fieldConstraints == nil {
		return constraints
	}

	if stringRules := fieldConstraints.GetString(); stringRules != nil {
		if stringRules.GetUuid() {
			constraints["format"] = "uuid"
		}
		if stringRules.GetEmail() {
			constraints["format"] = "email"
		}
		if pattern := stringRules.GetPattern(); pattern != "" {
			constraints["pattern"] = pattern
		}
		if stringRules.HasMinLen() {
			constraints["minLength"] = int(stringRules.GetMinLen())
		}
		if stringRules.HasMaxLen() {
			constraints["maxLength"] = int(stringRules.GetMaxLen())
		}
	}

	if int32Rules := fieldConstraints.GetInt32(); int32Rules != nil {
		if int32Rules.HasGt() {
			constraints["minimum"] = int(int32Rules.GetGt()) + 1
		} else if int32Rules.HasGte() {
			constraints["minimum"] = int(int32Rules.GetGte())
		}
		if int32Rules.HasLt() {
			constraints["maximum"] = int(int32Rules.GetLt()) - 1
		} else if int32Rules.HasLte() {
			constraints["maximum"] = int(int32Rules.GetLte())
		}
	}

	if int64Rules := fieldConstraints.GetInt64(); int64Rules != nil {
		if int64Rules.HasGt() {
			constraints["minimum"] = int(int64Rules.GetGt()) + 1
		} else if int64Rules.HasGte() {
			constraints["minimum"] = int(int64Rules.GetGte())
		}
		if int64Rules.HasLt() {
			constraints["maximum"] = int(int64Rules.GetLt()) - 1
		} else if int64Rules.HasLte() {
			constraints["maximum"] = int(int64Rules.GetLte())
		}
	}

	if uint32Rules := fieldConstraints.GetUint32(); uint32Rules != nil {
		if uint32Rules.HasGt() {
			constraints["minimum"] = int(uint32Rules.GetGt()) + 1
		} else if uint32Rules.HasGte() {
			constraints["minimum"] = int(uint32Rules.GetGte())
		}
		if uint32Rules.HasLt() {
			constraints["maximum"] = int(uint32Rules.GetLt()) - 1
		} else if uint32Rules.HasLte() {
			constraints["maximum"] = int(uint32Rules.GetLte())
		}
	}

	if uint64Rules := fieldConstraints.GetUint64(); uint64Rules != nil {
		if uint64Rules.HasGt() {
			constraints["minimum"] = int(uint64Rules.GetGt()) + 1
		} else if uint64Rules.HasGte() {
			constraints["minimum"] = int(uint64Rules.GetGte())
		}
		if uint64Rules.HasLt() {
			constraints["maximum"] = int(uint64Rules.GetLt()) - 1
		} else if uint64Rules.HasLte() {
			constraints["maximum"] = int(uint64Rules.GetLte())
		}
	}

	if floatRules := fieldConstraints.GetFloat(); floatRules != nil {
		if floatRules.HasGt() {
			constraints["exclusiveMinimum"] = float64(floatRules.GetGt())
		} else if floatRules.HasGte() {
			constraints["minimum"] = float64(floatRules.GetGte())
		}
		if floatRules.HasLt() {
			constraints["exclusiveMaximum"] = float64(floatRules.GetLt())
		} else if floatRules.HasLte() {
			constraints["maximum"] = float64(floatRules.GetLte())
		}
	}

	if doubleRules := fieldConstraints.GetDouble(); doubleRules != nil {
		if doubleRules.HasGt() {
			constraints["exclusiveMinimum"] = doubleRules.GetGt()
		} else if doubleRules.HasGte() {
			constraints["minimum"] = doubleRules.GetGte()
		}
		if doubleRules.HasLt() {
			constraints["exclusiveMaximum"] = doubleRules.GetLt()
		} else if doubleRules.HasLte() {
			constraints["maximum"] = doubleRules.GetLte()
		}
	}

	return constraints
}

// CleanComment removes tool-specific comment prefixes (buf:lint, @ignore-comment).
func CleanComment(comment string) string {
	var cleanedLines []string
	strippedPrefixes := []string{"buf:lint:", "@ignore-comment"}
outer:
	for _, line := range strings.Split(comment, "\n") {
		trimmed := strings.TrimSpace(line)
		for _, strip := range strippedPrefixes {
			if strings.HasPrefix(trimmed, strip) {
				continue outer
			}
		}
		cleanedLines = append(cleanedLines, trimmed)
	}
	return strings.Join(cleanedLines, "\n")
}

// Base36String encodes a byte slice as a base-36 string.
func Base36String(b []byte) string {
	n := new(big.Int).SetBytes(b)
	return n.Text(36)
}

// MangleHeadIfTooLong truncates a tool name if it exceeds maxLen,
// using a hash prefix + the tail of the name (most specific part).
func MangleHeadIfTooLong(name string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(name) <= maxLen {
		return name
	}
	hash := sha1.Sum([]byte(name))
	fullHash := Base36String(hash[:])
	hashPrefix := fullHash
	if len(hashPrefix) > 6 {
		hashPrefix = hashPrefix[:6]
	}
	if maxLen <= len(hashPrefix) {
		return hashPrefix[:maxLen]
	}
	available := maxLen - len(hashPrefix) - 1
	if available <= 0 {
		return hashPrefix
	}
	tail := name[len(name)-available:]
	return hashPrefix + "_" + tail
}

// ToolForMethod generates standard and OpenAI-compatible MCP tools
// for a given RPC method descriptor.
func ToolForMethod(method protoreflect.MethodDescriptor, comment string) (standard, openAI mcp.Tool) {
	toolName := MangleHeadIfTooLong(strings.ReplaceAll(string(method.FullName()), ".", "_"), 64)
	description := CleanComment(comment)

	// Standard schema
	standardSchema := MessageSchema(method.Input(), SchemaOptions{OpenAICompat: false})
	marshaledStandard, err := json.Marshal(standardSchema)
	if err != nil {
		panic(err)
	}

	// OpenAI schema
	openAISchema := MessageSchema(method.Input(), SchemaOptions{OpenAICompat: true})
	openAISchema["type"] = "object" // Top-level must be plain "object", not ["object","null"]
	marshaledOpenAI, err := json.Marshal(openAISchema)
	if err != nil {
		panic(err)
	}

	standard = mcp.Tool{
		Name:           toolName,
		Description:    description,
		RawInputSchema: json.RawMessage(marshaledStandard),
	}
	openAI = mcp.Tool{
		Name:           toolName,
		Description:    description,
		RawInputSchema: json.RawMessage(marshaledOpenAI),
	}
	return
}
