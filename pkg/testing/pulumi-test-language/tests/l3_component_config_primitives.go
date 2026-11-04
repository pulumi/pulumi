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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l3-component-config-primitives"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.PrimitiveProvider{} },
		},
		Runs: []TestRun{
			{
				Config: config.Map{
					config.MustMakeKey("l3-component-config-primitives", "plainBool"):     config.NewValue("true"),
					config.MustMakeKey("l3-component-config-primitives", "plainNumber"):   config.NewValue("6.5"),
					config.MustMakeKey("l3-component-config-primitives", "plainInteger"):  config.NewValue("2"),
					config.MustMakeKey("l3-component-config-primitives", "plainString"):   config.NewValue("plain"),
					config.MustMakeKey("l3-component-config-primitives", "secretBool"):    config.NewSecureValue("ZmFsc2U="), // false
					config.MustMakeKey("l3-component-config-primitives", "secretNumber"):  config.NewSecureValue("Ny41"),     // 7.5
					config.MustMakeKey("l3-component-config-primitives", "secretInteger"): config.NewSecureValue("Nw=="),     // 7
					config.MustMakeKey("l3-component-config-primitives", "secretString"):  config.NewSecureValue("c2VjcmV0"), // secret
				},
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)
					// Stack, provider, 2 components, 2 primitive resources.
					require.Len(l, snap.Resources, 6, "expected 6 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:primitive")

					plainComp := RequireSingleNamedResource(l, snap.Resources, "plain")
					assert.Equal(l, "components:index:PrimitiveComponent", string(plainComp.Type))

					secretComp := RequireSingleNamedResource(l, snap.Resources, "secret")
					assert.Equal(l, "components:index:PrimitiveComponent", string(secretComp.Type))

					plain := RequireSingleNamedResource(l, snap.Resources, "plain-res")
					assert.Equal(l, plainComp.URN, plain.Parent, "expected plain resource to have plain component as parent")

					secret := RequireSingleNamedResource(l, snap.Resources, "secret-res")
					assert.Equal(l, secretComp.URN, secret.Parent, "expected secret resource to have secret component as parent")

					expectedPlain := property.NewMap(map[string]property.Value{
						"boolean": property.New(true),
						"float":   property.New(6.5),
						"integer": property.New(2.0),
						"string":  property.New("plain"),
						"numberArray": property.New([]property.Value{
							property.New(-1.0), property.New(0.0), property.New(1.0),
						}),
						"booleanMap": property.New(map[string]property.Value{
							"t": property.New(true), "f": property.New(false),
						}),
					})
					require.Equal(l, expectedPlain, resource.FromResourcePropertyMap(plain.Inputs))
					require.Equal(l, expectedPlain, resource.FromResourcePropertyMap(plain.Outputs))

					expectedSecret := property.NewMap(map[string]property.Value{
						"boolean": property.New(false).WithSecret(true),
						"float":   property.New(7.5).WithSecret(true),
						"integer": property.New(7.0).WithSecret(true),
						"string":  property.New("secret").WithSecret(true),
						"numberArray": property.New([]property.Value{
							property.New(-1.0), property.New(0.0), property.New(1.0),
						}),
						"booleanMap": property.New(map[string]property.Value{
							"t": property.New(true), "f": property.New(false),
						}),
					})
					require.Equal(l, expectedSecret, resource.FromResourcePropertyMap(secret.Inputs))
					require.Equal(l, expectedSecret, resource.FromResourcePropertyMap(secret.Outputs))
				},
			},
		},
	}
}
