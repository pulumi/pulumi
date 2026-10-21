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
)

func init() {
	LanguageTests["l2-builtin-object"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.OutputProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")

					// Entries must be sorted alphabetically by key.
					assert.Equal(l, property.NewMap(map[string]property.Value{
						"entriesOutput": property.New([]property.Value{
							property.New(map[string]property.Value{
								"key":   property.New("a"),
								"value": property.New("hello"),
							}),
							property.New(map[string]property.Value{
								"key":   property.New("d"),
								"value": property.New("hello"),
							}),
							property.New(map[string]property.Value{
								"key":   property.New("m"),
								"value": property.New("hello"),
							}),
							property.New(map[string]property.Value{
								"key":   property.New("t"),
								"value": property.New("hello"),
							}),
							property.New(map[string]property.Value{
								"key":   property.New("x"),
								"value": property.New("hello"),
							}),
							property.New(map[string]property.Value{
								"key":   property.New("z"),
								"value": property.New("hello"),
							}),
						}),
						"lookupOutput":        property.New("hello"),
						"lookupOutputDefault": property.New("default"),
						"entriesObjectOutput": property.New([]property.Value{
							property.New(map[string]property.Value{
								"key":   property.New("output"),
								"value": property.New("hello"),
							}),
						}),
						"lookupObjectOutput":        property.New("hello"),
						"lookupObjectOutputDefault": property.New("default"),
						"lengthOutput":              property.New(6.0),
						"lengthObjectOutput":        property.New(1.0),
					}), resource.FromResourcePropertyMap(stack.Outputs))
				},
			},
		},
	}
}
