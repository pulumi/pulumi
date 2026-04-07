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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l3-component-primitive-conversions"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.PrimitiveProvider{} },
		},
		Runs: []TestRun{
			{
				//nolint:lll
				Config: config.Map{
					config.MustMakeKey("l3-component-primitive-conversions", "plainBool"):           config.NewValue("true"),
					config.MustMakeKey("l3-component-primitive-conversions", "plainNumber"):         config.NewValue("6.5"),
					config.MustMakeKey("l3-component-primitive-conversions", "plainInteger"):        config.NewValue("2"),
					config.MustMakeKey("l3-component-primitive-conversions", "plainString"):         config.NewValue("true"),
					config.MustMakeKey("l3-component-primitive-conversions", "plainNumericString"):  config.NewValue("42"),
					config.MustMakeKey("l3-component-primitive-conversions", "secretNumber"):        config.NewSecureValue("Ny41"),     // 7.5
					config.MustMakeKey("l3-component-primitive-conversions", "secretInteger"):       config.NewSecureValue("Nw=="),     // 7
					config.MustMakeKey("l3-component-primitive-conversions", "secretString"):        config.NewSecureValue("ZmFsc2U="), // false
					config.MustMakeKey("l3-component-primitive-conversions", "secretNumericString"): config.NewSecureValue("ODQ="),     // 84
				},
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)
					// Stack, provider, 2 components, and 2 primitive resources.
					require.Len(l, snap.Resources, 6, "expected 6 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:primitive")

					plainComponent := RequireSingleNamedResource(l, snap.Resources, "plainValues")
					assert.Equal(l, "components:index:ConversionComponent", string(plainComponent.Type))

					secretComponent := RequireSingleNamedResource(l, snap.Resources, "secretValues")
					assert.Equal(l, "components:index:ConversionComponent", string(secretComponent.Type))

					plainValues := RequireSingleNamedResource(l, snap.Resources, "plainValues-res")
					assert.Equal(l, plainComponent.URN, plainValues.Parent,
						"expected plain resource to have plain component as parent")

					secretValues := RequireSingleNamedResource(l, snap.Resources, "secretValues-res")
					assert.Equal(l, secretComponent.URN, secretValues.Parent,
						"expected secret resource to have secret component as parent")

					expectedPlainValues := resource.NewPropertyMapFromMap(map[string]any{
						"boolean":     true,
						"float":       2.0,
						"integer":     42,
						"string":      "6.5",
						"numberArray": []any{2.0, 42.0, 6.5},
						"booleanMap":  map[string]any{"fromBool": true, "fromString": true},
					})
					require.Equal(l, expectedPlainValues, plainValues.Inputs)
					require.Equal(l, expectedPlainValues, plainValues.Outputs)

					expectedSecretValues := resource.NewPropertyMapFromMap(map[string]any{
						"boolean": resource.MakeSecret(resource.NewProperty(false)),
						"float":   resource.MakeSecret(resource.NewProperty(7.0)),
						"integer": resource.MakeSecret(resource.NewProperty(84.0)),
						"string":  resource.MakeSecret(resource.NewProperty("7.5")),
						"numberArray": []any{
							2.0,
							42.0,
							6.5,
						},
						"booleanMap": map[string]any{"fromBool": true, "fromString": true},
					})
					require.Equal(l, expectedSecretValues, secretValues.Inputs)
					require.Equal(l, expectedSecretValues, secretValues.Outputs)
				},
			},
		},
	}
}
