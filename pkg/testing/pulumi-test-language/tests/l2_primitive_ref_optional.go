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
	LanguageTests["l2-primitive-ref-optional"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.OptionalPrimitiveRefProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					// stack + provider + 2 resources
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:optional-primitive-ref")
					setRes := RequireSingleNamedResource(l, snap.Resources, "setRes")
					unsetRes := RequireSingleNamedResource(l, snap.Resources, "unsetRes")

					setWant := resource.NewPropertyMapFromMap(map[string]any{
						"data": resource.NewPropertyMapFromMap(map[string]any{
							"boolean":     true,
							"float":       3.14,
							"integer":     42,
							"string":      "hello",
							"numberArray": []any{-1.0, 0.0, 1.0},
							"booleanMap":  map[string]any{"t": true, "f": false},
						}),
					})
					assert.Equal(l, setWant, setRes.Inputs, "setRes inputs")
					assert.Equal(l, setWant, setRes.Outputs, "setRes outputs")

					unsetWant := resource.NewPropertyMapFromMap(map[string]any{
						"data": resource.NewPropertyMapFromMap(map[string]any{}),
					})
					assert.Equal(l, unsetWant, unsetRes.Inputs, "unsetRes inputs")
					assert.Equal(l, unsetWant, unsetRes.Outputs, "unsetRes outputs")

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					outputs := stack.Outputs

					// Traversal of populated output object -> optional scalar yields the value.
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

					// Traversal of an output object with all unset inner fields yields null at each leaf.
					AssertPropertyMapMember(l, outputs, "unsetBoolean", resource.NewProperty("null"))
					AssertPropertyMapMember(l, outputs, "unsetFloat", resource.NewProperty("null"))
					AssertPropertyMapMember(l, outputs, "unsetInteger", resource.NewProperty("null"))
					AssertPropertyMapMember(l, outputs, "unsetString", resource.NewProperty("null"))
					AssertPropertyMapMember(l, outputs, "unsetNumberArray", resource.NewProperty("null"))
					AssertPropertyMapMember(l, outputs, "unsetBooleanMap", resource.NewProperty("null"))
				},
			},
		},
	}
}
