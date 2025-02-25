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
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-component-property-deps"] = LanguageTest{
		Providers: []plugin.Provider{&providers.ComponentPropertyDepsProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					assertOutput := func(outputs resource.PropertyMap, key resource.PropertyKey) {
						require.Len(l, outputs, 1)
						require.Contains(l, outputs, key)
						deps := outputs[key]
						require.NotNil(l, deps)
						require.True(l, deps.IsObject())
						for _, key := range []resource.PropertyKey{
							"resource",
							"resourceList",
							"resourceMap",
						} {
							v := deps.ObjectValue()[key]
							require.True(l, v.IsArray())
							require.Empty(l, v.ArrayValue())
						}
					}

					component1 := RequireSingleNamedResource(l, snap.Resources, "component1")
					assertOutput(component1.Outputs, "propertyDeps")

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					assertOutput(stack.Outputs, "propertyDepsFromCall")
				},
			},
		},
	}
}
