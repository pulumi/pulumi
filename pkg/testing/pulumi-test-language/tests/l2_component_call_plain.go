// Copyright 2026, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

// l2-component-call-plain exercises a remote component (the Configurer in the "configurer"
// package) whose methods return plain (non-Output) values, including:
//
//   - a plain integer (plainValue),
//   - a plain resource reference to a provider the component constructs internally (plainProvider),
//   - a plain object containing a plain provider reference and a plain integer (nestedPlainProvider).
//
// The program uses the provider returned from plainProvider()/nestedPlainProvider().provider to
// create `configurer:index:Custom` resources, and verifies via each resource's `config` output
// that the component-constructed provider was configured correctly (config propagated from
// parent → component → constructed provider → child resource).
//
// Regression coverage for pulumi/pulumi#20744 and pulumi/pulumi-terraform-bridge#3247, previously
// tested by TestConstructComponentConfigureProviderGo/Node/Python integration tests.
func init() {
	LanguageTests["l2-component-call-plain"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ConfigurerProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					stack := snap.Resources[0]

					// Plain integer return.
					AssertPropertyMapMember(l, stack.Outputs, "plainValue", resource.NewProperty(42.0))
					// Plain integer returned as a field of the plain struct from nestedPlainProvider().
					AssertPropertyMapMember(l, stack.Outputs, "nestedPlainValue", resource.NewProperty(42.0))

					// The Custom resource created with the provider returned from plainProvider()
					// should have echoed the providerConfig value that was passed into the
					// component, confirming component -> constructed-provider -> child propagation.
					custom := RequireSingleNamedResource(l, snap.Resources, "customFromPlainProvider")
					require.Equal(
						l, "propagated", custom.Outputs["config"].StringValue(),
						"expected customFromPlainProvider.config to propagate from Configurer.providerConfig",
					)

					// Same assertion via nestedPlainProvider().provider.
					customNested := RequireSingleNamedResource(l, snap.Resources, "customFromNestedPlainProvider")
					require.Equal(
						l, "propagated", customNested.Outputs["config"].StringValue(),
						"expected customFromNestedPlainProvider.config to propagate via nestedPlainProvider().provider",
					)
				},
			},
		},
	}
}
