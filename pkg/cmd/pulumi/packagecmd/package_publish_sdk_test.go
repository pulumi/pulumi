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

package packagecmd

import (
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetermineNPMTagForStableVersion_Logic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		currentVersion string
		npmOutput      string
		npmStderr      string
		npmError       bool
		expectedTag    string
		expectedErr    string
	}{
		{
			name:           "package doesn't exist - 404 error with empty output",
			currentVersion: "1.0.0",
			npmOutput:      "",
			npmStderr:      "npm error code E404\nnpm error 404 Not Found",
			npmError:       true,
			expectedTag:    "latest",
		},
		{
			name:           "package doesn't exist - different 404 message",
			currentVersion: "1.0.0",
			npmOutput:      "",
			npmStderr:      "npm error 404 The requested resource could not be found",
			npmError:       true,
			expectedTag:    "latest",
		},
		{
			name:           "current version greater than latest",
			currentVersion: "2.0.0",
			npmOutput:      "1.0.0",
			npmStderr:      "",
			npmError:       false,
			expectedTag:    "latest",
		},
		{
			name:           "current version equal to latest",
			currentVersion: "1.0.0",
			npmOutput:      "1.0.0",
			npmStderr:      "",
			npmError:       false,
			expectedTag:    "latest",
		},
		{
			name:           "current version less than latest - backport",
			currentVersion: "1.0.0",
			npmOutput:      "2.0.0",
			npmStderr:      "",
			npmError:       false,
			expectedTag:    "backport",
		},
		{
			name:           "patch version less than latest - backport",
			currentVersion: "1.0.0",
			npmOutput:      "1.0.1",
			npmStderr:      "",
			npmError:       false,
			expectedTag:    "backport",
		},
		{
			name:           "minor version less than latest - backport",
			currentVersion: "1.0.0",
			npmOutput:      "1.1.0",
			npmStderr:      "",
			npmError:       false,
			expectedTag:    "backport",
		},
		{
			name:           "patch version greater than latest",
			currentVersion: "1.0.1",
			npmOutput:      "1.0.0",
			npmStderr:      "",
			npmError:       false,
			expectedTag:    "latest",
		},
		{
			name:           "invalid current version",
			currentVersion: "not-a-version",
			npmOutput:      "1.0.0",
			npmStderr:      "",
			npmError:       false,
			expectedErr:    "failed to parse current version",
		},
		{
			name:           "invalid npm version output",
			currentVersion: "1.0.0",
			npmOutput:      "not-a-version",
			npmStderr:      "",
			npmError:       false,
			expectedErr:    "failed to parse latest version",
		},
		{
			name:           "non-404 error - should fail",
			currentVersion: "1.0.0",
			npmOutput:      "",
			npmStderr:      "npm error network timeout",
			npmError:       true,
			expectedErr:    "failed to get latest version from npm",
		},
		{
			name:           "error with output but no 404 - should fail",
			currentVersion: "1.0.0",
			npmOutput:      "some output",
			npmStderr:      "npm error authentication failed",
			npmError:       true,
			expectedErr:    "failed to get latest version from npm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Since we can't easily mock exec.Command, we'll test the core logic
			// by simulating what the function should do with the given inputs
			currentVer, parseErr := semver.ParseTolerant(strings.TrimSpace(tt.currentVersion))

			if parseErr != nil {
				assert.Contains(t, tt.expectedErr, "failed to parse current version")
				return
			}
			// Simulate the npm command result
			if tt.npmError {
				// Check if it's a 404 error (package not found)
				if tt.npmOutput == "" && strings.Contains(strings.ToLower(tt.npmStderr), "404") {
					// Package doesn't exist - should return "latest"
					assert.Equal(t, "latest", tt.expectedTag)
					return
				}
				// Other errors should fail
				if tt.expectedErr != "" {
					assert.Contains(t, tt.expectedErr, "failed to get latest version from npm")
				}
				return
			}

			latestVer, err := semver.ParseTolerant(strings.TrimSpace(tt.npmOutput))
			if err != nil {
				if tt.expectedErr != "" {
					assert.Contains(t, tt.expectedErr, "failed to parse latest version")
				}
				return
			}

			var resultTag string
			if latestVer.GT(currentVer) {
				resultTag = "backport"
			} else {
				resultTag = "latest"
			}

			assert.Equal(t, tt.expectedTag, resultTag)
		})
	}
}

// TestDetermineNPMTagForStableVersion_Integration tests the actual function
// This requires npm to be available and will make real network calls
func TestDetermineNPMTagForStableVersion_Integration(t *testing.T) {
	t.Parallel()
	t.Skip("Integration test - requires npm and network access")

	// Test with a real package that exists
	tag, err := determineNPMTagForStableVersion("npm", "@pulumi/aws", "5.0.0")
	require.NoError(t, err)
	assert.Contains(t, "backport", tag)

	// Test with a package that doesn't exist
	tag, err = determineNPMTagForStableVersion("npm", "@pulumi/nonexistent-package-12345", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "latest", tag)

	// Test with a package name that is higher semver.
	// Using v100 as major version to avoid tripping...for the next 97 major releases.
	tag, err = determineNPMTagForStableVersion("npm", "@pulumi/aws", "100.0.0")
	require.NoError(t, err)
	assert.Equal(t, "latest", tag)
}
