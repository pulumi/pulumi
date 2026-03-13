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
)

func init() {
	LanguageTests["l2-builtin-object"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.OutputProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")

					assert.Equal(l, resource.PropertyMap{
						"entriesOutput": resource.NewProperty([]resource.PropertyValue{
							resource.NewProperty(resource.PropertyMap{
								"key":   resource.NewProperty("x"),
								"value": resource.NewProperty("hello"),
							}),
						}),
						"lookupOutput":        resource.NewProperty("hello"),
						"lookupOutputDefault": resource.NewProperty("default"),
						"entriesObjectOutput": resource.NewProperty([]resource.PropertyValue{
							resource.NewProperty(resource.PropertyMap{
								"key":   resource.NewProperty("output"),
								"value": resource.NewProperty("hello"),
							}),
						}),
						"lookupObjectOutput":        resource.NewProperty("hello"),
						"lookupObjectOutputDefault": resource.NewProperty("default"),
					}, stack.Outputs)
				},
			},
		},
	}
}
