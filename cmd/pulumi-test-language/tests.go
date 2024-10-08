// Copyright 2016-2024, Pulumi Corporation.
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

package main

import (
	"embed"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/deepcopy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testRun struct {
	config config.Map
	// This can be used to set a main value for the test.
	main string
	// TODO: This should just return "string", if == "" then ok, else fail
	assert func(*L, string, error, *deploy.Snapshot, display.ResourceChanges)
	// updateOptions can be used to set the update options for the engine.
	updateOptions engine.UpdateOptions
}

type languageTest struct {
	// TODO: This should be a function so we don't have to load all providers in memory all the time.
	providers []plugin.Provider

	// stackReferences specifies other stack data that this test depends on.
	stackReferences map[string]resource.PropertyMap

	// runs is a list of test runs to execute.
	runs []testRun
}

// lorem is a long string used for testing large string values.
const lorem string = "Lorem ipsum dolor sit amet, consectetur adipiscing elit," +
	" sed do eiusmod tempor incididunt ut labore et dolore magna aliqua." +
	" Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat." +
	" Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur." +
	" Excepteur sint occaecat cupidatat non proident," +
	" sunt in culpa qui officia deserunt mollit anim id est laborum."

//go:embed testdata
var languageTestdata embed.FS

var languageTests = map[string]languageTest{
	// ==========
	// INTERNAL
	// ==========
	"internal-bad-schema": {
		providers: []plugin.Provider{&providers.BadProvider{}},
		runs:      []testRun{{}},
	},
	// ==========
	// L1 (Tests not using providers)
	// ==========
	"l1-empty": {
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					assertStackResource(l, err, changes)
				},
			},
		},
	},
	"l1-output-bool": {
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)

					// Check we have two outputs in the stack for true and false
					require.NotEmpty(l, snap.Resources, "expected at least 1 resource")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")

					outputs := stack.Outputs

					assertPropertyMapMember(l, outputs, "output_true", resource.NewBoolProperty(true))
					assertPropertyMapMember(l, outputs, "output_false", resource.NewBoolProperty(false))
				},
			},
		},
	},
	"l1-output-number": {
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)

					require.NotEmpty(l, snap.Resources, "expected at least 1 resource")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")

					outputs := stack.Outputs

					assert.Len(l, outputs, 6, "expected 6 outputs")
					assertPropertyMapMember(l, outputs, "zero", resource.NewNumberProperty(0))
					assertPropertyMapMember(l, outputs, "one", resource.NewNumberProperty(1))
					assertPropertyMapMember(l, outputs, "e", resource.NewNumberProperty(2.718))
					assertPropertyMapMember(l, outputs, "minInt32", resource.NewNumberProperty(math.MinInt32))
					assertPropertyMapMember(l, outputs, "max", resource.NewNumberProperty(math.MaxFloat64))
					assertPropertyMapMember(l, outputs, "min", resource.NewNumberProperty(math.SmallestNonzeroFloat64))
				},
			},
		},
	},
	"l1-output-string": {
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)

					require.NotEmpty(l, snap.Resources, "expected at least 1 resource")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")

					outputs := stack.Outputs

					assert.Len(l, outputs, 6, "expected 6 outputs")
					assertPropertyMapMember(l, outputs, "empty", resource.NewStringProperty(""))
					assertPropertyMapMember(l, outputs, "small", resource.NewStringProperty("Hello world!"))
					assertPropertyMapMember(l, outputs, "emoji", resource.NewStringProperty("👋 \"Hello \U0001019b!\" 😊"))
					assertPropertyMapMember(l, outputs, "escape", resource.NewStringProperty(
						"Some ${common} \"characters\" 'that' need escaping: "+
							"\\ (backslash), \t (tab), \u001b (escape), \u0007 (bell), \u0000 (null), \U000e0021 (tag space)"))
					assertPropertyMapMember(l, outputs, "escapeNewline", resource.NewStringProperty(
						"Some ${common} \"characters\" 'that' need escaping: "+
							"\\ (backslash), \n (newline), \t (tab), \u001b (escape), \u0007 (bell), \u0000 (null), \U000e0021 (tag space)"))

					large := strings.Repeat(lorem+"\n", 150)
					assertPropertyMapMember(l, outputs, "large", resource.NewStringProperty(large))
				},
			},
		},
	},
	"l1-output-array": {
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)

					require.NotEmpty(l, snap.Resources, "expected at least 1 resource")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")

					outputs := stack.Outputs

					assert.Len(l, outputs, 5, "expected 5 outputs")
					assertPropertyMapMember(l, outputs, "empty", resource.NewArrayProperty([]resource.PropertyValue{}))
					assertPropertyMapMember(l, outputs, "small", resource.NewArrayProperty([]resource.PropertyValue{
						resource.NewStringProperty("Hello"),
						resource.NewStringProperty("World"),
					}))
					assertPropertyMapMember(l, outputs, "numbers", resource.NewArrayProperty([]resource.PropertyValue{
						resource.NewNumberProperty(0), resource.NewNumberProperty(1), resource.NewNumberProperty(2),
						resource.NewNumberProperty(3), resource.NewNumberProperty(4), resource.NewNumberProperty(5),
					}))
					assertPropertyMapMember(l, outputs, "nested", resource.NewArrayProperty([]resource.PropertyValue{
						resource.NewArrayProperty([]resource.PropertyValue{
							resource.NewNumberProperty(1), resource.NewNumberProperty(2), resource.NewNumberProperty(3),
						}),
						resource.NewArrayProperty([]resource.PropertyValue{
							resource.NewNumberProperty(4), resource.NewNumberProperty(5), resource.NewNumberProperty(6),
						}),
						resource.NewArrayProperty([]resource.PropertyValue{
							resource.NewNumberProperty(7), resource.NewNumberProperty(8), resource.NewNumberProperty(9),
						}),
					}))

					large := []resource.PropertyValue{}
					for i := 0; i < 150; i++ {
						large = append(large, resource.NewStringProperty(lorem))
					}
					assertPropertyMapMember(l, outputs, "large", resource.NewArrayProperty(large))
				},
			},
		},
	},
	"l1-main": {
		runs: []testRun{
			{
				main: "subdir",
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)

					// Check we have an output in the stack for true
					require.NotEmpty(l, snap.Resources, "expected at least 1 resource")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")

					outputs := stack.Outputs

					assertPropertyMapMember(l, outputs, "output_true", resource.NewBoolProperty(true))
				},
			},
		},
	},
	"l1-stack-reference": {
		stackReferences: map[string]resource.PropertyMap{
			"organization/other/dev": {
				"plain":  resource.NewStringProperty("plain"),
				"secret": resource.MakeSecret(resource.NewStringProperty("secret")),
			},
		},
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 3, "expected at least 3 resources")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")
					prov := snap.Resources[1]
					require.Equal(l, "pulumi:providers:pulumi", prov.Type.String(), "expected a default pulumi provider resource")
					ref := snap.Resources[2]
					require.Equal(l, "pulumi:pulumi:StackReference", ref.Type.String(), "expected a stack reference resource")

					outputs := stack.Outputs

					assert.Len(l, outputs, 2, "expected 2 outputs")
					assertPropertyMapMember(l, outputs, "plain", resource.NewStringProperty("plain"))
					assertPropertyMapMember(l, outputs, "secret", resource.MakeSecret(resource.NewStringProperty("secret")))
				},
			},
		},
	},
	"l1-builtin-info": {
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)

					require.NotEmpty(l, snap.Resources, "expected at least 1 resource")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")

					outputs := stack.Outputs

					assert.Len(l, outputs, 3, "expected 3 outputs")
					assertPropertyMapMember(l, outputs, "stackOutput", resource.NewStringProperty("test"))
					assertPropertyMapMember(l, outputs, "projectOutput", resource.NewStringProperty("l1-builtin-info"))
					assertPropertyMapMember(l, outputs, "organizationOutput", resource.NewStringProperty("organization"))
				},
			},
		},
	},
	"l1-output-map": {
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)

					require.NotEmpty(l, snap.Resources, "expected at least 1 resource")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")

					outputs := stack.Outputs

					assert.Len(l, outputs, 4, "expected 4 outputs")
					assertPropertyMapMember(l, outputs, "empty", resource.NewObjectProperty(resource.PropertyMap{}))
					assertPropertyMapMember(l, outputs, "strings", resource.NewObjectProperty(resource.PropertyMap{
						"greeting": resource.NewStringProperty("Hello, world!"),
						"farewell": resource.NewStringProperty("Goodbye, world!"),
					}))
					assertPropertyMapMember(l, outputs, "numbers", resource.NewObjectProperty(resource.PropertyMap{
						"1": resource.NewNumberProperty(1),
						"2": resource.NewNumberProperty(2),
					}))
					assertPropertyMapMember(l, outputs, "keys", resource.NewObjectProperty(resource.PropertyMap{
						"my.key": resource.NewNumberProperty(1),
						"my-key": resource.NewNumberProperty(2),
						"my_key": resource.NewNumberProperty(3),
						"MY_KEY": resource.NewNumberProperty(4),
						"mykey":  resource.NewNumberProperty(5),
						"MYKEY":  resource.NewNumberProperty(6),
					}))
				},
			},
		},
	},
	// ==========
	// L2 (Tests using providers)
	// ==========
	"l2-resource-simple": {
		providers: []plugin.Provider{&providers.SimpleProvider{}},
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)

					// Check we have the one simple resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:simple", provider.Type.String(), "expected simple provider")

					simple := snap.Resources[2]
					assert.Equal(l, "simple:index:Resource", simple.Type.String(), "expected simple resource")

					want := resource.NewPropertyMapFromMap(map[string]any{"value": true})
					assert.Equal(l, want, simple.Inputs, "expected inputs to be {value: true}")
					assert.Equal(l, simple.Inputs, simple.Outputs, "expected inputs and outputs to match")
				},
			},
		},
	},
	"l2-resource-primitives": {
		providers: []plugin.Provider{&providers.PrimitiveProvider{}},
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)

					// Check we have the one simple resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:primitive", provider.Type.String(), "expected primitive provider")

					simple := snap.Resources[2]
					assert.Equal(l, "primitive:index:Resource", simple.Type.String(), "expected primitive resource")

					want := resource.NewPropertyMapFromMap(map[string]any{
						"boolean":     true,
						"float":       3.14,
						"integer":     42,
						"string":      "hello",
						"numberArray": []interface{}{-1.0, 0.0, 1.0},
						"booleanMap":  map[string]interface{}{"t": true, "f": false},
					})
					assert.Equal(l, want, simple.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, simple.Inputs, simple.Outputs, "expected inputs and outputs to match")
				},
			},
		},
	},
	"l2-resource-alpha": {
		providers: []plugin.Provider{&providers.AlphaProvider{}},
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)

					// Check we have the one simple resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:alpha", provider.Type.String(), "expected alpha provider")

					simple := snap.Resources[2]
					assert.Equal(l, "alpha:index:Resource", simple.Type.String(), "expected alpha resource")

					want := resource.NewPropertyMapFromMap(map[string]any{"value": true})
					assert.Equal(l, want, simple.Inputs, "expected inputs to be {value: true}")
					assert.Equal(l, simple.Inputs, simple.Outputs, "expected inputs and outputs to match")
				},
			},
		},
	},
	"l2-explicit-provider": {
		providers: []plugin.Provider{&providers.SimpleProvider{}},
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)

					// Check we have the one simple resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:simple", provider.Type.String(), "expected simple provider")
					assert.Equal(l, "prov", provider.URN.Name(), "expected explicit provider resource")

					simple := snap.Resources[2]
					assert.Equal(l, "simple:index:Resource", simple.Type.String(), "expected simple resource")
					assert.Equal(l, string(provider.URN)+"::"+string(provider.ID), simple.Provider)

					want := resource.NewPropertyMapFromMap(map[string]any{"value": true})
					assert.Equal(l, want, simple.Inputs, "expected inputs to be {value: true}")
					assert.Equal(l, simple.Inputs, simple.Outputs, "expected inputs and outputs to match")
				},
			},
		},
	},
	"l2-resource-asset-archive": {
		providers: []plugin.Provider{&providers.AssetArchiveProvider{}},
		runs: []testRun{
			{
				main: "subdir",
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)

					// Check we have the the asset, archive, and folder resources in the snapshot, the provider and the stack.
					require.Len(l, snap.Resources, 7, "expected 7 resources in snapshot")

					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:asset-archive", provider.Type.String(), "expected asset-archive provider")

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
						"https://raw.githubusercontent.com/pulumi/pulumi/master" +
							"/cmd/pulumi-test-language/testdata/l2-resource-asset-archive/test.txt",
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
	},
	"l2-engine-update-options": {
		providers: []plugin.Provider{&providers.SimpleProvider{}},
		runs: []testRun{
			{
				updateOptions: engine.UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{
						"**target**",
					}),
				},
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)
					require.Len(l, snap.Resources, 3, "expected 2 resource in snapshot")

					// Check that we have the target in the snapshot, but not the other resource.
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")
					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:simple", provider.Type.String(), "expected simple provider")
					target := snap.Resources[2]
					require.Equal(l, "simple:index:Resource", target.Type.String(), "expected simple resource")
					require.Equal(l, "target", target.URN.Name(), "expected target resource")
				},
			},
		},
	},
	"l2-destroy": {
		providers: []plugin.Provider{&providers.SimpleProvider{}},
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					// check that both expected resources are in the snapshot
					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:simple", provider.Type.String(), "expected simple provider")

					// Make sure we can assert the resource names in a consistent order
					sort.Slice(snap.Resources[2:4], func(i, j int) bool {
						i = i + 2
						j = j + 2
						return snap.Resources[i].URN.Name() < snap.Resources[j].URN.Name()
					})

					simple := snap.Resources[2]
					assert.Equal(l, "simple:index:Resource", simple.Type.String(), "expected simple resource")
					assert.Equal(l, "aresource", simple.URN.Name(), "expected aresource resource")
					simple2 := snap.Resources[3]
					assert.Equal(l, "simple:index:Resource", simple2.Type.String(), "expected simple resource")
					assert.Equal(l, "other", simple2.URN.Name(), "expected other resource")
				},
			},
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					assert.Equal(l, 1, changes[deploy.OpDelete], "expected a delete operation")
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					// No need to sort here, since we have only resources that depend on each other in a chain.
					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:simple", provider.Type.String(), "expected simple provider")
					// check that only the expected resource is left in the snapshot
					simple := snap.Resources[2]
					assert.Equal(l, "simple:index:Resource", simple.Type.String(), "expected simple resource")
					assert.Equal(l, "aresource", simple.URN.Name(), "expected aresource resource")
				},
			},
		},
	},
	"l2-target-up-with-new-dependency": {
		providers: []plugin.Provider{&providers.SimpleProvider{}},
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")
					err = snap.VerifyIntegrity()
					require.NoError(l, err, "expected snapshot to be valid")

					sort.Slice(snap.Resources, func(i, j int) bool {
						return snap.Resources[i].URN.Name() < snap.Resources[j].URN.Name()
					})

					target := snap.Resources[2]
					require.Equal(l, "simple:index:Resource", target.Type.String(), "expected simple resource")
					require.Equal(l, "targetOnly", target.URN.Name(), "expected target resource")
					unrelated := snap.Resources[3]
					require.Equal(l, "simple:index:Resource", unrelated.Type.String(), "expected simple resource")
					require.Equal(l, "unrelated", unrelated.URN.Name(), "expected target resource")
					require.Equal(l, 0, len(unrelated.Dependencies), "expected no dependencies")
				},
			},
			{
				updateOptions: engine.UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{
						"**targetOnly**",
					}),
				},
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					sort.Slice(snap.Resources, func(i, j int) bool {
						return snap.Resources[i].URN.Name() < snap.Resources[j].URN.Name()
					})

					target := snap.Resources[2]
					require.Equal(l, "simple:index:Resource", target.Type.String(), "expected simple resource")
					require.Equal(l, "targetOnly", target.URN.Name(), "expected target resource")
					unrelated := snap.Resources[3]
					require.Equal(l, "simple:index:Resource", unrelated.Type.String(), "expected simple resource")
					require.Equal(l, "unrelated", unrelated.URN.Name(), "expected target resource")
					require.Equal(l, 0, len(unrelated.Dependencies), "expected still no dependencies")
				},
			},
		},
	},
	"l2-failed-create-continue-on-error": {
		providers: []plugin.Provider{&providers.SimpleProvider{}, &providers.FailOnCreateProvider{}},
		runs: []testRun{
			{
				updateOptions: engine.UpdateOptions{
					ContinueOnError: true,
				},
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					require.True(l, result.IsBail(err), "expected a bail result")
					require.Equal(l, 1, len(changes), "expected 1 StepOp")
					require.Equal(l, 2, changes[deploy.OpCreate], "expected 2 Creates")
					require.NotNil(l, snap, "expected snapshot to be non-nil")
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot") // 1 stack, 2 providers, 1 resource
					require.NoError(l, snap.VerifyIntegrity(), "expected snapshot to be valid")

					sort.Slice(snap.Resources, func(i, j int) bool {
						return snap.Resources[i].URN.Name() < snap.Resources[j].URN.Name()
					})

					require.Equal(l, "independent", snap.Resources[2].URN.Name(), "expected independent resource")
				},
			},
		},
	},
	"l2-large-string": {
		providers: []plugin.Provider{&providers.LargeProvider{}},
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					// Check that the large string is in the snapshot
					largeString := resource.NewStringProperty(strings.Repeat("hello world", 9532509))
					large := snap.Resources[2]
					require.Equal(l, "large:index:String", large.Type.String(), "expected large string resource")
					require.Equal(l,
						resource.NewStringProperty("hello world"),
						large.Inputs["value"],
					)
					require.Equal(l,
						largeString,
						large.Outputs["value"],
					)

					// Check the stack output value is as well
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")
					require.Equal(l, largeString, stack.Outputs["output"], "expected large string stack output")
				},
			},
		},
	},
	"l2-resource-config": {
		providers: []plugin.Provider{&providers.ConfigProvider{}},
		runs: []testRun{
			{
				config: config.Map{
					config.MustParseKey("config:name"): config.NewValue("hello"),
				},
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					explicitProvider := snap.Resources[1]
					require.Equal(l, "pulumi:providers:config", explicitProvider.Type.String(), "expected explicit provider resource")
					expectedOutputs := resource.NewPropertyMapFromMap(map[string]interface{}{
						"name":              "my config",
						"pluginDownloadURL": "not the same as the pulumi resource option",
						"version":           "9.0.0",
					})
					expectedInputs := deepcopy.Copy(expectedOutputs).(resource.PropertyMap)
					// inputs should also have the __internal key
					expectedInputs[resource.PropertyKey("__internal")] = resource.NewObjectProperty(
						resource.NewPropertyMapFromMap(map[string]interface{}{
							"pluginDownloadURL": "http://example.com",
						}))
					require.Equal(l, expectedInputs, explicitProvider.Inputs)
					require.Equal(l, expectedOutputs, explicitProvider.Outputs)

					defaultProvider := snap.Resources[2]
					require.Equal(l, "pulumi:providers:config", defaultProvider.Type.String(), "expected default provider resource")
					require.Equal(l, "default_9_0_0_http_/example.com", defaultProvider.URN.Name())
					expectedOutputs = resource.NewPropertyMapFromMap(map[string]interface{}{
						"version": "9.0.0",
						"name":    "hello",
					})
					expectedInputs = deepcopy.Copy(expectedOutputs).(resource.PropertyMap)
					// inputs should also have the __internal key
					expectedInputs[resource.PropertyKey("__internal")] = resource.NewObjectProperty(
						resource.NewPropertyMapFromMap(map[string]interface{}{
							"pluginDownloadURL": "http://example.com",
						}))
					require.Equal(l, expectedInputs, defaultProvider.Inputs)
					require.Equal(l, expectedOutputs, defaultProvider.Outputs)
				},
			},
		},
	},
	"l2-provider-grpc-config": {
		providers: []plugin.Provider{&providers.ConfigGrpcProvider{}},
		runs: (func() []testRun {
			// Find ConfigGetter resource by name and extract the captured config.
			config := func(l *L, snap *deploy.Snapshot, resourceName string) string {
				for _, r := range snap.Resources {
					if r.URN.Name() != resourceName {
						continue
					}
					require.Equal(l, "testconfigprovider:index:ConfigGetter", string(r.Type))
					configOut, gotConfig := r.Outputs["config"]
					require.Truef(l, gotConfig, "No `config` output")
					require.Truef(l, configOut.IsString(), "`config` output must be a string")
					return configOut.StringValue()
				}
				require.Failf(l, "Resource not found", "resourceName=%s", resourceName)
				return ""
			}
			assert := func(l *L,
				projectDirectory string, err error,
				s *deploy.Snapshot, changes display.ResourceChanges,
			) {
				c1Expect := `
				[
				  {
				    "method": "pulumirpc.CheckRequest",
				    "message": {
				      "urn": "urn:pulumi:test::l2-provider-grpc-config::pulumi:providers:testconfigprovider::prov1",
				      "olds": {},
				      "news": {
					"b1": "true",
					"b2": "false",
					"i1": "0",
					"i2": "42",
					"li1": "[1,2]",
					"ls1": "[]",
					"ls2": "[\"\",\"foo\"]",
					"mi1": "{\"key1\":0,\"key2\":42}",
					"ms1": "{}",
					"ms2": "{\"key1\":\"value1\",\"key2\":\"value2\"}",
					"n1": "0",
					"n2": "42.42",
					"oi1": "{\"x\":42}",
					"os1": "{}",
					"os2": "{\"x\":\"x-value\"}",
					"s1": "",
					"s2": "x",
					"s3": "{}",
					"version": "0.0.1"
				      },
				      "name": "prov1",
				      "type": "pulumi:providers:testconfigprovider"
				    }
				  },
				  {
				    "method": "pulumirpc.ConfigureRequest",
				    "message": {
				      "variables": {
					"testconfigprovider:config:b1": "true",
					"testconfigprovider:config:b2": "false",
					"testconfigprovider:config:i1": "0",
					"testconfigprovider:config:i2": "42",
					"testconfigprovider:config:li1": "[1,2]",
					"testconfigprovider:config:ls1": "[]",
					"testconfigprovider:config:ls2": "[\"\",\"foo\"]",
					"testconfigprovider:config:mi1": "{\"key1\":0,\"key2\":42}",
					"testconfigprovider:config:ms1": "{}",
					"testconfigprovider:config:ms2": "{\"key1\":\"value1\",\"key2\":\"value2\"}",
					"testconfigprovider:config:n1": "0",
					"testconfigprovider:config:n2": "42.42",
					"testconfigprovider:config:oi1": "{\"x\":42}",
					"testconfigprovider:config:os1": "{}",
					"testconfigprovider:config:os2": "{\"x\":\"x-value\"}",
					"testconfigprovider:config:s1": "",
					"testconfigprovider:config:s2": "x",
					"testconfigprovider:config:s3": "{}"
				      },
				      "args": {
					"b1": "true",
					"b2": "false",
					"i1": "0",
					"i2": "42",
					"li1": "[1,2]",
					"ls1": "[]",
					"ls2": "[\"\",\"foo\"]",
					"mi1": "{\"key1\":0,\"key2\":42}",
					"ms1": "{}",
					"ms2": "{\"key1\":\"value1\",\"key2\":\"value2\"}",
					"n1": "0",
					"n2": "42.42",
					"oi1": "{\"x\":42}",
					"os1": "{}",
					"os2": "{\"x\":\"x-value\"}",
					"s1": "",
					"s2": "x",
					"s3": "{}",
					"version": "0.0.1"
				      },
				      "acceptSecrets": true,
				      "acceptResources": true,
				      "sendsOldInputs": true,
				      "sendsOldInputsToDelete": true
				    }
				  }
				]`
				require.JSONEq(l, c1Expect, config(l, s, "c1"))
			}
			return []testRun{{assert: assert}}
		})(),
	},
	"l2-invoke-simple": {
		providers: []plugin.Provider{&providers.SimpleInvokeProvider{}},
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 2, "expected 2 resource")

					// TODO: the root stack must be the first resource to be registered
					// such that snap.Resources[0].Type == resource.RootStackType
					// however with the python SDK, that is not the case, instead the default
					// provider gets registered first. This is indicating that something might be wrong
					// with the how python SDK registers resources
					var stack *resource.State
					for _, r := range snap.Resources {
						if r.Type == resource.RootStackType {
							stack = r
							break
						}
					}

					require.NotNil(l, stack, "expected a stack resource")

					outputs := stack.Outputs

					assertPropertyMapMember(l, outputs, "hello", resource.NewStringProperty("hello world"))
					assertPropertyMapMember(l, outputs, "goodbye", resource.NewStringProperty("goodbye world"))
				},
			},
		},
	},
	"l2-invoke-variants": {
		providers: []plugin.Provider{&providers.SimpleInvokeProvider{}},
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 3, "expected 3 resource")
					// TODO: the root stack must be the first resource to be registered
					// such that snap.Resources[0].Type == resource.RootStackType
					// however with the python SDK, that is not the case, instead the default
					// provider gets registered first. This is indicating that something might be wrong
					// with the how python SDK registers resources
					var stack *resource.State
					for _, r := range snap.Resources {
						if r.Type == resource.RootStackType {
							stack = r
							break
						}
					}

					require.NotNil(l, stack, "expected a stack resource")

					outputs := stack.Outputs

					assertPropertyMapMember(l, outputs, "outputInput", resource.NewStringProperty("Goodbye world"))
					assertPropertyMapMember(l, outputs, "unit", resource.NewStringProperty("Hello world"))
				},
			},
		},
	},
	"l2-invoke-secrets": {
		providers: []plugin.Provider{
			&providers.SimpleInvokeProvider{},
			&providers.SimpleProvider{},
		},
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)
					var stack *resource.State
					for _, r := range snap.Resources {
						if r.Type == resource.RootStackType {
							stack = r
							break
						}
					}

					require.NotNil(l, stack, "expected a stack resource")

					outputs := stack.Outputs
					assertPropertyMapMember(l, outputs, "nonSecret",
						resource.NewStringProperty("hello world"))
					assertPropertyMapMember(l, outputs, "firstSecret",
						resource.MakeSecret(resource.NewStringProperty("hello world")))
					assertPropertyMapMember(l, outputs, "secondSecret",
						resource.MakeSecret(resource.NewStringProperty("goodbye world")))
				},
			},
		},
	},
	"l2-invoke-dependencies": {
		providers: []plugin.Provider{
			&providers.SimpleInvokeProvider{},
			&providers.SimpleProvider{},
		},
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)
					var first *resource.State
					var second *resource.State
					for _, r := range snap.Resources {
						if r.URN.Name() == "first" {
							first = r
						}
						if r.URN.Name() == "second" {
							second = r
						}
					}

					require.NotNil(l, first, "expected first resource")
					require.NotNil(l, second, "expected second resource")
					require.Empty(l, first.Dependencies, "expected no dependencies")
					require.Len(l, second.Dependencies, 1, "expected one dependency")
					dependencies, ok := second.PropertyDependencies["value"]
					require.True(l, ok, "expected dependency on property 'value'")
					require.Len(l, dependencies, 1, "expected one dependency")
					require.Equal(l, first.URN, dependencies[0], "expected second to depend on first")
					require.Equal(l, first.URN, second.Dependencies[0], "expected second to depend on first")
				},
			},
		},
	},
	"l2-primitive-ref": {
		providers: []plugin.Provider{&providers.PrimitiveRefProvider{}},
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)

					// Check we have the one simple resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:primitive-ref", provider.Type.String(), "expected primitive-ref provider")

					simple := snap.Resources[2]
					assert.Equal(l, "primitive-ref:index:Resource", simple.Type.String(), "expected primitive-ref resource")

					want := resource.NewPropertyMapFromMap(map[string]any{
						"data": resource.NewPropertyMapFromMap(map[string]any{
							"boolean":   false,
							"float":     2.17,
							"integer":   -12,
							"string":    "Goodbye",
							"boolArray": []interface{}{false, true},
							"stringMap": map[string]interface{}{
								"two":   "turtle doves",
								"three": "french hens",
							},
						}),
					})
					assert.Equal(l, want, simple.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, simple.Inputs, simple.Outputs, "expected inputs and outputs to match")
				},
			},
		},
	},
	"l2-ref-ref": {
		providers: []plugin.Provider{&providers.RefRefProvider{}},
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)

					// Check we have the one simple resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:ref-ref", provider.Type.String(), "expected ref-ref provider")

					simple := snap.Resources[2]
					assert.Equal(l, "ref-ref:index:Resource", simple.Type.String(), "expected ref-ref resource")

					want := resource.NewPropertyMapFromMap(map[string]any{
						"data": resource.NewPropertyMapFromMap(map[string]any{
							"innerData": resource.NewPropertyMapFromMap(map[string]any{
								"boolean":   false,
								"float":     2.17,
								"integer":   -12,
								"string":    "Goodbye",
								"boolArray": []interface{}{false, true},
								"stringMap": map[string]interface{}{
									"two":   "turtle doves",
									"three": "french hens",
								},
							}),
							"boolean":   true,
							"float":     4.5,
							"integer":   1024,
							"string":    "Hello",
							"boolArray": []interface{}{},
							"stringMap": map[string]interface{}{
								"x": "100",
								"y": "200",
							},
						}),
					})
					assert.Equal(l, want, simple.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, simple.Inputs, simple.Outputs, "expected inputs and outputs to match")
				},
			},
		},
	},
	"l2-plain": {
		providers: []plugin.Provider{&providers.PlainProvider{}},
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, err, changes)

					// Check we have the one simple resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:plain", provider.Type.String(), "expected plain provider")

					plain := snap.Resources[2]
					assert.Equal(l, "plain:index:Resource", plain.Type.String(), "expected plain resource")

					want := resource.NewPropertyMapFromMap(map[string]any{
						"data": resource.NewPropertyMapFromMap(map[string]any{
							"innerData": resource.NewPropertyMapFromMap(map[string]any{
								"boolean":   false,
								"float":     2.17,
								"integer":   -12,
								"string":    "Goodbye",
								"boolArray": []interface{}{false, true},
								"stringMap": map[string]interface{}{
									"two":   "turtle doves",
									"three": "french hens",
								},
							}),
							"boolean":   true,
							"float":     4.5,
							"integer":   1024,
							"string":    "Hello",
							"boolArray": []interface{}{true, false},
							"stringMap": map[string]interface{}{
								"x": "100",
								"y": "200",
							},
						}),
					})
					assert.Equal(l, want, plain.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, plain.Inputs, plain.Outputs, "expected inputs and outputs to match")
				},
			},
		},
	},
}
