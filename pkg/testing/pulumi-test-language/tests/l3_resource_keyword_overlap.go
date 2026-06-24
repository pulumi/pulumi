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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l3-resource-keyword-overlap"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					// Stack, provider, component, and the two child resources.
					require.Len(l, snap.Resources, 5, "expected 5 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:simple")

					component := RequireSingleResource(l, snap.Resources, "components:index:KeywordComponent")
					assert.Equal(l, "comp", component.URN.Name())

					want := resource.NewPropertyMapFromMap(map[string]any{
						"value": true,
					})

					// A child resource named `this` collides with the receiver of the generated
					// ComponentResource class. If a language fails to rename the resource variable, or
					// renames the `parent` pointer along with it, the child ends up with the wrong parent
					// (or fails to compile). Asserting the parent proves the rename was applied correctly.
					thisRes := RequireSingleNamedResource(l, snap.Resources, "comp-this")
					assert.Equal(l, "simple:index:Resource", thisRes.Type.String())
					assert.Equal(l, component.URN, thisRes.Parent, "expected `this` resource to have the component as parent")
					assert.Equal(l, want, thisRes.Inputs, "expected `this` resource inputs to be %v", want)

					// `parent.value = this.value` exercises that the rename is applied to references as well
					// as the declaration: it must read the renamed resource variable, not the class
					// receiver. The resource is also named `parent`, which overlaps with the `parent`
					// resource-option key and must not be confused with it.
					parent := RequireSingleNamedResource(l, snap.Resources, "comp-parent")
					assert.Equal(l, "simple:index:Resource", parent.Type.String())
					assert.Equal(l, component.URN, parent.Parent,
						"expected `parent` resource to have the component as parent")
					assert.Equal(l, want, parent.Inputs, "expected `parent` resource inputs to be %v", want)
				},
			},
		},
	}
}
