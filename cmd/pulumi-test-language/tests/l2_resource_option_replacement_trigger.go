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
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-option-replacement-trigger"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.OutputProvider{} },
		},
		Runs: []TestRun{
			{
				AssertPreview: func(l *L,
					projectDirectory string, err error,
					plan *deploy.Plan, changes display.ResourceChanges,
					events []engine.Event,
				) {
					RequireStackResource(l, err, changes)
				},
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
					// 2. The default output provider
					// 3. A resource that has a replacement trigger
					// 4. A resource that does not have a replacement trigger
					// 5. A resource with an unknown output
					// 6. A resource that has a replacement trigger with an known value for this first run
					// 7. A resource that has a replacement trigger with a secret value
					require.Len(l, snap.Resources, 8, "expected 8 resources in snapshot")

					defaultProvider := RequireSingleNamedResource(l, snap.Resources, "default_2_0_0")
					require.Equal(l, "pulumi:providers:simple", defaultProvider.Type.String(), "expected default simple provider")

					defaultProviderRef, err := sdkproviders.NewReference(defaultProvider.URN, defaultProvider.ID)
					require.NoError(l, err, "expected to create default provider reference")

					replacementTrigger := RequireSingleNamedResource(l, snap.Resources, "replacementTrigger")
					notReplacementTrigger := RequireSingleNamedResource(l, snap.Resources, "notReplacementTrigger")
					unknownReplacementTrigger := RequireSingleNamedResource(l, snap.Resources, "unknownReplacementTrigger")
					secretReplacementTrigger := RequireSingleNamedResource(l, snap.Resources, "secretReplacementTrigger")

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

					// The resource that has a unknown replacement trigger should:
					//
					// * use the default provider
					// * have its replacementTrigger property set to the computed value specified in the program
					require.Equal(
						l, defaultProviderRef.String(), unknownReplacementTrigger.Provider,
						"expected replacement trigger resource to use default provider",
					)
					require.EqualValues(
						l, resource.NewProperty("hellohello"), unknownReplacementTrigger.ReplacementTrigger,
						"expected replacement trigger resource to have the replacement trigger value from the program",
					)

					// The resource that has a secret replacement trigger should:
					//
					// * use the default provider
					// * have its replacementTrigger property set to the secret value specified in the program
					require.Equal(
						l, defaultProviderRef.String(), secretReplacementTrigger.Provider,
						"expected secret replacement trigger resource to use default provider",
					)
					require.Equal(
						l, resource.MakeSecret(resource.NewProperty([]resource.PropertyValue{
							resource.NewProperty(1.0),
							resource.NewProperty(2.0),
							resource.NewProperty(3.0),
						})), secretReplacementTrigger.ReplacementTrigger,
						"expected replacement trigger resource to have a secret replacement trigger value",
					)
				},
			},
			{
				AssertPreview: func(l *L,
					projectDirectory string, err error,
					plan *deploy.Plan, changes display.ResourceChanges,
					events []engine.Event,
				) {
					// Preview should show that the resource with an unknown trigger is going to be replaced.
					var ops []display.StepOp
					for _, evt := range events {
						switch e := evt.Payload().(type) {
						case engine.ResourcePreEventPayload:
							if e.Metadata.URN.Name() == "unknownReplacementTrigger" {
								ops = append(ops, e.Metadata.Op)
							}
						}
					}
					require.NotNil(l, ops, "expected to find step event metadata for unknownReplacementTrigger resource")
					require.Contains(l, ops, deploy.OpReplace,
						"expected unknownReplacementTrigger resource to be replaced during preview")
				},
				Assert: func(l *L,
					projectDirectory string, _ error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					// We expect the following resources:
					//
					// 0. The stack
					// 1. The default simple provider
					// 2. The default output provider
					// 3. A resource that has a replacement trigger
					// 4. A resource that does not have a replacement trigger
					// 5. A resource with an unknown output
					// 6. A resource that has a replacement trigger with an unknown value
					// 7. A resource that has a replacement trigger with a secret value
					require.Len(l, snap.Resources, 8, "expected 8 resources in snapshot")

					defaultProvider := RequireSingleNamedResource(l, snap.Resources, "default_2_0_0")
					require.Equal(l, "pulumi:providers:simple", defaultProvider.Type.String(), "expected default simple provider")

					defaultProviderRef, err := sdkproviders.NewReference(defaultProvider.URN, defaultProvider.ID)
					require.NoError(l, err, "expected to create default provider reference")

					replacementTrigger := RequireSingleNamedResource(l, snap.Resources, "replacementTrigger")
					notReplacementTrigger := RequireSingleNamedResource(l, snap.Resources, "notReplacementTrigger")
					unknownReplacementTrigger := RequireSingleNamedResource(l, snap.Resources, "unknownReplacementTrigger")
					secretReplacementTrigger := RequireSingleNamedResource(l, snap.Resources, "secretReplacementTrigger")

					// The resource that has a replacement trigger should:
					//
					// * use the default provider
					// * have its replacementTrigger property set to the string value specified in the program
					require.Equal(
						l, defaultProviderRef.String(), replacementTrigger.Provider,
						"expected replacement trigger resource to use default provider",
					)
					require.EqualValues(
						l, resource.NewProperty("test2"), replacementTrigger.ReplacementTrigger,
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

					// The resource that has a unknown replacement trigger should:
					//
					// * use the default provider
					// * have its replacementTrigger property set to the computed value specified in the program
					require.Equal(
						l, defaultProviderRef.String(), unknownReplacementTrigger.Provider,
						"expected replacement trigger resource to use default provider",
					)
					require.EqualValues(
						l, resource.NewProperty("hellohello"), unknownReplacementTrigger.ReplacementTrigger,
						"expected replacement trigger resource to have the replacement trigger value from the program",
					)

					// Update should show that the resource with an unknown trigger wasn't replaced.
					var ops []display.StepOp
					for _, evt := range events {
						switch e := evt.Payload().(type) {
						case engine.ResourcePreEventPayload:
							if e.Metadata.URN.Name() == "unknownReplacementTrigger" {
								ops = append(ops, e.Metadata.Op)
							}
						}
					}
					require.NotNil(l, ops, "expected to find step event metadata for unknownReplacementTrigger resource")
					require.NotContains(l, ops, deploy.OpReplace,
						"expected unknownReplacementTrigger resource to not be replaced during update")

					// The resource that has a secret replacement trigger should:
					//
					// * use the default provider
					// * have its replacementTrigger property set to the secret value specified in the program
					require.Equal(
						l, defaultProviderRef.String(), secretReplacementTrigger.Provider,
						"expected secret replacement trigger resource to use default provider",
					)
					require.Equal(
						l, resource.MakeSecret(resource.NewProperty([]resource.PropertyValue{
							resource.NewProperty(3.0),
							resource.NewProperty(2.0),
							resource.NewProperty(1.0),
						})), secretReplacementTrigger.ReplacementTrigger,
						"expected replacement trigger resource to have a secret replacement trigger value",
					)

					// Update should show that the resource with an secret trigger was replaced.
					ops = []display.StepOp{}
					for _, evt := range events {
						switch e := evt.Payload().(type) {
						case engine.ResourcePreEventPayload:
							if e.Metadata.URN.Name() == "secretReplacementTrigger" {
								ops = append(ops, e.Metadata.Op)
							}
						}
					}
					require.NotNil(l, ops, "expected to find step event metadata for secretReplacementTrigger resource")
					require.Contains(l, ops, deploy.OpReplace,
						"expected secretReplacementTrigger resource to not be replaced during update")
				},
			},
		},
	}
}
