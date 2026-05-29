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
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// DiscriminatorKey is the property name of the oneof discriminator emitted in
// the nested wrapper object. It carries the protojson name of the field that is
// set within the oneof. The gen package mirrors this constant when rendering
// schemas, and DecodeArguments/EncodeMessage read and write it at runtime.
const DiscriminatorKey = "which"

// DefaultMaxRecursionDepth is the number of times a recursive message type is
// expanded in a generated schema before it is replaced with a JSON-string
// placeholder. EncodeMessage uses the same value so the JSON it emits matches
// the schema the model was given. The gen package mirrors this constant.
const DefaultMaxRecursionDepth = 3

// DecodeArguments rewrites model-supplied tool-call arguments in place so that
// protojson can unmarshal them into a proto message. It is the inverse of the
// two schema shapes the generator emits that protojson does not understand:
//
//   - oneof discriminated wrappers: a oneof renders as a nested object
//     {"which":"<member>", "<member>":<value>, ...}. This lifts the named
//     member to its native sibling key so protojson sees a normal oneof.
//   - recursion-depth placeholders: a message nested beyond MaxRecursionDepth
//     renders as a JSON-string. This parses that string back to an object.
//
// Everything else passes straight through to protojson untouched. Errors are
// phrased to be model-readable: a failed tool call is returned to the model for
// one-turn self-correction, so the message names the fix.
func DecodeArguments(md protoreflect.MessageDescriptor, args map[string]any) error {
	return decodeMessage(md, args)
}

func decodeMessage(md protoreflect.MessageDescriptor, obj map[string]any) error {
	// 1) Lift oneof discriminated wrappers to native member fields.
	for i := 0; i < md.Oneofs().Len(); i++ {
		oo := md.Oneofs().Get(i)
		if oo.IsSynthetic() {
			continue
		}
		if err := liftOneof(oo, obj); err != nil {
			return err
		}
	}

	// 2) Recurse into message-typed fields (including the lifted oneof member),
	//    parsing recursion-depth string placeholders back to objects.
	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		// Dynamic well-known types (Struct/Value/ListValue) cannot be expressed in
		// the strict tool-schema subset OpenAI and Anthropic accept, so a client
		// may downgrade them to a JSON-encoded string. Parse that string back to
		// native JSON here so protojson sees the shape it expects. A model that
		// sent native JSON (e.g. Gemini, whose schema is not downgraded) is left
		// untouched. Covers scalar, repeated and map-valued fields.
		if isDynamicWKTField(fd) {
			liftStringifiedWKT(fd, obj)
			continue
		}
		if fd.Kind() != protoreflect.MessageKind && fd.Kind() != protoreflect.GroupKind {
			continue
		}
		if isWellKnown(fd.Message()) {
			// protojson handles WKTs natively; they never carry a oneof wrapper
			// or a placeholder we emitted.
			continue
		}
		name := resolveFieldName(fd, obj)
		if name == "" {
			continue
		}

		switch {
		case fd.IsMap():
			if fd.MapValue().Kind() != protoreflect.MessageKind || isWellKnown(fd.MapValue().Message()) {
				continue
			}
			m, ok := obj[name].(map[string]any)
			if !ok {
				continue
			}
			for k, v := range m {
				child, err := asMessageObject(v)
				if err != nil {
					return fmt.Errorf("field %q[%q]: %w", name, k, err)
				}
				if child == nil {
					continue
				}
				if err := decodeMessage(fd.MapValue().Message(), child); err != nil {
					return err
				}
				m[k] = child
			}
		case fd.IsList():
			arr, ok := obj[name].([]any)
			if !ok {
				continue
			}
			for idx, v := range arr {
				child, err := asMessageObject(v)
				if err != nil {
					return fmt.Errorf("field %q[%d]: %w", name, idx, err)
				}
				if child == nil {
					continue
				}
				if err := decodeMessage(fd.Message(), child); err != nil {
					return err
				}
				arr[idx] = child
			}
		default:
			child, err := asMessageObject(obj[name])
			if err != nil {
				return fmt.Errorf("field %q: %w", name, err)
			}
			if child == nil {
				continue
			}
			if err := decodeMessage(fd.Message(), child); err != nil {
				return err
			}
			obj[name] = child
		}
	}
	return nil
}

