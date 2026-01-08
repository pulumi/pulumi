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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l1-output-map"] = LanguageTest{
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					projectDirectory, err, snap, changes, events, sdks := res.ProjectDirectory, res.Err, res.Snap, res.Changes, res.Events, res.SDKs
					_, _, _, _, _, _ = projectDirectory, err, snap, changes, events, sdks
					RequireStackResource(l, err, changes)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := stack.Outputs

					require.Len(l, outputs, 4, "expected 4 outputs")
					AssertPropertyMapMember(l, outputs, "empty", resource.NewProperty(resource.PropertyMap{}))
					AssertPropertyMapMember(l, outputs, "strings", resource.NewProperty(resource.PropertyMap{
						"greeting": resource.NewProperty("Hello, world!"),
						"farewell": resource.NewProperty("Goodbye, world!"),
					}))
					AssertPropertyMapMember(l, outputs, "numbers", resource.NewProperty(resource.PropertyMap{
						"1": resource.NewProperty(1.0),
						"2": resource.NewProperty(2.0),
					}))
					AssertPropertyMapMember(l, outputs, "keys", resource.NewProperty(resource.PropertyMap{
						"my.key": resource.NewProperty(1.0),
						"my-key": resource.NewProperty(2.0),
						"my_key": resource.NewProperty(3.0),
						"MY_KEY": resource.NewProperty(4.0),
						"mykey":  resource.NewProperty(5.0),
						"MYKEY":  resource.NewProperty(6.0),
					}))
				},
			},
		},
	}
}
