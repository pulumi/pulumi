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
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
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

					want := resource.NewPropertyMapFromMap(map[string]any{
						"boolean":     false,
						"float":       2.17,
						"integer":     -12,
						"string":      "Goodbye",
						"numberArray": []any{0, 1},
						"booleanMap": map[string]any{
							"my key": false,
							"my.key": true,
							"my-key": false,
							"my_key": true,
							"MY_KEY": false,
							"myKey":  true,
						},
					})
					assert.Equal(l, want, primResource.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, primResource.Inputs, primResource.Outputs, "expected inputs and outputs to match")

					want = resource.NewPropertyMapFromMap(map[string]any{
						"data": resource.NewPropertyMapFromMap(map[string]any{
							"boolean":   false,
							"float":     2.17,
							"integer":   -12,
							"string":    "Goodbye",
							"boolArray": []any{false, true},
							"stringMap": map[string]any{
								"my key": "one",
								"my.key": "two",
								"my-key": "three",
								"my_key": "four",
								"MY_KEY": "five",
								"myKey":  "six",
							},
						}),
					})
					assert.Equal(l, want, refResource.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, refResource.Inputs, refResource.Outputs, "expected inputs and outputs to match")

					want = resource.NewPropertyMapFromMap(map[string]any{
						"data": resource.NewPropertyMapFromMap(map[string]any{
							"innerData": resource.NewPropertyMapFromMap(map[string]any{
								"boolean":   false,
								"float":     -2.17,
								"integer":   123,
								"string":    "Goodbye",
								"boolArray": []any{},
								"stringMap": map[string]any{
									"my key": "one",
									"my.key": "two",
									"my-key": "three",
									"my_key": "four",
									"MY_KEY": "five",
									"myKey":  "six",
								},
							}),
							"boolean":   true,
							"float":     4.5,
							"integer":   1024,
							"string":    "Hello",
							"boolArray": []any{},
							"stringMap": map[string]any{
								"my key": "one",
								"my.key": "two",
								"my-key": "three",
								"my_key": "four",
								"MY_KEY": "five",
								"myKey":  "six",
							},
						}),
					})
					assert.Equal(l, want, rrefResource.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, rrefResource.Inputs, rrefResource.Outputs, "expected inputs and outputs to match")

					want = resource.NewPropertyMapFromMap(map[string]any{
						"data": resource.NewPropertyMapFromMap(map[string]any{
							"innerData": resource.NewPropertyMapFromMap(map[string]any{
								"boolean":   false,
								"float":     2.17,
								"integer":   -12,
								"string":    "Goodbye",
								"boolArray": []any{false, true},
								"stringMap": map[string]any{
									"my key": "one",
									"my.key": "two",
									"my-key": "three",
									"my_key": "four",
									"MY_KEY": "five",
									"myKey":  "six",
								},
							}),
							"boolean":   true,
							"float":     4.5,
							"integer":   1024,
							"string":    "Hello",
							"boolArray": []any{true, false},
							"stringMap": map[string]any{
								"my key": "one",
								"my.key": "two",
								"my-key": "three",
								"my_key": "four",
								"MY_KEY": "five",
								"myKey":  "six",
							},
						}),
						"nonPlainData": resource.NewPropertyMapFromMap(map[string]any{
							"innerData": resource.NewPropertyMapFromMap(map[string]any{
								"boolean":   false,
								"float":     2.17,
								"integer":   -12,
								"string":    "Goodbye",
								"boolArray": []any{false, true},
								"stringMap": map[string]any{
									"my key": "one",
									"my.key": "two",
									"my-key": "three",
									"my_key": "four",
									"MY_KEY": "five",
									"myKey":  "six",
								},
							}),
							"boolean":   true,
							"float":     4.5,
							"integer":   1024,
							"string":    "Hello",
							"boolArray": []any{true, false},
							"stringMap": map[string]any{
								"my key": "one",
								"my.key": "two",
								"my-key": "three",
								"my_key": "four",
								"MY_KEY": "five",
								"myKey":  "six",
							},
						}),
					})
					assert.Equal(l, want, plainResource.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, plainResource.Inputs, plainResource.Outputs, "expected inputs and outputs to match")
				},
			},
		},
	}
}
