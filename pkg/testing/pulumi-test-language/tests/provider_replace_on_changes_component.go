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
					require.Contains(l, ops, deploy.OpReplace,
						"expected component to be replaced when value changes (replaceOnChanges)")
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
