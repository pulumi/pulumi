// Copyright 2026, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-plain-component"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.PlainComponentProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					// We expect:
					// 0. The stack
					// 1. The default plaincomponent provider
					// 2. The component resource
					// 3. The child custom resource
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:plaincomponent")

					comp := RequireSingleNamedResource(l, snap.Resources, "myComponent")
					require.Equal(
						l, "plaincomponent:index:Component", comp.Type.String(),
						"expected component type",
					)
					require.Equal(
						l, "my-resource", comp.Outputs["label"].StringValue(),
						"expected label output",
					)

					child := RequireSingleNamedResource(l, snap.Resources, "myComponent-child")
					require.Equal(
						l, "plaincomponent:index:Custom", child.Type.String(),
						"expected child custom type",
					)
					require.Equal(
						l, "my-resource", child.Outputs["value"].StringValue(),
						"expected child value output",
					)
				},
			},
		},
	}
}
