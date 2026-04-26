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
	LanguageTests["l2-id-type"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.PrimitiveProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)
					require.Len(l, snap.Resources, 6, "expected 6 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:primitive")
					source1 := RequireSingleNamedResource(l, snap.Resources, "source1")
					source2 := RequireSingleNamedResource(l, snap.Resources, "source2")
					sink1 := RequireSingleNamedResource(l, snap.Resources, "sink1")
					sink2 := RequireSingleNamedResource(l, snap.Resources, "sink2")
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					assert.Equal(l, "1234", string(source1.ID))
					assert.Equal(l, "true", string(source2.ID))

					assert.Equal(l, resource.PropertyMap{
						"boolean": resource.NewProperty(false),
						"float":   resource.NewProperty(1234.0),
						"integer": resource.NewProperty(1234.0),
						"string":  resource.NewProperty("1234"),
						"numberArray": resource.NewProperty([]resource.PropertyValue{
							resource.NewProperty(1234.0),
						}),
						"booleanMap": resource.NewProperty(resource.PropertyMap{
							"sink": resource.NewProperty(false),
						}),
					}, sink1.Inputs)
					assert.Equal(l, resource.PropertyMap{
						"boolean": resource.NewProperty(true),
						"float":   resource.NewProperty(1.0),
						"integer": resource.NewProperty(2.0),
						"string":  resource.NewProperty("abc"),
						"numberArray": resource.NewProperty([]resource.PropertyValue{
							resource.NewProperty(3.0),
						}),
						"booleanMap": resource.NewProperty(resource.PropertyMap{
							"sink": resource.NewProperty(true),
						}),
					}, sink2.Inputs)

					assert.Equal(l, resource.PropertyMap{
						"ids": resource.NewProperty(resource.PropertyMap{
							"source2Token": resource.NewProperty("true"),
							"source1Token": resource.NewProperty("1234"),
						}),
						"base64": resource.NewProperty("YWJj"),
					}, stack.Outputs)
				},
			},
		},
	}
}
