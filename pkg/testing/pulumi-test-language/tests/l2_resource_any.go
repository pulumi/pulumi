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
	"github.com/stretchr/testify/require"
)

func init() {
	// Exercises passing values to a resource input property whose schema type is `any`, across the
	// distinct shapes that must be generated and round-tripped as untyped values in every language:
	// raw scalars (string, boolean, number), a list of scalars, an object, and an asset.
	LanguageTests["l2-resource-any"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.AnyHandledProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					asset, err := resource.NewTextAsset("the asset contents")
					require.NoError(l, err)

					assertValue := func(name string, want resource.PropertyValue) {
						r := RequireSingleNamedResource(l, res.Snap.Resources, name)
						require.Equal(l, want, r.Inputs["value"], name)
					}

					assertValue("aString", resource.NewProperty("a string"))
					assertValue("aBoolean", resource.NewProperty(true))
					assertValue("aNumber", resource.NewProperty(42.0))
					assertValue("aList", resource.NewProperty([]resource.PropertyValue{
						resource.NewProperty(1.0),
						resource.NewProperty(true),
						resource.NewProperty("three"),
					}))
					assertValue("anObject", resource.NewProperty(resource.PropertyMap{
						"key":    resource.NewProperty("value"),
						"nested": resource.NewProperty(resource.PropertyMap{"count": resource.NewProperty(1.0)}),
					}))
					assertValue("anAsset", resource.NewProperty(asset))
				},
			},
		},
	}
}
