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
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	deployProviders "github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-parent-inheritance"] = LanguageTest{
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					// We expect the following resources:
					//
					// 0. The stack
					//
					// 1. The default simple provider.
					// 2. The explicit simple provider, used to test provider inheritance.
					//
					// 3. A parent using the explicit provider.
					// 4. A child of the parent using the explicit provider.
					// 5. An orphan without a parent or explicit provider.
					//
					// 6. A parent with its protect flag set.
					// 7. A child of the parent with its protect flag set.
					// 8. An orphan without a parent or protect flag set.
					require.Len(l, snap.Resources, 9, "expected 9 resources in snapshot")

					defaultProvider := RequireSingleNamedResource(l, snap.Resources, "default_2_0_0")
					require.Equal(l, "pulumi:providers:simple", defaultProvider.Type.String(), "expected default simple provider")

					defaultProviderRef, err := deployProviders.NewReference(defaultProvider.URN, defaultProvider.ID)
					require.NoError(l, err, "expected to create default provider reference")

					explicitProvider := RequireSingleNamedResource(l, snap.Resources, "provider")
					require.Equal(l, "pulumi:providers:simple", explicitProvider.Type.String(), "expected explicit simple provider")

					explicitProviderRef, err := deployProviders.NewReference(explicitProvider.URN, explicitProvider.ID)
					require.NoError(l, err, "expected to create explicit provider reference")

					// Children should inherit providers.
					providerParent := RequireSingleNamedResource(l, snap.Resources, "parent1")
					providerChild := RequireSingleNamedResource(l, snap.Resources, "child1")
					providerOrphan := RequireSingleNamedResource(l, snap.Resources, "orphan1")

					require.Equal(
						l, explicitProviderRef.String(), providerParent.Provider,
						"expected parent to set explicit provider",
					)
					require.Equal(
						l, explicitProviderRef.String(), providerChild.Provider,
						"expected child to inherit explicit provider",
					)
					require.Equal(
						l, defaultProviderRef.String(), providerOrphan.Provider,
						"expected orphan to use default provider",
					)

					// Children should inherit protect flags.
					protectParent := RequireSingleNamedResource(l, snap.Resources, "parent2")
					protectChild := RequireSingleNamedResource(l, snap.Resources, "child2")
					protectOrphan := RequireSingleNamedResource(l, snap.Resources, "orphan2")

					require.True(l, protectParent.Protect, "expected parent to be protected")
					require.True(l, protectChild.Protect, "expected child to inherit protect flag")
					require.False(l, protectOrphan.Protect, "expected orphan to not be protected")
				},
			},
		},
	}
}
