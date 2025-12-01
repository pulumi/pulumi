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
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-option-replace-with"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
		},
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
					// 3. A resource that is replaced with the target resource
					// 4. A resource that is not replaced with any resource
					require.Len(l, snap.Resources, 5, "expected 5 resources in snapshot")

					defaultProvider := RequireSingleNamedResource(l, snap.Resources, "default_2_0_0")
					require.Equal(l, "pulumi:providers:simple", defaultProvider.Type.String(), "expected default simple provider")

					defaultProviderRef, err := sdkproviders.NewReference(defaultProvider.URN, defaultProvider.ID)
					require.NoError(l, err, "expected to create default provider reference")

					targetResource := RequireSingleNamedResource(l, snap.Resources, "target")
					replaceWith := RequireSingleNamedResource(l, snap.Resources, "replaceWith")
					notReplaceWith := RequireSingleNamedResource(l, snap.Resources, "notReplaceWith")

					// The target resource should:
					//
					// * use the default provider
					// * not have its replaceWith property set
					require.Equal(
						l, defaultProviderRef.String(), targetResource.Provider,
						"expected protected resource to use default provider",
					)
					require.Empty(
						l, targetResource.ReplaceWith,
						"expected target resource to not be deleted with any resource",
					)

					// The resource that is replaced with the target resource should:
					//
					// * use the default provider
					// * have its replaceWith property set to the URN of the target resource
					require.Equal(
						l, defaultProviderRef.String(), replaceWith.Provider,
						"expected replaceWith resource to use default provider",
					)
					require.Equal(
						l, []resource.URN{targetResource.URN}, replaceWith.ReplaceWith,
						"expected replaceWith resource to be replaced with target resource",
					)

					// The resource that is not replaced with any resource should:
					//
					// * use the default provider
					// * have its replaceWith property not set
					require.Equal(
						l, defaultProviderRef.String(), notReplaceWith.Provider,
						"expected notReplaceWith resource to use default provider",
					)
					require.Empty(
						l, notReplaceWith.ReplaceWith,
						"expected notReplaceWith resource to not be replaced with any resource",
					)
				},
			},
		},
	}
}
