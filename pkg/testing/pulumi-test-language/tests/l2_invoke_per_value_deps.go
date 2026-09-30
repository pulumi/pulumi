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

// This test runs the same program twice against different engine capabilities:
//
//  1. Engine advertises INVOKE_OUTPUT_VALUES (the default). PCL sends OutputValues in args, the provider
//     (`PickProvider`) preserves them, and returns `second` unchanged. d.text depends only on b.
//  2. Engine has INVOKE_OUTPUT_VALUES disabled via UpdateOptions.DisableInvokeOutputValues. PCL falls back
//     to the legacy path: unwrap OutputValues into a `dependsOn` list and apply the union to the whole
//     return. d.text depends on both a and b.
func init() {
	assertRun := func(l *L, dWantDepNames []string, eTextSecret bool, res AssertArgs) {
		// Cannot use RequireStackResource here: the second Run's update is an in-place change to `d` (its
		// Dependencies shrink from {a, b} to {b} or grow the other way), so the deployment records an Update
		// rather than a Create. Just require the run itself succeeded.
		require.NoError(l, res.Err, "expected no error")
		assert.NotEmpty(l, res.Changes, "expected at least 1 StepOp")

		urns := map[string]pkgresource.URN{}
		var d, e, f *pkgresource.State
		for _, r := range res.Snap.Resources {
			switch r.URN.Name() {
			case "a", "b":
				urns[r.URN.Name()] = r.URN
			case "d":
				d = r
			case "e":
				e = r
			case "f":
				f = r
			}
		}
		require.NotNil(l, urns["a"], "expected resource a")
		require.NotNil(l, urns["b"], "expected resource b")
		require.NotNil(l, d, "expected resource d")
		require.NotNil(l, e, "expected resource e")
		require.NotNil(l, f, "expected resource f")

		expected := make([]pkgresource.URN, 0, len(dWantDepNames))
		for _, n := range dWantDepNames {
			expected = append(expected, urns[n])
		}
		require.ElementsMatch(l, expected, d.Dependencies,
			"unexpected d.Dependencies")
		require.ElementsMatch(l, expected, d.PropertyDependencies["text"],
			"unexpected d.PropertyDependencies[text]")

		assert.Equal(l, eTextSecret, e.Inputs["text"].IsSecret(),
			"unexpected e.Inputs[text] secretness")
		assert.True(l, f.Inputs["text"].IsSecret(),
			"f.Inputs[text] should always be secret")
	}

	LanguageTests["l2-invoke-per-value-deps"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleInvokeProvider{} },
			func() plugin.Provider { return &providers.PickProvider{} },
		},
		RunsShareSource: true,
		Runs: []TestRun{
			{
				// Engine advertises INVOKE_OUTPUT_VALUES: per-value deps/secrets flow. d depends on b
				// only; e.text is not secret because the picked `second` was plain.
				Assert: func(l *L, res AssertArgs) {
					assertRun(l, []string{"b"}, false, res)
				},
			},
			{
				// Engine disables INVOKE_OUTPUT_VALUES: SDKs falls back to unioning arg deps and
				// secretness onto the whole return. d depends on {a, b}; e.text is secret because
				// `first` was secret.
				UpdateOptions: engine.UpdateOptions{
					DisableInvokeOutputValues: true,
				},
				Assert: func(l *L, res AssertArgs) {
					assertRun(l, []string{"a", "b"}, true, res)
				},
			},
		},
	}
}
