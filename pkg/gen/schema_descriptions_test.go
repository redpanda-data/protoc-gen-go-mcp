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

package gen

import (
	"testing"

	. "github.com/onsi/gomega"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
)

// TestExtractValidateConstraints_StringIn pins (buf.validate.field).string.in
// mapping onto a JSON schema "enum" with the values verbatim.
func TestExtractValidateConstraints_StringIn(t *testing.T) {
	g := NewWithT(t)
	md := (&testdata.TestValidationRequest{}).ProtoReflect().Descriptor()
	fd := md.Fields().ByName("action")
	g.Expect(fd).ToNot(BeNil())

	constraints := ExtractValidateConstraints(fd)
	g.Expect(constraints["enum"]).To(Equal([]string{"add", "remove"}))

	schema := FieldSchema(fd, SchemaOptions{})
	g.Expect(schema["type"]).To(Equal("string"))
	g.Expect(schema["enum"]).To(Equal([]string{"add", "remove"}))
}

// TestIsFieldRequired_BufValidate pins (buf.validate.field).required = true
// as a source of JSON schema "required", alongside the existing
// (google.api.field_behavior) = REQUIRED path.
func TestIsFieldRequired_BufValidate(t *testing.T) {
	g := NewWithT(t)
	md := (&testdata.TestValidationRequest{}).ProtoReflect().Descriptor()

	g.Expect(IsFieldRequired(md.Fields().ByName("title"))).To(BeTrue())
	g.Expect(IsFieldRequired(md.Fields().ByName("action"))).To(BeFalse())
	g.Expect(IsFieldRequired(md.Fields().ByName("email"))).To(BeFalse())

	// field_behavior REQUIRED still works.
	createMD := (&testdata.CreateItemRequest{}).ProtoReflect().Descriptor()
	g.Expect(IsFieldRequired(createMD.Fields().ByName("name"))).To(BeTrue())

	schema := MessageSchema(md, SchemaOptions{})
	g.Expect(schema["required"]).To(Equal([]string{"title"}))
}

// TestMessageSchema_FieldDescriptionsFromComments verifies that field leading
// comments (available via source info) are emitted as property descriptions.
// Compiled Go descriptors strip source info, so the test builds a descriptor
// with SourceCodeInfo by hand — the same shape protoc hands the plugin.
func TestMessageSchema_FieldDescriptionsFromComments(t *testing.T) {
	g := NewWithT(t)

	fdp := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("desctest/desc_test.proto"),
		Package: proto.String("desctest"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("ReactRequest"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name:     proto.String("channel_id"),
					Number:   proto.Int32(1),
					Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					JsonName: proto.String("channelId"),
				},
				{
					Name:     proto.String("emoji"),
					Number:   proto.Int32(2),
					Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					JsonName: proto.String("emoji"),
				},
			},
		}},
		SourceCodeInfo: &descriptorpb.SourceCodeInfo{
			Location: []*descriptorpb.SourceCodeInfo_Location{
				{
					// Path [4, 0, 2, 0]: message 0, field 0.
					Path:            []int32{4, 0, 2, 0},
					Span:            []int32{4, 2, 26},
					LeadingComments: proto.String(" The channel ID containing the message.\n"),
				},
				// Field 1 ("emoji") has no comment on purpose: no
				// description must be emitted for it.
			},
		},
	}

	file, err := protodesc.NewFile(fdp, nil)
	g.Expect(err).ToNot(HaveOccurred())

	schema := MessageSchema(file.Messages().Get(0), SchemaOptions{})
	props := schema["properties"].(map[string]any)

	channelID := props["channel_id"].(map[string]any)
	g.Expect(channelID["description"]).To(Equal("The channel ID containing the message."))

	emoji := props["emoji"].(map[string]any)
	g.Expect(emoji).ToNot(HaveKey("description"))
}
