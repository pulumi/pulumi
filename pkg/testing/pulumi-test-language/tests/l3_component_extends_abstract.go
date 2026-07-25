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
	// l3-component-extends-abstract constructs ConcreteChild, which extends the abstract component AbstractBase.
	// Constructing a concrete subclass of an abstract base is permitted, and it base-constructs the abstract base:
	// ConstructBase of an abstract type is always allowed (only a direct Construct of the abstract type is rejected,
	// which is covered by a provider-level unit test since generated code makes the abstract type non-constructable).
	LanguageTests["l3-component-extends-abstract"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.InheritanceAbstractProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					snap := res.Snap

					child := RequireSingleResource(l, snap.Resources, "inheritabstract:index:ConcreteChild")
					assert.Equal(l, "child", child.URN.Name())
					wantOutputs := resource.NewPropertyMapFromMap(map[string]any{
						"abstractOutput": "abstract-s",
						"concreteOutput": "concrete-e",
					})
					assert.Equal(l, wantOutputs, child.Outputs,
						"expected the concrete child to merge the abstract base's output with its own")

					// No resource may be registered under the abstract base type.
					for _, r := range snap.Resources {
						assert.NotEqualf(l, "inheritabstract:index:AbstractBase", r.Type.String(),
							"no resource may be registered under the abstract base type, found %s", r.URN)
					}

					// The abstract base's child parents directly to the single adopted URN.
					baseChild := RequireSingleResource(l, snap.Resources, "inheritabstract:index:Custom")
					assert.Equal(l, child.URN, baseChild.Parent,
						"expected the abstract base's child to be parented to the concrete component")
				},
			},
		},
	}
}
