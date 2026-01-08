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
)

func init() {
	LanguageTests["l1-keyword-overlap"] = LanguageTest{
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					projectDirectory, err, snap, changes, events, sdks := res.ProjectDirectory, res.Err, res.Snap, res.Changes, res.Events, res.SDKs
					_, _, _, _, _, _ = projectDirectory, err, snap, changes, events, sdks
					RequireStackResource(l, err, changes)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := stack.Outputs

					AssertPropertyMapMember(l, outputs, "class", resource.NewProperty("class_output_string"))
					AssertPropertyMapMember(l, outputs, "export", resource.NewProperty("export_output_string"))
					AssertPropertyMapMember(l, outputs, "import", resource.NewProperty("import_output_string"))
					AssertPropertyMapMember(l, outputs, "mod", resource.NewProperty("mod_output_string"))
					AssertPropertyMapMember(l, outputs, "object",
						resource.NewProperty(resource.PropertyMap{"object": resource.NewProperty("object_output_string")}),
					)
					AssertPropertyMapMember(l, outputs, "self", resource.NewProperty("self_output_string"))
					AssertPropertyMapMember(l, outputs, "this", resource.NewProperty("this_output_string"))
				},
			},
		},
	}
}
