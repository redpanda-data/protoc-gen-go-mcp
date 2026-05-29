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
	"testing"

	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"google.golang.org/protobuf/types/known/structpb"
)

// These tests cover the decode-side tolerance for dynamic well-known types that
// a strict-schema client (OpenAI, Anthropic) downgrades to a JSON-encoded
// string. The MCP server must parse the string back before protojson; a native
// JSON value (the Gemini path, schema not downgraded) must pass through.

func TestDecode_WKT_StructFromString(t *testing.T) {
	var req testdata.ProcessWellKnownTypesRequest
	// Struct sent as a JSON-encoded string, as OpenAI/Anthropic would after the
	// adapter collapsed it.
	args := mustJSON(t, `{"metadata":"{\"environment\":\"prod\",\"replicas\":3}"}`)
	if err := decodeInto(t, &req, args); err != nil {
		t.Fatalf("decode: %v", err)
	}
	md := req.GetMetadata()
	if md == nil || md.GetFields()["environment"].GetStringValue() != "prod" {
		t.Fatalf("struct not parsed from string: %#v", md)
	}
	if md.GetFields()["replicas"].GetNumberValue() != 3 {
		t.Fatalf("replicas wrong: %#v", md.GetFields()["replicas"])
	}
}

func TestDecode_WKT_StructFromNativeObject(t *testing.T) {
	// Gemini path: native object, not a string. Must pass through unchanged.
	var req testdata.ProcessWellKnownTypesRequest
	args := mustJSON(t, `{"metadata":{"environment":"prod"}}`)
	if err := decodeInto(t, &req, args); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if req.GetMetadata().GetFields()["environment"].GetStringValue() != "prod" {
		t.Fatalf("native struct lost: %#v", req.GetMetadata())
	}
}

func TestDecode_WKT_ValueObjectFromString(t *testing.T) {
	var req testdata.ProcessWellKnownTypesRequest
	args := mustJSON(t, `{"config":"{\"a\":1,\"b\":[true,null]}"}`)
	if err := decodeInto(t, &req, args); err != nil {
		t.Fatalf("decode: %v", err)
	}
	sv := req.GetConfig().GetStructValue()
	if sv == nil || sv.GetFields()["a"].GetNumberValue() != 1 {
		t.Fatalf("value object not parsed: %#v", req.GetConfig())
	}
}

func TestDecode_WKT_ValueStringifiedScalar(t *testing.T) {
	// A downgraded string Value "hello" arrives JSON-encoded as "\"hello\"" and
	// must be lifted back to the bare string "hello" (the common strict path).
	var req testdata.ProcessWellKnownTypesRequest
	args := mustJSON(t, `{"config":"\"hello\""}`)
	if err := decodeInto(t, &req, args); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := req.GetConfig().GetStringValue(); got != "hello" {
		t.Fatalf("stringified scalar Value not lifted: got %#v", req.GetConfig())
	}
}

func TestDecode_WKT_ValueNativeNonJSONString(t *testing.T) {
	// A native (non-downgraded) Value holding a non-JSON string is left as-is:
	// "hello" does not parse as JSON, so it stays the string "hello".
	var req testdata.ProcessWellKnownTypesRequest
	args := mustJSON(t, `{"config":"hello"}`)
	if err := decodeInto(t, &req, args); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := req.GetConfig().GetStringValue(); got != "hello" {
		t.Fatalf("native string Value lost: got %#v", req.GetConfig())
	}
}

func TestDecode_WKT_ValueNativeFromGemini(t *testing.T) {
	// Native JSON value (object) on the non-downgraded path passes through.
	var req testdata.ProcessWellKnownTypesRequest
	args := mustJSON(t, `{"config":{"nested":"x"}}`)
	if err := decodeInto(t, &req, args); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if req.GetConfig().GetStructValue().GetFields()["nested"].GetStringValue() != "x" {
		t.Fatalf("native value object lost: %#v", req.GetConfig())
	}
}

func TestDecode_WKT_InsideRepeatedMessage(t *testing.T) {
	// ItemWithMap.config (Value) and .extra (Struct) are dynamic WKTs nested in a
	// repeated message; each element's stringified WKTs must be lifted.
	var req testdata.RepeatedMessagesRequest
	args := mustJSON(t, `{"items":[{"name":"a","config":"\"hello\"","extra":"{\"k\":\"v\"}"}]}`)
	if err := decodeInto(t, &req, args); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(req.GetItems()) != 1 {
		t.Fatalf("want 1 item, got %d", len(req.GetItems()))
	}
	it := req.GetItems()[0]
	if it.GetConfig().GetStringValue() != "hello" {
		t.Fatalf("nested Value not lifted: %#v", it.GetConfig())
	}
	if it.GetExtra().GetFields()["k"].GetStringValue() != "v" {
		t.Fatalf("nested Struct not lifted: %#v", it.GetExtra())
	}
}

func TestRoundTrip_WKT_EncodeStaysNativeDecodeBack(t *testing.T) {
	// Encode emits native WKT JSON (matching the un-downgraded generated schema);
	// decode passes native through. Confirms the two sides compose.
	md, err := structpb.NewStruct(map[string]any{"team": "data", "n": float64(2)})
	if err != nil {
		t.Fatal(err)
	}
	orig := &testdata.ProcessWellKnownTypesRequest{
		Metadata: md,
		Config:   structpb.NewStringValue("v"),
	}
	roundTrip(t, orig, &testdata.ProcessWellKnownTypesRequest{})
}
