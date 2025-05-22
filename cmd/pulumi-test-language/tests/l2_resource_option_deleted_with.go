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
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	deployProviders "github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-option-deleted-with"] = LanguageTest{
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
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
					//
					// 1. The default simple provider
					//
					// 2. A target resource
					// 3. A resource that is deleted with the target resource
					// 4. A resource that is not deleted with any resource
					require.Len(l, snap.Resources, 5, "expected 5 resources in snapshot")

					defaultProvider := RequireSingleNamedResource(l, snap.Resources, "default_2_0_0")
					require.Equal(l, "pulumi:providers:simple", defaultProvider.Type.String(), "expected default simple provider")

					defaultProviderRef, err := deployProviders.NewReference(defaultProvider.URN, defaultProvider.ID)
					require.NoError(l, err, "expected to create default provider reference")

					targetResource := RequireSingleNamedResource(l, snap.Resources, "target")
					deletedWith := RequireSingleNamedResource(l, snap.Resources, "deletedWith")
					notDeletedWith := RequireSingleNamedResource(l, snap.Resources, "notDeletedWith")

					// The target resource should:
					//
					// * use the default provider
					// * not have its deletedWith property set
					require.Equal(
						l, defaultProviderRef.String(), targetResource.Provider,
						"expected protected resource to use default provider",
					)
					require.Empty(
						l, targetResource.DeletedWith,
						"expected target resource to not be deleted with any resource",
					)

					// The resource that is deleted with the target resource should:
					//
					// * use the default provider
					// * have its deletedWith property set to the URN of the target resource
					require.Equal(
						l, defaultProviderRef.String(), deletedWith.Provider,
						"expected deletedWith resource to use default provider",
					)
					require.Equal(
						l, targetResource.URN, deletedWith.DeletedWith,
						"expected deletedWith resource to be deleted with target resource",
					)

					// The resource that is not deleted with any resource should:
					//
					// * use the default provider
					// * have its deletedWith property not set
					require.Equal(
						l, defaultProviderRef.String(), notDeletedWith.Provider,
						"expected notDeletedWith resource to use default provider",
					)
					require.Empty(
						l, notDeletedWith.DeletedWith,
						"expected notDeletedWith resource to not be deleted with any resource",
					)
				},
			},
		},
	}
}
