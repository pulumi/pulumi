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
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l1-resource-keyword-overlap"] = LanguageTest{
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					require.NotEmpty(l, snap.Resources, "expected at least 1 resource")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")

					outputs := stack.Outputs

					AssertPropertyMapMember(l, outputs, "output_class", resource.NewStringProperty("class_output_string"))
					AssertPropertyMapMember(l, outputs, "output_export", resource.NewStringProperty("export_output_string"))
					AssertPropertyMapMember(l, outputs, "output_import", resource.NewStringProperty("import_output_string"))
					AssertPropertyMapMember(l, outputs, "output_mod", resource.NewStringProperty("mod_output_string"))
					AssertPropertyMapMember(l, outputs, "output_object",
						resource.NewObjectProperty(resource.PropertyMap{"object": resource.NewStringProperty("object_output_string")}),
					)
					AssertPropertyMapMember(l, outputs, "output_self", resource.NewStringProperty("self_output_string"))
					AssertPropertyMapMember(l, outputs, "output_this", resource.NewStringProperty("this_output_string"))
				},
			},
		},
	}
}
