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

	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-option-version"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.SimpleProviderV2{} },
		},
		SkipEnsurePluginsValidation: true,
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					require.Len(l, res.Snap.Resources, 6)

					providers := RequireNResources(l, res.Snap.Resources, "pulumi:providers:simple", 2)

					var providerV1URN, providerV2URN resource.URN
					for _, prov := range providers {
						urnStr := string(prov.URN)
						if strings.Contains(urnStr, "2_0_0") || strings.Contains(urnStr, "2.0.0") {
							providerV1URN = prov.URN
						} else if strings.Contains(urnStr, "26_0_0") || strings.Contains(urnStr, "26.0.0") {
							providerV2URN = prov.URN
						}
					}
					require.NotEmpty(l, providerV1URN, "expected to find provider with version 2.0.0 or 2_0_0")
					require.NotEmpty(l, providerV2URN, "expected to find provider with version 26.0.0 or 26_0_0")

					withV1 := RequireSingleNamedResource(l, res.Snap.Resources, "withV1")
					withV2 := RequireSingleNamedResource(l, res.Snap.Resources, "withV2")
					withDefault := RequireSingleNamedResource(l, res.Snap.Resources, "withDefault")

					assert.True(l, strings.HasPrefix(withV1.Provider, string(providerV1URN)),
						"withV1 should use provider version 2.0.0, got provider %s", withV1.Provider)
					assert.Equal(l, true, withV1.Outputs["value"].BoolValue())

					assert.True(l, strings.HasPrefix(withV2.Provider, string(providerV2URN)),
						"withV2 should use provider version 26.0.0, got provider %s", withV2.Provider)
					assert.Equal(l, false, withV2.Outputs["value"].BoolValue())

					assert.True(l, strings.HasPrefix(withDefault.Provider, string(providerV2URN)),
						"withDefault should use latest provider version 26.0.0, got provider %s", withDefault.Provider)
					assert.Equal(l, true, withDefault.Outputs["value"].BoolValue())
				},
			},
		},
	}
}
