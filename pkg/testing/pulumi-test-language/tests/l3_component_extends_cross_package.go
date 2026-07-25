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
	// l3-component-extends-cross-package constructs a DerivedComponent (package "inheritderived") that extends
	// BaseComponent (package "inheritbase") via a cross-package `extends` $ref. Two distinct provider instances take
	// part: the derived provider registers the single most-derived resource and issues ConstructBaseResource for the
	// base type owned by the other package, so the engine dispatches ConstructBase across a provider boundary. The
	// snapshot must still show a single component node with the merged outputs and the base's child parented to it.
	LanguageTests["l3-component-extends-cross-package"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.InheritanceBaseProvider{} },
			func() plugin.Provider { return &providers.InheritanceDerivedProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					snap := res.Snap

					derived := RequireSingleResource(l, snap.Resources, "inheritderived:index:DerivedComponent")
					assert.Equal(l, "derived", derived.URN.Name())
					wantOutputs := resource.NewPropertyMapFromMap(map[string]any{
						"baseOutput":    "base-hello",
						"derivedOutput": "derived-7",
					})
					assert.Equal(l, wantOutputs, derived.Outputs,
						"expected the derived component to merge the cross-package base output with its own")

					// The base's package must not register a node under its component type.
					for _, r := range snap.Resources {
						assert.NotEqualf(l, "inheritbase:index:BaseComponent", r.Type.String(),
							"no resource may be registered under the base type, found %s", r.URN)
					}

					// The base package's child parents directly to the single adopted URN.
					child := RequireSingleResource(l, snap.Resources, "inheritbase:index:Custom")
					assert.Equal(l, derived.URN, child.Parent,
						"expected the cross-package base's child to be parented to the derived component")
				},
			},
		},
	}
}
