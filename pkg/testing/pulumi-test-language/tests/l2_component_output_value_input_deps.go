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
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

// Verifies that per-leaf OutputValue dependencies on a nested resource input
// are preserved end-to-end rather than being flattened to the top-level
// propertyDependencies set.
//
// Program shape:
//
//	a: Custom
//	b: Custom
//	comp: Component { refs = { x = a, y = b } }
//
// The Component's Construct implementation registers a Child Custom resource
// with foo = refs.x and bar = refs.y, carrying OutputValues forward and
// sending no propertyDependencies.
//
// Legacy wire format (propDeps map + upgradeOutputValues stamping top-level
// deps onto every leaf) yields:
//
//	child.PropertyDependencies == {foo: [a,b], bar: [a,b]}   // WRONG
//
// New wire format (OVs inline, flag set, upgradeOutputValues skipped,
// engine harvests from OV tree) yields:
//
//	child.PropertyDependencies == {foo: [a], bar: [b]}       // CORRECT
func init() {
	assertChildDeps := func(l *L, res AssertArgs) {
		require.NoError(l, res.Err, "expected no error")

		a := RequireSingleNamedResource(l, res.Snap.Resources, "a")
		b := RequireSingleNamedResource(l, res.Snap.Resources, "b")
		child := RequireSingleNamedResource(l, res.Snap.Resources, "comp-child")

		require.Equal(l, []resource.URN{a.URN}, child.PropertyDependencies["foo"],
			"foo should depend only on a")
		require.Equal(l, []resource.URN{b.URN}, child.PropertyDependencies["bar"],
			"bar should depend only on b")
	}

	LanguageTests["l2-component-output-value-input-deps"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.OutputValueInputDepsProvider{} },
		},
		RunsShareSource: true,
		Runs: []TestRun{
			{
				// Engine advertises REGISTER_RESOURCE_OUTPUT_VALUES. The provider sends per-leaf
				// OutputValues on the child register with no propertyDependencies and
				// uses_output_values=true. The engine harvests per-leaf deps from the OV tree.
				Assert: assertChildDeps,
			},
			{
				// Engine disables REGISTER_RESOURCE_OUTPUT_VALUES. SupportsFeature returns false;
				// the provider falls back to the legacy wire shape — plain refs plus an explicit
				// propertyDependencies map. The engine stores the explicit map verbatim and we
				// observe the same per-leaf attribution.
				UpdateOptions: engine.UpdateOptions{
					DisableRegisterResourceOutputValues: true,
				},
				Assert: assertChildDeps,
			},
		},
	}
}
