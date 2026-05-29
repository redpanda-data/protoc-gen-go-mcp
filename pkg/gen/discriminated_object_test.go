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

package gen

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// allTestServices returns every service descriptor in the testdata file set.
func allTestServices(t *testing.T) []protoreflect.ServiceDescriptor {
	t.Helper()
	file := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor().ParentFile()
	edge := (&testdata.DeepNestingRequest{}).ProtoReflect().Descriptor().ParentFile()
	var out []protoreflect.ServiceDescriptor
	for _, f := range []protoreflect.FileDescriptor{file, edge} {
		for i := 0; i < f.Services().Len(); i++ {
			out = append(out, f.Services().Get(i))
		}
	}
	return out
}

// walkNoUnions fails if any object anywhere in the schema carries a forbidden
// keyword. Providers reject these as a tool input_schema.
func walkNoUnions(t *testing.T, v any, path string) {
	t.Helper()
	switch n := v.(type) {
	case map[string]any:
		for _, bad := range []string{"anyOf", "oneOf", "allOf", "$ref", "$defs"} {
			if _, ok := n[bad]; ok {
				t.Errorf("forbidden keyword %q at %s", bad, path)
			}
		}
		for k, child := range n {
			walkNoUnions(t, child, path+"."+k)
		}
	case []any:
		for i, child := range n {
			walkNoUnions(t, child, path)
			_ = i
		}
	}
}

func TestSchema_NoTopLevelOrNestedUnions(t *testing.T) {
	for _, sd := range allTestServices(t) {
		for i := 0; i < sd.Methods().Len(); i++ {
			m := sd.Methods().Get(i)
			for _, md := range []protoreflect.MessageDescriptor{m.Input(), m.Output()} {
				schema := MessageSchema(md, SchemaOptions{})
				if schema["type"] != "object" {
					// top-level may be []string{"object","null"}; marshalTopLevelSchema
					// forces "object", so emulate that here.
					schema["type"] = "object"
				}
				raw, err := json.Marshal(schema)
				if err != nil {
					t.Fatalf("marshal %s: %v", md.FullName(), err)
				}
				var generic any
				if err := json.Unmarshal(raw, &generic); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				walkNoUnions(t, generic, string(md.FullName()))
			}
		}
	}
}

func TestSchema_OneofIsDiscriminatedObject(t *testing.T) {
	md := (&testdata.MultipleOneofsRequest{}).ProtoReflect().Descriptor()
	schema := MessageSchema(md, SchemaOptions{})
	props := schema["properties"].(map[string]any)

	for _, oneof := range []struct {
		name    string
		members []string
	}{
		{"source", []string{"url", "raw_data", "file_path"}},
		{"output_format", []string{"as_json", "as_xml", "as_csv"}},
	} {
		wrapperAny, ok := props[oneof.name]
		if !ok {
			t.Fatalf("missing wrapper %q", oneof.name)
		}
		wrapper := wrapperAny.(map[string]any)
		if wrapper["type"] != "object" {
			t.Errorf("%s: wrapper type = %v, want object", oneof.name, wrapper["type"])
		}
		// required is exactly ["which"].
		req := toStringSlice(wrapper["required"])
		if len(req) != 1 || req[0] != "which" {
			t.Errorf("%s: required = %v, want [which]", oneof.name, req)
		}
		// "which" enum carries the proto member names.
		// wrapper["properties"] is an *orderedMap; round-trip through JSON.
		wp := remarshal(t, wrapper["properties"])
		which := wp["which"].(map[string]any)
		if which["type"] != "string" {
			t.Errorf("%s.which type = %v", oneof.name, which["type"])
		}
		gotEnum := toStringSlice(which["enum"])
		if strings.Join(gotEnum, ",") != strings.Join(oneof.members, ",") {
			t.Errorf("%s.which enum = %v, want %v", oneof.name, gotEnum, oneof.members)
		}
		// Members present as sibling properties; none are required.
		for _, mem := range oneof.members {
			if _, ok := wp[mem]; !ok {
				t.Errorf("%s: missing member property %q", oneof.name, mem)
			}
		}
	}

	// Neither oneof is required on the parent (no validate.oneof.required), but
	// the field-behavior REQUIRED "name" is.
	parentReq := toStringSlice(schema["required"])
	if !contains(parentReq, "name") {
		t.Errorf("parent required = %v, want to contain name", parentReq)
	}
	if contains(parentReq, "source") || contains(parentReq, "output_format") {
		t.Errorf("plain oneofs must not be in parent required: %v", parentReq)
	}
}