// liftOneof resolves a single oneof discriminated wrapper in obj into its
// native member field. It is a no-op when no wrapper is present (the oneof is
// unset, or a depth-limited subtree supplied native member fields directly).
func liftOneof(oo protoreflect.OneofDescriptor, obj map[string]any) error {
	oneofName := string(oo.Name())
	raw, present := obj[oneofName]
	if !present || raw == nil {
		return nil
	}
	wrapper, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("oneof %q must be an object like {%q:\"<field>\", \"<field>\":<value>}; got %T",
			oneofName, DiscriminatorKey, raw)
	}

	whichRaw, ok := wrapper[DiscriminatorKey]
	if !ok {
		return fmt.Errorf("oneof %q is missing the required %q field naming which member is set (one of %v)",
			oneofName, DiscriminatorKey, oneofFieldNames(oo))
	}
	which, ok := whichRaw.(string)
	if !ok {
		return fmt.Errorf("oneof %q discriminator %q must be a string naming one of %v; got %T",
			oneofName, DiscriminatorKey, oneofFieldNames(oo), whichRaw)
	}
	memberFd := oneofMemberByAnyName(oo, which)
	if memberFd == nil {
		return fmt.Errorf("oneof %q discriminator %q=%q is not one of its fields %v",
			oneofName, DiscriminatorKey, which, oneofFieldNames(oo))
	}
	memberName := string(memberFd.Name())

	whichVal, whichPopulated := populatedMember(wrapper, memberFd)
	var otherPopulated []string
	for j := 0; j < oo.Fields().Len(); j++ {
		fd := oo.Fields().Get(j)
		if fd.Name() == memberFd.Name() {
			continue
		}
		if _, pop := populatedMember(wrapper, fd); pop {
			otherPopulated = append(otherPopulated, string(fd.Name()))
		}
	}

	switch {
	case whichPopulated:
		// Use the named member; drop over-filled siblings silently. OpenAI
		// strict mode sends every member with the unused ones null, which is
		// the common, benign case.
		delete(obj, oneofName)
		obj[memberName] = whichVal
		return nil
	case len(otherPopulated) > 0:
		sort.Strings(otherPopulated)
		return fmt.Errorf("oneof %q says %s=%q but %q has no value while %v is set; set %s to match the field you filled, or move the value to %q",
			oneofName, DiscriminatorKey, which, memberName, otherPopulated, DiscriminatorKey, memberName)
	default:
		return fmt.Errorf("oneof %q says %s=%q but no value was provided for %q; set %q to the chosen value",
			oneofName, DiscriminatorKey, which, memberName, memberName)
	}
}

// populatedMember returns the value of fd within the wrapper and whether it is
// present and non-null.
func populatedMember(wrapper map[string]any, fd protoreflect.FieldDescriptor) (any, bool) {
	name := resolveFieldName(fd, wrapper)
	if name == "" {
		return nil, false
	}
	v := wrapper[name]
	if v == nil {
		return nil, false
	}
	return v, true
}

func oneofMemberByAnyName(oo protoreflect.OneofDescriptor, name string) protoreflect.FieldDescriptor {
	for j := 0; j < oo.Fields().Len(); j++ {
		fd := oo.Fields().Get(j)
		if string(fd.Name()) == name || fd.JSONName() == name {
			return fd
		}
	}
	return nil
}

func oneofFieldNames(oo protoreflect.OneofDescriptor) []string {
	names := make([]string, 0, oo.Fields().Len())
	for j := 0; j < oo.Fields().Len(); j++ {
		names = append(names, string(oo.Fields().Get(j).Name()))
	}
	return names
}

// asMessageObject returns the object form of a message-typed JSON value: a map
// as-is, or a recursion-depth placeholder string parsed back to a map. It
// returns (nil, nil) when the value is absent, null, or not an object form.
func asMessageObject(v any) (map[string]any, error) {
	switch t := v.(type) {
	case map[string]any:
		return t, nil
	case string:
		var parsed map[string]any
		if err := json.Unmarshal([]byte(t), &parsed); err != nil {
			return nil, fmt.Errorf("expected a JSON object (or a JSON-object string for a deeply nested message); could not parse %q: %w", t, err)
		}
		return parsed, nil
	default:
		return nil, nil
	}
}

// EncodeMessage marshals a proto message to the model-facing JSON shape: it
// runs protojson, then rewraps each set oneof into its discriminated object and
// stringifies any subtree nested beyond DefaultMaxRecursionDepth so the output
// matches the tool's generated output schema. It is the encode-side inverse of
// DecodeArguments.
func EncodeMessage(msg proto.Message) (json.RawMessage, error) {
	marshaled, err := (protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}).Marshal(msg)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal(marshaled, &obj); err != nil {
		return nil, err
	}
	m := msg.ProtoReflect()
	seen := map[protoreflect.FullName]int{m.Descriptor().FullName(): 1}
	if err := encodeMessage(m, obj, seen); err != nil {
		return nil, err
	}
	return json.Marshal(obj)
}

