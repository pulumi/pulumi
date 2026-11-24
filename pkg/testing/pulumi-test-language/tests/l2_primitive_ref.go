// Copyright 2024, Pulumi Corporation.
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
	LanguageTests["l2-primitive-ref"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.PrimitiveRefProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					// Check we have the one simple resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:primitive-ref")
					simple := RequireSingleResource(l, snap.Resources, "primitive-ref:index:Resource")

					want := property.NewMap(map[string]property.Value{
						"data": property.New(map[string]property.Value{
							"boolean": property.New(false),
							"float":   property.New(2.17),
							"integer": property.New(-12.0),
							"string":  property.New("Goodbye"),
							"boolArray": property.New([]property.Value{
								property.New(false), property.New(true),
							}),
							"stringMap": property.New(map[string]property.Value{
								"two":   property.New("turtle doves"),
								"three": property.New("french hens"),
							}),
						}),
					})
					assert.Equal(l, want, resource.FromResourcePropertyMap(simple.Inputs),
						"expected inputs to be %v", want)
					assert.Equal(l, simple.Inputs, simple.Outputs, "expected inputs and outputs to match")
				},
			},
		},
	}
}
