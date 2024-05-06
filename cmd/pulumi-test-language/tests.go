// Copyright 2016-2023, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testRun struct {
	config config.Map
	// This can be used to set a main value for the test.
	main string
	// TODO: This should just return "string", if == "" then ok, else fail
	assert func(*L, string, result.Result, *deploy.Snapshot, display.ResourceChanges)
	// updateOptions can be used to set the update options for the engine.
	updateOptions engine.UpdateOptions
}

type languageTest struct {
	// TODO: This should be a function so we don't have to load all providers in memory all the time.
	providers []plugin.Provider

	// stackReferences specifies other stack data that this test depends on.
	stackReferences map[string]resource.PropertyMap

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
	},
	// ==========
	// L1 (Tests not using providers)
	// ==========
	"l1-empty": {
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, res result.Result,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					assertStackResource(l, res, changes)
				},
			},
		},
	},
	"l1-output-bool": {
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, res result.Result,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, res, changes)

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
					projectDirectory string, res result.Result,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, res, changes)

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
					projectDirectory string, res result.Result,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, res, changes)

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
					projectDirectory string, res result.Result,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, res, changes)

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
					projectDirectory string, res result.Result,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, res, changes)

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
					projectDirectory string, res result.Result,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, res, changes)

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
	// ==========
	// L2 (Tests using providers)
	// ==========
	"l2-resource-simple": {
		providers: []plugin.Provider{&providers.SimpleProvider{}},
		runs: []testRun{
			{
				assert: func(l *L,
					projectDirectory string, res result.Result,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, res, changes)

					// Check we have the one simple resource in the snapshot, it's provider and the stack.
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
	"l2-resource-asset-archive": {
		providers: []plugin.Provider{&providers.AssetArchiveProvider{}},
		runs: []testRun{
			{
				main: "subdir",
				assert: func(l *L,
					projectDirectory string, res result.Result,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, res, changes)

					// Check we have the the asset, archive, and folder resources in the snapshot, the provider and the stack.
					require.Len(l, snap.Resources, 5, "expected 5 resources in snapshot")

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
					projectDirectory string, res result.Result,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, res, changes)
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
					projectDirectory string, res result.Result,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, res, changes)
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
					projectDirectory string, res result.Result,
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
					projectDirectory string, res result.Result,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, res, changes)
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")
					err := snap.VerifyIntegrity()
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
					projectDirectory string, res result.Result,
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
					projectDirectory string, res result.Result,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					require.True(l, res.IsBail(), "expected a bail result")
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
				updateOptions: engine.UpdateOptions{
					ContinueOnError: true,
				},
				assert: func(l *L,
					projectDirectory string, res result.Result,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					requireStackResource(l, res, changes)
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
}
