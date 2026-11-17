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
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
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

					want := property.NewMap(map[string]property.Value{
						"boolean": property.New(true),
						"float":   property.New(3.14),
						"integer": property.New(42.0),
						"string":  property.New("hello"),
						"numberArray": property.New([]property.Value{
							property.New(-1.0), property.New(0.0), property.New(1.0),
						}),
						"booleanMap": property.New(map[string]property.Value{
							"t": property.New(true), "f": property.New(false),
						}),
					})
					require.Equal(l, want, resource.FromResourcePropertyMap(setA.Inputs))
					require.Equal(l, want, resource.FromResourcePropertyMap(setA.Outputs))
					require.Equal(l, want, resource.FromResourcePropertyMap(setB.Inputs))
					require.Equal(l, want, resource.FromResourcePropertyMap(setB.Outputs))
					require.Equal(l, want, resource.FromResourcePropertyMap(fromPrimitive.Inputs))
					require.Equal(l, want, resource.FromResourcePropertyMap(fromPrimitive.Outputs))

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					outputs := resource.FromResourcePropertyMap(stack.Outputs)

					AssertPropertyMapMember(l, outputs, "unsetBoolean", property.New("null"))
					AssertPropertyMapMember(l, outputs, "unsetFloat", property.New("null"))
					AssertPropertyMapMember(l, outputs, "unsetInteger", property.New("null"))
					AssertPropertyMapMember(l, outputs, "unsetString", property.New("null"))
					AssertPropertyMapMember(l, outputs, "unsetNumberArray", property.New("null"))
					AssertPropertyMapMember(l, outputs, "unsetBooleanMap", property.New("null"))

					AssertPropertyMapMember(l, outputs, "setBoolean", property.New(true))
					AssertPropertyMapMember(l, outputs, "setFloat", property.New(3.14))
					AssertPropertyMapMember(l, outputs, "setInteger", property.New(42.0))
					AssertPropertyMapMember(l, outputs, "setString", property.New("hello"))
					AssertPropertyMapMember(l, outputs, "setNumberArray", property.New([]property.Value{
						property.New(-1.0),
						property.New(0.0),
						property.New(1.0),
					}))
					AssertPropertyMapMember(l, outputs, "setBooleanMap", property.New(map[string]property.Value{
						"t": property.New(true),
						"f": property.New(false),
					}))
				},
			},
		},
	}
}
