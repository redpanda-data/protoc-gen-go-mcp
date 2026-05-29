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

// This file builds dynamic protobuf descriptors for oneof shapes the checked-in
// fixtures do not exercise — oneof inside a repeated field, inside a map value,
// inside another oneof, with well-known-type members, proto3 optional (synthetic
// oneof), a oneof literally named "which", and wide / single-member oneofs — and
// asserts both the generated schema shape and an encode/decode round-trip on
// dynamicpb messages. Building descriptors dynamically avoids regenerating the
// checked-in testdata and proves the gen + runtime code paths work on any
// descriptor, including the dynamic registration path.
package gen_test

import (
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/gen"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func strField(name string, num int32, oneofIdx *int32) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:       proto.String(name),
		Number:     proto.Int32(num),
		Label:      descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:       descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
		OneofIndex: oneofIdx,
	}
}

func i64Field(name string, num int32, oneofIdx *int32) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:       proto.String(name),
		Number:     proto.Int32(num),
		Label:      descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:       descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
		OneofIndex: oneofIdx,
	}
}

// buildEdgeShapesFile constructs one proto3 file with the edge-case messages and
// resolves it against the global registry (for the well-known-type imports).
func buildEdgeShapesFile(t *testing.T) protoreflect.FileDescriptor {
	t.Helper()
	i32 := func(v int32) *int32 { return &v }

	msgType := func(name, typeName string, num int32, oneofIdx *int32, label descriptorpb.FieldDescriptorProto_Label) *descriptorpb.FieldDescriptorProto {
		return &descriptorpb.FieldDescriptorProto{
			Name:       proto.String(name),
			Number:     proto.Int32(num),
			Label:      label.Enum(),
			Type:       descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
			TypeName:   proto.String(typeName),
			OneofIndex: oneofIdx,
		}
	}

	// Wide oneof members o1..o12.
	wide := make([]*descriptorpb.FieldDescriptorProto, 0, 12)
	for i := 1; i <= 12; i++ {
		wide = append(wide, strField("o"+itoa(i), int32(i), i32(0)))
	}

	fdp := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("edge_shapes.proto"),
		Package: proto.String("edgeshapes"),
		Syntax:  proto.String("proto3"),
		Dependency: []string{
			"google/protobuf/timestamp.proto",
			"google/protobuf/struct.proto",
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name:      proto.String("ListElem"),
				Field:     []*descriptorpb.FieldDescriptorProto{strField("text", 1, i32(0)), i64Field("number", 2, i32(0))},
				OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String("kind")}},
			},
			{
				Name:  proto.String("OneofInList"),
				Field: []*descriptorpb.FieldDescriptorProto{msgType("elements", ".edgeshapes.ListElem", 1, nil, descriptorpb.FieldDescriptorProto_LABEL_REPEATED)},
			},
			{
				Name:      proto.String("MapVal"),
				Field:     []*descriptorpb.FieldDescriptorProto{strField("text", 1, i32(0)), boolField("flag", 2, i32(0))},
				OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String("kind")}},
			},
			mapFieldMessage("OneofInMap", "entries", ".edgeshapes.MapVal"),
			{
				Name:      proto.String("InnerOneof"),
				Field:     []*descriptorpb.FieldDescriptorProto{strField("a", 1, i32(0)), i64Field("b", 2, i32(0))},
				OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String("ic")}},
			},
			{
				Name:      proto.String("OneofInOneof"),
				Field:     []*descriptorpb.FieldDescriptorProto{msgType("inner", ".edgeshapes.InnerOneof", 1, i32(0), descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL), strField("plain", 2, i32(0))},
				OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String("outer")}},
			},
			{
				Name: proto.String("WktInOneof"),
				Field: []*descriptorpb.FieldDescriptorProto{
					msgType("ts", ".google.protobuf.Timestamp", 1, i32(0), descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					msgType("st", ".google.protobuf.Struct", 2, i32(0), descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					msgType("val", ".google.protobuf.Value", 3, i32(0), descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL),
					strField("text", 4, i32(0)),
				},
				OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String("payload")}},
			},
			{
				Name: proto.String("SynthOpt"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: proto.String("nickname"), Number: proto.Int32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(), OneofIndex: i32(0), Proto3Optional: proto.Bool(true)},
					{Name: proto.String("count"), Number: proto.Int32(2), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(), OneofIndex: i32(1), Proto3Optional: proto.Bool(true)},
				},
				OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String("_nickname")}, {Name: proto.String("_count")}},
			},
			{
				Name:      proto.String("OneofNamedWhich"),
				Field:     []*descriptorpb.FieldDescriptorProto{strField("payload", 1, i32(0)), i64Field("amount", 2, i32(0))},
				OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String("which")}},
			},
			{
				Name:      proto.String("WideOneof"),
				Field:     wide,
				OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String("pick")}},
			},
			{
				Name:      proto.String("SingleOneof"),
				Field:     []*descriptorpb.FieldDescriptorProto{strField("just_me", 1, i32(0))},
				OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String("only")}},
			},
		},
	}

	fd, err := protodesc.NewFile(fdp, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatalf("build file: %v", err)
	}
	return fd
}

