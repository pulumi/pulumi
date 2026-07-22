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
	// An invoke written inside a component must be parented to that component, just like a resource is.
	// The parent is what an invoke resolves its provider from, so an invoke in a component instantiated with
	// `providers = [prov]` must be served by `prov`. `getConfig` echoes back the `name` its provider was
	// configured with, which is how the program observes which provider instance served it.
	LanguageTests["l3-component-invoke-provider"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ConfigProvider{} },
			func() plugin.Provider { return &providers.MultiArgumentInvokeProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					// stack, explicit config provider, default multi-argument-invoke provider, component.
					// A second `pulumi:providers:config` would mean the getConfig invoke fell back to the
					// default provider instead of the one the component was given.
					require.Len(l, res.Snap.Resources, 4, "expected 4 resources in snapshot")

					provider := RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:config")
					assert.Equal(l, "prov", provider.URN.Name())

					component := RequireSingleResource(l, res.Snap.Resources, "components:index:InvokeComponent")
					want := resource.NewPropertyMapFromMap(map[string]any{"result": "my config: hello"})
					assert.Equal(l, want, component.Outputs)
				},
			},
		},
	}
}
