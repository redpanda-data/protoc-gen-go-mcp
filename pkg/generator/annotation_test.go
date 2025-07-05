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
	"testing"

	. "github.com/onsi/gomega"
)

func TestValidateToolName(t *testing.T) {
	tests := []struct {
		name          string
		toolName      string
		expectError   bool
		errorContains string
	}{
		// Valid cases
		{
			name:     "valid snake_case",
			toolName: "create_user",
		},
		{
			name:     "valid single word",
			toolName: "create",
		},
		{
			name:     "valid with numbers",
			toolName: "create_user_v2",
		},
		{
			name:     "valid long name",
			toolName: "create_very_long_but_still_valid_tool_name_here",
		},

		// Invalid cases
		{
			name:          "empty name",
			toolName:      "",
			expectError:   true,
			errorContains: "cannot be empty",
		},
		{
			name:          "too long",
			toolName:      "this_is_a_very_long_tool_name_that_exceeds_the_maximum_allowed_length_of_64_characters_by_quite_a_bit",
			expectError:   true,
			errorContains: "too long",
		},
		{
			name:          "starts with number",
			toolName:      "2_create_user",
			expectError:   true,
			errorContains: "must be snake_case",
		},
		{
			name:          "starts with underscore",
			toolName:      "_create_user",
			expectError:   true,
			errorContains: "must be snake_case",
		},
		{
			name:          "ends with underscore",
			toolName:      "create_user_",
			expectError:   true,
			errorContains: "cannot end with underscore",
		},
		{
			name:          "consecutive underscores",
			toolName:      "create__user",
			expectError:   true,
			errorContains: "consecutive underscores",
		},
		{
			name:          "camelCase",
			toolName:      "createUser",
			expectError:   true,
			errorContains: "must be snake_case",
		},
		{
			name:          "PascalCase",
			toolName:      "CreateUser",
			expectError:   true,
			errorContains: "must be snake_case",
		},
		{
			name:          "kebab-case",
			toolName:      "create-user",
			expectError:   true,
			errorContains: "must be snake_case",
		},
		{
			name:          "contains spaces",
			toolName:      "create user",
			expectError:   true,
			errorContains: "must be snake_case",
		},
		{
			name:          "contains special characters",
			toolName:      "create@user",
			expectError:   true,
			errorContains: "must be snake_case",
		},
		{
			name:          "mixed case with valid pattern",
			toolName:      "Create_User",
			expectError:   true,
			errorContains: "must be snake_case",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			err := validateToolName(tt.toolName)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				if tt.errorContains != "" {
					g.Expect(err.Error()).To(ContainSubstring(tt.errorContains))
				}
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func TestGetMCPToolName(t *testing.T) {
	g := NewWithT(t)

	// Create a file generator instance for testing
	fg := &FileGenerator{}

	// Test with nil method (should handle gracefully and return empty string)
	result := fg.getMCPToolName(nil)
	g.Expect(result).To(BeEmpty())
}
