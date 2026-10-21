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
	LanguageTests["l3-rewrite-conversions"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.PrimitiveProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 5, "expected 5 resources in snapshot")

					direct := RequireSingleNamedResource(l, snap.Resources, "direct")
					converted := RequireSingleNamedResource(l, snap.Resources, "converted")
					child := RequireSingleNamedResource(l, snap.Resources, "converted-res")

					require.Equal(l, converted.URN, child.Parent, "expected child resource to have converted component as parent")

					wantDirect := property.NewMap(map[string]property.Value{
						"boolean": property.New(true),
						"float":   property.New(3.14),
						"integer": property.New(42.0),
						"string":  property.New("false"),
						"numberArray": property.New([]property.Value{
							property.New(-1.0), property.New(0.0), property.New(1.0),
						}),
						"booleanMap": property.New(map[string]property.Value{
							"t": property.New(true),
							"f": property.New(false),
						}),
					})
					assert.Equal(l, wantDirect, resource.FromResourcePropertyMap(direct.Inputs),
						"expected direct inputs to be rewritten")

					wantComponentInputs := property.NewMap(map[string]property.Value{
						"boolean": property.New(false),
						"float":   property.New(2.5),
						"integer": property.New(7.0),
						"string":  property.New("true"),
						"numberArray": property.New([]property.Value{
							property.New(10.0), property.New(11.0),
						}),
						"booleanMap": property.New(map[string]property.Value{
							"left":  property.New(true),
							"right": property.New(false),
						}),
					})
					assert.Equal(l, wantComponentInputs, resource.FromResourcePropertyMap(converted.Inputs),
						"expected component inputs to be rewritten")
					assert.Equal(l, wantComponentInputs, resource.FromResourcePropertyMap(child.Inputs),
						"expected child inputs to reflect rewritten component inputs")
					assert.Equal(l, child.Inputs, child.Outputs, "expected child inputs and outputs to match")
				},
			},
		},
	}
}
