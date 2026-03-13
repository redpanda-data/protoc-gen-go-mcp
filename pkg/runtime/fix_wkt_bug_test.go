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

// TestFixOpenAI_WKT_WrapperTypes tests that FixOpenAI properly handles
// well-known wrapper types that the schema (schema.go) exposes as plain
// scalars to LLMs.
//
// The schema tells LLMs that wrapper types (StringValue, Int32Value, etc.)
// are just scalars. But FixOpenAI has no handling for the case where an LLM
// sends these as wrapped objects {"value": X}, which is the natural protobuf
// message representation. Since wrapper types fall through to the default
// "recurse into nested messages" case in FixOpenAI, they work when sent as
// objects. But the schema is misleading.
//
// The real bugs found here are around Timestamp and Duration, where the
// FixOpenAI recursion into nested messages silently corrupts the data,
// resulting in wrong values being deserialized.
func TestFixOpenAI_WKT_WrapperTypes(t *testing.T) {
	tests := []struct {
		name string
		// input simulates what an LLM sends based on the OpenAI schema
		input map[string]any
		// validate checks the deserialized proto message for correctness
		validate func(g Gomega, msg *testdata.WktTestMessage)
		// If true, we expect protojson.Unmarshal to fail AFTER FixOpenAI
		expectUnmarshalError bool
	}{
		// --- Wrapper types as plain scalars: these work ---
		{
			name:  "StringValue as plain string",
			input: map[string]any{"stringValue": "hello"},
			validate: func(g Gomega, msg *testdata.WktTestMessage) {
				g.Expect(msg.StringValue).ToNot(BeNil())
				g.Expect(msg.StringValue.Value).To(Equal("hello"))
			},
		},
		{
			name:  "BoolValue as plain bool",
			input: map[string]any{"boolValue": true},
			validate: func(g Gomega, msg *testdata.WktTestMessage) {
				g.Expect(msg.BoolValue).ToNot(BeNil())
				g.Expect(msg.BoolValue.Value).To(BeTrue())
			},
		},
		{
			name:  "Int32Value as plain number",
			input: map[string]any{"int32Value": float64(42)},
			validate: func(g Gomega, msg *testdata.WktTestMessage) {
				g.Expect(msg.Int32Value).ToNot(BeNil())
				g.Expect(msg.Int32Value.Value).To(Equal(int32(42)))
			},
		},

		// --- BUG: Timestamp sent as RFC3339 string ---
		// Schema says type: string, format: date-time. The LLM sends
		// a proper RFC3339 string. This works with protojson.
		{
			name:  "Timestamp as RFC3339 string - works",
			input: map[string]any{"timestamp": "2021-01-01T00:00:00Z"},
			validate: func(g Gomega, msg *testdata.WktTestMessage) {
				g.Expect(msg.Timestamp).ToNot(BeNil())
				g.Expect(msg.Timestamp.Seconds).To(Equal(int64(1609459200)))
			},
		},

		// --- BUG: Timestamp sent as non-RFC3339 string ---
		// LLM might send a date string that isn't strictly RFC3339.
		// protojson will reject it. FixOpenAI doesn't validate or convert.
		{
			name:                 "Timestamp as non-RFC3339 string - unmarshal fails",
			input:                map[string]any{"timestamp": "January 1, 2021"},
			expectUnmarshalError: true,
		},

		// --- BUG: Duration as plain seconds number ---
		// Schema says type: string with pattern ^-?[0-9]+(\.[0-9]+)?s$
		// But an LLM might just send a number (seconds). FixOpenAI doesn't
		// convert numbers to the required "Xs" string format.
		{
			name:                 "Duration as number - LLM sends seconds as number",
			input:                map[string]any{"duration": float64(30)},
			expectUnmarshalError: true,
		},

		// --- BUG: FieldMask sent as array ---
		// In OpenAI mode, schema says type: ["string", "null"].
		// protojson expects FieldMask as a comma-separated string like
		// "foo,bar.baz". But an LLM might send an array of paths.
		// FixOpenAI doesn't convert arrays to comma-separated strings
		// for FieldMask.
		{
			name:                 "FieldMask as array of paths - unmarshal fails",
			input:                map[string]any{"fieldMask": []any{"foo", "bar.baz"}},
			expectUnmarshalError: true,
		},

		// --- BUG: Wrapper types sent as wrapped objects ---
		// The schema tells LLMs these are plain scalars. But what if
		// an LLM sends the object form {"value": X}? FixOpenAI recurses
		// into it as a nested message (default case), which is a no-op.
		// protojson does NOT accept the object form for wrapper types -
		// it expects the unwrapped scalar.
		{
			name:  "Int32Value as wrapped object - FixOpenAI unwraps",
			input: map[string]any{"int32Value": map[string]any{"value": float64(42)}},
		},
		{
			name:  "StringValue as wrapped object - FixOpenAI unwraps",
			input: map[string]any{"stringValue": map[string]any{"value": "hello"}},
		},
		{
			name:  "BoolValue as wrapped object - FixOpenAI unwraps",
			input: map[string]any{"boolValue": map[string]any{"value": true}},
		},
		{
			name:  "Int64Value as wrapped object - FixOpenAI unwraps",
			input: map[string]any{"int64Value": map[string]any{"value": "123"}},
		},
		{
			name:  "BytesValue as wrapped object - FixOpenAI unwraps",
			input: map[string]any{"bytesValue": map[string]any{"value": "aGVsbG8="}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			msg := new(testdata.WktTestMessage)
			descriptor := msg.ProtoReflect().Descriptor()

			// Copy input
			fixed := make(map[string]any)
			for k, v := range tt.input {
				fixed[k] = v
			}

			// Run FixOpenAI
			FixOpenAI(descriptor, fixed)

			// Marshal back to JSON
			fixedJSON, err := json.Marshal(fixed)
			g.Expect(err).ToNot(HaveOccurred(), "json.Marshal should not fail")

			// Try to unmarshal into proto message
			target := new(testdata.WktTestMessage)
			err = protojson.Unmarshal(fixedJSON, target)

			if tt.expectUnmarshalError {
				g.Expect(err).To(HaveOccurred(),
					"BUG: FixOpenAI should have converted this value "+
						"but didn't, causing protojson.Unmarshal to fail. "+
						"Input JSON: %s", string(fixedJSON))
			} else {
				g.Expect(err).ToNot(HaveOccurred(),
					"protojson.Unmarshal should succeed. Input JSON: %s",
					string(fixedJSON))
				if tt.validate != nil {
					tt.validate(g, target)
				}
			}
		})
	}
}
