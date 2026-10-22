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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
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

					assertValue := func(name string, want property.Value) {
						r := RequireSingleNamedResource(l, res.Snap.Resources, name)
						inputs := resource.FromResourcePropertyMap(r.Inputs)
						require.Equal(l, want, inputs.Get("value"), name)
					}

					assertValue("aString", property.New("a string"))
					assertValue("aBoolean", property.New(true))
					assertValue("aNumber", property.New(42.0))
					assertValue("aList", property.New([]property.Value{
						property.New(1.0),
						property.New(true),
						property.New("three"),
					}))
					assertValue("anObject", property.New(map[string]property.Value{
						"key":    property.New("value"),
						"nested": property.New(map[string]property.Value{"count": property.New(1.0)}),
					}))
					assertValue("anAsset", property.New(asset))
				},
			},
		},
	}
}
