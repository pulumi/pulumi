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
	// Tests the ability to hydrate component resource references in other components and use their outputs as inputs to
	// child resources.
	LanguageTests["l2-component-component-resource-ref"] = LanguageTest{
		Providers: []plugin.Provider{&providers.ComponentProvider{}},
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
					// 1. The builtin Pulumi provider (used for hydrating resource references)
					// 2. The default component provider
					//
					// 3. The component1 resource
					// 4. The child of the component1 resource
					//
					// 5. The component2 resource
					// 6. The child of the component2 resource
					//
					// 7. The custom1 resource
					// 8. The custom2 resource
					require.Len(l, snap.Resources, 9, "expected 9 resources in snapshot")

					builtinProvider := RequireSingleNamedResource(l, snap.Resources, "default")
					require.Equal(
						l, "pulumi:providers:pulumi", builtinProvider.Type.String(),
						"expected builtin provider",
					)

					defaultProvider := RequireSingleNamedResource(l, snap.Resources, "default_13_3_7")
					require.Equal(
						l, "pulumi:providers:component", defaultProvider.Type.String(),
						"expected default component provider",
					)

					defaultProviderRef, err := deployProviders.NewReference(defaultProvider.URN, defaultProvider.ID)
					require.NoError(l, err, "expected to create default provider reference")

					component1 := RequireSingleNamedResource(l, snap.Resources, "component1")
					component1Child := RequireSingleNamedResource(l, snap.Resources, "component1-child")

					component2 := RequireSingleNamedResource(l, snap.Resources, "component2")
					component2Child := RequireSingleNamedResource(l, snap.Resources, "component2-child")

					custom1 := RequireSingleNamedResource(l, snap.Resources, "custom1")
					custom2 := RequireSingleNamedResource(l, snap.Resources, "custom2")

					// component1 should satisfy the following properties:
					//
					// * Its value output should be "foo-bar-baz".
					// * Its ref output should be a reference to component1-child by its URN and ID.
					require.Equal(
						l, "foo-bar-baz", component1.Outputs["value"].StringValue(),
						"expected component1 to have correct value output",
					)
					require.Equal(
						l, component1Child.URN, component1.Outputs["ref"].ResourceReferenceValue().URN,
						"expected component1 to return a reference to component1-child by its URN",
					)
					require.Equal(
						l,
						component1Child.ID.String(),
						component1.Outputs["ref"].ResourceReferenceValue().ID.StringValue(),
						"expected component1 to return a reference to component1-child by its ID",
					)

					// component1's child should satisfy the following properties:
					//
					// * Its provider should be the default component provider.
					// * Its value output should be "foo-bar-baz".
					require.Equal(
						l, defaultProviderRef.String(), component1Child.Provider,
						"expected component1-child to use default provider",
					)
					require.Equal(
						l, "foo-bar-baz", component1Child.Outputs["value"].StringValue(),
						"expected component1-child to have correct value output",
					)

					// component2 should satisfy the following properties:
					//
					// * Its inputRef output should be a reference to component1-child by its URN and ID
					// * Its outputRef output should be a reference to component2-child by its URN and ID
					require.Equal(
						l, component1Child.URN, component2.Outputs["inputRef"].ResourceReferenceValue().URN,
						"expected component2 to accept a reference to component1-child by its URN",
					)
					require.Equal(
						l,
						component1Child.ID.String(),
						component2.Outputs["inputRef"].ResourceReferenceValue().ID.StringValue(),
						"expected component2 to accept a reference to component1-child by its ID",
					)
					require.Equal(
						l, component2Child.URN, component2.Outputs["outputRef"].ResourceReferenceValue().URN,
						"expected component2 to return a reference to component2-child by its URN",
					)
					require.Equal(
						l,
						component2Child.ID.String(),
						component2.Outputs["outputRef"].ResourceReferenceValue().ID.StringValue(),
						"expected component2 to return a reference to component2-child by its ID",
					)

					// component2's child should satisfy the following properties:
					//
					// * Its provider should be the default component provider.
					// * Its value output should be "foo-bar-baz".
					require.Equal(
						l, defaultProviderRef.String(), component2Child.Provider,
						"expected component2-child to use default provider",
					)
					require.Equal(
						l, "foo-bar-baz", component2Child.Outputs["value"].StringValue(),
						"expected component2-child to have correct value output",
					)

					// custom1 and custom2 should satisfy the following properties:
					//
					// * Their provider should be the default component provider.
					// * Their value output should be "foo-bar-baz". custom1's should come from component2's input ref output,
					//   while custom2's should come from component2's output ref output (which should be the same).
					require.Equal(
						l, defaultProviderRef.String(), custom1.Provider,
						"expected custom1 to use default provider",
					)
					require.Equal(
						l, "foo-bar-baz", custom1.Outputs["value"].StringValue(),
						"expected custom1 to have correct value output",
					)

					require.Equal(
						l, defaultProviderRef.String(), custom2.Provider,
						"expected custom2 to use default provider",
					)
					require.Equal(
						l, "foo-bar-baz", custom2.Outputs["value"].StringValue(),
						"expected custom2 to have correct value output",
					)
				},
			},
		},
	}
}
