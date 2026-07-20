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
	// Regression test for https://github.com/pulumi/pulumi/issues/23896: a ComponentResource authored inside a
	// component provider must be able to instantiate and use a provider resource from within its constructor.
	LanguageTests["l3-component-provider"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ConfigProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					// stack, component, explicit provider, config resource
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					component := RequireSingleResource(l, snap.Resources, "components:index:ProviderComponent")
					assert.Equal(l, "myComponent", component.URN.Name())

					provider := RequireSingleResource(l, snap.Resources, "pulumi:providers:config")
					assert.Equal(l, component.URN, provider.Parent,
						"expected explicit provider to be parented to the component")

					configRes := RequireSingleResource(l, snap.Resources, "config:index:Resource")
					assert.Equal(l, component.URN, configRes.Parent,
						"expected config resource to be parented to the component")
					assert.Equal(l, string(provider.URN)+"::"+string(provider.ID), configRes.Provider,
						"expected config resource to use the explicit provider")

					want := resource.NewPropertyMapFromMap(map[string]any{"text": "hello"})
					assert.Equal(l, want, resource.ToResourcePropertyMap(configRes.Inputs), "expected inputs to be %v", want)
					wantOut := resource.NewPropertyMapFromMap(map[string]any{"text": ": hello"})
					assert.Equal(l, wantOut, resource.ToResourcePropertyMap(configRes.Outputs), "expected outputs to be %v", wantOut)
				},
			},
		},
	}
}
