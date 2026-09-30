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
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/stretchr/testify/require"
)

// This test pins the desired invoke-dependency semantics: a consumer of an invoke's return value
// must depend on the union of every resource that fed the invoke's args.
func init() {
	LanguageTests["l2-invoke-union-dependencies"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleInvokeProvider{} },
			func() plugin.Provider { return &providers.SimpleProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					var a, b, d *pkgresource.State
					for _, r := range res.Snap.Resources {
						switch r.URN.Name() {
						case "a":
							a = r
						case "b":
							b = r
						case "d":
							d = r
						}
					}
					require.NotNil(l, a, "expected resource a")
					require.NotNil(l, b, "expected resource b")
					require.NotNil(l, d, "expected resource d")

					require.Empty(l, a.Dependencies, "a has no invoke inputs")
					require.Empty(l, b.Dependencies, "b has no invoke inputs")

					// d.text was set from data.response, where data is the result of an invoke that read from both
					// a and b. Even though `response` is derived from just `a.text`, SDKs propagate the union of
					// all invoke arg dependencies to the result, so d must depend on both a and b.
					require.ElementsMatch(l, []pkgresource.URN{a.URN, b.URN}, d.Dependencies,
						"d must depend on both a and b (union of invoke arg dependencies)")

					textDeps, ok := d.PropertyDependencies["text"]
					require.True(l, ok, "expected d.PropertyDependencies to include 'text'")
					require.ElementsMatch(l, []pkgresource.URN{a.URN, b.URN}, textDeps,
						"d.text must be attributed to both invoke arg source resources")
				},
			},
		},
	}
}
