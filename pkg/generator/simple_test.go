package generator

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	_ "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestSimpleModeRemovesCallOption(t *testing.T) {
	g := NewWithT(t)

	srcInfoByPath := loadSourceCodeInfo(g)

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
		g.Expect(err).ToNot(HaveOccurred())
		if fd.Services().Len() == 0 {
			continue
		}
		filesToGenerate = append(filesToGenerate, path)
		collectDeps(fd)
	}
	g.Expect(filesToGenerate).ToNot(BeEmpty())

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
		NewFileGenerator(f, plugin).Generate("mcp", true)
	}

	resp := plugin.Response()
	g.Expect(resp.GetError()).To(BeEmpty())
	g.Expect(resp.File).ToNot(BeEmpty())

	for _, rf := range resp.File {
		content := rf.GetContent()

		// Simple mode should NOT have opts ...grpc.CallOption in Client interfaces
		g.Expect(content).ToNot(ContainSubstring("grpc.CallOption"),
			"simple mode should not emit grpc.CallOption in file %s", rf.GetName())

		// Simple mode should NOT import grpc (unused import would be a compilation error)
		g.Expect(content).ToNot(ContainSubstring(`grpc "google.golang.org/grpc"`),
			"simple mode should not import grpc in file %s", rf.GetName())

		// Should still have connect types
		g.Expect(content).To(ContainSubstring("connect.Request"))
		g.Expect(content).To(ContainSubstring("connect.Response"))

		// Verify the Client interface section specifically does not contain opts
		// Scan for the Client interface block: between "type XClient interface {" and "}"
		lines := strings.Split(content, "\n")
		inClientIface := false
		for _, line := range lines {
			if strings.Contains(line, "Client interface {") || strings.Contains(line, "Client interface{") {
				inClientIface = true
				continue
			}
			if inClientIface && strings.TrimSpace(line) == "}" {
				inClientIface = false
				continue
			}
			if inClientIface {
				g.Expect(line).ToNot(ContainSubstring("opts"),
					"Client interface method should not contain opts in simple mode: %s", strings.TrimSpace(line))
			}
		}
	}
}

func TestDefaultModeHasCallOption(t *testing.T) {
	g := NewWithT(t)

	srcInfoByPath := loadSourceCodeInfo(g)

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
		g.Expect(err).ToNot(HaveOccurred())
		if fd.Services().Len() == 0 {
			continue
		}
		filesToGenerate = append(filesToGenerate, path)
		collectDeps(fd)
	}
	g.Expect(filesToGenerate).ToNot(BeEmpty())

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
		NewFileGenerator(f, plugin).Generate("mcp", false)
	}

	resp := plugin.Response()
	g.Expect(resp.GetError()).To(BeEmpty())
	g.Expect(resp.File).ToNot(BeEmpty())

	for _, rf := range resp.File {
		content := rf.GetContent()
		// Default mode SHOULD have opts ...grpc.CallOption in Client interfaces
		g.Expect(content).To(ContainSubstring("grpc.CallOption"),
			"default mode should emit grpc.CallOption in file %s", rf.GetName())
		// Default mode SHOULD import grpc (it's used)
		g.Expect(content).To(ContainSubstring(`grpc "google.golang.org/grpc"`),
			"default mode should import grpc in file %s", rf.GetName())
	}
}
