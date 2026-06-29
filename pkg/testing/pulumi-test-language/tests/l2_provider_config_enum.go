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
	LanguageTests["l2-provider-config-enum"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ConfigEnumProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err, snap, changes := res.Err, res.Snap, res.Changes
					RequireStackResource(l, err, changes)

					// Expect three resources: the stack, the explicit provider and the
					// downstream resource that reads the provider's outputs.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					provider := RequireSingleResource(l, snap.Resources, "pulumi:providers:config-enum")
					assert.Equal(l, "prov", provider.URN.Name(), "expected explicit provider resource")

					downstream := RequireSingleResource(l, snap.Resources, "config-enum:index:Resource")
					assert.Equal(l, string(provider.URN)+"::"+string(provider.ID), downstream.Provider)

					// The downstream resource sources its inputs from the provider's
					// outputs. If enums are dropped from provider outputs then `theEnum`
					// will be missing here.
					want := resource.NewPropertyMapFromMap(map[string]any{
						"theString": "hello",
						"theEnum":   "two",
					})
					assert.Equal(l, want, downstream.Inputs, "expected inputs sourced from provider outputs")
					assert.Equal(l, downstream.Inputs, downstream.Outputs, "expected inputs and outputs to match")
				},
			},
		},
	}
}
