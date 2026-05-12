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
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-primitive-defaults"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.PrimitiveDefaultsProvider{} },
		},
		Runs: []TestRun{
			{
				// Run 0: two resources in one program: one explicit and one defaulted.
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					require.Len(l, res.Snap.Resources, 4, "expected 4 resources in snapshot")

					RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:primitive-defaults")
					explicit := RequireSingleNamedResource(l, res.Snap.Resources, "resExplicit")
					defaulted := RequireSingleNamedResource(l, res.Snap.Resources, "resDefaulted")
					require.Equal(l, "primitive-defaults:index:Resource", string(explicit.Type))
					require.Equal(l, "primitive-defaults:index:Resource", string(defaulted.Type))

					wantExplicit := resource.NewPropertyMapFromMap(map[string]any{
						"boolean": true,
						"float":   3.14,
						"integer": 42,
						"string":  "hello",
					})
					wantDefaulted := resource.NewPropertyMapFromMap(map[string]any{
						"boolean": false,
						"float":   0.5,
						"integer": 1,
						"string":  "default",
					})

					assert.Equal(l, wantExplicit, explicit.Inputs, "expected explicit inputs to be %v", wantExplicit)
					assert.Equal(l, explicit.Inputs, explicit.Outputs, "expected explicit inputs and outputs to match")

					assert.Equal(l, wantDefaulted, defaulted.Inputs, "expected defaulted inputs to be %v", wantDefaulted)
					assert.Equal(l, defaulted.Inputs, defaulted.Outputs, "expected defaulted inputs and outputs to match")
				},
			},
		},
	}
}
