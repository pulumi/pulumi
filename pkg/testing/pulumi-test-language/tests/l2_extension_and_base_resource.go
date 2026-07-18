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
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

func init() {
	// Shared instance so the parameter persists across the harness's per-load
	// factory calls; see l2_extension_parameterized_resource.go.
	shared := &providers.ExtensionParameterizedProvider{}
	LanguageTests["l2-extension-and-base-resource"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return shared },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					stackOutputs := resource.ToResourcePropertyMap(stack.Outputs)
					AssertPropertyMapMember(l, stackOutputs, "parameterValue", resource.NewProperty("Hello"))
					AssertPropertyMapMember(l, stackOutputs, "baseValue", resource.NewProperty("base"))

					greeting := RequireSingleResource(l, snap.Resources, "extbase:index:Greeting")
					require.NotEmpty(l, greeting.ExtensionRef,
						"extension resource state must carry an ExtensionRef")
					require.Contains(l, greeting.Provider, "pulumi:providers:extbase::",
						"greeting must be served by the base provider")

					base := RequireSingleResource(l, snap.Resources, "extbase:index:Base")
					require.Empty(l, base.ExtensionRef,
						"base resource state must not carry an ExtensionRef")
					require.Contains(l, base.Provider, "pulumi:providers:extbase::",
						"base must be served by the base provider")
				},
			},
		},
	}
}
