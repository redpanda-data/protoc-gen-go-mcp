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

package generator

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	anypb "google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func init() {
}

func TestCompat(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name  string
		input proto.Message
		// If rawJsonInput is set, it's preferred over input.
		// It can be used to simulate a wrong input, eg. not using base64 for byte fields.
		// input must still be provided, so we know the proto type.
		rawJsonInput  json.RawMessage
		errorExpected bool
		errorContains string
	}{
		{
			name: "any containing struct",
			input: func() proto.Message {
				val := &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"nested": structpb.NewStringValue("value"),
					},
				}
				any, err := anypb.New(val)
				g.Expect(err).ToNot(HaveOccurred())
				return &testdata.WktTestMessage{
					Any: any,
				}
			}(),
		},
		{
			name: "bytes value with weird base64",
			input: &testdata.WktTestMessage{
				BytesValue: wrapperspb.Bytes([]byte{0xde, 0xad, 0xbe, 0xef}),
			},
		},
		{
			name: "negative duration",
			input: &testdata.WktTestMessage{
				Duration: durationpb.New(-5 * time.Second),
			},
		},
		{
			name: "timestamp in the future",
			input: &testdata.WktTestMessage{
				Timestamp: timestamppb.New(time.Date(3000, 1, 1, 0, 0, 0, 0, time.UTC)),
			},
		},
		{
			name: "wrapper types with default values",
			input: &testdata.WktTestMessage{
				StringValue: wrapperspb.String(""),
				Int32Value:  wrapperspb.Int32(0),
				Int64Value:  wrapperspb.Int64(0),
				BoolValue:   wrapperspb.Bool(false),
				BytesValue:  wrapperspb.Bytes(nil),
			},
		},
		{
			name: "basic any test",
			input: func() proto.Message {
				any, err := anypb.New(wrapperspb.String("some-string-in-any"))
				g.Expect(err).ToNot(HaveOccurred())
				return &testdata.WktTestMessage{
					Any: any,
				}
			}(),
		},
		{
			name: "bytes as base64 works",
			input: &testdata.TestMessage{
				SomeBytes: []byte{1, 200, 125},
			},
		},
		{
			name:          "bytes must be base64 - fails if it's not",
			input:         &testdata.TestMessage{},
			rawJsonInput:  json.RawMessage(`{"some_bytes":"hello this is not base64"}`),
			errorExpected: true,
			errorContains: "/properties/some_bytes/contentEncoding",
		},
		{
			name: "a little bit of everything, required field is set",
			input: &testdata.WktTestMessage{
				Timestamp:   timestamppb.New(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)),
				Duration:    durationpb.New(3 * time.Second),
				StructField: &structpb.Struct{Fields: map[string]*structpb.Value{"foo": structpb.NewStringValue("bar")}},
				ValueField:  structpb.NewNumberValue(42),
				ListValue:   &structpb.ListValue{Values: []*structpb.Value{structpb.NewBoolValue(true)}},
				FieldMask:   &fieldmaskpb.FieldMask{Paths: []string{"foo", "bar"}},
				StringValue: wrapperspb.String("hello"),
				Int32Value:  wrapperspb.Int32(123),
				Int64Value:  wrapperspb.Int64(1234567890123),
				BoolValue:   wrapperspb.Bool(true),
				BytesValue:  wrapperspb.Bytes([]byte("hi")),
			},
		},
		{
			name:          "required field absent throws error",
			input:         &testdata.RequiredFieldTest{},
			errorExpected: true,
			errorContains: `missing properties: 'required_field'`,
		},
		{
			name:         "nullable timestamp as null",
			input:        &testdata.WktTestMessage{}, // Empty message, timestamp is nil
			rawJsonInput: json.RawMessage(`{"timestamp": null}`),
		},
		{
			name: "map as object",
			input: &testdata.MapTestMessage{
				StringMap: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			var input []byte
			if tt.rawJsonInput == nil {
				marshaled, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(tt.input)
				g.Expect(err).ToNot(HaveOccurred())
				input = marshaled
			} else {
				input = tt.rawJsonInput
			}

			// Create a generator instance to access messageSchema method
			fg := &FileGenerator{openAICompat: false}
			schemaMap := fg.messageSchema(tt.input.ProtoReflect().Descriptor())
			schemaJSON, err := json.Marshal(schemaMap)
			g.Expect(err).ToNot(HaveOccurred())

			// Step 4: Validate the marshaled JSON against the schema
			compiler := jsonschema.NewCompiler()
			// This is required, so it can assert that strings are base64.
			compiler.AssertContent = true

			err = compiler.AddResource("schema.json", bytes.NewReader(schemaJSON))
			g.Expect(err).ToNot(HaveOccurred())

			schema, err := compiler.Compile("schema.json")
			g.Expect(err).ToNot(HaveOccurred())

			var jsonData interface{}
			err = json.Unmarshal(input, &jsonData)
			g.Expect(err).ToNot(HaveOccurred())

			err = schema.Validate(jsonData)
			if tt.errorExpected {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			if tt.errorContains != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errorContains)))
			}
		})
	}
}

