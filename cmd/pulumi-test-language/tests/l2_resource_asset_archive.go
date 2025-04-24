// Copyright 2024, Pulumi Corporation.
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

	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-asset-archive"] = LanguageTest{
		Providers: []plugin.Provider{&providers.AssetArchiveProvider{}},
		Runs: []TestRun{
			{
				Main: "subdir",
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					// Check we have the the asset, archive, and folder resources in the snapshot, the provider and the stack.
					require.Len(l, snap.Resources, 7, "expected 7 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:asset-archive")

					// We don't know what order the resources will be in so we map by name
					resources := map[string]*resource.State{}
					for _, r := range snap.Resources[2:] {
						resources[r.URN.Name()] = r
					}

					asset, ok := resources["ass"]
					require.True(l, ok, "expected asset resource")
					assert.Equal(l, "asset-archive:index:AssetResource", asset.Type.String(), "expected asset resource")

					archive, ok := resources["arc"]
					require.True(l, ok, "expected archive resource")
					assert.Equal(l, "asset-archive:index:ArchiveResource", archive.Type.String(), "expected archive resource")

					folder, ok := resources["dir"]
					require.True(l, ok, "expected folder resource")
					assert.Equal(l, "asset-archive:index:ArchiveResource", folder.Type.String(), "expected archive resource")

					assarc, ok := resources["assarc"]
					require.True(l, ok, "expected asset archive resource")
					assert.Equal(l, "asset-archive:index:ArchiveResource", assarc.Type.String(), "expected archive resource")

					remoteass, ok := resources["remoteass"]
					require.True(l, ok, "expected remote asset resource")
					assert.Equal(l, "asset-archive:index:AssetResource", remoteass.Type.String(), "expected asset resource")

					main := filepath.Join(projectDirectory, "subdir")

					assetValue, err := resource.NewPathAssetWithWD("../test.txt", main)
					require.NoError(l, err)
					assert.Equal(l, "982d9e3eb996f559e633f4d194def3761d909f5a3b647d1a851fead67c32c9d1", assetValue.Hash)

					want := resource.NewPropertyMapFromMap(map[string]any{
						"value": assetValue,
					})

					assert.Equal(l, want, asset.Inputs, "expected inputs to be {value: %v}", assetValue)
					assert.Equal(l, asset.Inputs, asset.Outputs, "expected inputs and outputs to match")

					archiveValue, err := resource.NewPathArchiveWithWD("../archive.tar", main)
					require.NoError(l, err)
					assert.Equal(l, "2eee410fe85d360552a8c21238d67d43f4b64e60288914f893b67165e8ebfbcf", archiveValue.Hash)

					want = resource.NewPropertyMapFromMap(map[string]any{
						"value": archiveValue,
					})

					assert.Equal(l, want, archive.Inputs, "expected inputs to be {value: %v}", archiveValue)
					assert.Equal(l, archive.Inputs, archive.Outputs, "expected inputs and outputs to match")

					folderValue, err := resource.NewPathArchiveWithWD("../folder", main)
					require.NoError(l, err)
					assert.Equal(l, "25df47ed6b3c8e07479e5d9c908eff93d624ec693b6aa7559a9bcb084db70774", folderValue.Hash)

					want = resource.NewPropertyMapFromMap(map[string]any{
						"value": folderValue,
					})

					assert.Equal(l, want, folder.Inputs, "expected inputs to be {value: %v}", folderValue)
					assert.Equal(l, folder.Inputs, folder.Outputs, "expected inputs and outputs to match")

					stringAsset, err := resource.NewTextAsset("file contents")
					require.NoError(l, err)

					assarcValue, err := resource.NewAssetArchiveWithWD(map[string]interface{}{
						"string":  stringAsset,
						"file":    assetValue,
						"folder":  folderValue,
						"archive": archiveValue,
					}, main)
					require.NoError(l, err)

					want = resource.NewPropertyMapFromMap(map[string]any{
						"value": assarcValue,
					})

					assert.Equal(l, want, assarc.Inputs, "expected inputs to be {value: %v}", assarcValue)
					assert.Equal(l, assarc.Inputs, assarc.Outputs, "expected inputs and outputs to match")

					remoteassValue, err := resource.NewURIAsset(
						"https://raw.githubusercontent.com/pulumi/pulumi/7b0eb7fb10694da2f31c0d15edf671df843e0d4c" +
							"/cmd/pulumi-test-language/tests/testdata/l2-resource-asset-archive/test.txt",
					)
					require.NoError(l, err)

					want = resource.NewPropertyMapFromMap(map[string]any{
						"value": remoteassValue,
					})

					assert.Equal(l, want, remoteass.Inputs, "expected inputs to be {value: %v}", remoteassValue)
					assert.Equal(l, remoteass.Inputs, remoteass.Outputs, "expected inputs and outputs to match")
					bs, err := remoteassValue.Bytes()
					require.NoError(l, err)
					assert.Equal(l, "text", string(bs))
					assert.Equal(l, "982d9e3eb996f559e633f4d194def3761d909f5a3b647d1a851fead67c32c9d1", remoteassValue.Hash)
				},
			},
		},
	}
}
