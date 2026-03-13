// Copyright 2025, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-names"] = LanguageTest{
		Providers: []func() plugin.Provider{func() plugin.Provider { return &providers.NamesProvider{} }},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 8, "expected 8 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:names")

					for _, typ := range (&providers.NamesProvider{}).Types() {
						simple := RequireSingleResource(l, snap.Resources, tokens.Type(typ))
						want := resource.NewPropertyMapFromMap(map[string]any{"value": true})
						assert.Equal(l, want, simple.Inputs, "expected inputs to be {value: true}")
						assert.Equal(l, simple.Inputs, simple.Outputs, "expected inputs and outputs to match")
					}
				},
			},
		},
	}
}
