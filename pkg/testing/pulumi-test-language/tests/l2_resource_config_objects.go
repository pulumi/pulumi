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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-config-objects"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.PrimitiveProvider{} },
		},
		Runs: []TestRun{
			{
				Config: config.Map{
					config.MustMakeKey("l2-resource-config-objects", "plainNumberArray"): config.NewObjectValue(`[-1,0,1]`),             //nolint:lll
					config.MustMakeKey("l2-resource-config-objects", "plainBooleanMap"):  config.NewObjectValue(`{"t":true,"f":false}`), //nolint:lll
					// [-2,0,2]
					config.MustMakeKey("l2-resource-config-objects", "secretNumberArray"): config.NewSecureValue("Wy0yLDAsMl0="), //nolint:lll
					// {"t":true,"f":false}
					config.MustMakeKey("l2-resource-config-objects", "secretBooleanMap"): config.NewSecureValue("eyJ0Ijp0cnVlLCJmIjpmYWxzZX0="), //nolint:lll
				},
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:primitive")
					plain := RequireSingleNamedResource(l, snap.Resources, "plain")
					secret := RequireSingleNamedResource(l, snap.Resources, "secret")

					expectedPlain := resource.NewPropertyMapFromMap(map[string]any{
						"boolean":     true,
						"float":       3.5,
						"integer":     3,
						"string":      "plain",
						"numberArray": []any{-1.0, 0.0, 1.0},
						"booleanMap":  map[string]any{"t": true, "f": false},
					})
					require.Equal(l, expectedPlain, plain.Inputs)
					require.Equal(l, expectedPlain, plain.Outputs)

					expectedSecret := resource.NewPropertyMapFromMap(map[string]any{
						"boolean": true,
						"float":   3.5,
						"integer": 3,
						"string":  "secret",
						"numberArray": resource.MakeSecret(resource.NewProperty([]resource.PropertyValue{
							resource.NewProperty(-2.0),
							resource.NewProperty(0.0),
							resource.NewProperty(2.0),
						})),
						"booleanMap": resource.MakeSecret(resource.NewProperty(resource.PropertyMap{
							"t": resource.NewProperty(true),
							"f": resource.NewProperty(false),
						})),
					})
					require.Equal(l, expectedSecret, secret.Inputs)
					require.Equal(l, expectedSecret, secret.Outputs)
				},
			},
		},
	}
}
