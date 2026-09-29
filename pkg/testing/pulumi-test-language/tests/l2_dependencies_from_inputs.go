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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// assertDeps checks that a program with `b.value = a.value` produces the expected dependency graph in state,
	// regardless of whether the engine advertised the DEPENDENCIES_FROM_INPUTS feature. Both runs of this test share
	// the same source and expect identical dependencies/propertyDependencies, so that we know an SDK correctly
	// participates in either wire format.
	assertDeps := func(l *L, res AssertArgs) {
		RequireStackResource(l, res.Err, res.Changes)
		require.Len(l, res.Snap.Resources, 5, "expected 5 resources in snapshot")

		a := RequireSingleNamedResource(l, res.Snap.Resources, "a")
		b := RequireSingleNamedResource(l, res.Snap.Resources, "b")
		c := RequireSingleNamedResource(l, res.Snap.Resources, "c")

		assert.Empty(l, a.Dependencies, "resource a should have no dependencies")
		assert.Empty(l, a.PropertyDependencies, "resource a should have no property dependencies")

		assert.Equal(l, []resource.URN{a.URN}, b.Dependencies,
			"resource b should depend on a via its input")
		require.Contains(l, b.PropertyDependencies, resource.PropertyKey("value"),
			"resource b should track a dependency on its `value` input")
		assert.Equal(l, []resource.URN{a.URN}, b.PropertyDependencies["value"],
			"resource b's `value` input should depend on a")

		// c depends on a only through the `dependsOn` resource option, not through any input value. The engine
		// must still surface that in `Dependencies` regardless of the DEPENDENCIES_FROM_INPUTS wire format, since
		// option-level dependencies are not encoded in Output property values on the inputs.
		assert.Equal(l, []resource.URN{a.URN}, c.Dependencies,
			"resource c should depend on a via dependsOn")
	}

	LanguageTests["l2-dependencies-from-inputs"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
		},
		RunsShareSource: true,
		Runs: []TestRun{
			// Run 1: engine advertises DEPENDENCIES_FROM_INPUTS (default).
			{
				Assert: assertDeps,
			},
			// Run 2: engine does not advertise the feature; the SDK must fall back to filling the flat
			// dependencies / propertyDependencies fields on RegisterResourceRequest.
			{
				UpdateOptions: engine.UpdateOptions{
					DisableDependenciesFromInputs: true,
				},
				Assert: assertDeps,
			},
		},
	}
}
