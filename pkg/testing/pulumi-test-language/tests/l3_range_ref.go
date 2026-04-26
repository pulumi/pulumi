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
	LanguageTests["l3-range-ref"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.NestedObjectProvider{} },
		},
		Runs: []TestRun{
			{
				Config: config.Map{
					config.MustMakeKey("l3-range-ref", "numItems"):   config.NewValue("2"),
					config.MustMakeKey("l3-range-ref", "itemList"):   config.NewObjectValue(`["a", "b"]`),
					config.MustMakeKey("l3-range-ref", "itemMap"):    config.NewObjectValue(`{"k1": "v1", "k2": "v2"}`),
					config.MustMakeKey("l3-range-ref", "createBool"): config.NewValue("true"),
				},
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					// stack + nestedobject provider + 4 targets + 7 sources
					require.Len(l, res.Snap.Resources, 13)

					RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:nestedobject")

					numTarget := RequireSingleNamedResource(l, res.Snap.Resources, "numTarget")
					AssertPropertyMapMember(l, numTarget.Inputs, "name", resource.NewProperty("num-0+"))

					listTarget := RequireSingleNamedResource(l, res.Snap.Resources, "listTarget")
					AssertPropertyMapMember(l, listTarget.Inputs, "name", resource.NewProperty("1:b+"))

					mapTarget := RequireSingleNamedResource(l, res.Snap.Resources, "mapTarget")
					AssertPropertyMapMember(l, mapTarget.Inputs, "name", resource.NewProperty("k1=v1+"))

					boolTarget := RequireSingleNamedResource(l, res.Snap.Resources, "boolTarget")
					AssertPropertyMapMember(l, boolTarget.Inputs, "name", resource.NewProperty("bool-resource+"))
				},
			},
		},
	}
}
