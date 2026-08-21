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
	"fmt"

	"github.com/redpanda-data/protoc-gen-go-mcp/pkg/generator"
	"google.golang.org/protobuf/compiler/protogen"
)

func main() {
	var flagSet flag.FlagSet
	packageSuffix := flagSet.String(
		"package_suffix",
		"mcp",
		"Generate files into a sub-package of the package containing the base .pb.go files using the given suffix. An empty suffix denotes to generate into the same package as the base pb.go files.",
	)
	simpleString := flagSet.String(
		"simple",
		"false",
		"Generate client and handler interfaces with simple function signatures. This eliminates the wrapper connect.Request and connect.Response types, instead having functions directly use generated RPC request and responses. Clients and handlers will instead use context.Contexts to propagate information such as headers. Most users will be more familiar with these interfaces than the default.",
	)

	protogen.Options{
		ParamFunc: flagSet.Set,
	}.Run(func(gen *protogen.Plugin) error {
		simple, err := getSimpleBool(*simpleString)
		if err != nil {
			return err
		}
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			generator.NewFileGenerator(f, gen).Generate(*packageSuffix, simple)
		}
		return nil
	})
}

// "simple" is a bool, but we want to support just setting "simple" without needing to set "simple=true"
// We do this via making the flag a string, and then parsing manually here.
func getSimpleBool(simpleString string) (bool, error) {
	switch simpleString {
	case "", "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf(`unknown value for option "simple" (must be one of "", "true", "false"): %q`, simpleString)
	}
}
