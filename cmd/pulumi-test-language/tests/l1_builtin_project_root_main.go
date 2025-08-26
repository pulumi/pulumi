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

package tests

import (
	"path/filepath"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Test that both the project root directory (rootDirectory), and working directory (cwd) are correctly set when
	// using the 'main' option in the Pulumi.yaml. Root should be where the Pulumi.yaml file is, working directory
	// should be the subdir.
	LanguageTests["l1-builtin-project-root-main"] = LanguageTest{
		Runs: []TestRun{
			{
				Main: "subdir",
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					RequireStackResource(l, err, changes)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := stack.Outputs

					require.Len(l, outputs, 2, "expected 2 outputs")

					comparePath := func(propertyKey string, path string) {
						got, ok := outputs[resource.PropertyKey(propertyKey)]
						if !assert.True(l, ok, "expected property %q", propertyKey) {
							return
						}
						assert.True(l, got.IsString(), "expected property %q to be a string", propertyKey)

						expected, err := filepath.EvalSymlinks(path)
						require.NoError(l, err)
						actual, err := filepath.EvalSymlinks(got.StringValue())
						require.NoError(l, err)
						assert.Equal(l, expected, actual)
					}

					comparePath("rootDirectoryOutput", projectDirectory)
					comparePath("workingDirectoryOutput", filepath.Join(projectDirectory, "subdir"))
				},
			},
		},
	}
}
