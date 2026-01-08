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
	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-invoke-dynamic-function"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.AnyTypeFunctionProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					projectDirectory, err, snap, changes, events, sdks := res.ProjectDirectory, res.Err, res.Snap, res.Changes, res.Events, res.SDKs
					_, _, _, _, _, _ = projectDirectory, err, snap, changes, events, sdks
					RequireStackResource(l, err, changes)

					r := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					assert.Equal(l, resource.RootStackType, r.Type, "expected a stack resource")
					require.Len(l, r.Outputs, 1)

					AssertPropertyMapMember(
						l,
						r.Outputs,
						"dynamic",
						resource.NewProperty(
							resource.NewPropertyMapFromMap(
								map[string]any{"resultProperty": resource.NewProperty("resultValue")},
							),
						),
					)
				},
			},
		},
	}
}
