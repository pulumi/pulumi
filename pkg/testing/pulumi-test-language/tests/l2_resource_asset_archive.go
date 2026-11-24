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

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-asset-archive"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.AssetArchiveProvider{} },
		},
		Runs: []TestRun{
			{
				Main: "subdir",
				Assert: func(l *L, res AssertArgs) {
					projectDirectory := res.ProjectDirectory
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					// Check we have the asset, archive, and folder resources in the snapshot, the provider and the stack.
					require.Len(l, snap.Resources, 8, "expected 8 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:asset-archive")

					// We don't know what order the resources will be in so we map by name
					resources := map[string]*pkgresource.State{}
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

					remotearc, ok := resources["remotearc"]
					require.True(l, ok, "expected remote archive resource")
					assert.Equal(l, "asset-archive:index:ArchiveResource", remotearc.Type.String(), "expected archive resource")

					main := filepath.Join(projectDirectory, "subdir")

					assetValue, err := resource.NewPathAssetWithWD("../test.txt", main)
					require.NoError(l, err)
					assert.Equal(l, "982d9e3eb996f559e633f4d194def3761d909f5a3b647d1a851fead67c32c9d1", assetValue.Hash)

					want := property.NewMap(map[string]property.Value{
						"value": property.New(assetValue),
					})

					assert.Equal(l, want, resource.FromResourcePropertyMap(asset.Inputs),
						"expected inputs to be {value: %v}", assetValue)
					assert.Equal(l, asset.Inputs, asset.Outputs, "expected inputs and outputs to match")

					archiveValue, err := resource.NewPathArchiveWithWD("../archive.tar", main)
					require.NoError(l, err)
					assert.Equal(l, "2eee410fe85d360552a8c21238d67d43f4b64e60288914f893b67165e8ebfbcf", archiveValue.Hash)

					want = property.NewMap(map[string]property.Value{
						"value": property.New(archiveValue),
					})

					assert.Equal(l, want, resource.FromResourcePropertyMap(archive.Inputs),
						"expected inputs to be {value: %v}", archiveValue)
					assert.Equal(l, archive.Inputs, archive.Outputs, "expected inputs and outputs to match")

					folderValue, err := resource.NewPathArchiveWithWD("../folder", main)
					require.NoError(l, err)
					assert.Equal(l, "25df47ed6b3c8e07479e5d9c908eff93d624ec693b6aa7559a9bcb084db70774", folderValue.Hash)

					want = property.NewMap(map[string]property.Value{
						"value": property.New(folderValue),
					})

					assert.Equal(l, want, resource.FromResourcePropertyMap(folder.Inputs),
						"expected inputs to be {value: %v}", folderValue)
					assert.Equal(l, folder.Inputs, folder.Outputs, "expected inputs and outputs to match")

					stringAsset, err := resource.NewTextAsset("file contents")
					require.NoError(l, err)

					assarcValue, err := resource.NewAssetArchiveWithWD(map[string]any{
						"string":  stringAsset,
						"file":    assetValue,
						"folder":  folderValue,
						"archive": archiveValue,
					}, main)
					require.NoError(l, err)

					want = property.NewMap(map[string]property.Value{
						"value": property.New(assarcValue),
					})

					assert.Equal(l, want, resource.FromResourcePropertyMap(assarc.Inputs),
						"expected inputs to be {value: %v}", assarcValue)
					assert.Equal(l, assarc.Inputs, assarc.Outputs, "expected inputs and outputs to match")

					remoteassValue, err := resource.NewURIAsset(
						"https://raw.githubusercontent.com/pulumi/pulumi/7b0eb7fb10694da2f31c0d15edf671df843e0d4c" +
							"/cmd/pulumi-test-language/tests/testdata/l2-resource-asset-archive/test.txt",
					)
					require.NoError(l, err)

					want = property.NewMap(map[string]property.Value{
						"value": property.New(remoteassValue),
					})

					assert.Equal(l, want, resource.FromResourcePropertyMap(remoteass.Inputs),
						"expected inputs to be {value: %v}", remoteassValue)
					assert.Equal(l, remoteass.Inputs, remoteass.Outputs, "expected inputs and outputs to match")
					bs, err := remoteassValue.Bytes()
					require.NoError(l, err)
					assert.Equal(l, "text", string(bs))
					assert.Equal(l, "982d9e3eb996f559e633f4d194def3761d909f5a3b647d1a851fead67c32c9d1", remoteassValue.Hash)

					remotearcValue, err := resource.NewURIArchive(
						"https://raw.githubusercontent.com/pulumi/pulumi/7b0eb7fb10694da2f31c0d15edf671df843e0d4c" +
							"/cmd/pulumi-test-language/tests/testdata/l2-resource-asset-archive/archive.tar",
					)
					require.NoError(l, err)

					assert.Equal(l,
						property.NewMap(map[string]property.Value{"value": property.New(remotearcValue)}),
						resource.FromResourcePropertyMap(remotearc.Inputs),
						"expected inputs to be {value: %v}", remotearcValue)
					assert.Equal(l, remotearc.Inputs, remotearc.Outputs, "expected inputs and outputs to match")

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					stackOutputs := resource.FromResourcePropertyMap(stack.Outputs)
					AssertPropertyMapMember(l, stackOutputs, "assetOutput",
						property.New(assetValue))
					AssertPropertyMapMember(l, stackOutputs, "archiveOutput",
						property.New(archiveValue))
					AssertPropertyMapMember(l, stackOutputs, "assetList",
						property.New([]property.Value{
							property.New(assetValue),
							property.New(stringAsset),
						}))
					AssertPropertyMapMember(l, stackOutputs, "archiveList",
						property.New([]property.Value{
							property.New(archiveValue),
							property.New(folderValue),
						}))
					AssertPropertyMapMember(l, stackOutputs, "assetMap",
						property.New(map[string]property.Value{
							"file":   property.New(assetValue),
							"string": property.New(stringAsset),
						}))
					AssertPropertyMapMember(l, stackOutputs, "archiveMap",
						property.New(map[string]property.Value{
							"tar":    property.New(archiveValue),
							"folder": property.New(folderValue),
						}))
				},
			},
		},
	}
}
