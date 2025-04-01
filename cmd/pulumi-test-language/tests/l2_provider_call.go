// Copyright 2025, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-provider-call"] = LanguageTest{
		Providers: []plugin.Provider{&providers.CallProvider{}},
		Runs: []TestRun{
			{
				Config: config.Map{
					config.MustMakeKey("call", "value"): config.NewValue("defaultProvValue"),
				},
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					// We expect the following resources:
					//
					// 0. The stack
					// 1. The builtin Pulumi provider (used for hydrating resource references)
					//
					// 2. The default provider
					// 3. A resource using the default provider
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					builtinProvider := RequireSingleNamedResource(l, snap.Resources, "default")
					require.Equal(
						l, "pulumi:providers:pulumi", builtinProvider.Type.String(),
						"expected builtin provider",
					)

					defaultProvider := RequireSingleNamedResource(l, snap.Resources, "default_15_7_9")
					require.Equal(
						l, "pulumi:providers:call", defaultProvider.Type.String(),
						"expected default call provider",
					)

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					// The stack should have the following outputs:
					//
					// * defaultProviderValue, whose value should be the value of the default provider concatenated with the
					//   defaultRes resource's value.
					outputs := stack.Outputs
					require.Len(l, outputs, 1, "expected 1 output")
					AssertPropertyMapMember(
						l, outputs,
						"defaultProviderValue", resource.NewStringProperty("defaultProvValuedefaultValue"),
					)
				},
			},
		},
	}
}
