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

package gen_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/gen"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/testing/protocmp"
)

// populateMaxDepth deliberately exceeds the schema's recursion depth (3) so the
// fuzzer builds recursive structures that exercise the encode-side string
// placeholder / decode-side re-parse path.
const populateMaxDepth = 5

// messageFactories returns fresh instances of the messages under test.
func messageFactories() map[string]func() proto.Message {
	return map[string]func() proto.Message{
		"MultipleOneofsRequest":  func() proto.Message { return &testdata.MultipleOneofsRequest{} },
		"CreateItemRequest":      func() proto.Message { return &testdata.CreateItemRequest{} },
		"OneofRecursiveRequest":  func() proto.Message { return &testdata.OneofRecursiveRequest{} },
		"OneofRecursiveResponse": func() proto.Message { return &testdata.OneofRecursiveResponse{} },
		"DeepNestingRequest":     func() proto.Message { return &testdata.DeepNestingRequest{} },
		"RequiredOneofRequest":   func() proto.Message { return &testdata.RequiredOneofRequest{} },
		"MapVariantsRequest":     func() proto.Message { return &testdata.MapVariantsRequest{} },
		"EnumFieldsRequest":      func() proto.Message { return &testdata.EnumFieldsRequest{} },
		"AllScalarTypesRequest":  func() proto.Message { return &testdata.AllScalarTypesRequest{} },
		"RecursiveTreeRequest":   func() proto.Message { return &testdata.RecursiveTreeRequest{} },
	}
}

// TestProperty_RoundTrip is the guardrail that would have caught the original
// top-level-anyOf bug: for many randomly populated messages, encoding to the
// model-facing JSON and decoding back must reproduce the original message.
func TestProperty_RoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(0xC0DEC))
	for name, factory := range messageFactories() {
		t.Run(name, func(t *testing.T) {
			for i := 0; i < 200; i++ {
				orig := factory()
				populate(orig.ProtoReflect(), rng, 0)

				encoded, err := runtime.EncodeMessage(orig)
				if err != nil {
					t.Fatalf("encode: %v", err)
				}
				var args map[string]any
				if err := json.Unmarshal(encoded, &args); err != nil {
					t.Fatalf("unmarshal encoded: %v\n%s", err, encoded)
				}
				if err := runtime.DecodeArguments(orig.ProtoReflect().Descriptor(), args); err != nil {
					t.Fatalf("decode: %v\nencoded: %s", err, encoded)
				}
				reArgs, err := json.Marshal(args)
				if err != nil {
					t.Fatalf("re-marshal: %v", err)
				}
				got := factory()
				if err := protoUnmarshal(reArgs, got); err != nil {
					t.Fatalf("protojson: %v\ndecoded args: %s", err, reArgs)
				}
				if diff := cmp.Diff(orig, got, protocmp.Transform()); diff != "" {
					t.Fatalf("iter %d mismatch (-want +got):\n%s\nencoded: %s", i, diff, encoded)
				}
			}
		})
	}
}

// TestProperty_EncodedConformsToSchema validates that the JSON produced by the
// encode path is accepted by the very JSON Schema the generator emits.
func TestProperty_EncodedConformsToSchema(t *testing.T) {
	rng := rand.New(rand.NewSource(0x5CEDA))
	for name, factory := range messageFactories() {
		md := factory().ProtoReflect().Descriptor()
		schema := gen.MessageSchema(md, gen.SchemaOptions{})
		schema["type"] = "object"
		raw, err := json.Marshal(schema)
		if err != nil {
			t.Fatalf("marshal schema %s: %v", name, err)
		}
		compiled, err := jsonschema.CompileString(name+".json", string(raw))
		if err != nil {
			t.Fatalf("compile schema %s: %v\n%s", name, err, raw)
		}
		t.Run(name, func(t *testing.T) {
			for i := 0; i < 100; i++ {
				msg := factory()
				populate(msg.ProtoReflect(), rng, 0)
				encoded, err := runtime.EncodeMessage(msg)
				if err != nil {
					t.Fatalf("encode: %v", err)
				}
				var doc any
				if err := json.Unmarshal(encoded, &doc); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				if err := compiled.Validate(doc); err != nil {
					t.Fatalf("iter %d: encoded output violates its own schema: %v\nschema: %s\noutput: %s", i, err, raw, encoded)
				}
			}
		})
	}
}

func protoUnmarshal(b []byte, m proto.Message) error {
	return (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(b, m)
}

// --- reflective random populator ---------------------------------------------

func populate(m protoreflect.Message, rng *rand.Rand, depth int) {
	md := m.Descriptor()

	// Non-oneof (and synthetic-oneof / proto3 optional) fields.
	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		if oo := fd.ContainingOneof(); oo != nil && !oo.IsSynthetic() {
			continue
		}
		setField(m, fd, rng, depth)
	}

	// Real oneofs: always pick exactly one member so required oneofs are set.
	for i := 0; i < md.Oneofs().Len(); i++ {
		oo := md.Oneofs().Get(i)
		if oo.IsSynthetic() {
			continue
		}
		fd := oo.Fields().Get(rng.Intn(oo.Fields().Len()))
		// If picking a message member that we would skip for depth, fall back to
		// a scalar member when one exists, so the oneof is reliably set.
		if isSkippableMessage(fd, depth) {
			if alt := firstScalarMember(oo); alt != nil {
				fd = alt
			}
		}
		setField(m, fd, rng, depth)
	}
}

