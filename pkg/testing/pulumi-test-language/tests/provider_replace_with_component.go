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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["provider-replace-with-component"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.ConformanceComponentProvider{} },
		},
		LanguageProviders: []string{"conformance-component"},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					// We expect: 1 stack, 2 providers, 3 components, 3 component children, and 1 simpleResource.
					require.Len(l, snap.Resources, 10, "expected 10 resources in snapshot")

					targetResource := RequireSingleNamedResource(l, snap.Resources, "target")
					replaceWith := RequireSingleNamedResource(l, snap.Resources, "replaceWith")
					notReplaceWith := RequireSingleNamedResource(l, snap.Resources, "notReplaceWith")

					// The target resource should:
					//
					// * not have its replaceWith property set
					require.Empty(
						l, targetResource.ReplaceWith,
						"expected target resource to not be replaced with any resource",
					)

					// The resource that is replaced with the target resource should:
					//
					// * have its replaceWith property set to the URN of the target resource
					require.Equal(
						l, []resource.URN{targetResource.URN}, replaceWith.ReplaceWith,
						"expected replaceWith resource to be replaced with target resource",
					)

					// The resource that is not replaced with any resource should:
					//
					// * have its replaceWith property not set
					require.Empty(
						l, notReplaceWith.ReplaceWith,
						"expected notReplaceWith resource to not be replaced with any resource",
					)
				},
			},
		},
	}
}
