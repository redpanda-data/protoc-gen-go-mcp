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
package runtime

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"

	examplev1 "github.com/redpanda-data/protoc-gen-go-mcp/example-openai-compat/gen/go/proto/example/v1"
)

func TestFix(t *testing.T) {
	g := NewWithT(t)
	in := `{
  "nested": {
    "labels": [
      {
        "key": "my-key",
        "value": "my-value"
      }
    ]
  }
}`
	var inMap map[string]any
	err := json.Unmarshal([]byte(in), &inMap)
	g.Expect(err).ToNot(HaveOccurred())

	FixOpenAI(new(examplev1.CreateExampleRequest).ProtoReflect().Descriptor(), inMap)

	jzon, err := json.Marshal(inMap)
	g.Expect(err).ToNot(HaveOccurred())
	expected := `{"nested":{"labels":{"my-key":"my-value"}}}`
	g.Expect(jzon).To(MatchJSON([]byte(expected)))
}
