// Copyright 2025, Pulumi Corporation.
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
	LanguageTests["l1-output-null"] = LanguageTest{
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := stack.Outputs

					assert.Len(l, outputs, 1, "expected 1 outputs")
					// TODO(https://github.com/pulumi/pulumi/issues/19015): Ideally we'd allow for nulls in the output
					// map and in nested maps, but to do that we need to work out how to handle optional fields in
					// resource properties as well. Is it ok to start sending nulls for optional fields in the resource
					// properties?

					// These lines are commented out to show what _should_ be here, but isn't because of the issue above.
					// AssertPropertyMapMember(l, outputs, "null", resource.NewNullProperty())
					AssertPropertyMapMember(l, outputs, "array", resource.NewArrayProperty([]resource.PropertyValue{
						resource.NewNullProperty(),
					}))
					//AssertPropertyMapMember(l, outputs, "map", resource.NewObjectProperty(resource.PropertyMap{
					//	"key": resource.NewNullProperty(),
					//}))
				},
			},
		},
	}
}
