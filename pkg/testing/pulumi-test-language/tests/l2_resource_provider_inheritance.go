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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdkProviders "github.com/pulumi/pulumi/sdk/v3/go/common/providers"
)

func init() {
	LanguageTests["l2-resource-provider-inheritance"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.PrimitiveProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					parent1 := RequireSingleNamedResource(l, snap.Resources, "parent1")
					child1 := RequireSingleNamedResource(l, snap.Resources, "child1")
					require.Equal(l, parent1.Provider, child1.Provider,
						"expected parent1 and child1 to have the same provider reference")

					parent2 := RequireSingleNamedResource(l, snap.Resources, "parent2")
					child2 := RequireSingleNamedResource(l, snap.Resources, "child2")
					require.NotEqual(l, parent2.Provider, child2.Provider,
						"expected parent2 and child2 to have different provider references")

					getPackage := func(providerRef string) string {
						ref, err := sdkProviders.ParseReference(providerRef)
						require.NoError(l, err, "expected provider reference to parse correctly")
						return string(ref.URN().Type().Name())
					}

					assert.Equal(l, "simple", getPackage(parent1.Provider), "expected parent1 to use the simple provider")
					assert.Equal(l, "simple", getPackage(child1.Provider), "expected child1 to use the simple provider")
					assert.Equal(l, "primitive", getPackage(parent2.Provider), "expected parent2 to use the primitive provider")
					assert.Equal(l, "simple", getPackage(child2.Provider), "expected child2 to use the simple provider")
				},
			},
		},
	}
}