// encodeMessage transforms obj (the protojson encoding of m) in place. The
// caller has already accounted for m's own type in seen, mirroring the
// depth-limited expansion in gen.messageSchema.
func encodeMessage(m protoreflect.Message, obj map[string]any, seen map[protoreflect.FullName]int) error {
	md := m.Descriptor()

	// 1) Recurse into message-typed fields, applying depth stringification and
	//    nested oneof rewrapping. protojson emits the set oneof member as a flat
	//    key, so it is handled here like any other message field.
	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		if fd.Kind() != protoreflect.MessageKind && fd.Kind() != protoreflect.GroupKind {
			continue
		}
		if isWellKnown(fd.Message()) {
			continue
		}
		name := string(fd.Name())
		if _, ok := obj[name]; !ok {
			continue
		}

		switch {
		case fd.IsMap():
			if fd.MapValue().Kind() != protoreflect.MessageKind || isWellKnown(fd.MapValue().Message()) {
				continue
			}
			jsonMap, ok := obj[name].(map[string]any)
			if !ok {
				continue
			}
			protoMap := m.Get(fd).Map()
			var rangeErr error
			protoMap.Range(func(mk protoreflect.MapKey, v protoreflect.Value) bool {
				key := mk.Value().String()
				child, ok := jsonMap[key].(map[string]any)
				if !ok {
					return true
				}
				newVal, err := encodeChild(fd.MapValue().Message(), v.Message(), child, seen)
				if err != nil {
					rangeErr = err
					return false
				}
				jsonMap[key] = newVal
				return true
			})
			if rangeErr != nil {
				return rangeErr
			}
		case fd.IsList():
			arr, ok := obj[name].([]any)
			if !ok {
				continue
			}
			list := m.Get(fd).List()
			for idx := 0; idx < list.Len() && idx < len(arr); idx++ {
				child, ok := arr[idx].(map[string]any)
				if !ok {
					continue
				}
				newVal, err := encodeChild(fd.Message(), list.Get(idx).Message(), child, seen)
				if err != nil {
					return err
				}
				arr[idx] = newVal
			}
		default:
			child, ok := obj[name].(map[string]any)
			if !ok {
				continue
			}
			newVal, err := encodeChild(fd.Message(), m.Get(fd).Message(), child, seen)
			if err != nil {
				return err
			}
			obj[name] = newVal
		}
	}

	// 2) Rewrap each set oneof into its discriminated object. The set member's
	//    value has already been transformed by step 1.
	for i := 0; i < md.Oneofs().Len(); i++ {
		oo := md.Oneofs().Get(i)
		if oo.IsSynthetic() {
			continue
		}
		set := m.WhichOneof(oo)
		var (
			memberName string
			memberVal  any
			haveMember bool
		)
		if set != nil {
			memberName = string(set.Name())
			memberVal, haveMember = obj[memberName]
		}
		// Drop any member keys protojson emitted (only the set one exists).
		for j := 0; j < oo.Fields().Len(); j++ {
			delete(obj, string(oo.Fields().Get(j).Name()))
		}
		if set == nil || !haveMember {
			continue
		}
		wrapped, err := wrapOneof(memberName, memberVal)
		if err != nil {
			return err
		}
		obj[string(oo.Name())] = wrapped
	}
	return nil
}

// wrapOneof builds the discriminated wrapper object for a set oneof member as
// raw JSON, keeping the "which" discriminator first so the model reads it
// before the value.
func wrapOneof(memberName string, memberVal any) (json.RawMessage, error) {
	valJSON, err := json.Marshal(memberVal)
	if err != nil {
		return nil, err
	}
	keyJSON, err := json.Marshal(memberName)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	b.WriteString(`{"`)
	b.WriteString(DiscriminatorKey)
	b.WriteString(`":`)
	b.Write(keyJSON)
	b.WriteByte(',')
	b.Write(keyJSON)
	b.WriteByte(':')
	b.Write(valJSON)
	b.WriteByte('}')
	return json.RawMessage(b.String()), nil
}

// encodeChild transforms a single nested message value. If expanding child's
// type would exceed the depth budget, it stringifies the subtree to match the
// schema's placeholder; otherwise it recurses.
func encodeChild(childType protoreflect.MessageDescriptor, childMsg protoreflect.Message, child map[string]any, seen map[protoreflect.FullName]int) (any, error) {
	if seen[childType.FullName()] >= DefaultMaxRecursionDepth {
		// Beyond the depth boundary the schema is an opaque JSON-string. Emit
		// the protojson-native subtree as a string (no oneof rewrapping inside,
		// matching what the decode side parses back).
		b, err := json.Marshal(child)
		if err != nil {
			return nil, err
		}
		return string(b), nil
	}
	seen[childType.FullName()]++
	defer func() { seen[childType.FullName()]-- }()
	if err := encodeMessage(childMsg, child, seen); err != nil {
		return nil, err
	}
	return child, nil
}

