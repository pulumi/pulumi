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

package tests

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-docs"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.DocsProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event, sdks map[string]string,
				) {
					// Check the docs sdk doesn't show the "<pulumi ref=", string anywhere it should
					// have been replaced by the appropriate link.
					sdkPath, ok := sdks["docs-25.0.0"]
					if !ok {
						sdkPath = filepath.Join(projectDirectory, "sdks", "docs-25.0.0")
					}

					err = filepath.WalkDir(sdkPath, func(path string, d fs.DirEntry, err error) error {
						if err != nil {
							return err
						}

						if d.IsDir() {
							return nil
						}

						data, err := os.ReadFile(path)
						if err != nil {
							return err
						}
						assert.NotContainsf(l, string(data), "<pulumi ref=\"", "Should not contain unresolved pulumi ref in %s", d.Name())
						return nil
					})
					require.NoError(l, err)

					RequireStackResource(l, err, changes)

					// Check we have the one simple resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:docs")
					res := RequireSingleResource(l, snap.Resources, "docs:index:Resource")

					want := resource.NewPropertyMapFromMap(map[string]any{
						"in": true,
					})
					assert.Equal(l, want, res.Inputs, "expected inputs to be %v", want)
					want = resource.NewPropertyMapFromMap(map[string]any{
						"in":  true,
						"out": false,
						"data": map[string]any{
							"state": "internal data",
						},
					})
					assert.Equal(l, want, res.Outputs, "expected outputs to be %v", want)
				},
			},
		},
	}
}
