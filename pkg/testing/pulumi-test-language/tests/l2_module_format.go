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

					require.Len(l, snap.Resources, 10)

					RequireSingleResource(l, snap.Resources, "pulumi:providers:module-format")

					checkText := func(name, text string) {
						r := RequireSingleNamedResource(l, snap.Resources, name)
						want := property.NewMap(map[string]property.Value{"text": property.New(text)})
						inputs := resource.FromResourcePropertyMap(r.Inputs)
						assert.Equal(l, want, inputs, "expected inputs to be %v", want)
						assert.Equal(l, inputs, resource.FromResourcePropertyMap(r.Outputs), "expected inputs and outputs to match")
					}

					checkText("res1", "hello world")
					checkText("res2", "goodbye world")
					checkText("res3", "hello world")
					checkText("res4", "goodbye world")
					checkText("res5", "bonjour world")
					checkText("res6", "youkoso world")
					checkText("res7", "guten tag world")

					stk := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					want := property.NewMap(map[string]property.Value{
						"out1": property.New(12.0),
						"out2": property.New(15.0),
						"out3": property.New(12.0),
						"out4": property.New(15.0),
						"out5": property.New(14.0),
						"out6": property.New(15.0),
						"out7": property.New(18.0),
					})
					assert.Equal(l, want, resource.FromResourcePropertyMap(stk.Outputs), "expected stack outputs to be %v", want)
				},
			},
		},
	}
}
