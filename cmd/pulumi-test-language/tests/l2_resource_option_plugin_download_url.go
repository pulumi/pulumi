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
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	commonproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-option-plugin-download-url"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider {
				// New version so that the pluginDownloadURL doesn't conflict depending on who generates
				// the SDK.
				return &providers.SimpleProvider{
					Version:           &semver.Version{Major: 27},
					PluginDownloadURL: "https://github.com/pulumi/pulumi-simple/releases/v${VERSION}",
				}
			},
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					// Count resources: 1 stack + 3 providers + 4 resources = 8 total
					require.Equal(l, 8, len(res.Snap.Resources),
						"expected 8 resources (1 stack + 3 providers + 4 resources)")

					// Get the four resources
					withDefaultURL := RequireSingleNamedResource(l, res.Snap.Resources, "withDefaultURL")
					withExplicitDefaultURL := RequireSingleNamedResource(l, res.Snap.Resources, "withExplicitDefaultURL")
					withCustomURL1 := RequireSingleNamedResource(l, res.Snap.Resources, "withCustomURL1")
					withCustomURL2 := RequireSingleNamedResource(l, res.Snap.Resources, "withCustomURL2")

					// Find provider resources for any URL
					findProvider := func(providerRef string) *resource.State {
						urn, err := resource.ParseURN(providerRef[:strings.LastIndex(providerRef, "::")])
						require.NoError(l, err)
						for _, r := range res.Snap.Resources {
							if commonproviders.IsProviderType(r.Type) && r.URN == urn {
								return r
							}
						}
						require.Fail(l, "Not found", "Unable to find provider matching ref %q", providerRef)
						return nil
					}

					defaultProvider := findProvider(withDefaultURL.Provider)
					explicitDefaultProvider := findProvider(withExplicitDefaultURL.Provider)
					customProvider1 := findProvider(withCustomURL1.Provider)
					customProvider2 := findProvider(withCustomURL2.Provider)

					assert.Equal(l, defaultProvider, explicitDefaultProvider)
					assert.True(l, strings.HasPrefix(withDefaultURL.Provider, string(defaultProvider.URN)),
						"expected %q to be a prefix of %q", string(defaultProvider.URN), withDefaultURL.Provider)

					// Verify withDefaultURL and withExplicitDefaultURL use the same provider
					assert.Equal(l, withDefaultURL.Provider, withExplicitDefaultURL.Provider,
						"resources with default and explicit default URLs should use the same provider")

					// Verify custom URL providers have the correct pluginDownloadURL in __internal
					url1 := customProvider1.Inputs["__internal"].ObjectValue()["pluginDownloadURL"].StringValue()
					assert.Equal(l, "https://custom.pulumi.test/provider1", url1,
						"customProvider1 should have correct pluginDownloadURL")

					url2 := customProvider2.Inputs["__internal"].ObjectValue()["pluginDownloadURL"].StringValue()
					assert.Equal(l, "https://custom.pulumi.test/provider2", url2,
						"customProvider2 should have correct pluginDownloadURL")

					// Verify custom providers are different
					assert.NotEqual(l, customProvider1.URN, customProvider2.URN,
						"resources with different custom URLs should use different providers")
				},
			},
		},
	}
}
