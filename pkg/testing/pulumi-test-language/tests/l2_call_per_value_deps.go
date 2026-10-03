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
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Differential test for per-leaf OutputValue deps on Call vs the coarse per-top-level-key
// `returnDependencies` fallback. See main.pp for the setup. The signal is `{pb}` appearing in g's deps
// only when CALL_OUTPUT_VALUES is disabled.
func init() {
	assertRun := func(l *L, gWantDepNames []string, res AssertArgs) {
		require.NoError(l, res.Err, "expected no error")
		assert.NotEmpty(l, res.Changes, "expected at least 1 StepOp")

		urns := map[string]pkgresource.URN{}
		var g *pkgresource.State
		for _, r := range res.Snap.Resources {
			switch r.URN.Name() {
			case "c", "pa", "pb":
				urns[r.URN.Name()] = r.URN
			case "g":
				g = r
			}
		}
		require.NotNil(l, urns["c"], "expected resource c")
		require.NotNil(l, urns["pa"], "expected resource pa")
		require.NotNil(l, urns["pb"], "expected resource pb")
		require.NotNil(l, g, "expected resource g")

		expected := make([]pkgresource.URN, 0, len(gWantDepNames))
		for _, n := range gWantDepNames {
			expected = append(expected, urns[n])
		}
		require.ElementsMatch(l, expected, g.Dependencies, "unexpected g.Dependencies")
		require.ElementsMatch(l, expected, g.PropertyDependencies["text"],
			"unexpected g.PropertyDependencies[text]")
	}

	LanguageTests["l2-call-per-value-deps"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleInvokeProvider{} },
			func() plugin.Provider { return &providers.PickCallProvider{} },
		},
		RunsShareSource: true,
		Runs: []TestRun{
			{
				// CALL_OUTPUT_VALUES on: per-leaf OutputValue on `a` carries only pa.
				Assert: func(l *L, res AssertArgs) {
					assertRun(l, []string{"pa", "c"}, res)
				},
			},
			{
				// CALL_OUTPUT_VALUES off: coarse {pa, pb} on `wrapper` propagates on descent.
				UpdateOptions: engine.UpdateOptions{
					DisableCallOutputValues: true,
				},
				Assert: func(l *L, res AssertArgs) {
					assertRun(l, []string{"pa", "pb", "c"}, res)
				},
			},
		},
	}
}
