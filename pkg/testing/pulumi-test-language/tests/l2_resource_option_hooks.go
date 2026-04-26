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
	"os"
	"path/filepath"

	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-option-hooks"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
		},
		Runs: []TestRun{
			{
				Config: config.Map{
					config.MustMakeKey("l2-resource-option-hooks", "hookTestFile"):    config.NewValue("hook-test.txt"),
					config.MustMakeKey("l2-resource-option-hooks", "hookPreviewFile"): config.NewValue("hook-preview.txt"),
				},
				// During preview, only the hook with onDryRun=true should run.
				AssertPreview: func(l *L, res AssertPreviewArgs) {
					require.NoErrorf(l, res.Err, "expected no error in preview")

					previewFile := filepath.Join(res.ProjectDirectory, "hook-preview.txt_res")
					_, err := os.Stat(previewFile)
					require.NoError(l, err,
						"expected onDryRun hook to have created file during preview at %s", previewFile)

					// Clean up the preview file so it doesn't interfere with the update assertions.
					err = os.Remove(previewFile)
					require.NoError(l, err)

					testFile := filepath.Join(res.ProjectDirectory, "hook-test.txt")
					_, err = os.Stat(testFile)
					require.Error(l, err,
						"expected non-preview hook NOT to have created file during preview at %s", testFile)
				},
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					// The named hook block (createHook) should have created the file.
					testFile := filepath.Join(res.ProjectDirectory, "hook-test.txt")
					_, err := os.Stat(testFile)
					require.NoError(l, err,
						"expected named hook block to have created file at %s", testFile)

					// The onDryRun hook should also run during the actual update.
					previewFile := filepath.Join(res.ProjectDirectory, "hook-preview.txt_res")
					_, err = os.Stat(previewFile)
					require.NoError(l, err,
						"expected onDryRun hook to have created file during update at %s", previewFile)
				},
			},
		},
	}
}
