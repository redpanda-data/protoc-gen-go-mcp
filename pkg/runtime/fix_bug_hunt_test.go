package runtime

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"google.golang.org/protobuf/encoding/protojson"
)

// Bug: map<int32, string> keys sent as numbers by LLMs are silently dropped.
//
// FixOpenAI converts arrays of KV pairs back to maps, but it does
// pair["key"].(string) which fails when the key is a JSON number (float64
// after json.Unmarshal). For map<int32, string>, map<bool, string>, etc.,
// an LLM is likely to send the key as its native JSON type rather than a
// string. These entries are silently discarded.
func TestFixOpenAI_Bug_NonStringMapKeysDropped(t *testing.T) {
	g := NewWithT(t)

	descriptor := new(testdata.MapVariantsRequest)

	// Simulate what an LLM would produce for map<int32, string>:
	// The key is a JSON number, not a string.
	input := map[string]any{
		"int_to_string": []any{
			map[string]any{"key": float64(42), "value": "forty-two"},
			map[string]any{"key": float64(7), "value": "seven"},
		},
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)

	// The entries should be preserved with string keys "42" and "7",
	// since protojson expects map keys as strings in JSON regardless of
	// the proto key type.
	result, ok := input["int_to_string"].(map[string]any)
	g.Expect(ok).To(BeTrue(), "int_to_string should be converted to a map")
	g.Expect(result).To(HaveLen(2), "both map entries should be preserved")
	g.Expect(result["42"]).To(Equal("forty-two"))
	g.Expect(result["7"]).To(Equal("seven"))

	// Verify the result can actually be unmarshalled
	fixedJSON, err := json.Marshal(input)
	g.Expect(err).ToNot(HaveOccurred())

	var msg testdata.MapVariantsRequest
	err = protojson.Unmarshal(fixedJSON, &msg)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(msg.IntToString).To(Equal(map[int32]string{42: "forty-two", 7: "seven"}))
}

// Same bug, but for map<bool, string>. LLM sends boolean key as JSON bool.
func TestFixOpenAI_Bug_BoolMapKeysDropped(t *testing.T) {
	g := NewWithT(t)

	descriptor := new(testdata.MapVariantsRequest)

	input := map[string]any{
		"bool_to_string": []any{
			map[string]any{"key": true, "value": "yes"},
			map[string]any{"key": false, "value": "no"},
		},
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)

	result, ok := input["bool_to_string"].(map[string]any)
	g.Expect(ok).To(BeTrue(), "bool_to_string should be converted to a map")
	g.Expect(result).To(HaveLen(2), "both map entries should be preserved")
	g.Expect(result["true"]).To(Equal("yes"))
	g.Expect(result["false"]).To(Equal("no"))
}

// Same bug for map<uint64, string> with large numeric keys.
func TestFixOpenAI_Bug_Uint64MapKeysDropped(t *testing.T) {
	g := NewWithT(t)

	descriptor := new(testdata.MapVariantsRequest)

	input := map[string]any{
		"uint64_to_string": []any{
			map[string]any{"key": float64(123456789), "value": "big-number"},
		},
	}

	FixOpenAI(descriptor.ProtoReflect().Descriptor(), input)

	result, ok := input["uint64_to_string"].(map[string]any)
	g.Expect(ok).To(BeTrue(), "uint64_to_string should be converted to a map")
	g.Expect(result).To(HaveLen(1), "map entry should be preserved")
	g.Expect(result["123456789"]).To(Equal("big-number"))
}
