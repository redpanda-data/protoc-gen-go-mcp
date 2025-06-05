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
	"github.com/santhosh-tekuri/jsonschema/v5"
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
				return &WktTestMessage{
					Any: any,
				}
			}(),
		},
		{
			name: "bytes value with weird base64",
			input: &WktTestMessage{
				BytesValue: wrapperspb.Bytes([]byte{0xde, 0xad, 0xbe, 0xef}),
			},
		},
		{
			name: "negative duration",
			input: &WktTestMessage{
				Duration: durationpb.New(-5 * time.Second),
			},
		},
		{
			name: "timestamp in the future",
			input: &WktTestMessage{
				Timestamp: timestamppb.New(time.Date(3000, 1, 1, 0, 0, 0, 0, time.UTC)),
			},
		},
		{
			name: "wrapper types with default values",
			input: &WktTestMessage{
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
				return &WktTestMessage{
					Any: any,
				}
			}(),
		},
		{
			name: "bytes as base64 works",
			input: &TestMessage{
				SomeBytes: []byte{1, 200, 125},
			},
		},
		{
			name:          "bytes must be base64 - fails if it's not",
			input:         &TestMessage{},
			rawJsonInput:  json.RawMessage(`{"some_bytes":"hello this is not base64"}`),
			errorExpected: true,
			errorContains: "/properties/some_bytes/contentEncoding",
		},
		{
			name: "a little bit of everything, required field is set",
			input: &WktTestMessage{
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
			input:         &RequiredFieldTest{},
			errorExpected: true,
			errorContains: `missing properties: 'required_field'`,
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
