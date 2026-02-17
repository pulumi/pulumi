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
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-union"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.UnionProvider{} },
		},
		Runs: []TestRun{{
			Assert: func(l *L, res AssertArgs) {
				err, snapshot, changes := res.Err, res.Snap, res.Changes
				RequireStackResource(l, err, changes)
				stack := RequireSingleResource(l, snapshot.Resources, "pulumi:pulumi:Stack")

				l.Logf("# Checking Map[Map[Union]]")

				mmu := RequireSingleNamedResource(l, snapshot.Resources, "mapMapUnionExample")

				expected := resource.NewProperty(resource.PropertyMap{
					"key1": resource.NewProperty(resource.PropertyMap{
						"key1a": resource.NewProperty("value1a"),
					}),
				})

				require.Equal(l, expected, mmu.Outputs["mapMapUnionProperty"])
				require.Equal(l, expected, stack.Outputs["mapMapUnionOutput"])

				l.Logf("# Checking Union[String,Int]")

				si1 := RequireSingleNamedResource(l, snapshot.Resources, "stringOrIntegerExample1")
				si2 := RequireSingleNamedResource(l, snapshot.Resources, "stringOrIntegerExample2")

				require.Equal(l, resource.NewProperty(42.0),
					si1.Outputs["stringOrIntegerProperty"])

				require.Equal(l, resource.NewProperty("forty two"),
					si2.Outputs["stringOrIntegerProperty"])

				l.Logf("# Checking List<Union<String, Enum>>")

				seul := RequireSingleNamedResource(l, snapshot.Resources, "stringEnumUnionListExample")
				require.Equal(l,
					resource.NewProperty([]resource.PropertyValue{
						resource.NewProperty("Listen"),
						resource.NewProperty("Send"),
						resource.NewProperty("NotAnEnumValue"),
					}),
					seul.Outputs["stringEnumUnionListProperty"])

				l.Logf("# Checking typed enum (safe)")

				safeEnum := RequireSingleNamedResource(l, snapshot.Resources, "safeEnumExample")
				require.Equal(l,
					resource.NewProperty("Block"),
					safeEnum.Outputs["typedEnumProperty"])

				l.Logf("# Checking typed enum (output)")

				enumOutputExample := RequireSingleNamedResource(l, snapshot.Resources, "enumOutputExample")
				require.Equal(l,
					resource.NewProperty("Block"),
					enumOutputExample.Outputs["type"])

				outputEnum := RequireSingleNamedResource(l, snapshot.Resources, "outputEnumExample")
				require.Equal(l,
					resource.NewProperty("Block"),
					outputEnum.Outputs["typedEnumProperty"])
			},
		}},
	}
}
