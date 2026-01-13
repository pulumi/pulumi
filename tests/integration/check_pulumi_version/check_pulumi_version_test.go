// Copyright 2026, Pulumi Corporation.
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

package ints

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/stretchr/testify/require"
)

// Test the CheckPulumiVersion RPC being called from our SDKs
func TestCheckPulumiVersionSDK(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		runtime       string
		dependencies  []string
		errorContains string
	}{
		{
			runtime: "python",
			dependencies: []string{
				filepath.Join("..", "..", "..", "sdk", "python"),
			},
			errorContains: "__main__.py\", line 3", // We can see where the exception originated
		},
		{
			runtime:       "nodejs",
			dependencies:  []string{"@pulumi/pulumi"},
			errorContains: "index.ts:4",
		},
		{
			runtime:       "go",
			dependencies:  []string{"github.com/pulumi/pulumi/sdk/v3"},
			errorContains: "", // no stack traces in Go!
		},
	}
	//nolint:paralleltest // ProgramTest calls t.Parallel()
	for _, tc := range testCases {
		t.Run(tc.runtime, func(t *testing.T) {
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Dir:           tc.runtime,
				Dependencies:  tc.dependencies,
				Quick:         true,
				ExpectFailure: true,
				ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
					foundError := false
					for _, event := range stack.Events {
						if event.DiagnosticEvent != nil && event.DiagnosticEvent.Severity == "error" {
							require.Regexp(t, "Pulumi CLI version .* does not satisfy the version range \"=3\\.1\\.2\"",
								event.DiagnosticEvent.Message)
							foundError = true
							if tc.errorContains != "" {
								require.Contains(t, event.DiagnosticEvent.Message, tc.errorContains)
							}
						}
					}
					b, err := json.Marshal(stack.Events)
					require.NoError(t, err)
					require.True(t, foundError, "expected to find an error in the stack events, got: %s", b)
				},
			})
		})
	}
}
