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
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-index-mod"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.IndexModProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 5, "expected 5 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:index-mod")

					res1 := RequireSingleNamedResource(l, snap.Resources, "res1")
					want := property.NewMap(map[string]property.Value{"text": property.New("hello world")})
					res1Inputs := resource.FromResourcePropertyMap(res1.Inputs)
					assert.Equal(l, want, res1Inputs, "expected inputs to be %v", want)
					assert.Equal(l, res1Inputs, resource.FromResourcePropertyMap(res1.Outputs), "expected inputs and outputs to match")

					res2 := RequireSingleNamedResource(l, snap.Resources, "res2")
					want = property.NewMap(map[string]property.Value{"text": property.New("goodbye world")})
					res2Inputs := resource.FromResourcePropertyMap(res2.Inputs)
					assert.Equal(l, want, res2Inputs, "expected inputs to be %v", want)
					assert.Equal(l, res2Inputs, resource.FromResourcePropertyMap(res2.Outputs), "expected inputs and outputs to match")

					stk := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					want = property.NewMap(map[string]property.Value{
						"out1": property.New(12.0),
						"out2": property.New(15.0),
					})
					assert.Equal(l, want, resource.FromResourcePropertyMap(stk.Outputs), "expected stack outputs to be %v", want)
				},
			},
		},
	}
}
