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
	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-option-additional-secret-outputs"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					// Stack, provider, and 2 resources
					require.Len(l, res.Snap.Resources, 4, "expected 4 resources in snapshot")

					// Resource with additionalSecretOutputs
					withSecret := RequireSingleNamedResource(l, res.Snap.Resources, "withSecret")
					assert.Equal(l, []resource.PropertyKey{"value"}, withSecret.AdditionalSecretOutputs)
					assert.True(l, withSecret.Outputs["value"].IsSecret(),
						"expected 'value' output to be secret")

					// Resource without additionalSecretOutputs
					withoutSecret := RequireSingleNamedResource(l, res.Snap.Resources, "withoutSecret")
					assert.Empty(l, withoutSecret.AdditionalSecretOutputs)
					assert.False(l, withoutSecret.Outputs["value"].IsSecret(),
						"expected 'value' output to not be secret")
				},
			},
		},
	}
}
