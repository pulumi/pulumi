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
	LanguageTests["provider-replacement-trigger-component"] = LanguageTest{
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

					require.Len(l, snap.Resources, 6, "expected 6 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:conformance-component")
					component := RequireSingleResource(l, snap.Resources, "conformance-component:index:Simple")

					want := resource.NewPropertyMapFromMap(map[string]any{"value": true})
					assert.Equal(l, want, component.Inputs, "expected inputs to be {value: true}")
					assert.Equal(l, component.Inputs, component.Outputs, "expected inputs and outputs to match")

					require.EqualValues(
						l, resource.NewProperty("trigger-value"),
						component.ReplacementTrigger,
						"expected component to have replacementTrigger set",
					)

					RequireSingleResource(l, snap.Resources, "pulumi:providers:simple")
					simple := RequireSingleNamedResource(l, snap.Resources, "res-child")

					want = resource.NewPropertyMapFromMap(map[string]any{"value": false})
					assert.Equal(l, want, simple.Inputs, "expected inputs to be {value: false}")
					assert.Equal(l, simple.Inputs, simple.Outputs, "expected inputs and outputs to match")

					assert.Equal(l, component.URN, simple.Parent, "expected component to be parent of simple resource")
				},
			},
			{
				AssertPreview: func(l *L, res AssertPreviewArgs) {
					projectDirectory := res.ProjectDirectory
					err := res.Err
					plan := res.Plan
					changes := res.Changes
					events := res.Events
					sdks := res.SDKs
					_, _, _, _, _, _ = projectDirectory, err, plan, changes, events, sdks

					var ops []display.StepOp
					for _, evt := range events {
						switch e := evt.Payload().(type) {
						case engine.ResourcePreEventPayload:
							if e.Metadata.URN.Type().String() == "conformance-component:index:Simple" {
								ops = append(ops, e.Metadata.Op)
							}
						}
					}
					require.NotNil(l, ops, "expected to find step event metadata for component resource")
					require.Contains(l, ops, deploy.OpReplace,
						"expected component resource to be replaced during preview when replacementTrigger changes")
				},
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes
					events := res.Events
					require.NoError(l, err, "expected no error")

					require.GreaterOrEqual(l, changes[deploy.OpReplace], 1,
						"expected at least one replace operation")

					require.Len(l, snap.Resources, 6, "expected 6 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:conformance-component")
					component := RequireSingleResource(l, snap.Resources, "conformance-component:index:Simple")

					want := resource.NewPropertyMapFromMap(map[string]any{"value": true})
					assert.Equal(l, want, component.Inputs, "expected inputs to be {value: true}")
					assert.Equal(l, component.Inputs, component.Outputs, "expected inputs and outputs to match")

					require.EqualValues(
						l, resource.NewProperty("trigger-value-updated"),
						component.ReplacementTrigger,
						"expected component to have updated replacementTrigger",
					)

					var ops []display.StepOp
					for _, evt := range events {
						switch e := evt.Payload().(type) {
						case engine.ResourcePreEventPayload:
							if e.Metadata.URN.Type().String() == "conformance-component:index:Simple" {
								ops = append(ops, e.Metadata.Op)
							}
						}
					}
					require.NotNil(l, ops, "expected to find step event metadata for component resource")
					require.Contains(l, ops, deploy.OpReplace,
						"expected component resource to be replaced when replacementTrigger changes")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:simple")
					simple := RequireSingleNamedResource(l, snap.Resources, "res-child")

					want = resource.NewPropertyMapFromMap(map[string]any{"value": false})
					assert.Equal(l, want, simple.Inputs, "expected inputs to be {value: false}")
					assert.Equal(l, simple.Inputs, simple.Outputs, "expected inputs and outputs to match")

					assert.Equal(l, component.URN, simple.Parent, "expected component to be parent of simple resource")
				},
			},
		},
	}
}
