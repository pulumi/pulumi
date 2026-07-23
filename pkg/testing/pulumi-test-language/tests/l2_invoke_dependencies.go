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
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-invoke-dependencies"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleInvokeProvider{} },
			func() plugin.Provider { return &providers.SimpleProvider{} },
		},
		Runs: []TestRun{
			{
				AssertPreview: func(l *L, args AssertPreviewArgs) {
					require.NoError(l, args.Err, "expected no error in preview")
				},
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)
					var first *pkgresource.State
					var second *pkgresource.State
					var third *pkgresource.State
					var fourth *pkgresource.State
					for _, r := range snap.Resources {
						switch r.URN.Name() {
						case "first":
							first = r
						case "second":
							second = r
						case "third":
							third = r
						case "fourth":
							fourth = r
						}
					}

					require.NotNil(l, first, "expected first resource")
					require.NotNil(l, second, "expected second resource")
					require.Empty(l, first.Dependencies, "expected no dependencies")
					require.Len(l, second.Dependencies, 1, "expected one dependency")
					dependencies, ok := second.PropertyDependencies["value"]
					require.True(l, ok, "expected dependency on property 'value'")
					require.Len(l, dependencies, 1, "expected one dependency")
					require.Equal(l, first.URN, dependencies[0], "expected second to depend on first")
					require.Equal(l, first.URN, second.Dependencies[0], "expected second to depend on first")

					require.NotNil(l, third, "expected third resource")
					require.NotNil(l, fourth, "expected fourth resource")
					require.Empty(l, third.Dependencies, "expected no dependencies")
					require.Len(l, fourth.Dependencies, 1, "expected one dependency")
					dependencies, ok = fourth.PropertyDependencies["text"]
					require.True(l, ok, "expected dependency on property 'text'")
					require.Len(l, dependencies, 1, "expected one dependency")
					require.Equal(l, third.URN, dependencies[0], "expected fourth to depend on third")
					require.Equal(l, third.URN, fourth.Dependencies[0], "expected fourth to depend on third")
					AssertPropertyMapMember(l, fourth.Inputs, "text", resource.NewProperty("Goodbye world"))
				},
			},
		},
	}
}
