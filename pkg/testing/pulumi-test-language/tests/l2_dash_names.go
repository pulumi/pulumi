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
	LanguageTests["l2-dash-names"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.DashNamesProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					require.Len(l, res.Snap.Resources, 5)

					RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:dash-names")
					first := RequireSingleNamedResource(l, res.Snap.Resources, "first")
					third := RequireSingleNamedResource(l, res.Snap.Resources, "third")
					trailing := RequireSingleNamedResource(l, res.Snap.Resources, "trailing")

					assert.Equal(l, resource.PropertyMap{
						"the-input": resource.NewProperty(true),
						"nested-value": resource.NewProperty(resource.PropertyMap{
							"nested-value": resource.NewProperty("nested"),
						}),
					}, first.Inputs)

					assert.Equal(l, resource.PropertyMap{
						"the-output": resource.NewProperty([]resource.PropertyValue{
							resource.NewProperty(resource.PropertyMap{
								"nested-output": resource.NewProperty("nested"),
							}),
						}),
					}, first.Outputs)

					assert.Equal(l, resource.PropertyMap{
						"the-input": resource.NewProperty("fuzz"),
					}, third.Inputs)

					assert.Equal(l, resource.PropertyMap{
						"the-input": resource.NewProperty("fuzz"),
					}, third.Outputs)

					assert.Equal(l, resource.PropertyMap{
						"trailing-input-": resource.NewProperty("some-name-"),
					}, trailing.Inputs)

					assert.Equal(l, resource.PropertyMap{
						"trailing-output-": resource.NewProperty("some-name-"),
					}, trailing.Outputs)
				},
			},
		},
	}
}
