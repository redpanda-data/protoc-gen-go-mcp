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
package main

import (
	"flag"

	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/generator"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func main() {
	var flagSet flag.FlagSet
	packageSuffix := flagSet.String(
		"package_suffix",
		"mcp",
		"Generate files into a sub-package of the package containing the base .pb.go files using the given suffix. An empty suffix denotes to generate into the same package as the base pb.go files.",
	)
	packagePrefix := flagSet.String(
		"package_prefix",
		"",
		"Prefix to add to generated import paths. Example: 'github.com/example/gen' will transform 'buf.build/gen/go/example' to 'github.com/example/gen/buf.build/gen/go/example'",
	)

	protogen.Options{
		ParamFunc: flagSet.Set,
	}.Run(func(gen *protogen.Plugin) error {
		// Set supported editions and features on the plugin
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_SUPPORTS_EDITIONS)
		gen.SupportedEditionsMinimum = descriptorpb.Edition_EDITION_PROTO2
		gen.SupportedEditionsMaximum = descriptorpb.Edition_EDITION_2023

		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			generator.NewFileGenerator(f, gen, *packagePrefix).Generate(*packageSuffix)
		}
		return nil
	})
}
