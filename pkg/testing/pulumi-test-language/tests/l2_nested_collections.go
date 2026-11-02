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

// l2-nested-collections exercises deeply nested collection types: a resource with a
// list-of-lists-of-lists of an object type and a map-of-maps-of-maps of strings.
// Converted from the `go-nested-collections` codegen test (pulumi/pulumi#9887).
func init() {
	LanguageTests["l2-nested-collections"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.NestedCollectionsProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					snap := res.Snap

					// The stack, the provider, and the resource.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")
					RequireSingleResource(l, snap.Resources, "pulumi:providers:nestedcollections")

					foo := RequireSingleResource(l, snap.Resources, "nestedcollections:index:Foo")
					want := property.NewMap(map[string]property.Value{
						"conditionSets": property.New([]property.Value{
							property.New([]property.Value{
								property.New([]property.Value{
									property.New(map[string]property.Value{"prop": property.New("first")}),
									property.New(map[string]property.Value{"prop": property.New("second")}),
								}),
							}),
						}),
						"privateEndpoint": property.New(map[string]property.Value{
							"outer": property.New(map[string]property.Value{
								"inner": property.New(map[string]property.Value{
									"leaf": property.New("deep"),
								}),
							}),
						}),
					})
					assert.Equal(l, want, resource.FromResourcePropertyMap(foo.Outputs),
						"expected the computed nested collection outputs")

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					stackOutputs := resource.FromResourcePropertyMap(stack.Outputs)
					AssertPropertyMapMember(l, stackOutputs, "secondProp", property.New("second"))
					AssertPropertyMapMember(l, stackOutputs, "leaf", property.New("deep"))
				},
			},
		},
	}
}
