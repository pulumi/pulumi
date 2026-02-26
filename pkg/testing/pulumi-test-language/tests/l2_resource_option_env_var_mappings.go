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
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
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

					// Check that envVarMappings is set in the provider Inputs under __internal
					internal := provider.Inputs["__internal"]
					require.NotNil(l, internal, "expected __internal in provider inputs")
					internalObj := internal.ObjectValue()
					envVarMappings := internalObj["envVarMappings"]
					require.NotNil(l, envVarMappings, "expected envVarMappings in __internal")
					mappingsObj := envVarMappings.ObjectValue()
					assert.Equal(l, "PROVIDER_VAR", mappingsObj["MY_VAR"].StringValue(), "expected MY_VAR mapping")
					assert.Equal(l, "TARGET_VAR", mappingsObj["OTHER_VAR"].StringValue(), "expected OTHER_VAR mapping")
				},
			},
		},
	}
}
