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

					// stack + provider + 4 resources
					require.Len(l, snap.Resources, 6, "expected 6 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:optional-primitive-ref")
					setRes := RequireSingleNamedResource(l, snap.Resources, "setRes")
					unsetRes := RequireSingleNamedResource(l, snap.Resources, "unsetRes")
					fromNestedOptional := RequireSingleNamedResource(l, snap.Resources, "fromNestedOptional")

					setWant := property.NewMap(map[string]property.Value{
						"data": property.New(map[string]property.Value{
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
						}),
						"optionalData": property.New(map[string]property.Value{
							"string": property.New("optional parent"),
						}),
					})
					assert.Equal(l, setWant, resource.FromResourcePropertyMap(setRes.Inputs), "setRes inputs")
					assert.Equal(l, setWant, resource.FromResourcePropertyMap(setRes.Outputs), "setRes outputs")

					unsetWant := property.NewMap(map[string]property.Value{
						"data": property.New(property.Map{}),
					})
					assert.Equal(l, unsetWant, resource.FromResourcePropertyMap(unsetRes.Inputs), "unsetRes inputs")
					assert.Equal(l, unsetWant, resource.FromResourcePropertyMap(unsetRes.Outputs), "unsetRes outputs")

					fromNestedOptionalWant := property.NewMap(map[string]property.Value{
						"data": property.New(map[string]property.Value{
							"string": property.New("optional parent"),
						}),
					})
					assert.Equal(l, fromNestedOptionalWant,
						resource.FromResourcePropertyMap(fromNestedOptional.Inputs), "fromNestedOptional inputs")
					assert.Equal(l, fromNestedOptionalWant,
						resource.FromResourcePropertyMap(fromNestedOptional.Outputs), "fromNestedOptional outputs")

					// A list of optional list outputs keeps the element type of each list.
					fromOptionalLists := RequireSingleNamedResource(l, snap.Resources, "fromOptionalLists")
					fromOptionalListsWant := property.NewMap(map[string]property.Value{
						"data": property.New(map[string]property.Value{
							"numberArray": property.New([]property.Value{
								property.New(-1.0), property.New(0.0), property.New(1.0),
							}),
						}),
					})
					assert.Equal(l, fromOptionalListsWant,
						resource.FromResourcePropertyMap(fromOptionalLists.Inputs), "fromOptionalLists inputs")
					assert.Equal(l, fromOptionalListsWant,
						resource.FromResourcePropertyMap(fromOptionalLists.Outputs), "fromOptionalLists outputs")

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					outputs := resource.FromResourcePropertyMap(stack.Outputs)

					// Traversal of populated output object -> optional scalar yields the value.
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

					// Traversal of an output object with all unset inner fields yields null at each leaf.
					AssertPropertyMapMember(l, outputs, "unsetBoolean", property.New("null"))
					AssertPropertyMapMember(l, outputs, "unsetFloat", property.New("null"))
					AssertPropertyMapMember(l, outputs, "unsetInteger", property.New("null"))
					AssertPropertyMapMember(l, outputs, "unsetString", property.New("null"))
					AssertPropertyMapMember(l, outputs, "unsetNumberArray", property.New("null"))
					AssertPropertyMapMember(l, outputs, "unsetBooleanMap", property.New("null"))

					// Comparison of an optional object to null yields whether the object is set.
					AssertPropertyMapMember(l, outputs, "setOptionalData", property.New("not null"))
					AssertPropertyMapMember(l, outputs, "unsetOptionalData", property.New("null"))
				},
			},
		},
	}
}
