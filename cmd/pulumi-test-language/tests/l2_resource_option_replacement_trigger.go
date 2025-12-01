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
	LanguageTests["l2-resource-option-replacement-trigger"] = LanguageTest{
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
					// 1. The default simple provider
					// 2. A resource that has a replacement trigger
					// 3. A resource that does not have a replacement trigger
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					defaultProvider := RequireSingleNamedResource(l, snap.Resources, "default_2_0_0")
					require.Equal(l, "pulumi:providers:simple", defaultProvider.Type.String(), "expected default simple provider")

					defaultProviderRef, err := sdkproviders.NewReference(defaultProvider.URN, defaultProvider.ID)
					require.NoError(l, err, "expected to create default provider reference")

					replacementTrigger := RequireSingleNamedResource(l, snap.Resources, "replacementTrigger")
					notReplacementTrigger := RequireSingleNamedResource(l, snap.Resources, "notReplacementTrigger")

					// The resource that has a replacement trigger should:
					//
					// * use the default provider
					// * have its replacementTrigger property set to the string value specified in the program
					require.Equal(
						l, defaultProviderRef.String(), replacementTrigger.Provider,
						"expected replacement trigger resource to use default provider",
					)
					require.EqualValues(
						l, resource.NewProperty("test"), replacementTrigger.ReplacementTrigger,
						"expected replacement trigger resource to have the replacement trigger value from the program",
					)

					// The resource that does not have a replacement trigger should:
					//
					// * use the default provider
					// * have its replacementTrigger property not set
					require.Equal(
						l, defaultProviderRef.String(), notReplacementTrigger.Provider,
						"expected not replacement trigger resource to use default provider",
					)
					require.True(
						l, notReplacementTrigger.ReplacementTrigger.IsNull(),
						"expected not replacement trigger resource to not have a replacement trigger",
					)
				},
			},
		},
	}
}