// resolveFieldName returns the key actually present in obj for fd. It checks the
// proto name (snake_case) first, then the JSON name (camelCase), since different
// models use either convention. Returns "" if neither is present.
func resolveFieldName(fd protoreflect.FieldDescriptor, obj map[string]any) string {
	protoName := string(fd.Name())
	if _, ok := obj[protoName]; ok {
		return protoName
	}
	jsonName := fd.JSONName()
	if _, ok := obj[jsonName]; ok {
		return jsonName
	}
	return ""
}

// isDynamicWKTField reports whether fd carries a dynamic well-known type
// (Struct/Value/ListValue) as its value — directly, as a repeated element, or as
// a map value. These are the types a client may downgrade to a JSON-string.
func isDynamicWKTField(fd protoreflect.FieldDescriptor) bool {
	if fd.IsMap() {
		mv := fd.MapValue()
		return mv.Kind() == protoreflect.MessageKind && isDynamicWKT(mv.Message())
	}
	return fd.Kind() == protoreflect.MessageKind && isDynamicWKT(fd.Message())
}

// isDynamicWKT reports whether md is a protobuf well-known type that renders as
// open-ended/dynamic JSON (no fixed type). A strict-schema client collapses
// these to a JSON-encoded string, which DecodeArguments parses back. Any is
// excluded: it keeps a typed wrapper shape and is not stringified wholesale.
func isDynamicWKT(md protoreflect.MessageDescriptor) bool {
	switch string(md.FullName()) {
	case "google.protobuf.Struct",
		"google.protobuf.Value",
		"google.protobuf.ListValue":
		return true
	default:
		return false
	}
}

// liftStringifiedWKT parses any JSON-string values of a dynamic-WKT field back to
// native JSON in place, across scalar, repeated and map shapes. A value that is
// not a string, or not parseable as JSON, is left untouched: protojson then sees
// the native JSON (Gemini path) or reports the error itself.
func liftStringifiedWKT(fd protoreflect.FieldDescriptor, obj map[string]any) {
	name := resolveFieldName(fd, obj)
	if name == "" {
		return
	}
	switch {
	case fd.IsMap():
		m, ok := obj[name].(map[string]any)
		if !ok {
			return
		}
		for k, v := range m {
			if parsed, ok := parseJSONString(v); ok {
				m[k] = parsed
			}
		}
	case fd.IsList():
		arr, ok := obj[name].([]any)
		if !ok {
			return
		}
		for idx, v := range arr {
			if parsed, ok := parseJSONString(v); ok {
				arr[idx] = parsed
			}
		}
	default:
		if parsed, ok := parseJSONString(obj[name]); ok {
			obj[name] = parsed
		}
	}
}

// parseJSONString returns the JSON value encoded in v when v is a string holding
// valid JSON, and (v, false) otherwise. A value the client downgraded to a
// string is, by construction, valid JSON ("\"x\"", "42", "{...}"), so it is
// lifted; a google.protobuf.Value that natively holds a non-JSON string (e.g.
// "hello") fails to parse and is left as the string it is. The one ambiguous
// case — a native Value holding a string that happens to be valid JSON, such as
// "42" — is resolved in favor of the (far more common) downgraded reading.
func parseJSONString(v any) (any, bool) {
	s, ok := v.(string)
	if !ok {
		return v, false
	}
	var out any
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return v, false
	}
	return out, true
}

// isWellKnown reports whether md is a protobuf well-known type that the schema
// renders with a bespoke shape (not via message expansion), so the transform
// must leave protojson's native encoding untouched.
func isWellKnown(md protoreflect.MessageDescriptor) bool {
	switch string(md.FullName()) {
	case "google.protobuf.Timestamp",
		"google.protobuf.Duration",
		"google.protobuf.Struct",
		"google.protobuf.Value",
		"google.protobuf.ListValue",
		"google.protobuf.FieldMask",
		"google.protobuf.Any",
		"google.protobuf.DoubleValue",
		"google.protobuf.FloatValue",
		"google.protobuf.Int32Value",
		"google.protobuf.UInt32Value",
		"google.protobuf.Int64Value",
		"google.protobuf.UInt64Value",
		"google.protobuf.StringValue",
		"google.protobuf.BoolValue",
		"google.protobuf.BytesValue":
		return true
	default:
		return false
	}
}
