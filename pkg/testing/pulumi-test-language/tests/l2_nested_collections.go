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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// l2-nested-collections exercises deeply nested collection types: a resource with a
// list-of-lists-of-lists of an object type and a map-of-maps-of-maps of strings, plus a
// resource whose `elementType` property collides with the `ElementType()` method that
// generated Go SDK types must implement. Converted from the `go-nested-collections`
// codegen test (pulumi/pulumi#9887).
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

					// The stack, the provider, and the two resources.
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")
					RequireSingleResource(l, snap.Resources, "pulumi:providers:nestedcollections")

					foo := RequireSingleResource(l, snap.Resources, "nestedcollections:index:Foo")
					want := resource.NewPropertyMapFromMap(map[string]any{
						"conditionSets": []any{
							[]any{
								[]any{
									map[string]any{"prop": "first"},
									map[string]any{"prop": "second"},
								},
							},
						},
						"privateEndpoint": map[string]any{
							"outer": map[string]any{
								"inner": map[string]any{
									"leaf": "deep",
								},
							},
						},
					})
					assert.Equal(l, want, foo.Outputs, "expected the computed nested collection outputs")

					elem := RequireSingleResource(l, snap.Resources, "nestedcollections:elementType:ElementType")
					wantElem := resource.NewPropertyMapFromMap(map[string]any{
						"elementType": map[string]any{"elementType": "nested"},
					})
					assert.Equal(l, wantElem, elem.Outputs, "expected the computed elementType output")

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					AssertPropertyMapMember(l, stack.Outputs, "secondProp", resource.NewProperty("second"))
					AssertPropertyMapMember(l, stack.Outputs, "leaf", resource.NewProperty("deep"))
					AssertPropertyMapMember(l, stack.Outputs, "elementType", resource.NewProperty("nested"))
				},
			},
		},
	}
}
