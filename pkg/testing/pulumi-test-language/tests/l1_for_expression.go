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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
)

func init() {
	LanguageTests["l1-for-expression"] = LanguageTest{
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					assert.Equal(l, property.NewMap(map[string]property.Value{
						"prefixed": property.New([]property.Value{
							property.New("prefix-alpha"),
							property.New("prefix-beta"),
							property.New("prefix-gamma"),
						}),
						"filtered": property.New([]property.Value{
							property.New("alpha"),
							property.New("gamma"),
						}),
						"indexed": property.New([]property.Value{
							property.New("0:alpha"),
							property.New("1:beta"),
							property.New("2:gamma"),
						}),
						"tagList": property.New([]property.Value{
							property.New("Environment=production"),
							property.New("Team=infra"),
						}),
						"prefixedMap": property.New(map[string]property.Value{
							"alpha": property.New("prefix-alpha"),
							"beta":  property.New("prefix-beta"),
							"gamma": property.New("prefix-gamma"),
						}),
						"filteredTags": property.New(map[string]property.Value{
							"Environment": property.New("production"),
						}),
					}), resource.FromResourcePropertyMap(stack.Outputs))
				},
			},
		},
	}
}