func boolField(name string, num int32, oneofIdx *int32) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:       proto.String(name),
		Number:     proto.Int32(num),
		Label:      descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:       descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum(),
		OneofIndex: oneofIdx,
	}
}

// mapFieldMessage builds a message with a single map<string, valueType> field.
func mapFieldMessage(msgName, fieldName, valueTypeName string) *descriptorpb.DescriptorProto {
	entryName := capitalize(fieldName) + "Entry"
	return &descriptorpb.DescriptorProto{
		Name: proto.String(msgName),
		Field: []*descriptorpb.FieldDescriptorProto{
			{
				Name:     proto.String(fieldName),
				Number:   proto.Int32(1),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
				TypeName: proto.String(".edgeshapes." + msgName + "." + entryName),
			},
		},
		NestedType: []*descriptorpb.DescriptorProto{
			{
				Name:    proto.String(entryName),
				Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: proto.String("key"), Number: proto.Int32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
					{Name: proto.String("value"), Number: proto.Int32(2), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(valueTypeName)},
				},
			},
		},
	}
}

func capitalize(s string) string { return strings.ToUpper(s[:1]) + s[1:] }

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

func msgByName(t *testing.T, fd protoreflect.FileDescriptor, name string) protoreflect.MessageDescriptor {
	t.Helper()
	md := fd.Messages().ByName(protoreflect.Name(name))
	if md == nil {
		t.Fatalf("message %q not found", name)
	}
	return md
}

// assertNoUnions walks a schema and fails on union/$ref keywords.
func assertNoUnions(t *testing.T, schema map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(schema)
	var generic any
	_ = json.Unmarshal(raw, &generic)
	var walk func(any)
	walk = func(v any) {
		switch n := v.(type) {
		case map[string]any:
			for _, bad := range []string{"anyOf", "oneOf", "allOf", "$ref", "$defs"} {
				if _, ok := n[bad]; ok {
					t.Errorf("forbidden keyword %q in schema", bad)
				}
			}
			for _, c := range n {
				walk(c)
			}
		case []any:
			for _, c := range n {
				walk(c)
			}
		}
	}
	walk(generic)
}

func TestEdgeShapes_SchemaAndRoundTrip(t *testing.T) {
	fd := buildEdgeShapesFile(t)
	rng := rand.New(rand.NewSource(0x0FF1CE))

	names := []string{
		"OneofInList", "OneofInMap", "OneofInOneof", "WktInOneof",
		"SynthOpt", "OneofNamedWhich", "WideOneof", "SingleOneof",
	}
	for _, name := range names {
		md := msgByName(t, fd, name)
		t.Run(name+"/schema", func(t *testing.T) {
			schema := gen.MessageSchema(md, gen.SchemaOptions{})
			schema["type"] = "object"
			assertNoUnions(t, schema)
			raw, err := json.Marshal(schema)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := jsonschema.CompileString(name+".json", string(raw)); err != nil {
				t.Fatalf("schema does not compile: %v\n%s", err, raw)
			}
		})
		t.Run(name+"/roundtrip", func(t *testing.T) {
			for i := 0; i < 50; i++ {
				orig := dynamicpb.NewMessage(md)
				populate(orig, rng, 0)
				encoded, err := runtime.EncodeMessage(orig)
				if err != nil {
					t.Fatalf("encode: %v", err)
				}
				var args map[string]any
				if err := json.Unmarshal(encoded, &args); err != nil {
					t.Fatalf("unmarshal: %v\n%s", err, encoded)
				}
				if err := runtime.DecodeArguments(md, args); err != nil {
					t.Fatalf("decode: %v\nencoded: %s", err, encoded)
				}
				reArgs, err := json.Marshal(args)
				if err != nil {
					t.Fatal(err)
				}
				got := dynamicpb.NewMessage(md)
				if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(reArgs, got); err != nil {
					t.Fatalf("protojson: %v\nargs: %s", err, reArgs)
				}
				if diff := cmp.Diff(orig, got, protocmp.Transform()); diff != "" {
					t.Fatalf("iter %d mismatch (-want +got):\n%s\nencoded: %s", i, diff, encoded)
				}
			}
		})
	}
}

// TestEdgeShapes_SyntheticOptionalIsPlainField asserts a proto3 optional renders
// as a normal optional field, NOT a discriminated oneof wrapper.
func TestEdgeShapes_SyntheticOptionalIsPlainField(t *testing.T) {
	fd := buildEdgeShapesFile(t)
	md := msgByName(t, fd, "SynthOpt")
	schema := gen.MessageSchema(md, gen.SchemaOptions{})
	props := schema["properties"].(map[string]any)
	for _, f := range []string{"nickname", "count"} {
		p, ok := props[f].(map[string]any)
		if !ok {
			t.Fatalf("synthetic-optional field %q missing as plain property", f)
		}
		if _, isWrapper := p["properties"]; isWrapper {
			t.Errorf("%q rendered as a oneof wrapper, want plain field", f)
		}
	}
	// No wrapper named after a synthetic oneof.
	if _, ok := props["_nickname"]; ok {
		t.Errorf("synthetic oneof leaked into schema")
	}
}

