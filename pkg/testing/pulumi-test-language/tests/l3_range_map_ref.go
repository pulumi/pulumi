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
	LanguageTests["l3-range-map-ref"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.NestedObjectProvider{} },
		},
		Runs: []TestRun{
			{
				Config: config.Map{
					config.MustMakeKey("l3-range-map-ref", "itemMap"): config.NewObjectValue(`{"k1": "v1", "k2": "v2"}`),
				},
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					// stack + nestedobject provider + 2 mapResource + mapTarget
					require.Len(l, res.Snap.Resources, 5)

					RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:nestedobject")

					mapTarget := RequireSingleNamedResource(l, res.Snap.Resources, "mapTarget")
					AssertPropertyMapMember(l, mapTarget.Inputs, "name", resource.NewProperty("k1=v1+"))
				},
			},
		},
	}
}
