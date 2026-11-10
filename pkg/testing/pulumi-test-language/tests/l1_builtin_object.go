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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
)

func init() {
	LanguageTests["l1-builtin-object"] = LanguageTest{
		Runs: []TestRun{
			{
				Config: config.Map{
					config.MustMakeKey("l1-builtin-object", "aMap"): config.NewObjectValue(
						`{"keyPresent":"value","alpha":"a","delta":"d","omega":"o","tango":"t","zebra":"z"}`),
					config.MustMakeKey("l1-builtin-object", "alternativeNames"): config.NewObjectValue(
						`{"a":"first","b":"second"}`),
				},
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")

					// Entries must be sorted alphabetically by key.
					assert.Equal(l, property.NewMap(map[string]property.Value{
						"entriesOutput": property.New([]property.Value{
							property.New(map[string]property.Value{
								"key":   property.New("alpha"),
								"value": property.New("a"),
							}),
							property.New(map[string]property.Value{
								"key":   property.New("delta"),
								"value": property.New("d"),
							}),
							property.New(map[string]property.Value{
								"key":   property.New("keyPresent"),
								"value": property.New("value"),
							}),
							property.New(map[string]property.Value{
								"key":   property.New("omega"),
								"value": property.New("o"),
							}),
							property.New(map[string]property.Value{
								"key":   property.New("tango"),
								"value": property.New("t"),
							}),
							property.New(map[string]property.Value{
								"key":   property.New("zebra"),
								"value": property.New("z"),
							}),
						}),
						"lookupOutput":                property.New("value"),
						"lookupOutputDefault":         property.New("default"),
						"lookupLiteral":               property.New(3.0),
						"lookupLiteralMismatchedType": property.New(true),
						"deprecatedTwoArgumentLookup": property.New(1.0),
						"lengthOutput":                property.New(6.0),
						"lengthDynamicOutput":         property.New(2.0),
						"names": property.New([]property.Value{
							property.New("first"),
							property.New("second"),
						}),
					}), resource.FromResourcePropertyMap(stack.Outputs))
				},
			},
		},
	}
}