func TestSchema_WhichIsFirstPropertyInMarshaledBytes(t *testing.T) {
	// Ordering is a written invariant: the model must read "which" before the
	// members. marshalTopLevelSchema preserves insertion order via orderedMap.
	raw := marshalTopLevelSchema((&testdata.MultipleOneofsRequest{}).ProtoReflect().Descriptor(), SchemaOptions{})
	s := string(raw)
	for _, oneof := range []string{"source", "output_format"} {
		idx := strings.Index(s, `"`+oneof+`":{`)
		if idx < 0 {
			t.Fatalf("wrapper %q not found", oneof)
		}
		rest := s[idx:]
		propsIdx := strings.Index(rest, `"properties":{`)
		if propsIdx < 0 {
			t.Fatalf("%s properties not found", oneof)
		}
		afterProps := rest[propsIdx+len(`"properties":{`):]
		if !strings.HasPrefix(afterProps, `"which":`) {
			t.Errorf("%s: first property is not which: %.40s", oneof, afterProps)
		}
	}
}

func TestSchema_RequiredOneofLandsInParentRequired(t *testing.T) {
	md := (&testdata.RequiredOneofRequest{}).ProtoReflect().Descriptor()
	schema := MessageSchema(md, SchemaOptions{})
	req := toStringSlice(schema["required"])
	if !contains(req, "choice") {
		t.Errorf("required oneof 'choice' missing from parent required: %v", req)
	}
	if contains(req, "optional_choice") {
		t.Errorf("plain oneof 'optional_choice' must not be required: %v", req)
	}
}

func TestSchema_PanicsOnMemberNamedWhich(t *testing.T) {
	md := buildOneofWithMemberNamed(t, "which")
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic for oneof member named 'which'")
		}
		if !strings.Contains(toString(r), "which") {
			t.Fatalf("panic message should mention the collision: %v", r)
		}
	}()
	_ = MessageSchema(md, SchemaOptions{})
}

func TestSchema_OneofNamedWhichIsAllowed(t *testing.T) {
	// A oneof *named* which (not a member) is fine; it nests as which.which.
	md := buildOneofNamed(t, "which", "x")
	schema := MessageSchema(md, SchemaOptions{})
	props := schema["properties"].(map[string]any)
	if _, ok := props["which"]; !ok {
		t.Fatalf("expected wrapper named which")
	}
}

// --- helpers -----------------------------------------------------------------

func toStringSlice(v any) []string {
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		out := make([]string, 0, len(s))
		for _, e := range s {
			out = append(out, e.(string))
		}
		return out
	default:
		return nil
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func remarshal(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func toString(v any) string {
	return fmt.Sprintf("%v", v)
}

// buildOneofWithMemberNamed builds a dynamic message descriptor with a oneof
// that has a member field literally named memberName.
func buildOneofWithMemberNamed(t *testing.T, memberName string) protoreflect.MessageDescriptor {
	t.Helper()
	return buildMessage(t, &descriptorpb.DescriptorProto{
		Name: proto.String("WhichCollision"),
		Field: []*descriptorpb.FieldDescriptorProto{
			{
				Name:       proto.String(memberName),
				Number:     proto.Int32(1),
				Label:      descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:       descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				OneofIndex: proto.Int32(0),
			},
			{
				Name:       proto.String("other"),
				Number:     proto.Int32(2),
				Label:      descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:       descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				OneofIndex: proto.Int32(0),
			},
		},
		OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String("pick")}},
	})
}

// buildOneofNamed builds a message with a oneof named oneofName and one member.
func buildOneofNamed(t *testing.T, oneofName, memberName string) protoreflect.MessageDescriptor {
	t.Helper()
	return buildMessage(t, &descriptorpb.DescriptorProto{
		Name: proto.String("OneofNamed"),
		Field: []*descriptorpb.FieldDescriptorProto{
			{
				Name:       proto.String(memberName),
				Number:     proto.Int32(1),
				Label:      descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:       descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				OneofIndex: proto.Int32(0),
			},
		},
		OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String(oneofName)}},
	})
}

func buildMessage(t *testing.T, msg *descriptorpb.DescriptorProto) protoreflect.MessageDescriptor {
	t.Helper()
	fdp := &descriptorpb.FileDescriptorProto{
		Name:        proto.String("dynamic_test.proto"),
		Package:     proto.String("dyn"),
		Syntax:      proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{msg},
	}
	fd, err := protodesc.NewFile(fdp, nil)
	if err != nil {
		t.Fatalf("build file: %v", err)
	}
	return fd.Messages().Get(0)
}
