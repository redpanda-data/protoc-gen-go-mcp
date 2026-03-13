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

	"google.golang.org/protobuf/encoding/protojson"

	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
)

// TestFixOpenAI_Optional_NullValueInMapEntry tests that FixOpenAI handles null
// values inside map KV pairs. When an LLM sends the OpenAI array-of-KV format
// with a null value like [{"key":"k","value":null}], FixOpenAI converts it to
// {"k":null}. protojson then rejects this because null is not a valid string.
//
// BUG: FixOpenAI blindly copies the value from the KV pair without checking
// for nil. The converted map ends up with null values that protojson rejects.
func TestFixOpenAI_Optional_NullValueInMapEntry(t *testing.T) {
	desc := new(testdata.CreateItemRequest)

	// LLM sends map<string,string> as OpenAI array with null value
	input := map[string]any{
		"name": "test",
		"labels": []any{
			map[string]any{"key": "env", "value": "prod"},
			map[string]any{"key": "empty", "value": nil}, // null value
		},
	}

	FixOpenAI(desc.ProtoReflect().Descriptor(), input)

	fixedJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	msg := new(testdata.CreateItemRequest)
	err = protojson.Unmarshal(fixedJSON, msg)
	if err != nil {
		t.Fatalf("BUG: protojson.Unmarshal failed after FixOpenAI: %v\nJSON was: %s\n"+
			"FixOpenAI converts [{key:k,value:null}] to {k:null} which protojson rejects "+
			"because null is not a valid map value for map<string,string>", err, fixedJSON)
	}
}

// TestFixOpenAI_Optional_NullValueInMessageMapEntry tests null values in
// map<string, MessageType> KV pairs. Same bug as above but with message values.
//
// BUG: When a KV pair has value:null for a map<string, InnerMessage>, FixOpenAI
// converts it to {"key":null}. protojson rejects null for message map values.
func TestFixOpenAI_Optional_NullValueInMessageMapEntry(t *testing.T) {
	desc := new(testdata.MapVariantsRequest)

	// LLM sends map<string, InnerMessage> as array with null value
	input := map[string]any{
		"stringToMessage": []any{
			map[string]any{
				"key": "item1",
				"value": map[string]any{
					"id": "123",
				},
			},
			map[string]any{
				"key":   "item2",
				"value": nil, // null message value
			},
		},
	}

	FixOpenAI(desc.ProtoReflect().Descriptor(), input)

	fixedJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	msg := new(testdata.MapVariantsRequest)
	err = protojson.Unmarshal(fixedJSON, msg)
	if err != nil {
		t.Fatalf("BUG: protojson.Unmarshal failed after FixOpenAI: %v\nJSON was: %s\n"+
			"FixOpenAI produces {\"item2\":null} which protojson rejects for message-valued maps", err, fixedJSON)
	}
}

// TestFixOpenAI_Optional_BothOneofFieldsSet tests that FixOpenAI handles the
// case where an LLM populates both alternatives of a oneof with real values.
//
// In OpenAI strict mode, all fields are required, so the LLM must provide
// values for both "product" and "service" even though they are oneof
// alternatives. FixOpenAI recurses into both without noticing they belong
// to the same oneof, leaving both in the JSON. protojson then rejects the
// second one with "oneof already set".
//
// BUG: FixOpenAI does not track or handle oneof semantics at all. It should
// strip all but the first non-null oneof alternative.
func TestFixOpenAI_Optional_BothOneofFieldsSet(t *testing.T) {
	desc := new(testdata.CreateItemRequest)

	input := map[string]any{
		"name":    "test",
		"product": map[string]any{"price": 9.99, "quantity": float64(1)},
		"service": map[string]any{"duration": "1h", "recurring": true},
	}

	FixOpenAI(desc.ProtoReflect().Descriptor(), input)

	fixedJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	msg := new(testdata.CreateItemRequest)
	err = protojson.Unmarshal(fixedJSON, msg)
	if err != nil {
		t.Fatalf("BUG: protojson.Unmarshal failed after FixOpenAI: %v\nJSON was: %s\n"+
			"FixOpenAI leaves both oneof alternatives in JSON; protojson rejects the second one", err, fixedJSON)
	}
}

// TestFixOpenAI_Optional_OneofWithOneNullAlternative tests the common case
// where an LLM sends one real oneof value and null for the other.
// This should work: FixOpenAI should strip the null alternative.
//
// BUG: FixOpenAI leaves the null oneof alternative in place. While protojson
// happens to accept null for a message field, it's still semantically wrong
// and a latent issue if protojson behavior changes.
func TestFixOpenAI_Optional_OneofWithOneNullAlternative(t *testing.T) {
	desc := new(testdata.CreateItemRequest)

	input := map[string]any{
		"name":    "test",
		"product": map[string]any{"price": 9.99, "quantity": float64(1)},
		"service": nil, // null for the unset alternative
	}

	FixOpenAI(desc.ProtoReflect().Descriptor(), input)

	// After fix, the null oneof alternative should be removed
	if _, exists := input["service"]; exists {
		// This is a latent bug: null oneof alternatives should be stripped
		// For now, protojson tolerates null message fields, but this is
		// fragile and semantically incorrect
		t.Logf("WARN: FixOpenAI leaves null oneof alternative 'service' in place")
	}

	fixedJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	msg := new(testdata.CreateItemRequest)
	err = protojson.Unmarshal(fixedJSON, msg)
	if err != nil {
		t.Fatalf("protojson.Unmarshal failed: %v\nJSON was: %s", err, fixedJSON)
	}

	if msg.GetProduct() == nil {
		t.Fatal("expected product to be set")
	}
}
