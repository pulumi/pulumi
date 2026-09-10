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

					assert.Equal(l, resource.PropertyMap{
						"prefixed": resource.NewProperty([]resource.PropertyValue{
							resource.NewProperty("prefix-alpha"),
							resource.NewProperty("prefix-beta"),
							resource.NewProperty("prefix-gamma"),
						}),
						"filtered": resource.NewProperty([]resource.PropertyValue{
							resource.NewProperty("alpha"),
							resource.NewProperty("gamma"),
						}),
						"indexed": resource.NewProperty([]resource.PropertyValue{
							resource.NewProperty("0:alpha"),
							resource.NewProperty("1:beta"),
							resource.NewProperty("2:gamma"),
						}),
						"tagList": resource.NewProperty([]resource.PropertyValue{
							resource.NewProperty("Environment=production"),
							resource.NewProperty("Team=infra"),
						}),
						"prefixedMap": resource.NewProperty(resource.PropertyMap{
							"alpha": resource.NewProperty("prefix-alpha"),
							"beta":  resource.NewProperty("prefix-beta"),
							"gamma": resource.NewProperty("prefix-gamma"),
						}),
						"filteredTags": resource.NewProperty(resource.PropertyMap{
							"Environment": resource.NewProperty("production"),
						}),
					}, stack.Outputs)
				},
			},
		},
	}
}
