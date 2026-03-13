// Copyright 2025 Redpanda Data, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package generator

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	// Blank-import proto packages so their file descriptors (with correct
	// go_package from buf managed mode) are registered in GlobalFiles.
	_ "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

// Proto source paths whose generated .pb.mcp.go output is checked in
// under pkg/testdata/gen/go/ and serves as golden reference.
var goldenProtoFiles = []string{
	"testdata/test_service.proto",
	"testdata/edge_cases.proto",
}

// TestGoldenGeneration re-runs the code generator in-process and compares
// its output against the checked-in .pb.mcp.go files.
//
// This replaces the old sh_test + gen/go-golden approach. Descriptors come
// from two sources:
//   - Compiled Go packages (imported above) provide file descriptors with
//     correct go_package options set by buf managed mode.
//   - A FileDescriptorSet binary (gen/descriptors.binpb, produced by
//     buf build) provides source_code_info so that method comments end up
//     as tool descriptions in the generated code.
func TestGoldenGeneration(t *testing.T) {
	g := NewWithT(t)

	// Load source_code_info from the buf-built descriptor set.
	srcInfoByPath := loadSourceCodeInfo(g)

	// Collect file descriptor protos from the compiled Go registry,
	// patching in source_code_info where available.
	filesByPath := map[string]*descriptorpb.FileDescriptorProto{}
	var collectDeps func(protoreflect.FileDescriptor)
	collectDeps = func(fd protoreflect.FileDescriptor) {
		p := string(fd.Path())
		if _, ok := filesByPath[p]; ok {
			return
		}
		fdp := protodesc.ToFileDescriptorProto(fd)
		if sci, ok := srcInfoByPath[p]; ok {
			fdp.SourceCodeInfo = sci
		}
		filesByPath[p] = fdp
		for i := 0; i < fd.Imports().Len(); i++ {
			collectDeps(fd.Imports().Get(i).FileDescriptor)
		}
	}

	var filesToGenerate []string
	for _, path := range goldenProtoFiles {
		fd, err := protoregistry.GlobalFiles.FindFileByPath(path)
		g.Expect(err).ToNot(HaveOccurred(), "proto %s not registered — missing blank import?", path)
		if fd.Services().Len() == 0 {
			continue
		}
		filesToGenerate = append(filesToGenerate, path)
		collectDeps(fd)
	}
	g.Expect(filesToGenerate).ToNot(BeEmpty())

	// Topological sort: dependencies before dependents.
	sorted := topoSort(filesByPath)

	req := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: filesToGenerate,
		ProtoFile:      sorted,
		Parameter:      proto.String("paths=source_relative"),
	}

	plugin, err := protogen.Options{}.New(req)
	g.Expect(err).ToNot(HaveOccurred())

	for _, f := range plugin.Files {
		if !f.Generate {
			continue
		}
		NewFileGenerator(f, plugin).Generate("mcp")
	}

	resp := plugin.Response()
	g.Expect(resp.GetError()).To(BeEmpty())
	g.Expect(resp.File).ToNot(BeEmpty(), "generator produced no output files")

	// Compare each output file against its checked-in golden copy.
	for _, rf := range resp.File {
		goldenPath := testdataPath("gen/go/" + rf.GetName())
		expected, err := os.ReadFile(goldenPath)
		g.Expect(err).ToNot(HaveOccurred(), "reading golden file for %s", rf.GetName())
		g.Expect(rf.GetContent()).To(Equal(string(expected)),
			"generated output differs from checked-in %s\nRun: just generate", rf.GetName())
	}
}

// loadSourceCodeInfo reads the FileDescriptorSet produced by buf build
// and returns a map of proto path -> SourceCodeInfo.
func loadSourceCodeInfo(g Gomega) map[string]*descriptorpb.SourceCodeInfo {
	data, err := os.ReadFile(testdataPath("gen/descriptors.binpb"))
	g.Expect(err).ToNot(HaveOccurred(), "reading descriptors.binpb — run: cd pkg/testdata && buf build -o gen/descriptors.binpb --exclude-path buf/validate")

	fds := &descriptorpb.FileDescriptorSet{}
	g.Expect(proto.Unmarshal(data, fds)).To(Succeed())

	m := make(map[string]*descriptorpb.SourceCodeInfo, len(fds.File))
	for _, f := range fds.File {
		if f.SourceCodeInfo != nil {
			m[f.GetName()] = f.SourceCodeInfo
		}
	}
	return m
}

// testdataPath resolves a path relative to pkg/testdata/.
// In Bazel, files are in runfiles; in go test, they're relative to the
// test package directory.
func testdataPath(rel string) string {
	// Bazel runfiles path.
	if runfiles := os.Getenv("TEST_SRCDIR"); runfiles != "" {
		ws := os.Getenv("TEST_WORKSPACE")
		return filepath.Join(runfiles, ws, "pkg/testdata", rel)
	}
	return filepath.Join("..", "testdata", rel)
}

// topoSort returns file descriptor protos in dependency order.
func topoSort(files map[string]*descriptorpb.FileDescriptorProto) []*descriptorpb.FileDescriptorProto {
	var sorted []*descriptorpb.FileDescriptorProto
	visited := map[string]bool{}
	var visit func(string)
	visit = func(path string) {
		if visited[path] {
			return
		}
		visited[path] = true
		if fdp, ok := files[path]; ok {
			for _, dep := range fdp.Dependency {
				visit(dep)
			}
			sorted = append(sorted, fdp)
		}
	}
	for path := range files {
		visit(path)
	}
	return sorted
}
