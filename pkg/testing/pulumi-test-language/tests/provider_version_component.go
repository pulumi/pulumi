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
	LanguageTests["provider-version-component"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider {
				return &providers.ConformanceComponentRuntimeProvider{
					ConformanceComponentProvider: providers.ConformanceComponentProvider{
						Version: &semver.Version{Major: 22},
					},
				}
			},
		},
		LanguageProviders:            []string{"conformance-component"},
		SkipEnsurePluginsValidation: true,
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					var conformanceProviders []*resource.State
					for _, r := range res.Snap.Resources {
						if r.Type == "pulumi:providers:conformance-component" {
							conformanceProviders = append(conformanceProviders, r)
						}
					}
					require.NotEmpty(l, conformanceProviders, "expected at least 1 conformance-component provider")

					// Some hosts produce only a default provider ref, others include the versioned alias as well.
					providerV22URN := conformanceProviders[0].URN
					for _, prov := range conformanceProviders {
						if strings.Contains(string(prov.URN), "22_0_0") {
							providerV22URN = prov.URN
							break
						}
					}

					withV22 := RequireSingleNamedResource(l, res.Snap.Resources, "withV22")
					withDefault := RequireSingleNamedResource(l, res.Snap.Resources, "withDefault")

					providerRefsOmitted := withV22.Provider == "" && withDefault.Provider == ""
					if !providerRefsOmitted {
						assert.Truef(l, strings.HasPrefix(withV22.Provider, string(providerV22URN)),
							"expected %s to prefix %s", providerV22URN, withV22.Provider)
					}
					assert.True(l, withV22.Outputs["value"].BoolValue())
					assert.True(l, withDefault.Outputs["value"].BoolValue())
				},
			},
		},
	}
}
