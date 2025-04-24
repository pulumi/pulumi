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
	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/deepcopy"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-config"] = LanguageTest{
		Providers: []plugin.Provider{&providers.ConfigProvider{}},
		Runs: []TestRun{
			{
				Config: config.Map{
					config.MustParseKey("config:name"): config.NewValue("hello"),
				},
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					explicitProvider := RequireSingleNamedResource(l, snap.Resources, "prov")
					require.Equal(l, "pulumi:providers:config", explicitProvider.Type.String(), "expected explicit provider resource")
					expectedOutputs := resource.NewPropertyMapFromMap(map[string]interface{}{
						"name":              "my config",
						"pluginDownloadURL": "not the same as the pulumi resource option",
						"version":           "9.0.0",
					})
					expectedInputs := deepcopy.Copy(expectedOutputs).(resource.PropertyMap)
					// inputs should also have the __internal key
					expectedInputs[resource.PropertyKey("__internal")] = resource.NewObjectProperty(
						resource.NewPropertyMapFromMap(map[string]interface{}{
							"pluginDownloadURL": "http://example.com",
						}))
					require.Equal(l, expectedInputs, explicitProvider.Inputs)
					require.Equal(l, expectedOutputs, explicitProvider.Outputs)

					defaultProvider := RequireSingleNamedResource(l, snap.Resources, "default_9_0_0_http_/example.com")
					require.Equal(l, "pulumi:providers:config", defaultProvider.Type.String(), "expected default provider resource")
					expectedOutputs = resource.NewPropertyMapFromMap(map[string]interface{}{
						"version": "9.0.0",
						"name":    "hello",
					})
					expectedInputs = deepcopy.Copy(expectedOutputs).(resource.PropertyMap)
					// inputs should also have the __internal key
					expectedInputs[resource.PropertyKey("__internal")] = resource.NewObjectProperty(
						resource.NewPropertyMapFromMap(map[string]interface{}{
							"pluginDownloadURL": "http://example.com",
						}))
					require.Equal(l, expectedInputs, defaultProvider.Inputs)
					require.Equal(l, expectedOutputs, defaultProvider.Outputs)
				},
			},
		},
	}
}
