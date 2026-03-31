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
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-optional"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.OptionalPrimitiveProvider{} },
			func() plugin.Provider { return &providers.PrimitiveProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)
					require.Len(l, snap.Resources, 9, "expected 9 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:optionalprimitive")
					RequireSingleResource(l, snap.Resources, "pulumi:providers:primitive")

					unsetA := RequireSingleNamedResource(l, snap.Resources, "unsetA")
					unsetB := RequireSingleNamedResource(l, snap.Resources, "unsetB")
					setA := RequireSingleNamedResource(l, snap.Resources, "setA")
					setB := RequireSingleNamedResource(l, snap.Resources, "setB")
					fromPrimitive := RequireSingleNamedResource(l, snap.Resources, "fromPrimitive")

					require.Empty(l, unsetA.Inputs, "unsetA inputs should be empty")
					require.Empty(l, unsetA.Outputs, "unsetA outputs should be empty")
					require.Empty(l, unsetB.Inputs, "unsetB inputs should be empty")
					require.Empty(l, unsetB.Outputs, "unsetB outputs should be empty")

					want := resource.NewPropertyMapFromMap(map[string]any{
						"boolean":     true,
						"float":       3.14,
						"integer":     42,
						"string":      "hello",
						"numberArray": []any{-1.0, 0.0, 1.0},
						"booleanMap":  map[string]any{"t": true, "f": false},
					})
					require.Equal(l, want, setA.Inputs)
					require.Equal(l, want, setA.Outputs)
					require.Equal(l, want, setB.Inputs)
					require.Equal(l, want, setB.Outputs)
					require.Equal(l, want, fromPrimitive.Inputs)
					require.Equal(l, want, fromPrimitive.Outputs)

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					outputs := stack.Outputs

					AssertPropertyMapMember(l, outputs, "unsetBoolean", resource.NewProperty("null"))
					AssertPropertyMapMember(l, outputs, "unsetFloat", resource.NewProperty("null"))
					AssertPropertyMapMember(l, outputs, "unsetInteger", resource.NewProperty("null"))
					AssertPropertyMapMember(l, outputs, "unsetString", resource.NewProperty("null"))
					AssertPropertyMapMember(l, outputs, "unsetNumberArray", resource.NewProperty("null"))
					AssertPropertyMapMember(l, outputs, "unsetBooleanMap", resource.NewProperty("null"))

					AssertPropertyMapMember(l, outputs, "setBoolean", resource.NewProperty(true))
					AssertPropertyMapMember(l, outputs, "setFloat", resource.NewProperty(3.14))
					AssertPropertyMapMember(l, outputs, "setInteger", resource.NewProperty(42.0))
					AssertPropertyMapMember(l, outputs, "setString", resource.NewProperty("hello"))
					AssertPropertyMapMember(l, outputs, "setNumberArray", resource.NewProperty([]resource.PropertyValue{
						resource.NewProperty(-1.0),
						resource.NewProperty(0.0),
						resource.NewProperty(1.0),
					}))
					AssertPropertyMapMember(l, outputs, "setBooleanMap", resource.NewProperty(resource.PropertyMap{
						"t": resource.NewProperty(true),
						"f": resource.NewProperty(false),
					}))
				},
			},
		},
	}
}
