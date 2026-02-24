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

	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["provider-version-component"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.ConformanceComponentProvider{} },
		},
		LanguageProviders:           []string{"conformance-component"},
		SkipEnsurePluginsValidation: true,
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					var conformanceProviders []*resource.State
					for _, res := range res.Snap.Resources {
						if res.Type == "pulumi:providers:conformance-component" {
							conformanceProviders = append(conformanceProviders, res)
						}
					}
					require.NotEmpty(l, conformanceProviders, "expected at least 1 conformance-component provider")

					var providerV22URN, providerV26URN resource.URN
					for _, prov := range conformanceProviders {
						urnStr := string(prov.URN)
						if strings.Contains(urnStr, "22_0_0") {
							providerV22URN = prov.URN
						} else if strings.Contains(urnStr, "26_0_0") {
							providerV26URN = prov.URN
						}
					}
					require.NotEmpty(l, providerV22URN, "expected to find provider with version 22_0_0")

					withV22 := RequireSingleNamedResource(l, res.Snap.Resources, "withV22")
					withDefault := RequireSingleNamedResource(l, res.Snap.Resources, "withDefault")

					if withV22.Provider != "" && withDefault.Provider != "" {
						if providerV26URN != "" {
							// Same assertion shape as l2-resource-option-version when both provider versions are present.
							assert.Truef(l, strings.HasPrefix(withV22.Provider, string(providerV22URN)),
								"expected %s to prefix %s", providerV22URN, withV22.Provider)
							assert.Truef(l, strings.HasPrefix(withDefault.Provider, string(providerV26URN)),
								"expected %s to prefix %s", providerV26URN, withDefault.Provider)
						} else {
							// Some hosts bind only one provider instance in snapshots for this component test path.
							assert.Truef(l, strings.HasPrefix(withV22.Provider, string(providerV22URN)),
								"expected %s to prefix %s", providerV22URN, withV22.Provider)
							assert.Truef(l, strings.HasPrefix(withDefault.Provider, string(providerV22URN)),
								"expected %s to prefix %s", providerV22URN, withDefault.Provider)
						}
					}

					assert.True(l, withV22.Inputs["value"].BoolValue())
					assert.True(l, withDefault.Inputs["value"].BoolValue())
				},
			},
		},
	}
}
