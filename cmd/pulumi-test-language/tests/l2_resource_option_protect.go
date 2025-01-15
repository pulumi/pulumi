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
	LanguageTests["l2-resource-option-protect"] = LanguageTest{
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
					// 1. The default simple provider
					//
					// 2. A protected resource
					// 3. An explicitly unprotected resource
					// 4. An implicitly (by-default) unprotected resource
					require.Len(l, snap.Resources, 5, "expected 5 resources in snapshot")

					defaultProvider := RequireSingleNamedResource(l, snap.Resources, "default_2_0_0")
					require.Equal(l, "pulumi:providers:simple", defaultProvider.Type.String(), "expected default simple provider")

					defaultProviderRef, err := deployProviders.NewReference(defaultProvider.URN, defaultProvider.ID)
					require.NoError(l, err, "expected to create default provider reference")

					protectedResource := RequireSingleNamedResource(l, snap.Resources, "protected")
					unprotectedResource := RequireSingleNamedResource(l, snap.Resources, "unprotected")
					defaultedResource := RequireSingleNamedResource(l, snap.Resources, "defaulted")

					// All resources should use the default provider.
					// The protected resource should have its protect flag set.
					// The unprotected resource should not have its protect flag set.
					// The defaulted resource should not have its protect flag set.

					require.Equal(
						l, defaultProviderRef.String(), protectedResource.Provider,
						"expected protected resource to use default provider",
					)
					require.True(l, protectedResource.Protect, "expected protected resource to be protected")

					require.Equal(
						l, defaultProviderRef.String(), unprotectedResource.Provider,
						"expected unprotected resource to use default provider",
					)
					require.False(l, unprotectedResource.Protect, "expected unprotected resource to not be protected")

					require.Equal(
						l, defaultProviderRef.String(), defaultedResource.Provider,
						"expected defaulted resource to use default provider",
					)
					require.False(l, defaultedResource.Protect, "expected defaulted resource to not be protected")
				},
			},
		},
	}
}
