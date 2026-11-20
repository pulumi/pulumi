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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/tests/testutil"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestShutdownCleanupGo(t *testing.T) {
	cleanupDir := t.TempDir()
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "go",
		Dependencies: []string{"github.com/pulumi/pulumi/sdk/v3"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: testutil.TestProviderDir(t)},
		},
		Env:           []string{"CLEANUP_DIR=" + cleanupDir},
		Quick:         true,
		SkipRefresh:   true,
		ExpectFailure: true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			var errors []string
			for _, event := range stack.Events {
				if d := event.DiagnosticEvent; d != nil && d.Severity == "error" {
					errors = append(errors, d.Message)
				}
			}
			require.Contains(t, strings.Join(errors, "\n"), "Create always fails for the FailsOnCreate resource")
			require.FileExists(t, filepath.Join(cleanupDir, "started"))
			finished, err := os.ReadFile(filepath.Join(cleanupDir, "finished"))
			require.NoError(t, err)
			require.Equal(t, "done", string(finished))
		},
	})
}
