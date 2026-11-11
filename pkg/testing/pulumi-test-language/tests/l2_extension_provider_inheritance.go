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
	"github.com/stretchr/testify/require"
)

func init() {
	// Shared instance so the parameter persists across the harness's per-load
	// factory calls; see l2_extension_parameterized_resource.go.
	shared := &providers.ExtensionParameterizedProvider{}
	LanguageTests["l2-extension-provider-inheritance"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return shared },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					prov := RequireSingleNamedResource(l, snap.Resources, "prov")
					provRef, err := sdkproviders.NewReference(prov.URN, prov.ID)
					require.NoError(l, err)

					parent := RequireSingleNamedResource(l, snap.Resources, "parent")
					require.Equal(l, provRef.String(), parent.Provider,
						"the parent must use the explicit provider")

					child := RequireSingleNamedResource(l, snap.Resources, "child")
					require.Equal(l, parent.URN, child.Parent)
					require.Equal(l, provRef.String(), child.Provider,
						"the extension resource must inherit the base provider from its parent")
				},
			},
		},
	}
}
