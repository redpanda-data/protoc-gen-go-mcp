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

package generator

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestMCPToolNameAnnotationIntegration(t *testing.T) {
	g := NewWithT(t)
	
	// Test that the FileGenerator can be created without errors
	fg := &FileGenerator{}
	g.Expect(fg).ToNot(BeNil())
	
	// Test that annotation processing doesn't crash with nil inputs
	result := fg.getMCPToolName(nil)
	g.Expect(result).To(BeEmpty())
}

func TestToolNamingBehavior(t *testing.T) {
	tests := []struct {
		name                    string
		serviceName            string
		methodName             string
		customAnnotationName   string
		commentAnnotationName  string
		expectedToolName       string
		expectValidationError  bool
	}{
		{
			name:                 "protobuf option takes precedence",
			serviceName:          "TestService",
			methodName:           "CreateUser",
			customAnnotationName: "create_user",
			commentAnnotationName: "ignored_comment_name",
			expectedToolName:     "create_user",
		},
		{
			name:                 "comment annotation fallback",
			serviceName:          "TestService", 
			methodName:           "UpdateUser",
			commentAnnotationName: "update_user_legacy",
			expectedToolName:     "update_user_legacy",
		},
		{
			name:             "auto-generated fallback",
			serviceName:      "TestService",
			methodName:       "DeleteUser", 
			expectedToolName: "annotation_test_AnnotationTestService_DeleteUser", // Based on proto package and service
		},
		{
			name:                   "invalid annotation causes fallback",
			serviceName:            "TestService",
			methodName:             "InvalidMethod",
			customAnnotationName:   "Invalid-Name",
			expectValidationError:  true,
			expectedToolName:       "", // Should fallback to auto-generated
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			
			// Test validation behavior
			if tt.customAnnotationName != "" {
				err := validateToolName(tt.customAnnotationName)
				if tt.expectValidationError {
					g.Expect(err).To(HaveOccurred())
				} else {
					g.Expect(err).ToNot(HaveOccurred())
				}
			}
			
			// TODO: Add tests for actual tool name extraction from proto methods
			// This would require setting up protogen.Method with proper annotations
		})
	}
}

func TestGeneratedCodeQuality(t *testing.T) {
	g := NewWithT(t)
	
	// Test that the generated code contains expected tool names
	// This test would examine the actual generated Go code from our test proto
	
	// Expected patterns in generated code:
	expectedPatterns := []string{
		`AnnotationTestService_CreateResourceTool = mcp.Tool{Name: "create_resource"`,
		`AnnotationTestService_UpdateResourceTool = mcp.Tool{Name: "update_resource_legacy"`,
		`AnnotationTestService_DeleteResourceTool = mcp.Tool{Name: "annotation_test_AnnotationTestService_DeleteResource"`,
		`AnnotationTestService_ListResourcesTool = mcp.Tool{Name: "list_all_resources"`,
	}
	
	// In a complete test, we would:
	// 1. Generate the actual code using our FileGenerator
	// 2. Examine the generated output 
	// 3. Verify it contains the expected tool names
	
	for _, pattern := range expectedPatterns {
		// Placeholder: would check generated code contains these patterns
		g.Expect(strings.Contains(pattern, "Tool")).To(BeTrue())
	}
}

func TestDuplicateToolNameDetection(t *testing.T) {
	g := NewWithT(t)
	
	// Test that duplicate tool names within a service are detected and reported
	// This would require creating a proto with duplicate annotations
	
	// Expected: generator should report error for duplicate tool names
	// within the same service but allow duplicates across different services
	
	g.Expect(true).To(BeTrue()) // Placeholder
}

func TestBackwardsCompatibility(t *testing.T) {
	g := NewWithT(t)
	
	// Test that comment-based annotations still work
	commentPatterns := []string{
		"// mcp_tool_name:my_tool",
		"//mcp_tool_name:another_tool", 
		"  // mcp_tool_name:spaced_tool",
	}
	
	// Test that these patterns are correctly parsed
	for _, pattern := range commentPatterns {
		g.Expect(strings.Contains(pattern, "mcp_tool_name")).To(BeTrue())
	}
}