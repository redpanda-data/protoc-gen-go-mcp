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
	"testing"

	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/runtime"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// FuzzDecodeArguments feeds arbitrary JSON objects to DecodeArguments against a
// set of descriptors that cover oneofs, recursion, maps, lists and dynamic WKTs.
// The contract under test: DecodeArguments must never panic, and when it returns
// nil the rewritten map must marshal to protojson-acceptable bytes (clean error
// otherwise). Both decode errors and protojson errors are acceptable; a panic or
// a silent production of un-unmarshalable output is not.
func FuzzDecodeArguments(f *testing.F) {
	descriptors := []protoreflect.MessageDescriptor{
		(&testdata.MultipleOneofsRequest{}).ProtoReflect().Descriptor(),
		(&testdata.CreateItemRequest{}).ProtoReflect().Descriptor(),
		(&testdata.OneofRecursiveRequest{}).ProtoReflect().Descriptor(),
		(&testdata.ProcessWellKnownTypesRequest{}).ProtoReflect().Descriptor(),
		(&testdata.DeepNestingRequest{}).ProtoReflect().Descriptor(),
	}
	newMsgs := []func() proto.Message{
		func() proto.Message { return &testdata.MultipleOneofsRequest{} },
		func() proto.Message { return &testdata.CreateItemRequest{} },
		func() proto.Message { return &testdata.OneofRecursiveRequest{} },
		func() proto.Message { return &testdata.ProcessWellKnownTypesRequest{} },
		func() proto.Message { return &testdata.DeepNestingRequest{} },
	}

	seeds := []string{
		`{}`,
		`{"name":"n","source":{"which":"url","url":"x"}}`,
		`{"name":"n","source":{"which":"url"}}`,
		`{"name":"n","source":{"which":123}}`,
		`{"name":"n","source":{"which":"bogus","url":"x"}}`,
		`{"name":"n","source":"flat"}`,
		`{"name":"n","source":{"which":"url","url":null,"file_path":"/p"}}`,
		`{"item_type":{"which":"product","product":{"price":1.5}}}`,
		`{"node":{"which":"tree","tree":{"value":"v","children":["{\"value\":\"x\"}"]}}}`,
		`{"metadata":"{\"k\":\"v\"}","config":"42"}`,
		`{"metadata":{"k":"v"},"config":{"a":1}}`,
		`{"source":{"which":"raw_data","raw_data":"!!notbase64"}}`,
		`{"source":{"which":"url","url":{"nested":"object-where-string-expected"}}}`,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, in string) {
		var args map[string]any
		if err := json.Unmarshal([]byte(in), &args); err != nil {
			return // only object inputs are meaningful tool arguments
		}
		for i, md := range descriptors {
			// Each descriptor gets its own copy: DecodeArguments mutates in place.
			cp := deepCopyAny(args).(map[string]any)
			err := func() (err error) {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("DecodeArguments panicked on %s for %s: %v", in, md.FullName(), r)
					}
				}()
				return runtime.DecodeArguments(md, cp)
			}()
			if err != nil {
				continue // clean error is acceptable
			}
			// When decode succeeds, the result must be protojson-marshalable bytes
			// that protojson either accepts or rejects cleanly (no panic).
			b, mErr := json.Marshal(cp)
			if mErr != nil {
				t.Fatalf("decoded map not marshalable: %v", mErr)
			}
			msg := newMsgs[i]()
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("protojson panicked on decoded %s: %v", b, r)
					}
				}()
				_ = (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(b, msg)
			}()
		}
	})
}

func deepCopyAny(v any) any {
	switch t := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[k] = deepCopyAny(val)
		}
		return m
	case []any:
		s := make([]any, len(t))
		for i, val := range t {
			s[i] = deepCopyAny(val)
		}
		return s
	default:
		return v
	}
}
