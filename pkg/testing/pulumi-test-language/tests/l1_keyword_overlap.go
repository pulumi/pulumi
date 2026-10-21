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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func init() {
	LanguageTests["l1-keyword-overlap"] = LanguageTest{
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := resource.FromResourcePropertyMap(stack.Outputs)

					AssertPropertyMapMember(l, outputs, "class", property.New("class_output_string"))
					AssertPropertyMapMember(l, outputs, "export", property.New("export_output_string"))
					AssertPropertyMapMember(l, outputs, "import", property.New("import_output_string"))
					AssertPropertyMapMember(l, outputs, "mod", property.New("mod_output_string"))
					AssertPropertyMapMember(l, outputs, "object",
						property.New(map[string]property.Value{"object": property.New("object_output_string")}),
					)
					AssertPropertyMapMember(l, outputs, "self", property.New("self_output_string"))
					AssertPropertyMapMember(l, outputs, "this", property.New("this_output_string"))
				},
			},
		},
	}
}
