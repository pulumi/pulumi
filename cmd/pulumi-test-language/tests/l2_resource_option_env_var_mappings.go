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
	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-option-env-var-mappings"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					// Stack and provider
					require.Len(l, res.Snap.Resources, 2, "expected 2 resources in snapshot")

					// Provider with envVarMappings
					provider := RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:simple")
					assert.Equal(l, "prov", provider.URN.Name(), "expected provider named 'prov'")

					// Check that EnvVarMappings is set on the provider state
					assert.Equal(l, map[string]string{
						"MY_VAR":    "PROVIDER_VAR",
						"OTHER_VAR": "TARGET_VAR",
					}, provider.EnvVarMappings, "expected env var mappings to be set")
				},
			},
		},
	}
}
