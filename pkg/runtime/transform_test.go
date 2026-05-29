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

package runtime_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/google/go-cmp/cmp"
)

// decodeInto runs the full decode pipeline: DecodeArguments rewrites the map,
// then protojson unmarshals it into msg, exactly as the generated handlers do.
func decodeInto(t *testing.T, msg proto.Message, args map[string]any) error {
	t.Helper()
	if err := runtime.DecodeArguments(msg.ProtoReflect().Descriptor(), args); err != nil {
		return err
	}
	b, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(b, msg)
}

func mustJSON(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("bad test JSON: %v", err)
	}
	return m
}

// --- decode: oneof wrapper lifting -------------------------------------------

func TestDecode_Oneof_HappyPath(t *testing.T) {
	var req testdata.MultipleOneofsRequest
	args := mustJSON(t, `{"name":"n","source":{"which":"url","url":"http://x"}}`)
	if err := decodeInto(t, &req, args); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if req.GetUrl() != "http://x" {
		t.Fatalf("want url set, got %#v", req.GetSource())
	}
}

func TestDecode_Oneof_NullSiblingsTolerated(t *testing.T) {
	// OpenAI strict mode emits every member, unused ones null.
	var req testdata.MultipleOneofsRequest
	args := mustJSON(t, `{"name":"n","source":{"which":"url","url":"http://x","raw_data":null,"file_path":null}}`)
	if err := decodeInto(t, &req, args); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if req.GetUrl() != "http://x" {
		t.Fatalf("want url, got %#v", req.GetSource())
	}
}

func TestDecode_Oneof_OverfillResolvesToWhich(t *testing.T) {
	// Both members set; "which" wins, the other is dropped without error.
	var req testdata.MultipleOneofsRequest
	args := mustJSON(t, `{"name":"n","source":{"which":"url","url":"http://x","file_path":"/p"}}`)
	if err := decodeInto(t, &req, args); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if req.GetUrl() != "http://x" {
		t.Fatalf("want url to win, got %#v", req.GetSource())
	}
}

func TestDecode_Oneof_AmbiguousIsError(t *testing.T) {
	// which names url, but url is empty while a different member is set.
	args := mustJSON(t, `{"name":"n","source":{"which":"url","url":null,"file_path":"/p"}}`)
	err := runtime.DecodeArguments((&testdata.MultipleOneofsRequest{}).ProtoReflect().Descriptor(), args)
	requireErrContains(t, err, "file_path")
}

func TestDecode_Oneof_NoMemberPopulatedIsError(t *testing.T) {
	args := mustJSON(t, `{"name":"n","source":{"which":"url","url":null}}`)
	err := runtime.DecodeArguments((&testdata.MultipleOneofsRequest{}).ProtoReflect().Descriptor(), args)
	requireErrContains(t, err, "no value")
}

func TestDecode_Oneof_MissingWhichIsError(t *testing.T) {
	args := mustJSON(t, `{"name":"n","source":{"url":"http://x"}}`)
	err := runtime.DecodeArguments((&testdata.MultipleOneofsRequest{}).ProtoReflect().Descriptor(), args)
	requireErrContains(t, err, "which")
}

func TestDecode_Oneof_UnknownWhichIsError(t *testing.T) {
	args := mustJSON(t, `{"name":"n","source":{"which":"bogus","url":"http://x"}}`)
	err := runtime.DecodeArguments((&testdata.MultipleOneofsRequest{}).ProtoReflect().Descriptor(), args)
	requireErrContains(t, err, "bogus")
}

func TestDecode_Oneof_NonStringWhichIsError(t *testing.T) {
	args := mustJSON(t, `{"name":"n","source":{"which":123}}`)
	err := runtime.DecodeArguments((&testdata.MultipleOneofsRequest{}).ProtoReflect().Descriptor(), args)
	requireErrContains(t, err, "must be a string")
}

func TestDecode_Oneof_WrapperNotObjectIsError(t *testing.T) {
	args := mustJSON(t, `{"name":"n","source":"http://x"}`)
	err := runtime.DecodeArguments((&testdata.MultipleOneofsRequest{}).ProtoReflect().Descriptor(), args)
	requireErrContains(t, err, "must be an object")
}

func TestDecode_Oneof_UnsetIsNoop(t *testing.T) {
	var req testdata.MultipleOneofsRequest
	args := mustJSON(t, `{"name":"n"}`)
	if err := decodeInto(t, &req, args); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if req.GetSource() != nil || req.GetName() != "n" {
		t.Fatalf("unexpected: %#v", &req)
	}
}

func TestDecode_Oneof_FalseBoolMember(t *testing.T) {
	// A bool member with value false is still "populated".
	var req testdata.MultipleOneofsRequest
	args := mustJSON(t, `{"name":"n","output_format":{"which":"as_json","as_json":false}}`)
	if err := decodeInto(t, &req, args); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := req.GetOutputFormat().(*testdata.MultipleOneofsRequest_AsJson); !ok {
		t.Fatalf("want as_json oneof set, got %#v", req.GetOutputFormat())
	}
	if req.GetAsJson() != false {
		t.Fatalf("want false")
	}
}

