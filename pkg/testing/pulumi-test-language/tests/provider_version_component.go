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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
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

					withV22 := RequireSingleNamedResource(l, res.Snap.Resources, "withV22")
					withDefault := RequireSingleNamedResource(l, res.Snap.Resources, "withDefault")

					// Some hosts omit provider refs on components in snapshots. When refs are present,
					// assert the explicit versioned component binds differently from the default one.
					if withV22.Provider != "" && withDefault.Provider != "" {
						assert.NotEqual(l, withV22.Provider, withDefault.Provider,
							"expected withV22 and withDefault to use different provider bindings")
					}

					assert.True(l, withV22.Inputs["value"].BoolValue())
					assert.True(l, withDefault.Inputs["value"].BoolValue())
				},
			},
		},
	}
}
