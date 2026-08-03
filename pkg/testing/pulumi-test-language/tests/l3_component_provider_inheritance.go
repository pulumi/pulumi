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
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	sdkproviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l3-component-provider-inheritance"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.ComponentProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					// * The stack
					//
					// * The explicit simple provider
					// * The default component provider
					//
					// * The local component
					// * The remote component, parented to the local component
					// * The remote component's child simple resource
					urns := make([]resource.URN, len(snap.Resources))
					for i, r := range snap.Resources {
						urns[i] = r.URN
					}
					require.Len(l, snap.Resources, 6, "expected 6 resources in snapshot, got %v", urns)

					// The only simple provider should be the explicit one; an inheritance failure shows up
					// as an extra default simple provider.
					provider := RequireSingleResource(l, snap.Resources, "pulumi:providers:simple")
					require.Equal(l, "explicit", provider.URN.Name(), "expected explicit provider resource")

					providerRef, err := sdkproviders.NewReference(provider.URN, provider.ID)
					require.NoError(l, err, "expected no error creating provider reference")

					local := RequireSingleResource(l, snap.Resources, "components:index:Local")
					remote := RequireSingleResource(l, snap.Resources, "component:index:ComponentForeignChild")
					require.Equal(l, local.URN, remote.Parent, "expected remote component to be parented to the local component")

					child := RequireSingleNamedResource(l, snap.Resources, "withProviders-mlc-child")
					require.Equal(l, "simple:index:Resource", child.Type.String())
					require.Equal(
						l, providerRef.String(), child.Provider,
						"expected the remote component's child to use the inherited explicit provider",
					)
				},
			},
		},
	}
}
