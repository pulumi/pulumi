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
	LanguageTests["l2-module-format"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ModuleFormatProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 7, "expected 7 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:module-format")

					res1 := RequireSingleNamedResource(l, snap.Resources, "res1")
					want := resource.NewPropertyMapFromMap(map[string]any{"text": "hello world"})
					assert.Equal(l, want, res1.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, res1.Inputs, res1.Outputs, "expected inputs and outputs to match")

					res2 := RequireSingleNamedResource(l, snap.Resources, "res2")
					want = resource.NewPropertyMapFromMap(map[string]any{"text": "goodbye world"})
					assert.Equal(l, want, res2.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, res2.Inputs, res2.Outputs, "expected inputs and outputs to match")

					res3 := RequireSingleNamedResource(l, snap.Resources, "res3")
					want = resource.NewPropertyMapFromMap(map[string]any{"text": "hello world"})
					assert.Equal(l, want, res3.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, res3.Inputs, res3.Outputs, "expected inputs and outputs to match")

					res4 := RequireSingleNamedResource(l, snap.Resources, "res4")
					want = resource.NewPropertyMapFromMap(map[string]any{"text": "goodbye world"})
					assert.Equal(l, want, res4.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, res4.Inputs, res4.Outputs, "expected inputs and outputs to match")

					stk := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					want = resource.NewPropertyMapFromMap(map[string]any{
						"out1": 12,
						"out2": 15,
						"out3": 12,
						"out4": 15,
					})
					assert.Equal(l, want, stk.Outputs, "expected stack outputs to be %v", want)
				},
			},
		},
	}
}
