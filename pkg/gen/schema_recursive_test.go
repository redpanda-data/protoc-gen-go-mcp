package gen

import (
	"testing"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// BUG: MessageSchema has no recursion guard. messageFieldSchema (schema.go
// line 265) calls MessageSchema unconditionally for nested messages, so any
// self-referencing or mutually-recursive protobuf message causes unbounded
// recursion and a fatal stack overflow that kills the process.
//
// Self-referencing messages are a common, valid protobuf pattern:
//   message TreeNode { string val = 1; TreeNode left = 2; TreeNode right = 3; }
//   message LinkedList { int32 val = 1; LinkedList next = 2; }
//
// Fix direction: Pass a "visited" set (or depth counter) through the
// recursion. When a message is encountered that is already on the current
// path, emit an empty object {} (or a $ref if the consumer supports it)
// instead of recursing into it again.
//
// These tests are skipped by default because the stack overflow is fatal
// and kills the test binary. Remove the skip once the bug is fixed;
// the tests should then pass and produce finite schemas.

func buildSelfRefMessage(t *testing.T) protoreflect.MessageDescriptor {
	t.Helper()
	// message TreeNode { string value = 1; TreeNode left = 2; TreeNode right = 3; }
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    sp("test_recursive.proto"),
		Package: sp("testrecursive"),
		Syntax:  sp("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: sp("TreeNode"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: sp("value"), Number: i32p(1), Type: ftp(descriptorpb.FieldDescriptorProto_TYPE_STRING), Label: flp(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL), JsonName: sp("value")},
					{Name: sp("left"), Number: i32p(2), Type: ftp(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: sp(".testrecursive.TreeNode"), Label: flp(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL), JsonName: sp("left")},
					{Name: sp("right"), Number: i32p(3), Type: ftp(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: sp(".testrecursive.TreeNode"), Label: flp(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL), JsonName: sp("right")},
				},
			},
		},
	}
	file, err := protodesc.NewFile(fdp, nil)
	if err != nil {
		t.Fatalf("failed to create file descriptor: %v", err)
	}
	return file.Messages().Get(0)
}

func buildMutuallyRecursiveMessage(t *testing.T) protoreflect.MessageDescriptor {
	t.Helper()
	// message Alpha { Beta b = 1; }
	// message Beta  { Alpha a = 1; }
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    sp("test_mutual.proto"),
		Package: sp("testmutual"),
		Syntax:  sp("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: sp("Alpha"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: sp("b"), Number: i32p(1), Type: ftp(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: sp(".testmutual.Beta"), Label: flp(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL), JsonName: sp("b")},
				},
			},
			{
				Name: sp("Beta"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: sp("a"), Number: i32p(1), Type: ftp(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: sp(".testmutual.Alpha"), Label: flp(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL), JsonName: sp("a")},
				},
			},
		},
	}
	file, err := protodesc.NewFile(fdp, nil)
	if err != nil {
		t.Fatalf("failed to create file descriptor: %v", err)
	}
	return file.Messages().Get(0) // Alpha
}

func buildLinkedListMessage(t *testing.T) protoreflect.MessageDescriptor {
	t.Helper()
	// message LinkedList { int32 value = 1; LinkedList next = 2; }
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    sp("test_linkedlist.proto"),
		Package: sp("testlinkedlist"),
		Syntax:  sp("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: sp("LinkedList"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{Name: sp("value"), Number: i32p(1), Type: ftp(descriptorpb.FieldDescriptorProto_TYPE_INT32), Label: flp(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL), JsonName: sp("value")},
					{Name: sp("next"), Number: i32p(2), Type: ftp(descriptorpb.FieldDescriptorProto_TYPE_MESSAGE), TypeName: sp(".testlinkedlist.LinkedList"), Label: flp(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL), JsonName: sp("next")},
				},
			},
		},
	}
	file, err := protodesc.NewFile(fdp, nil)
	if err != nil {
		t.Fatalf("failed to create file descriptor: %v", err)
	}
	return file.Messages().Get(0)
}

// TestRecursiveSelfReference tests that MessageSchema produces a finite schema
// for self-referencing TreeNode messages. The schema expands 3 levels deep,
// then emits a JSON-string placeholder.
func TestRecursiveSelfReference_StackOverflow(t *testing.T) {
	md := buildSelfRefMessage(t)
	schema := MessageSchema(md, SchemaOptions{})
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}

	// Level 1: full schema with left/right as objects
	props := schema["properties"].(map[string]any)
	if _, ok := props["left"]; !ok {
		t.Error("expected 'left' field in schema")
	}
	if _, ok := props["right"]; !ok {
		t.Error("expected 'right' field in schema")
	}

	// Drill down: level 1 -> level 2 -> level 3 -> string placeholder
	left := props["left"].(map[string]any)
	if left["type"] != "object" {
		t.Errorf("level 1 left should be object, got %v", left["type"])
	}
	l2props := left["properties"].(map[string]any)
	l2left := l2props["left"].(map[string]any)
	if l2left["type"] != "object" {
		t.Errorf("level 2 left should be object, got %v", l2left["type"])
	}
	l3props := l2left["properties"].(map[string]any)
	l3left := l3props["left"].(map[string]any)
	// Level 3 is the recursion cutoff: should be a string placeholder
	if l3left["type"] != "string" {
		t.Errorf("level 3 left should be string placeholder, got %v", l3left["type"])
	}
	desc, _ := l3left["description"].(string)
	if desc == "" {
		t.Error("level 3 placeholder should have a description")
	}
}

// TestRecursiveMutual tests mutually recursive messages (Alpha -> Beta -> Alpha).
func TestRecursiveMutual_StackOverflow(t *testing.T) {
	md := buildMutuallyRecursiveMessage(t)
	schema := MessageSchema(md, SchemaOptions{})
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
	// Alpha has field b (Beta), which has field a (Alpha), etc.
	props := schema["properties"].(map[string]any)
	b := props["b"].(map[string]any)
	if b["type"] != "object" {
		t.Errorf("Alpha.b should be object, got %v", b["type"])
	}
}

// TestRecursiveOpenAICompat tests recursion in OpenAI mode. The placeholder
// must be a string (consistent with Struct/Value handling), not a bare object.
func TestRecursiveOpenAICompat_StackOverflow(t *testing.T) {
	md := buildLinkedListMessage(t)
	schema := MessageSchema(md, SchemaOptions{OpenAICompat: true})
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
	props := schema["properties"].(map[string]any)
	if _, ok := props["next"]; !ok {
		t.Error("expected 'next' field in schema")
	}

	// Drill to the cutoff
	next := props["next"].(map[string]any)
	if next["type"] != "object" {
		t.Errorf("level 1 next should be object, got %v", next["type"])
	}
	n2 := next["properties"].(map[string]any)["next"].(map[string]any)
	if n2["type"] != "object" {
		t.Errorf("level 2 next should be object, got %v", n2["type"])
	}
	n3 := n2["properties"].(map[string]any)["next"].(map[string]any)
	if n3["type"] != "string" {
		t.Errorf("level 3 next should be string placeholder, got %v", n3["type"])
	}
}

func sp(s string) *string                                                                      { return &s }
func i32p(i int32) *int32                                                                      { return &i }
func ftp(t descriptorpb.FieldDescriptorProto_Type) *descriptorpb.FieldDescriptorProto_Type     { return &t }
func flp(l descriptorpb.FieldDescriptorProto_Label) *descriptorpb.FieldDescriptorProto_Label   { return &l }