func TestCompatOpenAI(t *testing.T) {
	tests := []struct {
		name  string
		input proto.Message
		// If rawJsonInput is set, it's preferred over input.
		// It can be used to simulate a wrong input, eg. not using base64 for byte fields.
		// input must still be provided, so we know the proto type.
		rawJsonInput  json.RawMessage
		errorExpected bool
		errorContains string
	}{
		{
			name: "google.protobuf.Value/ListValue/Struct as strings",
			input: &testdata.WktTestMessage{
				Timestamp:   timestamppb.New(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)),
				Duration:    durationpb.New(3 * time.Second),
				StructField: &structpb.Struct{Fields: map[string]*structpb.Value{"foo": structpb.NewStringValue("bar")}},
				ValueField:  structpb.NewNumberValue(42),
				ListValue:   &structpb.ListValue{Values: []*structpb.Value{structpb.NewBoolValue(true)}},
				FieldMask:   &fieldmaskpb.FieldMask{Paths: []string{"foo", "bar"}},
				StringValue: wrapperspb.String("hello"),
				Int32Value:  wrapperspb.Int32(123),
				Int64Value:  wrapperspb.Int64(1234567890123),
				BoolValue:   wrapperspb.Bool(true),
				BytesValue:  wrapperspb.Bytes([]byte("hi")),
			},
			rawJsonInput: json.RawMessage(`{
				"timestamp": "2023-01-01T12:00:00Z",
				"duration": "3s",
				"struct_field": "{\"foo\":\"bar\"}",
				"value_field": "42",
				"list_value": "[true]",
				"field_mask": "foo,bar",
				"any": {"@type": "type.googleapis.com/google.protobuf.StringValue", "value": "test"},
				"string_value": "hello",
				"int32_value": 123,
				"int64_value": "1234567890123",
				"bool_value": true,
				"bytes_value": "aGk="
			}`),
		},
		{
			name: "all fields required",
			input: &testdata.WktTestMessage{
				Timestamp:   timestamppb.New(time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)),
				Duration:    durationpb.New(3 * time.Second),
				StructField: &structpb.Struct{Fields: map[string]*structpb.Value{"foo": structpb.NewStringValue("bar")}},
				ValueField:  structpb.NewNumberValue(42),
				ListValue:   &structpb.ListValue{Values: []*structpb.Value{structpb.NewBoolValue(true)}},
				FieldMask:   &fieldmaskpb.FieldMask{Paths: []string{"foo", "bar"}},
				StringValue: wrapperspb.String("hello"),
				Int32Value:  wrapperspb.Int32(123),
				Int64Value:  wrapperspb.Int64(1234567890123),
				BoolValue:   wrapperspb.Bool(true),
				BytesValue:  wrapperspb.Bytes([]byte("hi")),
			},
			rawJsonInput: json.RawMessage(`{
				"timestamp": "2023-01-01T12:00:00Z",
				"duration": "3s",
				"struct_field": "{\"foo\":\"bar\"}",
				"value_field": "42",
				"list_value": "[true]",
				"field_mask": "foo,bar",
				"any": {"@type": "type.googleapis.com/google.protobuf.StringValue", "value": "test"},
				"string_value": "hello",
				"int32_value": 123,
				"int64_value": "1234567890123",
				"bool_value": true,
				"bytes_value": "aGk="
			}`),
		},
		{
			name: "map as array of key-value pairs",
			input: &testdata.MapTestMessage{
				StringMap: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			rawJsonInput: json.RawMessage(`{
				"string_map": [
					{"key": "key1", "value": "value1"},
					{"key": "key2", "value": "value2"}
				]
			}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			var input []byte
			if tt.rawJsonInput == nil {
				marshaled, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(tt.input)
				g.Expect(err).ToNot(HaveOccurred())
				input = marshaled
			} else {
				input = tt.rawJsonInput
			}

			// Create a generator instance to access messageSchema method with OpenAI compatibility
			fg := &FileGenerator{openAICompat: true}
			schemaMap := fg.messageSchema(tt.input.ProtoReflect().Descriptor())
			schemaJSON, err := json.Marshal(schemaMap)
			g.Expect(err).ToNot(HaveOccurred())

			// Step 4: Validate the marshaled JSON against the schema
			compiler := jsonschema.NewCompiler()
			// This is required, so it can assert that strings are base64.
			compiler.AssertContent = true

			err = compiler.AddResource("schema.json", bytes.NewReader(schemaJSON))
			g.Expect(err).ToNot(HaveOccurred())

			schema, err := compiler.Compile("schema.json")
			g.Expect(err).ToNot(HaveOccurred())

			var jsonData interface{}
			err = json.Unmarshal(input, &jsonData)
			g.Expect(err).ToNot(HaveOccurred())

			// First validate against the schema (expects array format for maps in OpenAI mode)
			err = schema.Validate(jsonData)
			if tt.errorExpected {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			// Then apply Fix for OpenAI compatibility mode (converts all OpenAI format back to protobuf format)
			// This is what would happen in the actual MCP tool execution before protojson.Unmarshal
			if rawData, ok := jsonData.(map[string]interface{}); ok {
				runtime.FixOpenAI(tt.input.ProtoReflect().Descriptor(), rawData)

				// Verify that the converted data can be unmarshaled into the proto message
				fixedJSON, err := json.Marshal(rawData)
				g.Expect(err).ToNot(HaveOccurred())

				var testProto proto.Message
				switch tt.input.(type) {
				case *testdata.MapTestMessage:
					testProto = &testdata.MapTestMessage{}
				case *testdata.WktTestMessage:
					testProto = &testdata.WktTestMessage{}
				}

				if testProto != nil {
					err = protojson.Unmarshal(fixedJSON, testProto)
					g.Expect(err).ToNot(HaveOccurred())
				}
			}
			if tt.errorExpected {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			if tt.errorContains != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.errorContains)))
			}
		})
	}
}
