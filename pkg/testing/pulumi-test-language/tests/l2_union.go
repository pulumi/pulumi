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
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
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
				stackOutputs := resource.FromResourcePropertyMap(stack.Outputs)

				l.Logf("# Checking Map[Map[Union]]")

				mmu := RequireSingleNamedResource(l, snapshot.Resources, "mapMapUnionExample")
				mmuOutputs := resource.FromResourcePropertyMap(mmu.Outputs)

				expected := property.New(map[string]property.Value{
					"key1": property.New(map[string]property.Value{
						"key1a": property.New("value1a"),
					}),
				})

				require.Equal(l, expected, mmuOutputs.Get("mapMapUnionProperty"))
				require.Equal(l, expected, stackOutputs.Get("mapMapUnionOutput"))

				l.Logf("# Checking Union[String,Int]")

				si1 := RequireSingleNamedResource(l, snapshot.Resources, "stringOrIntegerExample1")
				si2 := RequireSingleNamedResource(l, snapshot.Resources, "stringOrIntegerExample2")

				require.Equal(l, property.New(42.0),
					resource.FromResourcePropertyMap(si1.Outputs).Get("stringOrIntegerProperty"))

				require.Equal(l, property.New("forty two"),
					resource.FromResourcePropertyMap(si2.Outputs).Get("stringOrIntegerProperty"))

				l.Logf("# Checking List<Union<String, Enum>>")

				seul := RequireSingleNamedResource(l, snapshot.Resources, "stringEnumUnionListExample")
				require.Equal(l,
					property.New([]property.Value{
						property.New("Listen"),
						property.New("Send"),
						property.New("NotAnEnumValue"),
					}),
					resource.FromResourcePropertyMap(seul.Outputs).Get("stringEnumUnionListProperty"))

				l.Logf("# Checking typed enum (safe)")

				safeEnum := RequireSingleNamedResource(l, snapshot.Resources, "safeEnumExample")
				require.Equal(l,
					property.New("Block"),
					resource.FromResourcePropertyMap(safeEnum.Outputs).Get("typedEnumProperty"))

				l.Logf("# Checking typed enum (output)")

				enumOutputExample := RequireSingleNamedResource(l, snapshot.Resources, "enumOutputExample")
				require.Equal(l,
					property.New("Block"),
					resource.FromResourcePropertyMap(enumOutputExample.Outputs).Get("type"))

				outputEnum := RequireSingleNamedResource(l, snapshot.Resources, "outputEnumExample")
				require.Equal(l,
					property.New("Block"),
					resource.FromResourcePropertyMap(outputEnum.Outputs).Get("typedEnumProperty"))
			},
		}},
	}
}
