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
	"context"
	"testing"

	mcpserver "github.com/mark3labs/mcp-go/server"
	. "github.com/onsi/gomega"
	testdata "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata"
	testdatamcp "github.com/redpanda-data/protoc-gen-go-mcp/pkg/testdata/gen/go/testdata/testdatamcp"
	"google.golang.org/protobuf/proto"
)

// Test that edition 2023 proto files work correctly with the MCP generator
func TestEdition2023Compatibility(t *testing.T) {
	g := NewWithT(t)

	// Create MCP server
	mcpServer := mcpserver.NewMCPServer("test-server", "1.0.0")

	// Create test implementation
	srv := &testServerEdition2023{}

	// Verify that the registration function exists and can be called
	// This confirms the code generation worked correctly
	g.Expect(func() {
		testdatamcp.RegisterTestServiceEdition2023Handler(mcpServer, srv)
	}).ToNot(Panic())
}

// Test that edition 2023 field presence works correctly
func TestEdition2023FieldPresence(t *testing.T) {
	g := NewWithT(t)

	// Test that string fields are pointers in edition 2023
	req := &testdata.CreateItemRequestEdition2023{
		Name:        proto.String("test-item"),
		Description: proto.String("test description"),
	}

	g.Expect(req.Name).To(Equal(proto.String("test-item")))
	g.Expect(req.Description).To(Equal(proto.String("test description")))
	g.Expect(req.GetName()).To(Equal("test-item"))
	g.Expect(req.GetDescription()).To(Equal("test description"))

	// Test nil handling
	reqNil := &testdata.CreateItemRequestEdition2023{}
	g.Expect(reqNil.GetName()).To(Equal(""))
	g.Expect(reqNil.GetDescription()).To(Equal(""))
}

// testServerEdition2023 implements the edition 2023 service for testing
type testServerEdition2023 struct{}

func (t *testServerEdition2023) CreateItem(ctx context.Context, in *testdata.CreateItemRequestEdition2023) (*testdata.CreateItemResponseEdition2023, error) {
	return &testdata.CreateItemResponseEdition2023{
		Id: proto.String("item-123-edition2023"),
	}, nil
}

func (t *testServerEdition2023) GetItem(ctx context.Context, in *testdata.GetItemRequestEdition2023) (*testdata.GetItemResponseEdition2023, error) {
	return &testdata.GetItemResponseEdition2023{
		Item: &testdata.ItemEdition2023{
			Id:   proto.String(in.GetId()),
			Name: proto.String("Retrieved item edition 2023"),
		},
	}, nil
}

func (t *testServerEdition2023) ProcessWellKnownTypes(ctx context.Context, in *testdata.ProcessWellKnownTypesRequestEdition2023) (*testdata.ProcessWellKnownTypesResponseEdition2023, error) {
	return &testdata.ProcessWellKnownTypesResponseEdition2023{
		Success: proto.Bool(true),
		Message: proto.String("Processed well-known types edition 2023"),
	}, nil
}
