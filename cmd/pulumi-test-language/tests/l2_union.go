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
	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-union"] = LanguageTest{
		Providers: []plugin.Provider{&providers.UnionProvider{}},
		Runs: []TestRun{{
			Assert: func(
				l *L,
				s1 string,
				err error,
				snapshot *deploy.Snapshot,
				changes display.ResourceChanges,
				e []engine.Event,
			) {
				RequireStackResource(l, err, changes)
				stack := RequireSingleResource(l, snapshot.Resources, "pulumi:pulumi:Stack")

				l.Logf("# Checking Map[Map[Union]]")

				mmu := RequireSingleNamedResource(l, snapshot.Resources, "mapMapUnionExample")

				expected := resource.NewObjectProperty(resource.PropertyMap{
					"key1": resource.NewObjectProperty(resource.PropertyMap{
						"key1a": resource.NewStringProperty("value1a"),
					}),
				})

				require.Equal(l, expected, mmu.Outputs["mapMapUnionProperty"])
				require.Equal(l, expected, stack.Outputs["mapMapUnionOutput"])

				l.Logf("# Checking Union[String,Int]")

				si1 := RequireSingleNamedResource(l, snapshot.Resources, "stringOrIntegerExample1")
				si2 := RequireSingleNamedResource(l, snapshot.Resources, "stringOrIntegerExample2")

				require.Equal(l, resource.NewNumberProperty(42),
					si1.Outputs["stringOrIntegerProperty"])

				require.Equal(l, resource.NewStringProperty("fourty two"),
					si2.Outputs["stringOrIntegerProperty"])
			},
		}},
	}
}
