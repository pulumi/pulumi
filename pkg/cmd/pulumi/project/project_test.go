// Copyright 2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package project

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetOrgFromStackName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		stackRef string
		expected string
	}{
		{
			name:     "org/project/stack",
			stackRef: "org/project/stack",
			expected: "org",
		},
		{
			name:     "org/project/stack/with/slashes",
			stackRef: "org/project/stack/with/slashes",
			expected: "org",
		},
		{
			name:     "project/stack-no-org",
			stackRef: "project/stack",
			expected: "",
		},
		{
			name:     "just-stack",
			stackRef: "stack",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := GetOrgFromStackName(tt.stackRef)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewProjectCmd(t *testing.T) {
	t.Parallel()
	cmd := NewProjectCmd()
	assert.Equal(t, "project", cmd.Use)
	assert.Equal(t, "Manage Pulumi projects", cmd.Short)
	assert.Equal(t, 1, len(cmd.Commands()))

	// Check ls command
	lsCmd := cmd.Commands()[0]
	assert.Equal(t, "ls", lsCmd.Use)
	assert.Equal(t, "List your Pulumi projects", lsCmd.Short)
}
