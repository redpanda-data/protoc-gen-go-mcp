package runtime

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
)

var fuzzDescriptors = []struct {
	name string
	msg  proto.Message
}{
	{"CreateItemRequest", new(testdata.CreateItemRequest)},
	{"WktTestMessage", new(testdata.WktTestMessage)},
	{"DeepNestingRequest", new(testdata.DeepNestingRequest)},
}

func FuzzFixOpenAI(f *testing.F) {
	// Seed corpus with some representative JSON inputs
	seeds := []string{
		`{}`,
		`{"labels": [{"key": "k", "value": "v"}]}`,
		`{"struct_field": "{\"foo\": \"bar\"}"}`,
		`{"value_field": "\"hello\""}`,
		`{"list_value": "[1, 2, 3]"}`,
		`{"labels": "not-an-array"}`,
		`{"labels": null}`,
		`{"unknown_field": 42}`,
		`{"middle": {"inner": {"tags": [{"key": "a", "value": "b"}]}}}`,
		`{"labels": [{"key": 123, "value": "bad-key"}]}`,
		`{"labels": ["not-a-map"]}`,
		`{"struct_field": "not-json"}`,
		`{"value_field": ""}`,
		`null`,
		`[]`,
		`"just a string"`,
		`42`,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	descriptors := make([]protoreflect.MessageDescriptor, len(fuzzDescriptors))
	for i, d := range fuzzDescriptors {
		descriptors[i] = d.msg.ProtoReflect().Descriptor()
	}

	f.Fuzz(func(t *testing.T, data string) {
		// Try to parse as JSON; if it's not valid JSON, skip
		var parsed map[string]any
		if err := json.Unmarshal([]byte(data), &parsed); err != nil {
			return
		}

		// FixOpenAI must never panic, regardless of input, for any descriptor
		for _, desc := range descriptors {
			// Make a copy so each descriptor gets fresh input
			inputCopy := make(map[string]any)
			for k, v := range parsed {
				inputCopy[k] = v
			}
			FixOpenAI(desc, inputCopy)
		}
	})
}
