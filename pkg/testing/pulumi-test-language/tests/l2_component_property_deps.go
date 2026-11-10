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
	LanguageTests["l2-component-property-deps"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ComponentPropertyDepsProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					assertOutput := func(outputs property.Map, key string) {
						require.Equal(l, 1, outputs.Len())
						deps, ok := outputs.GetOk(key)
						require.True(l, ok)
						require.True(l, deps.IsMap())
						for _, k := range []string{
							"resource",
							"resourceList",
							"resourceMap",
						} {
							v, ok := deps.AsMap().GetOk(k)
							require.True(l, ok)
							require.True(l, v.IsArray())
							require.Empty(l, v.AsArray().AsSlice())
						}
					}

					component1 := RequireSingleNamedResource(l, snap.Resources, "component1")
					assertOutput(resource.FromResourcePropertyMap(component1.Outputs), "propertyDeps")

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					assertOutput(resource.FromResourcePropertyMap(stack.Outputs), "propertyDepsFromCall")
				},
			},
		},
	}
}