// TestEdgeShapes_OneofNamedWhich asserts a oneof literally named "which" nests as
// which.which and round-trips.
func TestEdgeShapes_OneofNamedWhich(t *testing.T) {
	fd := buildEdgeShapesFile(t)
	md := msgByName(t, fd, "OneofNamedWhich")
	schema := gen.MessageSchema(md, gen.SchemaOptions{})
	props := schema["properties"].(map[string]any)
	wrapper, ok := props["which"].(map[string]any)
	if !ok {
		t.Fatalf("expected wrapper named which")
	}
	raw, _ := json.Marshal(wrapper["properties"])
	var wp map[string]any
	_ = json.Unmarshal(raw, &wp)
	if _, ok := wp["which"]; !ok {
		t.Errorf("discriminator which.which missing: %v", wp)
	}

	// Round-trip with the discriminator path which.which.
	orig := dynamicpb.NewMessage(md)
	args := map[string]any{"which": map[string]any{"which": "payload", "payload": "hi"}}
	if err := runtime.DecodeArguments(md, args); err != nil {
		t.Fatalf("decode: %v", err)
	}
	reArgs, _ := json.Marshal(args)
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(reArgs, orig); err != nil {
		t.Fatalf("protojson: %v\n%s", err, reArgs)
	}
	if orig.Get(md.Fields().ByName("payload")).String() != "hi" {
		t.Fatalf("payload not set via which.which: %v", orig)
	}
}

// TestEdgeShapes_WktInOneofMembers exercises well-known-type members of a oneof
// explicitly (the random populator falls back to scalar members), on both the
// native path and the stringified strict-provider path.
func TestEdgeShapes_WktInOneofMembers(t *testing.T) {
	fd := buildEdgeShapesFile(t)
	md := msgByName(t, fd, "WktInOneof")

	t.Run("struct member native round trip", func(t *testing.T) {
		orig := dynamicpb.NewMessage(md)
		sv, err := structpb.NewStruct(map[string]any{"k": "v", "n": float64(1)})
		if err != nil {
			t.Fatal(err)
		}
		orig.Set(md.Fields().ByName("st"), protoreflect.ValueOfMessage(sv.ProtoReflect()))

		encoded, err := runtime.EncodeMessage(orig)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		// Encode must rewrap into the discriminated object with which=st.
		var decoded map[string]any
		_ = json.Unmarshal(encoded, &decoded)
		payload, ok := decoded["payload"].(map[string]any)
		if !ok || payload["which"] != "st" {
			t.Fatalf("payload not rewrapped to which=st: %s", encoded)
		}

		got := dynamicpb.NewMessage(md)
		if err := runtime.DecodeArguments(md, decoded); err != nil {
			t.Fatalf("decode: %v", err)
		}
		re, _ := json.Marshal(decoded)
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(re, got); err != nil {
			t.Fatalf("protojson: %v\n%s", err, re)
		}
		if diff := cmp.Diff(orig, got, protocmp.Transform()); diff != "" {
			t.Fatalf("struct-in-oneof mismatch:\n%s", diff)
		}
	})

	t.Run("struct member stringified strict path decodes", func(t *testing.T) {
		// A strict provider downgrades the st member to a JSON string inside the
		// wrapper; decode must lift the oneof and parse the stringified Struct.
		got := dynamicpb.NewMessage(md)
		args := map[string]any{"payload": map[string]any{"which": "st", "st": `{"k":"v"}`}}
		if err := runtime.DecodeArguments(md, args); err != nil {
			t.Fatalf("decode: %v", err)
		}
		re, _ := json.Marshal(args)
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(re, got); err != nil {
			t.Fatalf("protojson: %v\n%s", err, re)
		}
		stFd := md.Fields().ByName("st")
		if !got.Has(stFd) {
			t.Fatalf("st oneof member not set: %v", got)
		}
		// Verify the nested Struct via protojson rather than a concrete type
		// assertion (dynamicpb nests it as a *dynamicpb.Message).
		out, err := protojson.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(out), `"k":"v"`) {
			t.Fatalf("stringified struct member not parsed into Struct: %s", out)
		}
	})

	t.Run("timestamp member native round trip", func(t *testing.T) {
		orig := dynamicpb.NewMessage(md)
		ts := timestamppb.New(time.Unix(1700000000, 0).UTC())
		orig.Set(md.Fields().ByName("ts"), protoreflect.ValueOfMessage(ts.ProtoReflect()))
		encoded, err := runtime.EncodeMessage(orig)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		got := dynamicpb.NewMessage(md)
		var decoded map[string]any
		_ = json.Unmarshal(encoded, &decoded)
		if err := runtime.DecodeArguments(md, decoded); err != nil {
			t.Fatalf("decode: %v", err)
		}
		re, _ := json.Marshal(decoded)
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(re, got); err != nil {
			t.Fatalf("protojson: %v\n%s", err, re)
		}
		if diff := cmp.Diff(orig, got, protocmp.Transform()); diff != "" {
			t.Fatalf("timestamp-in-oneof mismatch:\n%s", diff)
		}
	})
}
