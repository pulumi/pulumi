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
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-provider-call-explicit"] = LanguageTest{
		Providers: []plugin.Provider{&providers.CallProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					RequireStackResource(l, err, changes)

					// We expect the following resources:
					//
					// 0. The stack
					// 1. The builtin Pulumi provider (used for hydrating resource references)
					//
					// 2. An explicit provider
					// 3. A resource using the explicit provider
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					builtinProvider := RequireSingleNamedResource(l, snap.Resources, "default")
					require.Equal(
						l, "pulumi:providers:pulumi", builtinProvider.Type.String(),
						"expected builtin provider",
					)

					explicitProvider := RequireSingleNamedResource(l, snap.Resources, "explicitProv")
					require.Equal(
						l, "pulumi:providers:call", explicitProvider.Type.String(),
						"expected explicit call provider",
					)

					stack := snap.Resources[0]

					// The stack should have the following outputs:
					//
					// * explicitProviderValue, whose value should be the value of the explicit provider concatenated with the
					//   explicitRes resource's value.
					// * explicitProvFromIdentity, whose value should be the value output of the explicit provider.
					// * explicitProvFromPrefixed, whose value should be the value output of the explicit provider, prefixed with
					//   "call-prefix-".
					outputs := stack.Outputs
					require.Len(l, outputs, 3, "expected 3 outputs")
					AssertPropertyMapMember(
						l, outputs,
						"explicitProviderValue", resource.NewStringProperty("explicitProvValueexplicitValue"),
					)
					AssertPropertyMapMember(
						l, outputs,
						"explicitProvFromIdentity", resource.NewStringProperty("explicitProvValue"),
					)
					AssertPropertyMapMember(
						l, outputs,
						"explicitProvFromPrefixed", resource.NewStringProperty("call-prefix-explicitProvValue"),
					)
				},
			},
		},
	}
}
