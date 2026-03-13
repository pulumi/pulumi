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
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-parameterized-resource-twice"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ParameterizedProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					require.Equal(l,
						resource.NewProperty("HelloWorld"),
						stack.Outputs["parameterValue1"],
						"first parameter value should be correct")
					require.Equal(l,
						resource.NewProperty("HelloWorldComponent"),
						stack.Outputs["parameterValueFromComponent1"],
						"first parameter value from component should be correct")
					require.Equal(l,
						resource.NewProperty("GoodbyeWorld"),
						stack.Outputs["parameterValue2"],
						"second parameter value should be correct")
					require.Equal(l,
						resource.NewProperty("GoodbyeWorldComponent"),
						stack.Outputs["parameterValueFromComponent2"],
						"second parameter value from component should be correct")
				},
			},
		},
	}
}
