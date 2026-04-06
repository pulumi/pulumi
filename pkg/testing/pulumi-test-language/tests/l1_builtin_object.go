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
	"github.com/stretchr/testify/assert"
)

func init() {
	LanguageTests["l1-builtin-object"] = LanguageTest{
		Runs: []TestRun{
			{
				Config: config.Map{
					config.MustMakeKey("l1-builtin-object", "aMap"): config.NewObjectValue(
						`{"keyPresent":"value","alpha":"a","delta":"d","omega":"o","tango":"t","zebra":"z"}`),
				},
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")

					// Entries must be sorted alphabetically by key.
					assert.Equal(l, resource.PropertyMap{
						"entriesOutput": resource.NewProperty([]resource.PropertyValue{
							resource.NewProperty(resource.PropertyMap{
								"key":   resource.NewProperty("alpha"),
								"value": resource.NewProperty("a"),
							}),
							resource.NewProperty(resource.PropertyMap{
								"key":   resource.NewProperty("delta"),
								"value": resource.NewProperty("d"),
							}),
							resource.NewProperty(resource.PropertyMap{
								"key":   resource.NewProperty("keyPresent"),
								"value": resource.NewProperty("value"),
							}),
							resource.NewProperty(resource.PropertyMap{
								"key":   resource.NewProperty("omega"),
								"value": resource.NewProperty("o"),
							}),
							resource.NewProperty(resource.PropertyMap{
								"key":   resource.NewProperty("tango"),
								"value": resource.NewProperty("t"),
							}),
							resource.NewProperty(resource.PropertyMap{
								"key":   resource.NewProperty("zebra"),
								"value": resource.NewProperty("z"),
							}),
						}),
						"lookupOutput":        resource.NewProperty("value"),
						"lookupOutputDefault": resource.NewProperty("default"),
					}, stack.Outputs)
				},
			},
		},
	}
}
