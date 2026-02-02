// Copyright 2016-2025, Pulumi Corporation.
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

package insights

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateESCEnvironmentYAML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		roleARN  string
		contains []string
	}{
		{
			name:    "standard AWS role",
			roleARN: "arn:aws:iam::123456789012:role/pulumi-insights-123456789012",
			contains: []string{
				"values:",
				"aws:",
				"login:",
				"fn::open::aws-login:",
				"oidc:",
				"duration: 1h",
				"roleArn: arn:aws:iam::123456789012:role/pulumi-insights-123456789012",
				"sessionName: pulumi-insights-discovery",
			},
		},
		{
			name:    "GovCloud role",
			roleARN: "arn:aws-us-gov:iam::123456789012:role/pulumi-insights-123456789012",
			contains: []string{
				"roleArn: arn:aws-us-gov:iam::123456789012:role/pulumi-insights-123456789012",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			yaml := generateESCEnvironmentYAML(tt.roleARN)
			yamlStr := string(yaml)

			for _, expected := range tt.contains {
				assert.Contains(t, yamlStr, expected)
			}
		})
	}
}

func TestESCEnvironmentRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		orgName     string
		projectName string
		envName     string
		expected    string
	}{
		{
			name:        "standard reference",
			orgName:     "my-org",
			projectName: "insights",
			envName:     "aws-123456789012",
			expected:    "my-org/insights/aws-123456789012",
		},
		{
			name:        "different org",
			orgName:     "acme-corp",
			projectName: "platform",
			envName:     "prod-aws",
			expected:    "acme-corp/platform/prod-aws",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := escEnvironmentRef(tt.orgName, tt.projectName, tt.envName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseESCEnvironmentRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		ref             string
		defaultOrg      string
		expectedOrg     string
		expectedProject string
		expectedEnv     string
		expectError     bool
	}{
		{
			name:            "two-part reference",
			ref:             "insights/aws-123456789012",
			defaultOrg:      "my-org",
			expectedOrg:     "my-org",
			expectedProject: "insights",
			expectedEnv:     "aws-123456789012",
		},
		{
			name:            "three-part reference",
			ref:             "acme-corp/insights/aws-123456789012",
			defaultOrg:      "my-org",
			expectedOrg:     "acme-corp",
			expectedProject: "insights",
			expectedEnv:     "aws-123456789012",
		},
		{
			name:            "reference with version",
			ref:             "insights/aws-123456789012@v1",
			defaultOrg:      "my-org",
			expectedOrg:     "my-org",
			expectedProject: "insights",
			expectedEnv:     "aws-123456789012",
		},
		{
			name:            "three-part with version",
			ref:             "acme-corp/insights/aws-123456789012@v2",
			defaultOrg:      "my-org",
			expectedOrg:     "acme-corp",
			expectedProject: "insights",
			expectedEnv:     "aws-123456789012",
		},
		{
			name:        "single-part reference (invalid)",
			ref:         "just-one-part",
			defaultOrg:  "my-org",
			expectError: true,
		},
		{
			name:        "four-part reference (invalid)",
			ref:         "too/many/parts/here",
			defaultOrg:  "my-org",
			expectError: true,
		},
		{
			name:        "empty reference (invalid)",
			ref:         "",
			defaultOrg:  "my-org",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			org, project, env, err := parseESCEnvironmentRef(tt.ref, tt.defaultOrg)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid ESC environment reference")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedOrg, org)
				assert.Equal(t, tt.expectedProject, project)
				assert.Equal(t, tt.expectedEnv, env)
			}
		})
	}
}

func TestDefaultESCEnvName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		accountID string
		expected  string
	}{
		{
			name:      "standard account",
			accountID: "123456789012",
			expected:  "aws-123456789012",
		},
		{
			name:      "different account",
			accountID: "987654321098",
			expected:  "aws-987654321098",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := defaultESCEnvName(tt.accountID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateESCEnvironmentYAML_ValidYAML(t *testing.T) {
	t.Parallel()

	roleARN := "arn:aws:iam::123456789012:role/test-role"
	yaml := generateESCEnvironmentYAML(roleARN)
	yamlStr := string(yaml)

	// Verify indentation is consistent (YAML is whitespace-sensitive)
	lines := strings.Split(yamlStr, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			// Check that leading spaces are multiples of 2
			leadingSpaces := len(line) - len(strings.TrimLeft(line, " "))
			assert.Equal(t, 0, leadingSpaces%2, "Line %d has invalid indentation: %q", i, line)
		}
	}

	// Verify no tabs (YAML should use spaces)
	assert.NotContains(t, yamlStr, "\t", "YAML should not contain tabs")
}
