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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l1-config-types"] = LanguageTest{
		Runs: []TestRun{
			{
				Config: config.Map{
					config.MustMakeKey("l1-config-types", "aNumber"):   config.NewValue("3.5"),
					config.MustMakeKey("l1-config-types", "aString"):   config.NewValue("Hello"),
					config.MustMakeKey("l1-config-types", "aMap"):      config.NewObjectValue("{\"a\": 1, \"b\": 2}"),
					config.MustMakeKey("l1-config-types", "anObject"):  config.NewObjectValue("{\"prop\": [true]}"),
					config.MustMakeKey("l1-config-types", "anyObject"): config.NewObjectValue("{\"a\": 10, \"b\": 20}"),
				},
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					require.NotEmpty(l, snap.Resources, "expected at least 1 resource")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")

					outputs := stack.Outputs

					assert.Len(l, outputs, 5, "expected 5 outputs")
					AssertPropertyMapMember(l, outputs, "theNumber", resource.NewNumberProperty(4.75))
					AssertPropertyMapMember(l, outputs, "theString", resource.NewStringProperty("Hello World"))
					AssertPropertyMapMember(l, outputs, "theMap", resource.NewObjectProperty(resource.PropertyMap{
						"a": resource.NewNumberProperty(2),
						"b": resource.NewNumberProperty(3),
					}))
					AssertPropertyMapMember(l, outputs, "theObject", resource.NewBoolProperty(true))
					AssertPropertyMapMember(l, outputs, "theThing", resource.NewNumberProperty(30))
				},
			},
		},
	}
}
