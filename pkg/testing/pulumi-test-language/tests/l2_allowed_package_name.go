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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-allowed-package-name"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ExtraPackageNamesProvider{} },
		},
		Runs: []TestRun{{
			Assert: func(l *L, res AssertArgs) {
				err, snapshot, changes := res.Err, res.Snap, res.Changes
				RequireStackResource(l, err, changes)

				require.Len(l, snapshot.Resources, 5,
					"stack, explicit provider, default provider, and 2 resources")

				viaProvider := RequireSingleNamedResource(l, snapshot.Resources, "viaProvider")
				assert.Equal(l, resource.PropertyMap{
					"choice": resource.NewProperty("first"),
					"obj": resource.NewProperty(resource.PropertyMap{
						"label":  resource.NewProperty("explicit"),
						"choice": resource.NewProperty("second"),
					}),
				}, viaProvider.Outputs)

				explicitProvider := RequireSingleNamedResource(l, snapshot.Resources, "prov")
				assert.Equal(l, viaProvider.Provider, string(explicitProvider.URN)+"::"+string(explicitProvider.ID))

				viaPackage := RequireSingleNamedResource(l, snapshot.Resources, "viaPackage")
				assert.Equal(l, resource.PropertyMap{
					"choice": resource.NewProperty("second"),
					"obj": resource.NewProperty(resource.PropertyMap{
						"label":  resource.NewProperty("bare"),
						"choice": resource.NewProperty("first"),
					}),
				}, viaPackage.Outputs)

				defaultProvider := RequireSingleNamedResource(l, snapshot.Resources, "default_47_0_0")
				assert.Equal(l, viaPackage.Provider, string(defaultProvider.URN)+"::"+string(defaultProvider.ID))

				// The stack is not reliably the first resource; a default provider can
				// register ahead of it. See pulumi/pulumi#17816.
				stack := RequireSingleResource(l, snapshot.Resources, resource.RootStackType)
				assert.Equal(l, resource.PropertyMap{
					"result": resource.NewProperty("got: hello"),
				}, stack.Outputs)
			},
		}},
	}
}
