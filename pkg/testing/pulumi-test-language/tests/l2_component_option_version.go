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
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-component-option-version"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.ConformanceComponentProvider{Version: &semver.Version{Major: 2}} },
			func() plugin.Provider { return &providers.ConformanceComponentProvider{Version: &semver.Version{Major: 22}} },
		},
		LanguageProviders:            []string{"conformance-component"},
		SkipEnsurePluginsValidation: true,
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					providers := RequireNResources(l, res.Snap.Resources, "pulumi:providers:conformance-component", 2)

					var providerV2URN, providerV22URN resource.URN
					for _, prov := range providers {
						urnStr := string(prov.URN)
						if strings.Contains(urnStr, "2_0_0") {
							providerV2URN = prov.URN
						} else if strings.Contains(urnStr, "22_0_0") {
							providerV22URN = prov.URN
						}
					}
					require.NotEmpty(l, providerV2URN, "expected to find provider with version 2_0_0")
					require.NotEmpty(l, providerV22URN, "expected to find provider with version 22_0_0")

					withV2 := RequireSingleNamedResource(l, res.Snap.Resources, "withV2")
					withV22 := RequireSingleNamedResource(l, res.Snap.Resources, "withV22")
					withDefault := RequireSingleNamedResource(l, res.Snap.Resources, "withDefault")

					assert.Truef(l, strings.HasPrefix(withV2.Provider, string(providerV2URN)),
						"expected %s to prefix %s", providerV2URN, withV2.Provider)
					assert.True(l, withV2.Inputs["value"].BoolValue())

					assert.Truef(l, strings.HasPrefix(withV22.Provider, string(providerV22URN)),
						"expected %s to prefix %s", providerV22URN, withV22.Provider)
					assert.False(l, withV22.Inputs["value"].BoolValue())

					assert.Truef(l, strings.HasPrefix(withDefault.Provider, string(providerV22URN)),
						"expected %s to prefix %s", providerV22URN, withDefault.Provider)
					assert.True(l, withDefault.Inputs["value"].BoolValue())
				},
			},
		},
	}
}
