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
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l1-output-array"] = LanguageTest{
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					RequireStackResource(l, err, changes)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := stack.Outputs

					require.Len(l, outputs, 5, "expected 5 outputs")
					AssertPropertyMapMember(l, outputs, "empty", resource.NewArrayProperty([]resource.PropertyValue{}))
					AssertPropertyMapMember(l, outputs, "small", resource.NewArrayProperty([]resource.PropertyValue{
						resource.NewStringProperty("Hello"),
						resource.NewStringProperty("World"),
					}))
					AssertPropertyMapMember(l, outputs, "numbers", resource.NewArrayProperty([]resource.PropertyValue{
						resource.NewNumberProperty(0), resource.NewNumberProperty(1), resource.NewNumberProperty(2),
						resource.NewNumberProperty(3), resource.NewNumberProperty(4), resource.NewNumberProperty(5),
					}))
					AssertPropertyMapMember(l, outputs, "nested", resource.NewArrayProperty([]resource.PropertyValue{
						resource.NewArrayProperty([]resource.PropertyValue{
							resource.NewNumberProperty(1), resource.NewNumberProperty(2), resource.NewNumberProperty(3),
						}),
						resource.NewArrayProperty([]resource.PropertyValue{
							resource.NewNumberProperty(4), resource.NewNumberProperty(5), resource.NewNumberProperty(6),
						}),
						resource.NewArrayProperty([]resource.PropertyValue{
							resource.NewNumberProperty(7), resource.NewNumberProperty(8), resource.NewNumberProperty(9),
						}),
					}))

					large := []resource.PropertyValue{}
					for i := 0; i < 150; i++ {
						large = append(large, resource.NewStringProperty(lorem))
					}
					AssertPropertyMapMember(l, outputs, "large", resource.NewArrayProperty(large))
				},
			},
		},
	}
}
