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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l3-range-parent-scope"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.NestedObjectProvider{} },
		},
		Runs: []TestRun{
			{
				// A config variable (prefix) defined in the parent scope is referenced inside
				// a ranged resource's inputs. Without the parent-scope fix, the config variable
				// would resolve to an unknown value inside the range's child eval context.
				Config: config.Map{
					config.MustMakeKey("l3-range-parent-scope", "prefix"): config.NewValue("item"),
				},
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					// stack + nestedobject provider + 2 items
					require.Len(l, res.Snap.Resources, 4)

					targets := RequireNResources(l, res.Snap.Resources, "nestedobject:index:Target", 2)

					names := make([]string, 0, len(targets))
					for _, target := range targets {
						name, ok := target.Inputs["name"]
						require.True(l, ok, "expected target to have 'name' input")
						require.True(l, name.IsString(), "expected 'name' to be a string")
						names = append(names, name.StringValue())
					}
					assert.ElementsMatch(l, []string{"item-0", "item-1"}, names)
				},
			},
		},
	}
}
