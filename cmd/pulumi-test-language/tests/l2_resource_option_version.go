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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-option-version"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{Version: &semver.Version{Major: 2}} },
			func() plugin.Provider { return &providers.SimpleProvider{Version: &semver.Version{Major: 26}} },
		},
		SkipEnsurePluginsValidation: true,
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					require.Len(l, res.Snap.Resources, 6)

					providers := RequireNResources(l, res.Snap.Resources, "pulumi:providers:simple", 2)

					var providerV2URN, providerV26URN resource.URN
					for _, prov := range providers {
						urnStr := string(prov.URN)
						if strings.Contains(urnStr, "2_0_0") {
							providerV2URN = prov.URN
						} else if strings.Contains(urnStr, "26_0_0") {
							providerV26URN = prov.URN
						}
					}
					require.NotEmpty(l, providerV2URN, "expected to find provider with version 2_0_0")
					require.NotEmpty(l, providerV26URN, "expected to find provider with version 26_0_0")

					withV2 := RequireSingleNamedResource(l, res.Snap.Resources, "withV2")
					withV26 := RequireSingleNamedResource(l, res.Snap.Resources, "withV26")
					withDefault := RequireSingleNamedResource(l, res.Snap.Resources, "withDefault")

					assert.Truef(l, strings.HasPrefix(withV2.Provider, string(providerV2URN)),
						"expected %s to prefix %s", providerV2URN, withV2.Provider)
					assert.True(l, withV2.Outputs["value"].BoolValue())

					assert.Truef(l, strings.HasPrefix(withV26.Provider, string(providerV26URN)),
						"expected %s to prefix %s", providerV26URN, withV26.Provider)
					assert.False(l, withV26.Outputs["value"].BoolValue())

					assert.Truef(l, strings.HasPrefix(withDefault.Provider, string(providerV26URN)),
						"expected %s to prefix %s", providerV26URN, withDefault.Provider)
					assert.True(l, withDefault.Outputs["value"].BoolValue())
				},
			},
		},
	}
}
