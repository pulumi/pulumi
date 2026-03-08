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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l3-range-resource-output-traversal"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.NestedObjectProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					// Expect: stack, nestedobject provider, container, 2 targets (one per input).
					// We have 2 inputs -> 2 details -> 2 targets.
					// Resources: stack + nestedobject provider + container + 2 targets = 5
					require.Len(l, res.Snap.Resources, 5, "expected 5 resources in snapshot")

					RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:nestedobject")
					RequireSingleResource(l, res.Snap.Resources, "nestedobject:index:Container")

					targets := RequireNResources(l, res.Snap.Resources, "nestedobject:index:Target", 2)
					for _, target := range targets {
						name, ok := target.Inputs["name"]
						assert.True(l, ok, "expected target to have 'name' input")
						assert.True(l, name.IsString(), "expected 'name' to be a string")
						assert.Contains(l, name.StringValue(), "computed-",
							"expected name to contain 'computed-'")
					}
				},
			},
		},
	}
}
