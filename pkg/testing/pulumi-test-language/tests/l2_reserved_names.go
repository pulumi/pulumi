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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// l2-reserved-names exercises schema names that collide with members generated SDKs must
// define themselves: a resource whose `elementType` property collides with the
// `ElementType()` method that generated Go SDK types must implement, referencing a type
// that shares the resource's own token.
func init() {
	LanguageTests["l2-reserved-names"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ReservedNamesProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					snap := res.Snap

					// The stack, the provider, and the resource.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")
					RequireSingleResource(l, snap.Resources, "pulumi:providers:reservednames")

					elem := RequireSingleResource(l, snap.Resources, "reservednames:index:ElementType")
					want := resource.NewPropertyMapFromMap(map[string]any{
						"elementType": map[string]any{"elementType": "nested"},
					})
					assert.Equal(l, want, elem.Outputs, "expected the computed elementType output")

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					AssertPropertyMapMember(l, stack.Outputs, "elementType", resource.NewProperty("nested"))
				},
			},
		},
	}
}
