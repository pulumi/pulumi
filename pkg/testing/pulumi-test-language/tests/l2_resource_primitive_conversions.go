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

					expectedInvokeValues := resource.NewPropertyMapFromMap(map[string]any{
						"boolean":     true,
						"float":       2.0,
						"integer":     42,
						"string":      "true",
						"numberArray": []any{2.0, 42.0, 6.5},
						"booleanMap":  map[string]any{"fromBool": true, "fromString": true},
					})
					require.Equal(l, expectedInvokeValues, invokeValues.Inputs)
					require.Equal(l, expectedInvokeValues, invokeValues.Outputs)
				},
			},
		},
	}
}
