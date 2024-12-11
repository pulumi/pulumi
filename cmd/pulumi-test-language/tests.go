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
	"github.com/pulumi/pulumi/cmd/pulumi-test-language/tests"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	deployProviders "github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/deepcopy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lorem is a long string used for testing large string values.
const lorem string = "Lorem ipsum dolor sit amet, consectetur adipiscing elit," +
	" sed do eiusmod tempor incididunt ut labore et dolore magna aliqua." +
	" Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat." +
	" Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur." +
	" Excepteur sint occaecat cupidatat non proident," +
	" sunt in culpa qui officia deserunt mollit anim id est laborum."

//go:embed testdata
var languageTestdata embed.FS

var languageTests = map[string]tests.LanguageTest{
	// ==========
	// INTERNAL
	// ==========
	"internal-bad-schema": {
		Providers: []plugin.Provider{&providers.BadProvider{}},
		Runs:      []tests.TestRun{{}},
	},
	// ==========
	// L1 (Tests not using providers)
	// ==========
	"l1-empty": {
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.AssertStackResource(l, err, changes)
				},
			},
		},
	},
	"l1-output-bool": {
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

					// Check we have two outputs in the stack for true and false
					require.NotEmpty(l, snap.Resources, "expected at least 1 resource")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")

					outputs := stack.Outputs

					tests.AssertPropertyMapMember(l, outputs, "output_true", resource.NewBoolProperty(true))
					tests.AssertPropertyMapMember(l, outputs, "output_false", resource.NewBoolProperty(false))
				},
			},
		},
	},
	"l1-output-number": {
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

					require.NotEmpty(l, snap.Resources, "expected at least 1 resource")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")

					outputs := stack.Outputs

					assert.Len(l, outputs, 6, "expected 6 outputs")
					tests.AssertPropertyMapMember(l, outputs, "zero", resource.NewNumberProperty(0))
					tests.AssertPropertyMapMember(l, outputs, "one", resource.NewNumberProperty(1))
					tests.AssertPropertyMapMember(l, outputs, "e", resource.NewNumberProperty(2.718))
					tests.AssertPropertyMapMember(l, outputs, "minInt32", resource.NewNumberProperty(math.MinInt32))
					tests.AssertPropertyMapMember(l, outputs, "max", resource.NewNumberProperty(math.MaxFloat64))
					tests.AssertPropertyMapMember(l, outputs, "min", resource.NewNumberProperty(math.SmallestNonzeroFloat64))
				},
			},
		},
	},
	"l1-output-string": {
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

					require.NotEmpty(l, snap.Resources, "expected at least 1 resource")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")

					outputs := stack.Outputs

					assert.Len(l, outputs, 6, "expected 6 outputs")
					tests.AssertPropertyMapMember(l, outputs, "empty", resource.NewStringProperty(""))
					tests.AssertPropertyMapMember(l, outputs, "small", resource.NewStringProperty("Hello world!"))
					tests.AssertPropertyMapMember(l, outputs, "emoji", resource.NewStringProperty("👋 \"Hello \U0001019b!\" 😊"))
					tests.AssertPropertyMapMember(l, outputs, "escape", resource.NewStringProperty(
						"Some ${common} \"characters\" 'that' need escaping: "+
							"\\ (backslash), \t (tab), \u001b (escape), \u0007 (bell), \u0000 (null), \U000e0021 (tag space)"))
					tests.AssertPropertyMapMember(l, outputs, "escapeNewline", resource.NewStringProperty(
						"Some ${common} \"characters\" 'that' need escaping: "+
							"\\ (backslash), \n (newline), \t (tab), \u001b (escape), \u0007 (bell), \u0000 (null), \U000e0021 (tag space)"))

					large := strings.Repeat(lorem+"\n", 150)
					tests.AssertPropertyMapMember(l, outputs, "large", resource.NewStringProperty(large))
				},
			},
		},
	},
	"l1-output-array": {
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

					require.NotEmpty(l, snap.Resources, "expected at least 1 resource")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")

					outputs := stack.Outputs

					assert.Len(l, outputs, 5, "expected 5 outputs")
					tests.AssertPropertyMapMember(l, outputs, "empty", resource.NewArrayProperty([]resource.PropertyValue{}))
					tests.AssertPropertyMapMember(l, outputs, "small", resource.NewArrayProperty([]resource.PropertyValue{
						resource.NewStringProperty("Hello"),
						resource.NewStringProperty("World"),
					}))
					tests.AssertPropertyMapMember(l, outputs, "numbers", resource.NewArrayProperty([]resource.PropertyValue{
						resource.NewNumberProperty(0), resource.NewNumberProperty(1), resource.NewNumberProperty(2),
						resource.NewNumberProperty(3), resource.NewNumberProperty(4), resource.NewNumberProperty(5),
					}))
					tests.AssertPropertyMapMember(l, outputs, "nested", resource.NewArrayProperty([]resource.PropertyValue{
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
					tests.AssertPropertyMapMember(l, outputs, "large", resource.NewArrayProperty(large))
				},
			},
		},
	},
	"l1-main": {
		Runs: []tests.TestRun{
			{
				Main: "subdir",
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

					// Check we have an output in the stack for true
					require.NotEmpty(l, snap.Resources, "expected at least 1 resource")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")

					outputs := stack.Outputs

					tests.AssertPropertyMapMember(l, outputs, "output_true", resource.NewBoolProperty(true))
				},
			},
		},
	},
	"l1-stack-reference": {
		StackReferences: map[string]resource.PropertyMap{
			"organization/other/dev": {
				"plain":  resource.NewStringProperty("plain"),
				"secret": resource.MakeSecret(resource.NewStringProperty("secret")),
			},
		},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 3, "expected at least 3 resources")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")
					prov := snap.Resources[1]
					require.Equal(l, "pulumi:providers:pulumi", prov.Type.String(), "expected a default pulumi provider resource")
					ref := snap.Resources[2]
					require.Equal(l, "pulumi:pulumi:StackReference", ref.Type.String(), "expected a stack reference resource")

					outputs := stack.Outputs

					assert.Len(l, outputs, 2, "expected 2 outputs")
					tests.AssertPropertyMapMember(l, outputs, "plain", resource.NewStringProperty("plain"))
					tests.AssertPropertyMapMember(l, outputs, "secret", resource.MakeSecret(resource.NewStringProperty("secret")))
				},
			},
		},
	},
	"l1-builtin-info": {
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

					require.NotEmpty(l, snap.Resources, "expected at least 1 resource")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")

					outputs := stack.Outputs

					assert.Len(l, outputs, 3, "expected 3 outputs")
					tests.AssertPropertyMapMember(l, outputs, "stackOutput", resource.NewStringProperty("test"))
					tests.AssertPropertyMapMember(l, outputs, "projectOutput", resource.NewStringProperty("l1-builtin-info"))
					tests.AssertPropertyMapMember(l, outputs, "organizationOutput", resource.NewStringProperty("organization"))
				},
			},
		},
	},
	"l1-output-map": {
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

					require.NotEmpty(l, snap.Resources, "expected at least 1 resource")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")

					outputs := stack.Outputs

					assert.Len(l, outputs, 4, "expected 4 outputs")
					tests.AssertPropertyMapMember(l, outputs, "empty", resource.NewObjectProperty(resource.PropertyMap{}))
					tests.AssertPropertyMapMember(l, outputs, "strings", resource.NewObjectProperty(resource.PropertyMap{
						"greeting": resource.NewStringProperty("Hello, world!"),
						"farewell": resource.NewStringProperty("Goodbye, world!"),
					}))
					tests.AssertPropertyMapMember(l, outputs, "numbers", resource.NewObjectProperty(resource.PropertyMap{
						"1": resource.NewNumberProperty(1),
						"2": resource.NewNumberProperty(2),
					}))
					tests.AssertPropertyMapMember(l, outputs, "keys", resource.NewObjectProperty(resource.PropertyMap{
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
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

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
		Providers: []plugin.Provider{&providers.PrimitiveProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

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
		Providers: []plugin.Provider{&providers.AlphaProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

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
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

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
		Providers: []plugin.Provider{&providers.AssetArchiveProvider{}},
		Runs: []tests.TestRun{
			{
				Main: "subdir",
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

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
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		Runs: []tests.TestRun{
			{
				UpdateOptions: engine.UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{
						"**target**",
					}),
				},
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)
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
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)
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
				Assert: func(l *tests.L,
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
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)
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
				UpdateOptions: engine.UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{
						"**targetOnly**",
					}),
				},
				Assert: func(l *tests.L,
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
		Providers: []plugin.Provider{&providers.SimpleProvider{}, &providers.FailOnCreateProvider{}},
		Runs: []tests.TestRun{
			{
				UpdateOptions: engine.UpdateOptions{
					ContinueOnError: true,
				},
				Assert: func(l *tests.L,
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
		Providers: []plugin.Provider{&providers.LargeProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)
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
		Providers: []plugin.Provider{&providers.ConfigProvider{}},
		Runs: []tests.TestRun{
			{
				Config: config.Map{
					config.MustParseKey("config:name"): config.NewValue("hello"),
				},
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)
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
		Providers: []plugin.Provider{&providers.ConfigGrpcProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					g := &grpcTestContext{l: l, s: snap}

					r := g.CheckConfigReq("config")
					assert.Equal(l, "", r.News.Fields["string1"].AsInterface(), "string1")
					assert.Equal(l, "x", r.News.Fields["string2"].AsInterface(), "string2")
					assert.Equal(l, "{}", r.News.Fields["string3"].AsInterface(), "string3")
					tests.AssertEqualOrJSONEncoded(l, float64(0), r.News.Fields["int1"].AsInterface(), "int1")
					tests.AssertEqualOrJSONEncoded(l, float64(42), r.News.Fields["int2"].AsInterface(), "int2")
					tests.AssertEqualOrJSONEncoded(l, float64(0), r.News.Fields["num1"].AsInterface(), "num1")
					tests.AssertEqualOrJSONEncoded(l, float64(42.42), r.News.Fields["num2"].AsInterface(), "num2")
					tests.AssertEqualOrJSONEncoded(l, true, r.News.Fields["bool1"].AsInterface(), "bool1")
					tests.AssertEqualOrJSONEncoded(l, false, r.News.Fields["bool2"].AsInterface(), "bool2")
					tests.AssertEqualOrJSONEncoded(l, []any{}, r.News.Fields["listString1"].AsInterface(), "listString1")
					tests.AssertEqualOrJSONEncoded(l, []any{"", "foo"}, r.News.Fields["listString2"].AsInterface(), "listString2")
					tests.AssertEqualOrJSONEncoded(l,
						[]any{float64(1), float64(2)},
						r.News.Fields["listInt1"].AsInterface(), "listInt1")

					tests.AssertEqualOrJSONEncoded(l, map[string]any{}, r.News.Fields["mapString1"].AsInterface(), "mapString1")

					tests.AssertEqualOrJSONEncoded(l,
						map[string]any{"key1": "value1", "key2": "value2"},
						r.News.Fields["mapString2"].AsInterface(), "mapString2")

					tests.AssertEqualOrJSONEncoded(l,
						map[string]any{"key1": float64(0), "key2": float64(42)},
						r.News.Fields["mapInt1"].AsInterface(), "mapInt1")

					tests.AssertEqualOrJSONEncoded(l, map[string]any{}, r.News.Fields["objString1"].AsInterface(), "objString1")

					tests.AssertEqualOrJSONEncoded(l, map[string]any{"x": "x-value"},
						r.News.Fields["objString2"].AsInterface(), "objString2")

					tests.AssertEqualOrJSONEncoded(l,
						map[string]any{"x": float64(42)},
						r.News.Fields["objInt1"].AsInterface(), "objInt1")

					// Check what schemaprov received in ConfigureRequest.
					c := g.ConfigureReq("config")
					assert.Equal(l, "", c.Args.Fields["string1"].AsInterface(), "string1")
					assert.Equal(l, "x", c.Args.Fields["string2"].AsInterface(), "string2")
					assert.Equal(l, "{}", c.Args.Fields["string3"].AsInterface(), "string3")
					tests.AssertEqualOrJSONEncoded(l, float64(0), c.Args.Fields["int1"].AsInterface(), "int1")
					tests.AssertEqualOrJSONEncoded(l, float64(42), c.Args.Fields["int2"].AsInterface(), "int2")
					tests.AssertEqualOrJSONEncoded(l, float64(0), c.Args.Fields["num1"].AsInterface(), "num1")
					tests.AssertEqualOrJSONEncoded(l, float64(42.42), c.Args.Fields["num2"].AsInterface(), "num2")
					tests.AssertEqualOrJSONEncoded(l, true, c.Args.Fields["bool1"].AsInterface(), "bool1")
					tests.AssertEqualOrJSONEncoded(l, false, c.Args.Fields["bool2"].AsInterface(), "bool2")
					tests.AssertEqualOrJSONEncoded(l, []any{}, c.Args.Fields["listString1"].AsInterface(), "listString1")
					tests.AssertEqualOrJSONEncoded(l, []any{"", "foo"}, c.Args.Fields["listString2"].AsInterface(), "listString2")
					tests.AssertEqualOrJSONEncoded(l,
						[]any{float64(1), float64(2)},
						c.Args.Fields["listInt1"].AsInterface(), "listInt1")

					tests.AssertEqualOrJSONEncoded(l, map[string]any{}, c.Args.Fields["mapString1"].AsInterface(), "mapString1")

					tests.AssertEqualOrJSONEncoded(l,
						map[string]any{"key1": "value1", "key2": "value2"},
						c.Args.Fields["mapString2"].AsInterface(), "mapString2")

					tests.AssertEqualOrJSONEncoded(l,
						map[string]any{"key1": float64(0), "key2": float64(42)},
						c.Args.Fields["mapInt1"].AsInterface(), "mapInt1")

					tests.AssertEqualOrJSONEncoded(l, map[string]any{}, c.Args.Fields["objString1"].AsInterface(), "objString1")

					tests.AssertEqualOrJSONEncoded(l, map[string]any{"x": "x-value"},
						c.Args.Fields["objString2"].AsInterface(), "objString2")

					tests.AssertEqualOrJSONEncoded(l,
						map[string]any{"x": float64(42)},
						c.Args.Fields["objInt1"].AsInterface(), "objInt1")

					v := c.GetVariables()
					assert.Equal(l, "", v["config-grpc:config:string1"], "string1")
					assert.Equal(l, "x", v["config-grpc:config:string2"], "string2")
					assert.Equal(l, "{}", v["config-grpc:config:string3"], "string3")
					assert.Equal(l, "0", v["config-grpc:config:int1"], "int1")
					assert.Equal(l, "42", v["config-grpc:config:int2"], "int2")
					assert.Equal(l, "0", v["config-grpc:config:num1"], "num1")
					assert.Equal(l, "42.42", v["config-grpc:config:num2"], "num2")
					assert.Equal(l, "true", v["config-grpc:config:bool1"], "bool1")
					assert.Equal(l, "false", v["config-grpc:config:bool2"], "bool2")
					assert.JSONEq(l, "[]", v["config-grpc:config:listString1"], "listString1")
					assert.JSONEq(l, "[\"\",\"foo\"]", v["config-grpc:config:listString2"], "listString2")
					assert.JSONEq(l, "[1,2]", v["config-grpc:config:listInt1"], "listInt1")
					assert.JSONEq(l, "{}", v["config-grpc:config:mapString1"], "mapString1")
					assert.JSONEq(l, "{\"key1\":\"value1\",\"key2\":\"value2\"}", v["config-grpc:config:mapString2"], "mapString2")
					assert.JSONEq(l, "{\"key1\":0,\"key2\":42}", v["config-grpc:config:mapInt1"], "mapInt1")
					assert.JSONEq(l, "{}", v["config-grpc:config:objString1"], "objString1")
					assert.JSONEq(l, "{\"x\":\"x-value\"}", v["config-grpc:config:objString2"], "objString2")
					assert.JSONEq(l, "{\"x\":42}", v["config-grpc:config:objInt1"], "objInt1")

					tests.AssertNoSecretLeaks(l, snap, tests.AssertNoSecretLeaksOpts{
						// ConfigFetcher is a test helper that retains secret material in its
						// state by design, and should not be part of the check.
						IgnoreResourceTypes: []tokens.Type{"config-grpc:index:ConfigFetcher"},
						Secrets:             []string{"SECRET", "SECRET2"},
					})
				},
			},
		},
	},
	// Looks like in the test setup, proper partitioning of provider space is not yet working and Configure calls
	// race with Create calls when talking to a provider. It makes it too difficult to test more than one explicit
	// provider per test case. To compensate, more test cases are added.
	"l2-provider-grpc-config-secret": {
		// Check what schemaprov received in CheckRequest.
		Providers: []plugin.Provider{&providers.ConfigGrpcProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					g := &grpcTestContext{l: l, s: snap}

					// Now check first-class secrets for programsecretprov.
					r := g.CheckConfigReq("config")

					// These asserts do not look right, but are based on Go behavior. Should SECRET
					// be wrapped in secret tags instead when passing to CheckConfig? Or not?
					assert.Equal(l, "SECRET", r.News.Fields["string1"].AsInterface(), "string1")
					tests.AssertEqualOrJSONEncoded(l, float64(1234567890), r.News.Fields["int1"].AsInterface(), "int1")
					tests.AssertEqualOrJSONEncoded(l, float64(123456.789), r.News.Fields["num1"].AsInterface(), "num1")
					tests.AssertEqualOrJSONEncoded(l, true, r.News.Fields["bool1"].AsInterface(), "bool1")
					tests.AssertEqualOrJSONEncoded(l, []any{"SECRET", "SECRET2"},
						r.News.Fields["listString1"].AsInterface(), "listString1")
					tests.AssertEqualOrJSONEncoded(l, []any{"VALUE", "SECRET"},
						r.News.Fields["listString2"].AsInterface(), "listString2")
					tests.AssertEqualOrJSONEncoded(l, map[string]any{"key1": "value1", "key2": "SECRET"},
						r.News.Fields["mapString2"].AsInterface(), "mapString2")
					tests.AssertEqualOrJSONEncoded(l, map[string]any{"x": "SECRET"},
						r.News.Fields["objString2"].AsInterface(), "objString2")

					// The secret versions have two options, JSON-encoded or not. Languages do not
					// agree yet on which form to use.
					c := g.ConfigureReq("config")
					assert.Equal(l, tests.Secret("SECRET"), c.Args.Fields["string1"].AsInterface(), "string1")

					tests.AssertEqualOrJSONEncodedSecret(l,
						tests.Secret(float64(1234567890)),
						float64(1234567890),
						c.Args.Fields["int1"].AsInterface(), "int1")

					tests.AssertEqualOrJSONEncodedSecret(l,
						tests.Secret(float64(123456.789)),
						float64(123456.789),
						c.Args.Fields["num1"].AsInterface(), "num1")

					tests.AssertEqualOrJSONEncodedSecret(l,
						tests.Secret(true),
						true,
						c.Args.Fields["bool1"].AsInterface(), "bool1")

					tests.AssertEqualOrJSONEncodedSecret(l,
						tests.Secret([]any{"SECRET", "SECRET2"}),
						[]any{"SECRET", "SECRET2"},
						c.Args.Fields["listString1"].AsInterface(), "listString1")

					// Secret floating happened here, perhaps []any{"VALUE", tests.Secret("SECRET")}
					// would be preferable instead at some point.
					tests.AssertEqualOrJSONEncodedSecret(l,
						tests.Secret([]any{"VALUE", "SECRET"}),
						[]any{"VALUE", "SECRET"},
						c.Args.Fields["listString2"].AsInterface(), "listString2")

					tests.AssertEqualOrJSONEncodedSecret(l,
						map[string]any{"key1": "value1", "key2": tests.Secret("SECRET")},
						map[string]any{"key1": "value1", "key2": "SECRET"},
						c.Args.Fields["mapString2"].AsInterface(), "mapString2")

					tests.AssertEqualOrJSONEncodedSecret(l,
						map[string]any{"x": tests.Secret("SECRET")},
						map[string]any{"x": "SECRET"},
						c.Args.Fields["objString2"].AsInterface(), "objString2")

					// Secretness is not exposed in GetVariables. Instead the data is JSON-encoded.
					v := c.GetVariables()
					assert.Equal(l, "SECRET", v["config-grpc:config:string1"], "string1")
					assert.Equal(l, "1234567890", v["config-grpc:config:int1"], "int1")
					assert.Equal(l, "123456.789", v["config-grpc:config:num1"], "num1")
					assert.Equal(l, "true", v["config-grpc:config:bool1"], "bool1")
					assert.JSONEq(l, "[\"SECRET\",\"SECRET2\"]", v["config-grpc:config:listString1"], "listString1")
					assert.JSONEq(l, "[\"VALUE\",\"SECRET\"]", v["config-grpc:config:listString2"], "listString2")
					assert.JSONEq(l, "{\"key1\":\"value1\",\"key2\":\"SECRET\"}", v["config-grpc:config:mapString2"], "mapString2")
					assert.JSONEq(l, "{\"x\":\"SECRET\"}", v["config-grpc:config:objString2"], "objString2")

					tests.AssertNoSecretLeaks(l, snap, tests.AssertNoSecretLeaksOpts{
						// ConfigFetcher is a test helper that retains secret material in its
						// state by design, and should not be part of the check.
						IgnoreResourceTypes: []tokens.Type{"config-grpc:index:ConfigFetcher"},
						Secrets:             []string{"SECRET", "SECRET2"},
					})
				},
			},
		},
	},
	// This test checks how SDKs propagate properties marked as secret to the provider Configure on the gRPC level.
	"l2-provider-grpc-config-schema-secret": {
		Providers: []plugin.Provider{&providers.ConfigGrpcProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					g := &grpcTestContext{l: l, s: snap}

					// Verify the CheckConfig request received by the provider.
					r := g.CheckConfigReq("config")

					// TODO[pulumi/pulumi#16876]: CheckConfig request gets the secrets in the plain.
					// This is suspect, probably has to do with secret negotiation happening later
					// in the gRPC provider cycle.
					assert.Equal(l, "SECRET",
						r.News.Fields["secretString1"].AsInterface(), "secretString1")

					tests.AssertEqualOrJSONEncoded(l, float64(16),
						r.News.Fields["secretInt1"].AsInterface(), "secretInt1")

					tests.AssertEqualOrJSONEncoded(l, float64(123456.7890),
						r.News.Fields["secretNum1"].AsInterface(), "secretNum1")

					tests.AssertEqualOrJSONEncoded(l, true,
						r.News.Fields["secretBool1"].AsInterface(), "secretBool1")

					tests.AssertEqualOrJSONEncoded(l, []any{"SECRET", "SECRET2"},
						r.News.Fields["listSecretString1"].AsInterface(), "listSecretString1")

					tests.AssertEqualOrJSONEncoded(l, map[string]any{"key1": "SECRET", "key2": "SECRET2"},
						r.News.Fields["mapSecretString1"].AsInterface(), "mapSecretString1")

					// Now verify the Configure request.
					c := g.ConfigureReq("config")

					// All the fields are coming in as secret-wrapped fields into Configure.
					assert.Equal(l, tests.Secret("SECRET"),
						c.Args.Fields["secretString1"].AsInterface(), "secretString1")

					tests.AssertEqualOrJSONEncodedSecret(l,
						tests.Secret(float64(16)), float64(16),
						c.Args.Fields["secretInt1"].AsInterface(), "secretInt1")

					tests.AssertEqualOrJSONEncodedSecret(l,
						tests.Secret(float64(123456.7890)), float64(123456.7890),
						c.Args.Fields["secretNum1"].AsInterface(), "secretNum1")

					tests.AssertEqualOrJSONEncodedSecret(l,
						tests.Secret(true), true,
						c.Args.Fields["secretBool1"].AsInterface(), "secretBool1")

					tests.AssertEqualOrJSONEncodedSecret(l,
						tests.Secret([]any{"SECRET", "SECRET2"}),
						[]any{"SECRET", "SECRET2"},
						c.Args.Fields["listSecretString1"].AsInterface(), "listSecretString1")

					tests.AssertEqualOrJSONEncodedSecret(l,
						tests.Secret(map[string]any{"key1": "SECRET", "key2": "SECRET2"}),
						map[string]any{"key1": "SECRET", "key2": "SECRET2"},
						c.Args.Fields["mapSecretString1"].AsInterface(), "mapSecretString1")

					// Secretness is not exposed in GetVariables. Instead the data is JSON-encoded.
					v := c.GetVariables()
					assert.Equal(l, "SECRET", v["config-grpc:config:secretString1"], "secretString1")
					assert.JSONEq(l, "16", v["config-grpc:config:secretInt1"], "secretInt1")
					assert.JSONEq(l, "123456.7890", v["config-grpc:config:secretNum1"], "secretNum1")
					assert.JSONEq(l, "true", v["config-grpc:config:secretBool1"], "secretBool1")
					assert.JSONEq(l, `["SECRET", "SECRET2"]`, v["config-grpc:config:listSecretString1"], "listSecretString1")

					assert.JSONEq(l, `{"key1":"SECRET","key2":"SECRET2"}`,
						v["config-grpc:config:mapSecretString1"], "mapSecretString1")

					// TODO[pulumi/pulumi#17652] Languages do not agree on the object property
					// casing sent to CheckConfig, Node and Go send "secretX", Python sends
					// "secret_x" though.
					//
					// tests.AssertEqualOrJSONEncoded(l, map[string]any{"secretX": "SECRET"},
					// 	r.News.Fields["objSecretString1"].AsInterface(), "objSecretString1")
					// tests.AssertEqualOrJSONEncodedSecret(l,
					//      map[string]any{"secretX": tests.Secret("SECRET")}, map[string]any{"secretX": "SECRET"},
					// 	r.Args.Fields["objSecretString1"].AsInterface(), "objSecretString1")
					// assert.JSONEq(l, `{"secretX":"SECRET"}`,
					// 	v["config-grpc:config:objectSecretString1"], "objSecretString1")

					tests.AssertNoSecretLeaks(l, snap, tests.AssertNoSecretLeaksOpts{
						// ConfigFetcher is a test helper that retains secret material in its
						// state by design, and should not be part of the check.
						IgnoreResourceTypes: []tokens.Type{"config-grpc:index:ConfigFetcher"},
						Secrets:             []string{"SECRET", "SECRET2"},
					})
				},
			},
		},
	},
	"l2-invoke-simple": {
		Providers: []plugin.Provider{&providers.SimpleInvokeProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 2, "expected 2 resource")

					// TODO https://github.com/pulumi/pulumi/issues/17816
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

					tests.AssertPropertyMapMember(l, outputs, "hello", resource.NewStringProperty("hello world"))
					tests.AssertPropertyMapMember(l, outputs, "goodbye", resource.NewStringProperty("goodbye world"))
				},
			},
		},
	},
	"l2-invoke-options": {
		Providers: []plugin.Provider{&providers.SimpleInvokeProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)
					require.Len(l, snap.Resources, 2, "expected 2 resource")
					// TODO https://github.com/pulumi/pulumi/issues/17816
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
					tests.AssertPropertyMapMember(l, outputs, "hello", resource.NewStringProperty("hello world"))
				},
			},
		},
	},
	"l2-invoke-options-depends-on": {
		Providers: []plugin.Provider{&providers.SimpleInvokeProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)
					require.Len(l, snap.Resources, 5, "expected 5 resources")
					// TODO https://github.com/pulumi/pulumi/issues/17816
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
					tests.AssertPropertyMapMember(l, outputs, "hello", resource.NewStringProperty("hello world"))

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
					dependencies, ok := second.PropertyDependencies["text"]
					require.True(l, ok, "expected dependency on property 'text'")
					require.Len(l, dependencies, 1, "expected one dependency")
					require.Equal(l, first.URN, dependencies[0], "expected second to depend on first")
					require.Equal(l, first.URN, second.Dependencies[0], "expected second to depend on first")
				},
			},
		},
	},
	"l2-invoke-variants": {
		Providers: []plugin.Provider{&providers.SimpleInvokeProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 3, "expected 3 resource")
					// TODO https://github.com/pulumi/pulumi/issues/17816
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

					tests.AssertPropertyMapMember(l, outputs, "outputInput", resource.NewStringProperty("Goodbye world"))
					tests.AssertPropertyMapMember(l, outputs, "unit", resource.NewStringProperty("Hello world"))
				},
			},
		},
	},
	"l2-invoke-secrets": {
		Providers: []plugin.Provider{
			&providers.SimpleInvokeProvider{},
			&providers.SimpleProvider{},
		},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)
					var stack *resource.State
					for _, r := range snap.Resources {
						if r.Type == resource.RootStackType {
							stack = r
							break
						}
					}

					require.NotNil(l, stack, "expected a stack resource")

					outputs := stack.Outputs
					tests.AssertPropertyMapMember(l, outputs, "nonSecret",
						resource.NewStringProperty("hello world"))
					tests.AssertPropertyMapMember(l, outputs, "firstSecret",
						resource.MakeSecret(resource.NewStringProperty("hello world")))
					tests.AssertPropertyMapMember(l, outputs, "secondSecret",
						resource.MakeSecret(resource.NewStringProperty("goodbye world")))
				},
			},
		},
	},
	"l2-invoke-dependencies": {
		Providers: []plugin.Provider{
			&providers.SimpleInvokeProvider{},
			&providers.SimpleProvider{},
		},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)
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
		Providers: []plugin.Provider{&providers.PrimitiveRefProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

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
		Providers: []plugin.Provider{&providers.RefRefProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

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
		Providers: []plugin.Provider{&providers.PlainProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

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
	"l2-parameterized-resource": {
		Providers: []plugin.Provider{&providers.ParameterizedProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")
					require.Equal(l,
						resource.NewStringProperty("HelloWorld"),
						stack.Outputs["parameterValue"],
						"parameter value should be correct")
				},
			},
		},
	},
	"l2-map-keys": {
		Providers: []plugin.Provider{
			&providers.PrimitiveProvider{}, &providers.PrimitiveRefProvider{},
			&providers.RefRefProvider{}, &providers.PlainProvider{},
		},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 9, "expected 9 resources in snapshot")

					tests.RequireSingleResource(l, snap.Resources, "pulumi:providers:primitive")
					primResource := tests.RequireSingleResource(l, snap.Resources, "primitive:index:Resource")
					tests.RequireSingleResource(l, snap.Resources, "pulumi:providers:primitive-ref")
					refResource := tests.RequireSingleResource(l, snap.Resources, "primitive-ref:index:Resource")
					tests.RequireSingleResource(l, snap.Resources, "pulumi:providers:ref-ref")
					rrefResource := tests.RequireSingleResource(l, snap.Resources, "ref-ref:index:Resource")
					tests.RequireSingleResource(l, snap.Resources, "pulumi:providers:plain")
					plainResource := tests.RequireSingleResource(l, snap.Resources, "plain:index:Resource")

					want := resource.NewPropertyMapFromMap(map[string]any{
						"boolean":     false,
						"float":       2.17,
						"integer":     -12,
						"string":      "Goodbye",
						"numberArray": []interface{}{0, 1},
						"booleanMap": map[string]interface{}{
							"my key": false,
							"my.key": true,
							"my-key": false,
							"my_key": true,
							"MY_KEY": false,
							"myKey":  true,
						},
					})
					assert.Equal(l, want, primResource.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, primResource.Inputs, primResource.Outputs, "expected inputs and outputs to match")

					want = resource.NewPropertyMapFromMap(map[string]any{
						"data": resource.NewPropertyMapFromMap(map[string]any{
							"boolean":   false,
							"float":     2.17,
							"integer":   -12,
							"string":    "Goodbye",
							"boolArray": []interface{}{false, true},
							"stringMap": map[string]interface{}{
								"my key": "one",
								"my.key": "two",
								"my-key": "three",
								"my_key": "four",
								"MY_KEY": "five",
								"myKey":  "six",
							},
						}),
					})
					assert.Equal(l, want, refResource.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, refResource.Inputs, refResource.Outputs, "expected inputs and outputs to match")

					want = resource.NewPropertyMapFromMap(map[string]any{
						"data": resource.NewPropertyMapFromMap(map[string]any{
							"innerData": resource.NewPropertyMapFromMap(map[string]any{
								"boolean":   false,
								"float":     -2.17,
								"integer":   123,
								"string":    "Goodbye",
								"boolArray": []interface{}{},
								"stringMap": map[string]interface{}{
									"my key": "one",
									"my.key": "two",
									"my-key": "three",
									"my_key": "four",
									"MY_KEY": "five",
									"myKey":  "six",
								},
							}),
							"boolean":   true,
							"float":     4.5,
							"integer":   1024,
							"string":    "Hello",
							"boolArray": []interface{}{},
							"stringMap": map[string]interface{}{
								"my key": "one",
								"my.key": "two",
								"my-key": "three",
								"my_key": "four",
								"MY_KEY": "five",
								"myKey":  "six",
							},
						}),
					})
					assert.Equal(l, want, rrefResource.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, rrefResource.Inputs, rrefResource.Outputs, "expected inputs and outputs to match")

					want = resource.NewPropertyMapFromMap(map[string]any{
						"data": resource.NewPropertyMapFromMap(map[string]any{
							"innerData": resource.NewPropertyMapFromMap(map[string]any{
								"boolean":   false,
								"float":     2.17,
								"integer":   -12,
								"string":    "Goodbye",
								"boolArray": []interface{}{false, true},
								"stringMap": map[string]interface{}{
									"my key": "one",
									"my.key": "two",
									"my-key": "three",
									"my_key": "four",
									"MY_KEY": "five",
									"myKey":  "six",
								},
							}),
							"boolean":   true,
							"float":     4.5,
							"integer":   1024,
							"string":    "Hello",
							"boolArray": []interface{}{true, false},
							"stringMap": map[string]interface{}{
								"my key": "one",
								"my.key": "two",
								"my-key": "three",
								"my_key": "four",
								"MY_KEY": "five",
								"myKey":  "six",
							},
						}),
						"nonPlainData": resource.NewPropertyMapFromMap(map[string]any{
							"innerData": resource.NewPropertyMapFromMap(map[string]any{
								"boolean":   false,
								"float":     2.17,
								"integer":   -12,
								"string":    "Goodbye",
								"boolArray": []interface{}{false, true},
								"stringMap": map[string]interface{}{
									"my key": "one",
									"my.key": "two",
									"my-key": "three",
									"my_key": "four",
									"MY_KEY": "five",
									"myKey":  "six",
								},
							}),
							"boolean":   true,
							"float":     4.5,
							"integer":   1024,
							"string":    "Hello",
							"boolArray": []interface{}{true, false},
							"stringMap": map[string]interface{}{
								"my key": "one",
								"my.key": "two",
								"my-key": "three",
								"my_key": "four",
								"MY_KEY": "five",
								"myKey":  "six",
							},
						}),
					})
					assert.Equal(l, want, plainResource.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, plainResource.Inputs, plainResource.Outputs, "expected inputs and outputs to match")
				},
			},
		},
	},
	"l2-resource-parent-inheritance": {
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

					// We expect the following resources:
					//
					// 0. The stack
					//
					// 1. The default simple provider.
					// 2. The explicit simple provider, used to test provider inheritance.
					//
					// 3. A parent using the explicit provider.
					// 4. A child of the parent using the explicit provider.
					// 5. An orphan without a parent or explicit provider.
					//
					// 6. A parent with its protect flag set.
					// 7. A child of the parent with its protect flag set.
					// 8. An orphan without a parent or protect flag set.
					require.Len(l, snap.Resources, 9, "expected 9 resources in snapshot")

					defaultProvider := tests.RequireSingleNamedResource(l, snap.Resources, "default_2_0_0")
					require.Equal(l, "pulumi:providers:simple", defaultProvider.Type.String(), "expected default simple provider")

					defaultProviderRef, err := deployProviders.NewReference(defaultProvider.URN, defaultProvider.ID)
					require.NoError(l, err, "expected to create default provider reference")

					explicitProvider := tests.RequireSingleNamedResource(l, snap.Resources, "provider")
					require.Equal(l, "pulumi:providers:simple", explicitProvider.Type.String(), "expected explicit simple provider")

					explicitProviderRef, err := deployProviders.NewReference(explicitProvider.URN, explicitProvider.ID)
					require.NoError(l, err, "expected to create explicit provider reference")

					// Children should inherit providers.
					providerParent := tests.RequireSingleNamedResource(l, snap.Resources, "parent1")
					providerChild := tests.RequireSingleNamedResource(l, snap.Resources, "child1")
					providerOrphan := tests.RequireSingleNamedResource(l, snap.Resources, "orphan1")

					require.Equal(
						l, explicitProviderRef.String(), providerParent.Provider,
						"expected parent to set explicit provider",
					)
					require.Equal(
						l, explicitProviderRef.String(), providerChild.Provider,
						"expected child to inherit explicit provider",
					)
					require.Equal(
						l, defaultProviderRef.String(), providerOrphan.Provider,
						"expected orphan to use default provider",
					)

					// Children should inherit protect flags.
					protectParent := tests.RequireSingleNamedResource(l, snap.Resources, "parent2")
					protectChild := tests.RequireSingleNamedResource(l, snap.Resources, "child2")
					protectOrphan := tests.RequireSingleNamedResource(l, snap.Resources, "orphan2")

					require.True(l, protectParent.Protect, "expected parent to be protected")
					require.True(l, protectChild.Protect, "expected child to inherit protect flag")
					require.False(l, protectOrphan.Protect, "expected orphan to not be protected")
				},
			},
		},
	},
	"l2-resource-secret": {
		Providers: []plugin.Provider{&providers.SecretProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

					// Check we have the one simple resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:secret", provider.Type.String(), "expected secret provider")

					simple := snap.Resources[2]
					assert.Equal(l, "secret:index:Resource", simple.Type.String(), "expected secret resource")

					want := resource.NewPropertyMapFromMap(map[string]any{
						"public":  "open",
						"private": resource.MakeSecret(resource.NewStringProperty("closed")),
						"publicData": map[string]interface{}{
							"public": "open",
							// TODO https://github.com/pulumi/pulumi/issues/10319: This should be a secret,
							// but currently _all_ the SDKs send it as a plain value and the engine doesn't
							// fix it. We should fix the engine to ensure this ends up as secret as well.
							"private": "closed",
						},
						"privateData": resource.MakeSecret(resource.NewObjectProperty(resource.NewPropertyMapFromMap(map[string]any{
							"public":  "open",
							"private": "closed",
						}))),
					})
					assert.Equal(l, want, simple.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, simple.Inputs, simple.Outputs, "expected inputs and outputs to match")
				},
			},
		},
	},
	"l2-explicit-parameterized-provider": {
		Providers: []plugin.Provider{&providers.ParameterizedProvider{}},
		Runs: []tests.TestRun{
			{
				Assert: func(l *tests.L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					tests.RequireStackResource(l, err, changes)

					// Check we have the one resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")
					require.Equal(l,
						resource.NewStringProperty("Goodbye World"),
						stack.Outputs["parameterValue"],
						"parameter value and provider config should be correct")

					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:goodbye", provider.Type.String(), "expected goodbye provider")
					assert.Equal(l, "prov", provider.URN.Name(), "expected explicit provider resource")

					simple := snap.Resources[2]
					assert.Equal(l, "goodbye:index:Goodbye", simple.Type.String(), "expected Goodbye resource")
					assert.Equal(l, string(provider.URN)+"::"+string(provider.ID), simple.Provider)
				},
			},
		},
	},
}
