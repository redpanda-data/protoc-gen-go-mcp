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

package generator

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestGetTypeStandard(t *testing.T) {
	tests := []struct {
		name       string
		setupField func() protoreflect.FieldDescriptor
		wantSchema func(*WithT, map[string]any)
	}{
		// Map field tests
		{
			name: "map field in standard mode",
			setupField: func() protoreflect.FieldDescriptor {
				// Use the test proto's map field
				msg := &MapTestMessage{}
				return msg.ProtoReflect().Descriptor().Fields().ByName("string_map")
			},
			wantSchema: func(g *WithT, schema map[string]any) {
				g.Expect(schema["type"]).To(Equal("object"))
				g.Expect(schema).To(HaveKey("additionalProperties"))
				g.Expect(schema).To(HaveKey("propertyNames"))
			},
		},
		// Well-known types
		{
			name: "google.protobuf.Struct in standard mode",
			setupField: func() protoreflect.FieldDescriptor {
				msg := &WktTestMessage{}
				return msg.ProtoReflect().Descriptor().Fields().ByName("struct_field")
			},
			wantSchema: func(g *WithT, schema map[string]any) {
				g.Expect(schema["type"]).To(Equal("object"))
				g.Expect(schema["additionalProperties"]).To(Equal(true))
			},
		},
		{
			name: "google.protobuf.Value in standard mode",
			setupField: func() protoreflect.FieldDescriptor {
				msg := &WktTestMessage{}
				return msg.ProtoReflect().Descriptor().Fields().ByName("value_field")
			},
			wantSchema: func(g *WithT, schema map[string]any) {
				g.Expect(schema["description"]).To(ContainSubstring("dynamic JSON value"))
				g.Expect(schema).ToNot(HaveKey("type")) // Any type
			},
		},
		{
			name: "google.protobuf.ListValue in standard mode",
			setupField: func() protoreflect.FieldDescriptor {
				msg := &WktTestMessage{}
				return msg.ProtoReflect().Descriptor().Fields().ByName("list_value")
			},
			wantSchema: func(g *WithT, schema map[string]any) {
				g.Expect(schema["type"]).To(Equal("array"))
				g.Expect(schema).To(HaveKey("items"))
				g.Expect(schema["description"]).To(ContainSubstring("JSON array"))
			},
		},
		// Timestamp field
		{
			name: "timestamp field",
			setupField: func() protoreflect.FieldDescriptor {
				msg := &WktTestMessage{}
				return msg.ProtoReflect().Descriptor().Fields().ByName("timestamp")
			},
			wantSchema: func(g *WithT, schema map[string]any) {
				g.Expect(schema["type"]).To(Equal([]string{"string", "null"}))
				g.Expect(schema["format"]).To(Equal("date-time"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			
			fg := &FileGenerator{
				openAICompat: false,
			}
			
			field := tt.setupField()
			schema := fg.getType(field)
			
			tt.wantSchema(g, schema)
		})
	}
}

func TestGetTypeOpenAI(t *testing.T) {
	tests := []struct {
		name       string
		setupField func() protoreflect.FieldDescriptor
		wantSchema func(*WithT, map[string]any)
	}{
		{
			name: "map field in OpenAI mode",
			setupField: func() protoreflect.FieldDescriptor {
				msg := &MapTestMessage{}
				return msg.ProtoReflect().Descriptor().Fields().ByName("string_map")
			},
			wantSchema: func(g *WithT, schema map[string]any) {
				g.Expect(schema["type"]).To(Equal("array"))
				g.Expect(schema["description"]).To(Equal("List of key value pairs"))
				items := schema["items"].(map[string]any)
				g.Expect(items["type"]).To(Equal("object"))
				props := items["properties"].(map[string]any)
				g.Expect(props).To(HaveKey("key"))
				g.Expect(props).To(HaveKey("value"))
				g.Expect(items["required"]).To(Equal([]string{"key", "value"}))
				g.Expect(items["additionalProperties"]).To(Equal(false))
			},
		},
		{
			name: "google.protobuf.Struct in OpenAI mode",
			setupField: func() protoreflect.FieldDescriptor {
				msg := &WktTestMessage{}
				return msg.ProtoReflect().Descriptor().Fields().ByName("struct_field")
			},
			wantSchema: func(g *WithT, schema map[string]any) {
				g.Expect(schema["type"]).To(Equal("string"))
				g.Expect(schema["description"]).To(ContainSubstring("string representation of any JSON object"))
			},
		},
		{
			name: "google.protobuf.Value in OpenAI mode",
			setupField: func() protoreflect.FieldDescriptor {
				msg := &WktTestMessage{}
				return msg.ProtoReflect().Descriptor().Fields().ByName("value_field")
			},
			wantSchema: func(g *WithT, schema map[string]any) {
				g.Expect(schema["type"]).To(Equal("string"))
				g.Expect(schema["description"]).To(ContainSubstring("string representation of any JSON value"))
			},
		},
		{
			name: "google.protobuf.ListValue in OpenAI mode",
			setupField: func() protoreflect.FieldDescriptor {
				msg := &WktTestMessage{}
				return msg.ProtoReflect().Descriptor().Fields().ByName("list_value")
			},
			wantSchema: func(g *WithT, schema map[string]any) {
				g.Expect(schema["type"]).To(Equal("string"))
				g.Expect(schema["description"]).To(ContainSubstring("string representation of a JSON array"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			
			fg := &FileGenerator{
				openAICompat: true,
			}
			
			field := tt.setupField()
			schema := fg.getType(field)
			
			tt.wantSchema(g, schema)
		})
	}
}

func TestMessageSchemaStandard(t *testing.T) {
	g := NewWithT(t)
	
	fg := &FileGenerator{
		openAICompat: false,
	}
	
	msgDesc := (&WktTestMessage{}).ProtoReflect().Descriptor()
	schema := fg.messageSchema(msgDesc)
	
	g.Expect(schema["type"]).To(Equal("object"))
	g.Expect(schema).To(HaveKey("properties"))
	g.Expect(schema).To(HaveKey("required"))
	// Standard mode should not have additionalProperties: false
	g.Expect(schema).ToNot(HaveKey("additionalProperties"))
}

func TestMessageSchemaOpenAI(t *testing.T) {
	g := NewWithT(t)
	
	fg := &FileGenerator{
		openAICompat: true,
	}
	
	msgDesc := (&WktTestMessage{}).ProtoReflect().Descriptor()
	schema := fg.messageSchema(msgDesc)
	
	// In OpenAI mode, the type becomes ["object", "null"]
	g.Expect(schema["type"]).To(Equal([]string{"object", "null"}))
	g.Expect(schema).To(HaveKey("properties"))
	g.Expect(schema).To(HaveKey("required"))
	// OpenAI mode should have additionalProperties: false
	g.Expect(schema["additionalProperties"]).To(Equal(false))
	
	// In OpenAI mode, all fields should be required
	required := schema["required"].([]string)
	props := schema["properties"].(map[string]any)
	g.Expect(len(required)).To(Equal(len(props)))
}

func TestKindToType(t *testing.T) {
	tests := []struct {
		kind protoreflect.Kind
		want string
	}{
		{protoreflect.BoolKind, "boolean"},
		{protoreflect.StringKind, "string"},
		{protoreflect.Int32Kind, "integer"},
		{protoreflect.Int64Kind, "string"}, // encoded as string for safety
		{protoreflect.FloatKind, "number"},
		{protoreflect.DoubleKind, "number"},
		{protoreflect.BytesKind, "string"},
		{protoreflect.EnumKind, "string"},
	}

	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(kindToType(tt.kind)).To(Equal(tt.want))
		})
	}
}

func TestSchemaMarshaling(t *testing.T) {
	g := NewWithT(t)
	
	fg := &FileGenerator{
		openAICompat: false,
	}
	
	// Test that generated schemas can be marshaled to JSON
	msg := &WktTestMessage{}
	schema := fg.messageSchema(msg.ProtoReflect().Descriptor())
	
	marshaled, err := json.Marshal(schema)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(marshaled).ToNot(BeEmpty())
	
	// Verify it's valid JSON
	var unmarshaled map[string]any
	err = json.Unmarshal(marshaled, &unmarshaled)
	g.Expect(err).ToNot(HaveOccurred())
}