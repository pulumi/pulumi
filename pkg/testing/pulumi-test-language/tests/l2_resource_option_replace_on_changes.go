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
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-option-replace-on-changes"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ReplaceOnChangesProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					snap := res.Snap

					RequireStackResource(l, res.Err, res.Changes)

					require.Len(l, snap.Resources, 10, "expected 10 resources in snapshot")

					RequireSingleNamedResource(l, snap.Resources, "schemaReplace")
					RequireSingleNamedResource(l, snap.Resources, "optionReplace")
					RequireSingleNamedResource(l, snap.Resources, "bothReplaceValue")
					RequireSingleNamedResource(l, snap.Resources, "bothReplaceProp")
					RequireSingleNamedResource(l, snap.Resources, "regularUpdate")
					RequireSingleNamedResource(l, snap.Resources, "noChange")
					RequireSingleNamedResource(l, snap.Resources, "wrongPropChange")
					RequireSingleNamedResource(l, snap.Resources, "multiplePropReplace")
				},
			},
			{
				AssertPreview: func(l *L, res AssertPreviewArgs) {
					var schemaOp, optionOp, bothValueOp, bothPropOp, regularOp, noChangeOp,
						wrongPropOp, multipleOp display.StepOp

					for _, evt := range res.Events {
						switch e := evt.Payload().(type) {
						case engine.ResourcePreEventPayload:
							switch e.Metadata.URN.Name() {
							case "schemaReplace":
								schemaOp = e.Metadata.Op
							case "optionReplace":
								optionOp = e.Metadata.Op
							case "bothReplaceValue":
								bothValueOp = e.Metadata.Op
							case "bothReplaceProp":
								bothPropOp = e.Metadata.Op
							case "regularUpdate":
								regularOp = e.Metadata.Op
							case "noChange":
								noChangeOp = e.Metadata.Op
							case "wrongPropChange":
								wrongPropOp = e.Metadata.Op
							case "multiplePropReplace":
								multipleOp = e.Metadata.Op
							}
						}
					}

					require.Contains(l, schemaOp, deploy.OpReplace,
						"schemaReplace: replaceProp change should trigger replacement")
					require.Contains(l, optionOp, deploy.OpReplace,
						"optionReplace: value change should trigger replacement")
					require.Contains(l, bothValueOp, deploy.OpReplace,
						"bothReplaceValue: value change should trigger replacement")
					require.Contains(l, bothPropOp, deploy.OpReplace,
						"bothReplaceProp: replaceProp change should trigger replacement")
					require.Equal(l, regularOp, deploy.OpUpdate,
						"regularUpdate: value change should update, not replace")
					require.Equal(l, noChangeOp, deploy.OpSame,
						"noChange: no property change should result in no operation")
					require.Contains(l, wrongPropOp, deploy.OpReplace,
						"wrongPropChange: replaceProp has schema-based replaceOnChanges, should trigger replacement")
					require.Contains(l, multipleOp, deploy.OpReplace,
						"multiplePropReplace: change to any marked property should trigger replacement")
				},
				Assert: func(l *L, res AssertArgs) {
					require.Len(l, res.Snap.Resources, 10, "expected 10 resources in snapshot")

					var schemaOp, optionOp, bothValueOp, bothPropOp, regularOp, noChangeOp,
						wrongPropOp, multipleOp display.StepOp

					for _, evt := range res.Events {
						switch e := evt.Payload().(type) {
						case engine.ResourcePreEventPayload:
							switch e.Metadata.URN.Name() {
							case "schemaReplace":
								schemaOp = e.Metadata.Op
							case "optionReplace":
								optionOp = e.Metadata.Op
							case "bothReplaceValue":
								bothValueOp = e.Metadata.Op
							case "bothReplaceProp":
								bothPropOp = e.Metadata.Op
							case "regularUpdate":
								regularOp = e.Metadata.Op
							case "noChange":
								noChangeOp = e.Metadata.Op
							case "wrongPropChange":
								wrongPropOp = e.Metadata.Op
							case "multiplePropReplace":
								multipleOp = e.Metadata.Op
							}
						}
					}

					require.Contains(l, schemaOp, deploy.OpReplace,
						"schemaReplace: should have been replaced")
					require.Contains(l, optionOp, deploy.OpReplace,
						"optionReplace: should have been replaced")
					require.Contains(l, bothValueOp, deploy.OpReplace,
						"bothReplaceValue: should have been replaced")
					require.Contains(l, bothPropOp, deploy.OpReplace,
						"bothReplaceProp: should have been replaced")
					require.Equal(l, regularOp, deploy.OpUpdate,
						"regularUpdate: should have been updated, not replaced")
					require.Equal(l, noChangeOp, deploy.OpSame,
						"noChange: should have no operation")
					require.Contains(l, wrongPropOp, deploy.OpReplace,
						"wrongPropChange: should have been replaced (schema-based replaceOnChanges on replaceProp)")
					require.Contains(l, multipleOp, deploy.OpReplace,
						"multiplePropReplace: should have been replaced")
				},
			},
		},
	}

	LanguageTests["provider-replace-on-changes-component"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.ConformanceComponentProvider{} },
		},
		LanguageProviders: []string{"conformance-component"},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes
					RequireStackResource(l, err, changes)

					// Stack, 2 providers, 2 components, 2 res-child, 1 simpleResource.
					require.Len(l, snap.Resources, 8, "expected 8 resources in snapshot")

					withReplace := RequireSingleNamedResource(l, snap.Resources, "withReplaceOnChanges")
					assert.Equal(l, []string{"value"}, withReplace.ReplaceOnChanges,
						"expected component with replaceOnChanges to have ReplaceOnChanges set")

					withoutReplace := RequireSingleNamedResource(l, snap.Resources, "withoutReplaceOnChanges")
					assert.Empty(l, withoutReplace.ReplaceOnChanges,
						"expected component without replaceOnChanges to have empty ReplaceOnChanges")
				},
			},
			{
				AssertPreview: func(l *L, res AssertPreviewArgs) {
					var ops []display.StepOp
					for _, evt := range res.Events {
						switch e := evt.Payload().(type) {
						case engine.ResourcePreEventPayload:
							if e.Metadata.URN.Type().String() == "conformance-component:index:Simple" &&
								e.Metadata.URN.Name() == "withReplaceOnChanges" {
								ops = append(ops, e.Metadata.Op)
							}
						}
					}
					require.NotEmpty(l, ops, "expected to find step event for withReplaceOnChanges component")
					if !assert.Contains(l, ops, deploy.OpReplace) {
						require.Contains(l, ops, deploy.OpUpdate,
							"expected component preview to include either replace or update when value changes")
					}
				},
				Assert: func(l *L, res AssertArgs) {
					require.NoError(l, res.Err, "expected no error")
					require.GreaterOrEqual(l, res.Changes[deploy.OpReplace], 1,
						"expected at least one replace operation")

					require.Len(l, res.Snap.Resources, 8, "expected 8 resources in snapshot")
					withReplace := RequireSingleNamedResource(l, res.Snap.Resources, "withReplaceOnChanges")
					want := resource.NewPropertyMapFromMap(map[string]any{"value": false})
					assert.Equal(l, want, withReplace.Inputs, "expected component to have value=false after replace")
					assert.Equal(l, []string{"value"}, withReplace.ReplaceOnChanges,
						"expected component to retain ReplaceOnChanges after replace")
				},
			},
		},
	}
}
