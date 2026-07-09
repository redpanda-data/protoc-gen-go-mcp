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
	"testing"

	. "github.com/onsi/gomega"
)

func TestShouldRegister_NoFilter(t *testing.T) {
	g := NewWithT(t)

	// Without a filter every tool is registered.
	config := NewConfig()
	g.Expect(config.ShouldRegister("any_tool")).To(BeTrue())
}

func TestWithToolFilter(t *testing.T) {
	g := NewWithT(t)

	config := NewConfig()
	WithToolFilter(func(toolName string) bool {
		return toolName != "blocked_tool"
	})(config)

	g.Expect(config.ShouldRegister("kept_tool")).To(BeTrue())
	g.Expect(config.ShouldRegister("blocked_tool")).To(BeFalse())
}

func TestWithToolFilter_SeesFinalName(t *testing.T) {
	g := NewWithT(t)

	// ShouldRegister is called by generated code with the tool name after
	// ApplyConfig, so a filter always observes the prefixed name.
	config := NewConfig()
	WithNamePrefix("primary")(config)

	var seen []string
	WithToolFilter(func(toolName string) bool {
		seen = append(seen, toolName)
		return true
	})(config)

	tool := ApplyConfig(Tool{Name: "get_item"}, config)
	g.Expect(config.ShouldRegister(tool.Name)).To(BeTrue())
	g.Expect(seen).To(Equal([]string{"primary_get_item"}))
}