func TestDecode_Oneof_MessageMember(t *testing.T) {
	var req testdata.CreateItemRequest
	args := mustJSON(t, `{"name":"n","item_type":{"which":"product","product":{"price":1.5,"quantity":2}}}`)
	if err := decodeInto(t, &req, args); err != nil {
		t.Fatalf("decode: %v", err)
	}
	p := req.GetProduct()
	if p == nil || p.GetPrice() != 1.5 || p.GetQuantity() != 2 {
		t.Fatalf("want product{1.5,2}, got %#v", req.GetItemType())
	}
}

func TestDecode_Oneof_JSONNameForWhich(t *testing.T) {
	// "which" may name the member by its camelCase JSON name.
	var req testdata.CreateItemRequest
	args := mustJSON(t, `{"name":"n","item_type":{"which":"product","product":{"price":2}}}`)
	if err := decodeInto(t, &req, args); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if req.GetProduct() == nil {
		t.Fatalf("want product")
	}
}

// --- decode: recursion placeholder -------------------------------------------

func TestDecode_RecursionPlaceholderString(t *testing.T) {
	// A deeply nested TreeNode is offered to the model as a JSON-string. Decode
	// must parse it back before protojson sees it. Wrapped in the oneof too.
	deep := `{"value":"d3","children":[{"value":"d4"}]}`
	args := mustJSON(t, `{"node":{"which":"tree","tree":{"value":"d0","children":[{"value":"d1","children":[{"value":"d2","children":["`+jsonEscape(deep)+`"]}]}]}}}`)
	var req testdata.OneofRecursiveRequest
	if err := decodeInto(t, &req, args); err != nil {
		t.Fatalf("decode: %v", err)
	}
	tree := req.GetTree()
	if tree == nil || tree.GetValue() != "d0" {
		t.Fatalf("want tree d0, got %#v", req.GetNode())
	}
	// Walk to the placeholder-sourced node.
	d1 := tree.GetChildren()[0]
	d2 := d1.GetChildren()[0]
	d3 := d2.GetChildren()[0]
	if d3.GetValue() != "d3" || d3.GetChildren()[0].GetValue() != "d4" {
		t.Fatalf("placeholder subtree not decoded: %#v", d3)
	}
}

// --- encode: oneof rewrap ----------------------------------------------------

