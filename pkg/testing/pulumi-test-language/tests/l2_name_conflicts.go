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
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-name-conflicts"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.NamesProvider{} },
			func() plugin.Provider { return &providers.ModuleFormatProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 5, "expected 5 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:names")
					RequireSingleResource(l, snap.Resources, "pulumi:providers:module-format")

					namesResource := RequireSingleNamedResource(l, snap.Resources, "namesResource")
					modResource := RequireSingleNamedResource(l, snap.Resources, "modResource")

					wantNames := resource.NewPropertyMapFromMap(map[string]any{
						"value": true,
					})
					assert.Equal(l, wantNames, namesResource.Inputs, "expected names resource inputs to be %v", wantNames)
					assert.Equal(l, wantNames, namesResource.Outputs, "expected names resource outputs to be %v", wantNames)

					wantMod := resource.NewPropertyMapFromMap(map[string]any{
						"text": "module-format",
					})
					assert.Equal(l, wantMod, modResource.Inputs, "expected module-format resource inputs to be %v", wantMod)
					assert.Equal(l, wantMod, modResource.Outputs, "expected module-format resource outputs to be %v", wantMod)

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					wantOutputs := resource.NewPropertyMapFromMap(map[string]any{
						"modResourceText":  "module-format",
						"modVariables":     "module-format",
						"namesResourceVal": true,
						"nameVariables":    true,
					})
					assert.Equal(l, wantOutputs, stack.Outputs, "expected stack outputs to be %v", wantOutputs)
				},
			},
		},
	}
}
