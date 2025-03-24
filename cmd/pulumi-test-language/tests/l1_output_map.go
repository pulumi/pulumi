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
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func init() {
	LanguageTests["l1-output-map"] = LanguageTest{
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := stack.Outputs

					assert.Len(l, outputs, 4, "expected 4 outputs")
					AssertPropertyMapMember(l, outputs, "empty", resource.NewObjectProperty(resource.PropertyMap{}))
					AssertPropertyMapMember(l, outputs, "strings", resource.NewObjectProperty(resource.PropertyMap{
						"greeting": resource.NewStringProperty("Hello, world!"),
						"farewell": resource.NewStringProperty("Goodbye, world!"),
					}))
					AssertPropertyMapMember(l, outputs, "numbers", resource.NewObjectProperty(resource.PropertyMap{
						"1": resource.NewNumberProperty(1),
						"2": resource.NewNumberProperty(2),
					}))
					AssertPropertyMapMember(l, outputs, "keys", resource.NewObjectProperty(resource.PropertyMap{
						"my.key": resource.NewNumberProperty(1),
						"my-key": resource.NewNumberProperty(2),
						"my_key": resource.NewNumberProperty(3),
						"MY_KEY": resource.NewNumberProperty(4),
						"mykey":  resource.NewNumberProperty(5),
						"MYKEY":  resource.NewNumberProperty(6),
					}))
				},
			},
		},
	}
}
