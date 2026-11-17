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
	LanguageTests["l2-map-keys"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.PrimitiveProvider{} },
			func() plugin.Provider { return &providers.PrimitiveRefProvider{} },
			func() plugin.Provider { return &providers.RefRefProvider{} },
			func() plugin.Provider { return &providers.PlainProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 9, "expected 9 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:primitive")
					primResource := RequireSingleResource(l, snap.Resources, "primitive:index:Resource")
					RequireSingleResource(l, snap.Resources, "pulumi:providers:primitive-ref")
					refResource := RequireSingleResource(l, snap.Resources, "primitive-ref:index:Resource")
					RequireSingleResource(l, snap.Resources, "pulumi:providers:ref-ref")
					rrefResource := RequireSingleResource(l, snap.Resources, "ref-ref:index:Resource")
					RequireSingleResource(l, snap.Resources, "pulumi:providers:plain")
					plainResource := RequireSingleResource(l, snap.Resources, "plain:index:Resource")

					stringMapAlt := func() property.Value {
						return property.New(map[string]property.Value{
							"my key": property.New("one"),
							"my.key": property.New("two"),
							"my-key": property.New("three"),
							"my_key": property.New("four"),
							"MY_KEY": property.New("five"),
							"myKey":  property.New("six"),
						})
					}

					want := property.NewMap(map[string]property.Value{
						"boolean": property.New(false),
						"float":   property.New(2.17),
						"integer": property.New(-12.0),
						"string":  property.New("Goodbye"),
						"numberArray": property.New([]property.Value{
							property.New(0.0), property.New(1.0),
						}),
						"booleanMap": property.New(map[string]property.Value{
							"my key": property.New(false),
							"my.key": property.New(true),
							"my-key": property.New(false),
							"my_key": property.New(true),
							"MY_KEY": property.New(false),
							"myKey":  property.New(true),
						}),
					})
					primInputs := resource.FromResourcePropertyMap(primResource.Inputs)
					assert.Equal(l, want, primInputs, "expected inputs to be %v", want)
					assert.Equal(l, primInputs,
						resource.FromResourcePropertyMap(primResource.Outputs),
						"expected inputs and outputs to match")

					want = property.NewMap(map[string]property.Value{
						"data": property.New(map[string]property.Value{
							"boolean":   property.New(false),
							"float":     property.New(2.17),
							"integer":   property.New(-12.0),
							"string":    property.New("Goodbye"),
							"boolArray": property.New([]property.Value{property.New(false), property.New(true)}),
							"stringMap": stringMapAlt(),
						}),
					})
					refInputs := resource.FromResourcePropertyMap(refResource.Inputs)
					assert.Equal(l, want, refInputs, "expected inputs to be %v", want)
					assert.Equal(l, refInputs,
						resource.FromResourcePropertyMap(refResource.Outputs),
						"expected inputs and outputs to match")

					want = property.NewMap(map[string]property.Value{
						"data": property.New(map[string]property.Value{
							"innerData": property.New(map[string]property.Value{
								"boolean":   property.New(false),
								"float":     property.New(-2.17),
								"integer":   property.New(123.0),
								"string":    property.New("Goodbye"),
								"boolArray": property.New([]property.Value{}),
								"stringMap": stringMapAlt(),
							}),
							"boolean":   property.New(true),
							"float":     property.New(4.5),
							"integer":   property.New(1024.0),
							"string":    property.New("Hello"),
							"boolArray": property.New([]property.Value{}),
							"stringMap": stringMapAlt(),
						}),
					})
					rrefInputs := resource.FromResourcePropertyMap(rrefResource.Inputs)
					assert.Equal(l, want, rrefInputs, "expected inputs to be %v", want)
					assert.Equal(l, rrefInputs,
						resource.FromResourcePropertyMap(rrefResource.Outputs),
						"expected inputs and outputs to match")

					want = property.NewMap(map[string]property.Value{
						"data": property.New(map[string]property.Value{
							"innerData": property.New(map[string]property.Value{
								"boolean":   property.New(false),
								"float":     property.New(2.17),
								"integer":   property.New(-12.0),
								"string":    property.New("Goodbye"),
								"boolArray": property.New([]property.Value{property.New(false), property.New(true)}),
								"stringMap": stringMapAlt(),
							}),
							"boolean":   property.New(true),
							"float":     property.New(4.5),
							"integer":   property.New(1024.0),
							"string":    property.New("Hello"),
							"boolArray": property.New([]property.Value{property.New(true), property.New(false)}),
							"stringMap": stringMapAlt(),
						}),
						"nonPlainData": property.New(map[string]property.Value{
							"innerData": property.New(map[string]property.Value{
								"boolean":   property.New(false),
								"float":     property.New(2.17),
								"integer":   property.New(-12.0),
								"string":    property.New("Goodbye"),
								"boolArray": property.New([]property.Value{property.New(false), property.New(true)}),
								"stringMap": stringMapAlt(),
							}),
							"boolean":   property.New(true),
							"float":     property.New(4.5),
							"integer":   property.New(1024.0),
							"string":    property.New("Hello"),
							"boolArray": property.New([]property.Value{property.New(true), property.New(false)}),
							"stringMap": stringMapAlt(),
						}),
					})
					plainInputs := resource.FromResourcePropertyMap(plainResource.Inputs)
					assert.Equal(l, want, plainInputs, "expected inputs to be %v", want)
					assert.Equal(l, plainInputs,
						resource.FromResourcePropertyMap(plainResource.Outputs),
						"expected inputs and outputs to match")
				},
			},
		},
	}
}
