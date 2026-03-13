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
	"testing"

	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/encoding/protojson"

	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
)

// TestFixOpenAI_WKTWrapper_UnwrapObjects demonstrates a real bug in FixOpenAI:
// wrapper types sent in object form {"value": X} are not unwrapped to plain
// scalars, causing protojson.Unmarshal to fail.
//
// Context: The OpenAI-compatible schema tells LLMs that wrapper types
// (Int32Value, StringValue, BoolValue, etc.) are plain scalars. Most LLMs
// comply and send plain values. But some LLMs (or in some contexts) may send
// the protobuf message form {"value": X}. protojson rejects this form for
// wrapper types -- it expects the unwrapped scalar.
//
// FixOpenAI should detect wrapper type fields and unwrap {"value": X} -> X.
// Currently it falls through to the generic "recurse into nested message"
// case, which is a no-op for wrappers (they only contain a scalar "value"
// field), leaving the broken form intact.
func TestFixOpenAI_WKTWrapper_UnwrapObjects(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		validate func(g Gomega, msg *testdata.WktTestMessage)
	}{
		{
			name:  "Int32Value wrapped object should be unwrapped to plain number",
			input: map[string]any{"int32Value": map[string]any{"value": float64(42)}},
			validate: func(g Gomega, msg *testdata.WktTestMessage) {
				g.Expect(msg.Int32Value).ToNot(BeNil())
				g.Expect(msg.Int32Value.Value).To(Equal(int32(42)))
			},
		},
		{
			name:  "StringValue wrapped object should be unwrapped to plain string",
			input: map[string]any{"stringValue": map[string]any{"value": "hello"}},
			validate: func(g Gomega, msg *testdata.WktTestMessage) {
				g.Expect(msg.StringValue).ToNot(BeNil())
				g.Expect(msg.StringValue.Value).To(Equal("hello"))
			},
		},
		{
			name:  "BoolValue wrapped object should be unwrapped to plain bool",
			input: map[string]any{"boolValue": map[string]any{"value": true}},
			validate: func(g Gomega, msg *testdata.WktTestMessage) {
				g.Expect(msg.BoolValue).ToNot(BeNil())
				g.Expect(msg.BoolValue.Value).To(BeTrue())
			},
		},
		{
			name:  "Int64Value wrapped object should be unwrapped to plain string",
			input: map[string]any{"int64Value": map[string]any{"value": "999999999999"}},
			validate: func(g Gomega, msg *testdata.WktTestMessage) {
				g.Expect(msg.Int64Value).ToNot(BeNil())
				g.Expect(msg.Int64Value.Value).To(Equal(int64(999999999999)))
			},
		},
		{
			name:  "BytesValue wrapped object should be unwrapped to plain string",
			input: map[string]any{"bytesValue": map[string]any{"value": "aGVsbG8="}},
			validate: func(g Gomega, msg *testdata.WktTestMessage) {
				g.Expect(msg.BytesValue).ToNot(BeNil())
				g.Expect(string(msg.BytesValue.Value)).To(Equal("hello"))
			},
		},
		{
			// Multiple wrapper fields in one message, all in object form.
			// This is the realistic scenario: an LLM that consistently
			// wraps all wrapper types.
			name: "multiple wrapper fields all in object form",
			input: map[string]any{
				"int32Value":  map[string]any{"value": float64(7)},
				"stringValue": map[string]any{"value": "test"},
				"boolValue":   map[string]any{"value": false},
			},
			validate: func(g Gomega, msg *testdata.WktTestMessage) {
				g.Expect(msg.Int32Value).ToNot(BeNil())
				g.Expect(msg.Int32Value.Value).To(Equal(int32(7)))
				g.Expect(msg.StringValue).ToNot(BeNil())
				g.Expect(msg.StringValue.Value).To(Equal("test"))
				g.Expect(msg.BoolValue).ToNot(BeNil())
				g.Expect(msg.BoolValue.Value).To(BeFalse())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			msg := new(testdata.WktTestMessage)
			descriptor := msg.ProtoReflect().Descriptor()

			fixed := make(map[string]any)
			for k, v := range tt.input {
				fixed[k] = v
			}

			FixOpenAI(descriptor, fixed)

			fixedJSON, err := json.Marshal(fixed)
			g.Expect(err).ToNot(HaveOccurred())

			target := new(testdata.WktTestMessage)
			err = protojson.Unmarshal(fixedJSON, target)
			g.Expect(err).ToNot(HaveOccurred(),
				"FixOpenAI should unwrap wrapper objects so protojson succeeds. Got JSON: %s", string(fixedJSON))

			tt.validate(g, target)
		})
	}
}
