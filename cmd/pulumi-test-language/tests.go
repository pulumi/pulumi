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
	assert func(*L, result.Result, *deploy.Snapshot, display.ResourceChanges)
	// updateOptions can be used to set the update options for the engine.
	updateOptions engine.UpdateOptions
}

type languageTest struct {
	// TODO: This should be a function so we don't have to load all providers in memory all the time.
	providers []plugin.Provider

	runs []testRun
}

//go:embed testdata
var languageTestdata embed.FS

var languageTests = map[string]languageTest{
	// ==========
	// INTERNAL
	// ==========
	"internal-bad-schema": {
		providers: []plugin.Provider{&badProvider{}},
	},
	// ==========
	// L1 (Tests not using providers)
	// ==========
	"l1-empty": {
		runs: []testRun{
			{
				assert: func(l *L, res result.Result, snap *deploy.Snapshot, changes display.ResourceChanges) {
					assertStackResource(l, res, changes)
				},
			},
		},
	},
	"l1-output-bool": {
		runs: []testRun{
			{
				assert: func(l *L, res result.Result, snap *deploy.Snapshot, changes display.ResourceChanges) {
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
	"l1-main": {
		runs: []testRun{
			{
				main: "subdir",
				assert: func(l *L, res result.Result, snap *deploy.Snapshot, changes display.ResourceChanges) {
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
	// ==========
	// L2 (Tests using providers)
	// ==========
	"l2-resource-simple": {
		providers: []plugin.Provider{&simpleProvider{}},
		runs: []testRun{
			{
				assert: func(l *L, res result.Result, snap *deploy.Snapshot, changes display.ResourceChanges) {
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
	"l2-engine-update-options": {
		providers: []plugin.Provider{&simpleProvider{}},
		runs: []testRun{
			{
				updateOptions: engine.UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{
						"**target**",
					}),
				},
				assert: func(l *L, res result.Result, snap *deploy.Snapshot, changes display.ResourceChanges) {
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
		providers: []plugin.Provider{&simpleProvider{}},
		runs: []testRun{
			{
				assert: func(l *L, res result.Result, snap *deploy.Snapshot, changes display.ResourceChanges) {
					requireStackResource(l, res, changes)
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:simple", provider.Type.String(), "expected simple provider")
					// check that both expected resources are in the snapshot
					simple := snap.Resources[2]
					assert.Equal(l, "simple:index:Resource", simple.Type.String(), "expected simple resource")
					simple2 := snap.Resources[3]
					assert.Equal(l, "simple:index:Resource", simple2.Type.String(), "expected simple resource")
				},
			},
			{
				assert: func(l *L, res result.Result, snap *deploy.Snapshot, changes display.ResourceChanges) {
					assert.Equal(l, 1, changes[deploy.OpDelete], "expected a delete operation")
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:simple", provider.Type.String(), "expected simple provider")
					// check that only the expected resource is left in the snapshot
					simple := snap.Resources[2]
					assert.Equal(l, "simple:index:Resource", simple.Type.String(), "expected simple resource")
				},
			},
		},
	},
}
