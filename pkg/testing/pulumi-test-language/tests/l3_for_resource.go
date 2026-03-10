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
	LanguageTests["l3-for-resource"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.NestedObjectProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					require.Len(l, res.Snap.Resources, 5)

					RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:nestedobject")

					source := RequireSingleNamedResource(l, res.Snap.Resources, "source")
					assert.Equal(l, resource.PropertyMap{
						"inputs": resource.NewProperty([]resource.PropertyValue{
							resource.NewProperty("a"),
							resource.NewProperty("b"),
							resource.NewProperty("c"),
						}),
					}, source.Inputs)

					receiver := RequireSingleNamedResource(l, res.Snap.Resources, "receiver")
					assert.Equal(l, resource.PropertyMap{
						"details": resource.NewProperty([]resource.PropertyValue{
							resource.NewProperty(resource.PropertyMap{
								"key":   resource.NewProperty("a"),
								"value": resource.NewProperty("computed-a"),
							}),
							resource.NewProperty(resource.PropertyMap{
								"key":   resource.NewProperty("b"),
								"value": resource.NewProperty("computed-b"),
							}),
							resource.NewProperty(resource.PropertyMap{
								"key":   resource.NewProperty("c"),
								"value": resource.NewProperty("computed-c"),
							}),
						}),
					}, receiver.Inputs)

					fromSimple := RequireSingleNamedResource(l, res.Snap.Resources, "fromSimple")
					assert.Equal(l, resource.PropertyMap{
						"inputs": resource.NewProperty([]resource.PropertyValue{
							resource.NewProperty("computed-a"),
							resource.NewProperty("computed-b"),
							resource.NewProperty("computed-c"),
						}),
					}, fromSimple.Inputs)
				},
			},
		},
	}
}
