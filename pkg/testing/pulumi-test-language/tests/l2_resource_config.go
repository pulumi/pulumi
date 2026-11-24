// Copyright 2024, Pulumi Corporation.
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
	LanguageTests["l2-resource-config"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ConfigProvider{} },
		},
		Runs: []TestRun{
			{
				Config: config.Map{
					config.MustParseKey("config:name"): config.NewValue("hello"),
				},
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					explicitProvider := RequireSingleNamedResource(l, snap.Resources, "prov")
					require.Equal(l, "pulumi:providers:config", explicitProvider.Type.String(), "expected explicit provider resource")
					expectedOutputs := property.NewMap(map[string]property.Value{
						"name":              property.New("my config"),
						"pluginDownloadURL": property.New("not the same as the pulumi resource option"),
						"version":           property.New("9.0.0"),
					})
					expectedInputs := expectedOutputs.Set("__internal", property.New(map[string]property.Value{
						"pluginDownloadURL": property.New("http://example.com"),
					}))
					require.Equal(l, expectedInputs, resource.FromResourcePropertyMap(explicitProvider.Inputs))
					require.Equal(l, expectedOutputs, resource.FromResourcePropertyMap(explicitProvider.Outputs))

					defaultProvider := RequireSingleNamedResource(l, snap.Resources, "default_9_0_0_http_/example.com")
					require.Equal(l, "pulumi:providers:config", defaultProvider.Type.String(), "expected default provider resource")
					expectedOutputs = property.NewMap(map[string]property.Value{
						"version": property.New("9.0.0"),
						"name":    property.New("hello"),
					})
					expectedInputs = expectedOutputs.Set("__internal", property.New(map[string]property.Value{
						"pluginDownloadURL": property.New("http://example.com"),
					}))
					require.Equal(l, expectedInputs, resource.FromResourcePropertyMap(defaultProvider.Inputs))
					require.Equal(l, expectedOutputs, resource.FromResourcePropertyMap(defaultProvider.Outputs))
				},
			},
		},
	}
}
