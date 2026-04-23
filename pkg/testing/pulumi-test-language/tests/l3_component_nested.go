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
	LanguageTests["l3-component-nested"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					snap := res.Snap
					require.Len(l, snap.Resources, 5)

					outer := RequireSingleResource(l, snap.Resources, "components:index:OuterComponent")
					assert.Equal(l, "outerComponent", outer.URN.Name())
					assert.Equal(l,
						resource.NewPropertyMapFromMap(map[string]any{"input": true}),
						outer.Inputs)
					assert.Equal(l,
						resource.NewPropertyMapFromMap(map[string]any{"output": true}),
						outer.Outputs)

					inner := RequireSingleResource(l, snap.Resources, "components:index:InnerComponent")
					assert.Equal(l, "outerComponent-innerComponent", inner.URN.Name())
					assert.Equal(l, outer.URN, inner.Parent)
					assert.Equal(l,
						resource.NewPropertyMapFromMap(map[string]any{"input": false}),
						inner.Inputs)
					assert.Equal(l,
						resource.NewPropertyMapFromMap(map[string]any{"output": true}),
						inner.Outputs)

					RequireSingleResource(l, snap.Resources, "pulumi:providers:simple")

					simple := RequireSingleResource(l, snap.Resources, "simple:index:Resource")
					assert.Equal(l, "outerComponent-innerComponent-res", simple.URN.Name())
					assert.Equal(l, inner.URN, simple.Parent)
					assert.Equal(l,
						resource.NewPropertyMapFromMap(map[string]any{"value": true}),
						simple.Inputs)
					assert.Equal(l, simple.Inputs, simple.Outputs)
				},
			},
		},
	}
}