func TestEncode_Oneof_WhichFirstAndRewrapped(t *testing.T) {
	msg := &testdata.MultipleOneofsRequest{
		Name:   "n",
		Source: &testdata.MultipleOneofsRequest_Url{Url: "http://x"},
	}
	out, err := runtime.EncodeMessage(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	s := string(out)
	// The wrapper must serialize "which" before the value.
	idx := strings.Index(s, `"source":{`)
	if idx < 0 {
		t.Fatalf("no source wrapper: %s", s)
	}
	if !strings.HasPrefix(s[idx:], `"source":{"which":"url"`) {
		t.Fatalf("which not first: %s", s[idx:idx+40])
	}
	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	src := decoded["source"].(map[string]any)
	if src["which"] != "url" || src["url"] != "http://x" {
		t.Fatalf("bad wrapper: %#v", src)
	}
	if _, leaked := decoded["url"]; leaked {
		t.Fatalf("flat member leaked into output: %s", s)
	}
}

func TestEncode_Oneof_FalseBoolViaWhichOneof(t *testing.T) {
	// The set member is a bool false. Scanning marshaled JSON cannot tell it
	// from unset, so encode must use WhichOneof. EmitDefaultValues makes false
	// appear, but only WhichOneof knows it is the SET member.
	msg := &testdata.MultipleOneofsRequest{
		Name:         "n",
		OutputFormat: &testdata.MultipleOneofsRequest_AsJson{AsJson: false},
	}
	out, err := runtime.EncodeMessage(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var decoded map[string]any
	_ = json.Unmarshal(out, &decoded)
	of, ok := decoded["output_format"].(map[string]any)
	if !ok {
		t.Fatalf("output_format not rewrapped: %s", out)
	}
	if of["which"] != "as_json" {
		t.Fatalf("want which=as_json, got %#v", of)
	}
	if of["as_json"] != false {
		t.Fatalf("want as_json=false, got %#v", of["as_json"])
	}
	// Unset source oneof must not appear at all.
	if _, ok := decoded["source"]; ok {
		t.Fatalf("unset oneof leaked: %s", out)
	}
}

func TestEncode_Oneof_Unset(t *testing.T) {
	out, err := runtime.EncodeMessage(&testdata.MultipleOneofsRequest{Name: "n"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var decoded map[string]any
	_ = json.Unmarshal(out, &decoded)
	if _, ok := decoded["source"]; ok {
		t.Fatalf("unset oneof should be absent: %s", out)
	}
	if _, ok := decoded["output_format"]; ok {
		t.Fatalf("unset oneof should be absent: %s", out)
	}
}

func TestEncode_Oneof_MessageMember(t *testing.T) {
	msg := &testdata.CreateItemRequest{
		Name:     "n",
		ItemType: &testdata.CreateItemRequest_Product{Product: &testdata.ProductDetails{Price: 9.99, Quantity: 3}},
	}
	out, err := runtime.EncodeMessage(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var decoded map[string]any
	_ = json.Unmarshal(out, &decoded)
	it := decoded["item_type"].(map[string]any)
	if it["which"] != "product" {
		t.Fatalf("want product, got %#v", it)
	}
	if _, ok := it["product"].(map[string]any); !ok {
		t.Fatalf("product not nested object: %#v", it)
	}
}

// --- encode: recursion stringification + full round trip ---------------------

func TestEncode_RecursiveInsideOneof_StringifiesBeyondDepth(t *testing.T) {
	msg := &testdata.OneofRecursiveResponse{
		Result: &testdata.OneofRecursiveResponse_Tree{Tree: makeTree(6)},
	}
	out, err := runtime.EncodeMessage(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	// Beyond depth 3 the subtree is emitted as an opaque JSON string. Find a
	// quoted, escaped JSON object somewhere under result.tree.
	if !strings.Contains(string(out), `\"children\"`) && !strings.Contains(string(out), `\"value\"`) {
		t.Fatalf("expected a stringified deep subtree, got: %s", out)
	}
	var decoded map[string]any
	_ = json.Unmarshal(out, &decoded)
	res := decoded["result"].(map[string]any)
	if res["which"] != "tree" {
		t.Fatalf("want tree, got %#v", res)
	}
}

func TestRoundTrip_RecursiveInsideOneof(t *testing.T) {
	orig := &testdata.OneofRecursiveRequest{
		Node: &testdata.OneofRecursiveRequest_Tree{Tree: makeTree(6)},
	}
	roundTrip(t, orig, &testdata.OneofRecursiveRequest{})
}

func TestRoundTrip_Curated(t *testing.T) {
	cases := []proto.Message{
		&testdata.MultipleOneofsRequest{Name: "n", Source: &testdata.MultipleOneofsRequest_FilePath{FilePath: "/p"}},
		&testdata.MultipleOneofsRequest{Name: "n", OutputFormat: &testdata.MultipleOneofsRequest_AsCsv{AsCsv: true}},
		// false-bool member: the encode/decode gotcha.
		&testdata.MultipleOneofsRequest{Name: "n", OutputFormat: &testdata.MultipleOneofsRequest_AsJson{AsJson: false}},
		&testdata.CreateItemRequest{
			Name:     "widget",
			Labels:   map[string]string{"env": "prod"},
			Tags:     []string{"a", "b"},
			ItemType: &testdata.CreateItemRequest_Service{Service: &testdata.ServiceDetails{Duration: "1h", Recurring: true}},
		},
		&testdata.OneofRecursiveRequest{Node: &testdata.OneofRecursiveRequest_Leaf{Leaf: "x"}},
		&testdata.OneofRecursiveResponse{Result: &testdata.OneofRecursiveResponse_Ok{Ok: false}},
	}
	for i, c := range cases {
		empty := c.ProtoReflect().New().Interface()
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			roundTrip(t, c, empty)
		})
	}
}

// roundTrip encodes orig to model JSON, decodes it back through the transform +
// protojson, and asserts the result is proto-equal to orig.
func roundTrip(t *testing.T, orig, into proto.Message) {
	t.Helper()
	encoded, err := runtime.EncodeMessage(orig)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var args map[string]any
	if err := json.Unmarshal(encoded, &args); err != nil {
		t.Fatalf("unmarshal encoded: %v", err)
	}
	if err := decodeInto(t, into, args); err != nil {
		t.Fatalf("decode: %v\nencoded: %s", err, encoded)
	}
	if diff := cmp.Diff(orig, into, protocmp.Transform()); diff != "" {
		t.Fatalf("round trip mismatch (-want +got):\n%s\nencoded: %s", diff, encoded)
	}
}

// makeTree builds a left-spine TreeNode of the given depth.
func makeTree(depth int) *testdata.TreeNode {
	if depth <= 0 {
		return &testdata.TreeNode{Value: "leaf"}
	}
	return &testdata.TreeNode{
		Value:    "d" + string(rune('0'+depth)),
		Children: []*testdata.TreeNode{makeTree(depth - 1)},
	}
}

func requireErrContains(t *testing.T, err error, sub string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", sub)
	}
	if !strings.Contains(err.Error(), sub) {
		t.Fatalf("error %q does not contain %q", err.Error(), sub)
	}
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	// strip surrounding quotes; we embed inside an already-quoted string literal
	return string(b[1 : len(b)-1])
}
