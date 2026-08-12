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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l3-range-invoke-output-traversal"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.NestedObjectProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					// Expect: stack, nestedobject provider, 1 container, 3 targets (one per value).
					require.Len(l, res.Snap.Resources, 6, "expected 6 resources in snapshot")

					RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:nestedobject")
					RequireSingleResource(l, res.Snap.Resources, "nestedobject:index:Container")

					names := map[string]string{}
					targets := RequireNResources(l, res.Snap.Resources, "nestedobject:index:Target", 3)
					for _, target := range targets {
						name, ok := target.Inputs["name"]
						require.True(l, ok && name.IsString(), "expected target to have 'name' input of type string")
						names[target.URN.Name()] = name.StringValue()
					}
					assert.Equal(l, map[string]string{
						"routes-0": "alpha",
						"routes-1": "bravo",
						"routes-2": "charlie",
					}, names)
				},
			},
		},
	}
}