func setField(m protoreflect.Message, fd protoreflect.FieldDescriptor, rng *rand.Rand, depth int) {
	switch {
	case fd.IsMap():
		populateMap(m, fd, rng, depth)
	case fd.IsList():
		populateList(m, fd, rng, depth)
	default:
		v, ok := scalarOrMessage(m, fd, rng, depth)
		if ok {
			m.Set(fd, v)
		}
	}
}

func populateList(m protoreflect.Message, fd protoreflect.FieldDescriptor, rng *rand.Rand, depth int) {
	if fd.Kind() == protoreflect.MessageKind && (isWKT(fd.Message()) || depth >= populateMaxDepth) {
		return
	}
	lst := m.NewField(fd).List()
	n := rng.Intn(3)
	for i := 0; i < n; i++ {
		if fd.Kind() == protoreflect.MessageKind {
			ev := lst.NewElement()
			populate(ev.Message(), rng, depth+1)
			lst.Append(ev)
		} else {
			lst.Append(scalarValue(fd, rng))
		}
	}
	if lst.Len() > 0 {
		m.Set(fd, protoreflect.ValueOfList(lst))
	}
}

func populateMap(m protoreflect.Message, fd protoreflect.FieldDescriptor, rng *rand.Rand, depth int) {
	valFd := fd.MapValue()
	if valFd.Kind() == protoreflect.MessageKind && (isWKT(valFd.Message()) || depth >= populateMaxDepth) {
		return
	}
	mp := m.NewField(fd).Map()
	n := rng.Intn(3)
	for i := 0; i < n; i++ {
		key := mapKey(fd.MapKey(), rng, i)
		if valFd.Kind() == protoreflect.MessageKind {
			vv := mp.NewValue()
			populate(vv.Message(), rng, depth+1)
			mp.Set(key, vv)
		} else {
			mp.Set(key, scalarValue(valFd, rng))
		}
	}
	if mp.Len() > 0 {
		m.Set(fd, protoreflect.ValueOfMap(mp))
	}
}

func scalarOrMessage(m protoreflect.Message, fd protoreflect.FieldDescriptor, rng *rand.Rand, depth int) (protoreflect.Value, bool) {
	if fd.Kind() == protoreflect.MessageKind {
		if isWKT(fd.Message()) || depth >= populateMaxDepth {
			return protoreflect.Value{}, false
		}
		v := m.NewField(fd)
		populate(v.Message(), rng, depth+1)
		return v, true
	}
	return scalarValue(fd, rng), true
}

func scalarValue(fd protoreflect.FieldDescriptor, rng *rand.Rand) protoreflect.Value {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return protoreflect.ValueOfBool(rng.Intn(2) == 1)
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return protoreflect.ValueOfInt32(rng.Int31() - rng.Int31())
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return protoreflect.ValueOfUint32(rng.Uint32())
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return protoreflect.ValueOfInt64(rng.Int63() - rng.Int63())
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return protoreflect.ValueOfUint64(rng.Uint64())
	case protoreflect.FloatKind:
		return protoreflect.ValueOfFloat32(float32(rng.Intn(100000)) / 100)
	case protoreflect.DoubleKind:
		return protoreflect.ValueOfFloat64(float64(rng.Intn(100000)) / 100)
	case protoreflect.StringKind:
		return protoreflect.ValueOfString(randString(rng))
	case protoreflect.BytesKind:
		b := make([]byte, rng.Intn(8))
		_, _ = rng.Read(b)
		return protoreflect.ValueOfBytes(b)
	case protoreflect.EnumKind:
		vals := fd.Enum().Values()
		return protoreflect.ValueOfEnum(vals.Get(rng.Intn(vals.Len())).Number())
	default:
		return protoreflect.ValueOfString(randString(rng))
	}
}

func mapKey(fd protoreflect.FieldDescriptor, rng *rand.Rand, i int) protoreflect.MapKey {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return protoreflect.ValueOfBool(i%2 == 0).MapKey()
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return protoreflect.ValueOfInt32(int32(i + 1)).MapKey()
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return protoreflect.ValueOfUint32(uint32(i + 1)).MapKey()
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return protoreflect.ValueOfInt64(int64(i + 1)).MapKey()
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return protoreflect.ValueOfUint64(uint64(i + 1)).MapKey()
	default:
		return protoreflect.ValueOfString(fmt.Sprintf("k%d", i)).MapKey()
	}
}

func randString(rng *rand.Rand) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 -_"
	n := 1 + rng.Intn(8)
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[rng.Intn(len(alphabet))]
	}
	return string(b)
}

func isSkippableMessage(fd protoreflect.FieldDescriptor, depth int) bool {
	return fd.Kind() == protoreflect.MessageKind && !fd.IsList() && (isWKT(fd.Message()) || depth >= populateMaxDepth)
}

func firstScalarMember(oo protoreflect.OneofDescriptor) protoreflect.FieldDescriptor {
	for i := 0; i < oo.Fields().Len(); i++ {
		fd := oo.Fields().Get(i)
		if fd.Kind() != protoreflect.MessageKind {
			return fd
		}
	}
	return nil
}

func isWKT(md protoreflect.MessageDescriptor) bool {
	switch string(md.FullName()) {
	case "google.protobuf.Timestamp", "google.protobuf.Duration", "google.protobuf.Struct",
		"google.protobuf.Value", "google.protobuf.ListValue", "google.protobuf.FieldMask",
		"google.protobuf.Any", "google.protobuf.DoubleValue", "google.protobuf.FloatValue",
		"google.protobuf.Int32Value", "google.protobuf.UInt32Value", "google.protobuf.Int64Value",
		"google.protobuf.UInt64Value", "google.protobuf.StringValue", "google.protobuf.BoolValue",
		"google.protobuf.BytesValue":
		return true
	default:
		return false
	}
}
