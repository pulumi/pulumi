// Copyright 2024, Pulumi Corporation.
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

func init() {
	LanguageTests["l3-component-simple"] = LanguageTest{
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

					// Check we have the two simple resources in the snapshot, their provider, the component
					// and the stack.
					require.Len(l, snap.Resources, 5, "expected 5 resources in snapshot")

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					assert.Empty(l, stack.Inputs, "expected stack to have no inputs")

					component := RequireSingleResource(l, snap.Resources, "components:index:MyComponent")
					assert.Equal(l, "someComponent", component.URN.Name())

					want := resource.NewPropertyMapFromMap(map[string]any{
						"input": true,
					})
					assert.Equal(l, want, component.Inputs, "expected component inputs to be %v", want)
					want = resource.NewPropertyMapFromMap(map[string]any{
						"output": true,
					})
					assert.Equal(l, want, component.Outputs, "expected component outputs to be %v", want)

					RequireSingleResource(l, snap.Resources, "pulumi:providers:simple")

					input := RequireSingleNamedResource(l, snap.Resources, "input")
					assert.Equal(l, "simple:index:Resource", input.Type.String())
					want = resource.NewPropertyMapFromMap(map[string]any{
						"value": true,
					})
					assert.Equal(l, want, input.Inputs, "expected input resource inputs to be %v", want)
					assert.Equal(l, input.Inputs, input.Outputs, "expected input resource inputs and outputs to match")

					simple := RequireSingleNamedResource(l, snap.Resources, "someComponent-res")
					assert.Equal(l, "simple:index:Resource", simple.Type.String())
					assert.Equal(l, component.URN, simple.Parent, "expected simple resource to have component as parent")
					assert.Equal(l, want, simple.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, simple.Inputs, simple.Outputs, "expected inputs and outputs to match")

					// The top-level `input` resource has a literal value, so it has no dependencies.
					assert.Empty(l, input.Dependencies, "expected input resource to have no dependencies")
					for k, deps := range input.PropertyDependencies {
						assert.Empty(l, deps, "expected input resource property %q to have no dependencies", k)
					}

					// The component's `input` argument is set to `input.value`, so the component must record a
					// dependency on the top-level `input` resource, both overall and per-property.
					assert.Equal(l, []resource.URN{input.URN}, component.Dependencies,
						"expected component to depend on the top-level input resource")
					componentInputDeps, ok := component.PropertyDependencies["input"]
					require.True(l, ok, "expected component to have property dependencies for input")
					assert.Equal(l, []resource.URN{input.URN}, componentInputDeps,
						"expected component.input to depend on the top-level input resource")

					// Inside the component, `res.value = input` flows the component's `input` config -- which
					// itself traces back to the top-level `input` resource -- into the child resource. The
					// dependency on `input` must propagate through the component boundary so the engine can
					// correctly order operations.
					assert.Contains(l, simple.Dependencies, input.URN,
						"expected simple resource inside the component to depend on the top-level input resource")
					simpleValueDeps, ok := simple.PropertyDependencies["value"]
					require.True(l, ok, "expected simple resource to have property dependencies for value")
					assert.Contains(l, simpleValueDeps, input.URN,
						"expected simple.value inside the component to depend on the top-level input resource")
				},
			},
		},
	}
}
