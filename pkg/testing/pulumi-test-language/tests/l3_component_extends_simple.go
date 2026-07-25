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
)

func init() {
	// l3-component-extends-simple constructs a Derived component that extends Base within a single package. The
	// derived provider registers one resource under the most-derived token and issues ConstructBaseResource for the
	// base; the base adopts that URN and parents a child to it. The snapshot must show exactly one component node,
	// under the derived token, carrying the merged base+own outputs, with the base's child parented to it and nothing
	// registered under the base type.
	LanguageTests["l3-component-extends-simple"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.InheritanceProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					snap := res.Snap

					derived := RequireSingleResource(l, snap.Resources, "inherit:index:Derived")
					assert.Equal(l, "derived", derived.URN.Name())
					wantOutputs := resource.NewPropertyMapFromMap(map[string]any{
						"baseOutput":    "base-hello",
						"derivedOutput": "derived-3",
					})
					assert.Equal(l, wantOutputs, derived.Outputs,
						"expected the derived component to merge base and own outputs")

					// No resource may be registered under the base type: a base is not a resource.
					for _, r := range snap.Resources {
						assert.NotEqualf(l, "inherit:index:Base", r.Type.String(),
							"no resource may be registered under the base type, found %s", r.URN)
					}

					// The base's child parents directly to the single adopted URN.
					child := RequireSingleResource(l, snap.Resources, "inherit:index:Custom")
					assert.Equal(l, derived.URN, child.Parent,
						"expected the base's child to be parented to the derived component")
				},
			},
		},
	}
}
