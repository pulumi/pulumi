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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-special-chars"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SpecialCharsProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					require.Len(l, res.Snap.Resources, 3)

					RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:specialchars")
					first := RequireSingleNamedResource(l, res.Snap.Resources, "first")

					assert.Equal(l, resource.PropertyMap{
						"value": resource.NewProperty("hello"),
					}, first.Inputs)

					item := resource.NewProperty(resource.PropertyMap{
						"@timestamp":  resource.NewProperty("2026-01-02T03:04:05Z"),
						"entity.name": resource.NewProperty("hello"),
					})
					assert.Equal(l, resource.PropertyMap{
						"value": resource.NewProperty("hello"),
						"item":  item,
					}, first.Outputs)

					// Stack outputs round-trip through the language SDK: the special property
					// names must be preserved in their wire format.
					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					AssertPropertyMapMember(l, stack.Outputs, "itemOutput", item)
					AssertPropertyMapMember(l, stack.Outputs, "invoked", resource.NewProperty(resource.PropertyMap{
						"item": resource.NewProperty(resource.PropertyMap{
							"@timestamp":  resource.NewProperty("2026-01-02T03:04:05Z"),
							"entity.name": resource.NewProperty("world"),
						}),
					}))
				},
			},
		},
	}
}
