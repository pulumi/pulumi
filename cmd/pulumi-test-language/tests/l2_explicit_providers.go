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
	deployProviders "github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-explicit-providers"] = LanguageTest{
		Providers: []plugin.Provider{&providers.ComponentProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					RequireStackResource(l, err, changes)

					// * The stack
					//
					// * An explicit component provider
					//
					// * A component that passes a list of providers
					// * The list-providers component's child custom resource
					// * A component that passes a map of providers
					// * The map-providers component's child custom resource
					require.Len(l, snap.Resources, 6, "expected 6 resources in snapshot")

					// There should just be a single provider resource -- the explicit one.
					provider := RequireSingleResource(l, snap.Resources, "pulumi:providers:component")
					require.Equal(l, "explicit", provider.URN.Name(), "expected explicit provider resource")

					providerRef, err := deployProviders.NewReference(provider.URN, provider.ID)
					require.NoError(l, err, "expected no error creating provider reference")

					// The list-providers component should register a custom resource using the explicit provider, which was sent
					// as part of the "providers" (plural) resource option list.
					listCustom := RequireSingleNamedResource(l, snap.Resources, "list-child")
					require.Equal(
						l, providerRef.String(), listCustom.Provider,
						"expected explicit provider to be used for list child resource",
					)

					// The map-providers component should register a custom resource using the explicit provider, which was sent
					// as part of the "providers" (plural) resource option map.
					mapCustom := RequireSingleNamedResource(l, snap.Resources, "map-child")
					require.Equal(
						l, providerRef.String(), mapCustom.Provider,
						"expected explicit provider to be used for map child resource",
					)
				},
			},
		},
	}
}
