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
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l3-range-list-ref"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.NestedObjectProvider{} },
		},
		Runs: []TestRun{
			{
				Config: config.Map{
					config.MustMakeKey("l3-range-list-ref", "numItems"): config.NewValue("2"),
					config.MustMakeKey("l3-range-list-ref", "itemList"): config.NewObjectValue(`["a", "b"]`),
				},
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					// stack + nestedobject provider + 2 numResource + numTarget + 2 listResource +
					// listTarget + 2 listDynTarget
					require.Len(l, res.Snap.Resources, 10)

					RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:nestedobject")

					numTarget := RequireSingleNamedResource(l, res.Snap.Resources, "numTarget")
					numTargetInputs := resource.ToResourcePropertyMap(numTarget.Inputs)
					AssertPropertyMapMember(l, numTargetInputs, "name", resource.NewProperty("num-0+"))

					listTarget := RequireSingleNamedResource(l, res.Snap.Resources, "listTarget")
					AssertPropertyMapMember(l, resource.ToResourcePropertyMap(listTarget.Inputs), "name", resource.NewProperty("1:b+"))

					listDynTarget0 := RequireSingleNamedResource(l, res.Snap.Resources, "listDynTarget-0")
					listDynTarget0Inputs := resource.ToResourcePropertyMap(listDynTarget0.Inputs)
					AssertPropertyMapMember(l, listDynTarget0Inputs, "name", resource.NewProperty("0:a!"))

					listDynTarget1 := RequireSingleNamedResource(l, res.Snap.Resources, "listDynTarget-1")
					listDynTarget1Inputs := resource.ToResourcePropertyMap(listDynTarget1.Inputs)
					AssertPropertyMapMember(l, listDynTarget1Inputs, "name", resource.NewProperty("1:b!"))
				},
			},
		},
	}
}
