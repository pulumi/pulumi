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
	LanguageTests["l3-range"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.NestedObjectProvider{} },
		},
		RunsShareSource: true,
		Runs: []TestRun{
			{
				// Run 0: empty list/map, bool=false → only numResource instances created.
				Config: config.Map{
					config.MustMakeKey("l3-range", "numItems"):   config.NewValue("3"),
					config.MustMakeKey("l3-range", "itemList"):   config.NewObjectValue(`[]`),
					config.MustMakeKey("l3-range", "itemMap"):    config.NewObjectValue(`{}`),
					config.MustMakeKey("l3-range", "createBool"): config.NewValue("false"),
				},
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					// stack + nestedobject provider + 3 numResource + 0 list + 0 map + 0 bool = 5
					require.Len(l, res.Snap.Resources, 5)

					RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:nestedobject")

					targets := RequireNResources(l, res.Snap.Resources, "nestedobject:index:Target", 3)

					names := make([]string, 0, len(targets))
					for _, target := range targets {
						name, ok := target.Inputs["name"]
						require.True(l, ok, "expected target to have 'name' input")
						require.True(l, name.IsString(), "expected 'name' to be a string")
						names = append(names, name.StringValue())
					}
					assert.ElementsMatch(l, []string{"num-0", "num-1", "num-2"}, names)
				},
			},
			{
				// Run 1: non-empty list/map, bool=true → all resource groups created.
				Config: config.Map{
					config.MustMakeKey("l3-range", "numItems"):   config.NewValue("3"),
					config.MustMakeKey("l3-range", "itemList"):   config.NewObjectValue(`["a","b","c"]`),
					config.MustMakeKey("l3-range", "itemMap"):    config.NewObjectValue(`{"x":"foo","y":"bar"}`),
					config.MustMakeKey("l3-range", "createBool"): config.NewValue("true"),
				},
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					// stack + nestedobject provider + 3 numResource + 3 listResource + 2 mapResource + 1 boolResource = 11
					require.Len(l, res.Snap.Resources, 11)

					RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:nestedobject")

					targets := RequireNResources(l, res.Snap.Resources, "nestedobject:index:Target", 9)

					resources := make(map[string]string, len(targets))
					for _, target := range targets {
						input, ok := target.Inputs["name"]
						require.True(l, ok, "expected target to have 'name' input")
						require.True(l, input.IsString(), "expected 'name' to be a string")
						resources[target.URN.Name()] = input.StringValue()
					}

					assert.Equal(l, map[string]string{
						"boolResource":   "bool-resource",
						"listResource-0": "0:a",
						"listResource-1": "1:b",
						"listResource-2": "2:c",
						"mapResource-x":  "x=foo",
						"mapResource-y":  "y=bar",
						"numResource-0":  "num-0",
						"numResource-1":  "num-1",
						"numResource-2":  "num-2",
					}, resources)
				},
			},
		},
	}
}
