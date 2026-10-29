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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-primitive-conversions"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.PrimitiveProvider{} },
		},
		Runs: []TestRun{
			{
				//nolint:lll
				Config: config.Map{
					config.MustMakeKey("l2-resource-primitive-conversions", "plainBool"):           config.NewValue("true"),
					config.MustMakeKey("l2-resource-primitive-conversions", "plainNumber"):         config.NewValue("6.5"),
					config.MustMakeKey("l2-resource-primitive-conversions", "plainInteger"):        config.NewValue("2"),
					config.MustMakeKey("l2-resource-primitive-conversions", "plainString"):         config.NewValue("true"),
					config.MustMakeKey("l2-resource-primitive-conversions", "plainNumericString"):  config.NewValue("42"),
					config.MustMakeKey("l2-resource-primitive-conversions", "secretNumber"):        config.NewSecureValue("Ny41"),     // 7.5
					config.MustMakeKey("l2-resource-primitive-conversions", "secretInteger"):       config.NewSecureValue("Nw=="),     // 7
					config.MustMakeKey("l2-resource-primitive-conversions", "secretString"):        config.NewSecureValue("ZmFsc2U="), // false
					config.MustMakeKey("l2-resource-primitive-conversions", "secretNumericString"): config.NewSecureValue("ODQ="),     // 84
				},
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)
					// Stack, provider, local invoke result, and 3 primitive resources.
					require.Len(l, snap.Resources, 5, "expected 5 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:primitive")
					plainValues := RequireSingleNamedResource(l, snap.Resources, "plainValues")
					secretValues := RequireSingleNamedResource(l, snap.Resources, "secretValues")
					invokeValues := RequireSingleNamedResource(l, snap.Resources, "invokeValues")

					expectedPlainValues := property.NewMap(map[string]property.Value{
						"boolean": property.New(true),
						"float":   property.New(2.0),
						"integer": property.New(42.0),
						"string":  property.New("6.5"),
						"numberArray": property.New([]property.Value{
							property.New(2.0), property.New(42.0), property.New(6.5),
						}),
						"booleanMap": property.New(map[string]property.Value{
							"fromBool": property.New(true), "fromString": property.New(true),
						}),
					})
					require.Equal(l, expectedPlainValues, resource.FromResourcePropertyMap(plainValues.Inputs))
					require.Equal(l, expectedPlainValues, resource.FromResourcePropertyMap(plainValues.Outputs))

					expectedSecretValues := property.NewMap(map[string]property.Value{
						"boolean": property.New(false).WithSecret(true),
						"float":   property.New(7.0).WithSecret(true),
						"integer": property.New(84.0).WithSecret(true),
						"string":  property.New("7.5").WithSecret(true),
						"numberArray": property.New([]property.Value{
							property.New(2.0), property.New(42.0), property.New(6.5),
						}),
						"booleanMap": property.New(map[string]property.Value{
							"fromBool": property.New(true), "fromString": property.New(true),
						}),
					})
					require.Equal(l, expectedSecretValues, resource.FromResourcePropertyMap(secretValues.Inputs))
					require.Equal(l, expectedSecretValues, resource.FromResourcePropertyMap(secretValues.Outputs))

					expectedInvokeValues := property.NewMap(map[string]property.Value{
						"boolean": property.New(true),
						"float":   property.New(2.0),
						"integer": property.New(42.0),
						"string":  property.New("true"),
						"numberArray": property.New([]property.Value{
							property.New(2.0), property.New(42.0), property.New(6.5),
						}),
						"booleanMap": property.New(map[string]property.Value{
							"fromBool": property.New(true), "fromString": property.New(true),
						}),
					})
					require.Equal(l, expectedInvokeValues, resource.FromResourcePropertyMap(invokeValues.Inputs))
					require.Equal(l, expectedInvokeValues, resource.FromResourcePropertyMap(invokeValues.Outputs))
				},
			},
		},
	}
}
