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
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-invoke-provider-inheritance"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleInvokeProvider{} },
		},
		Runs: []TestRun{
			{
				// An invoke that falls back to a default provider fails the update, so a passing update
				// shows that each invoke used the explicit provider of its parent.
				Config: config.Map{
					config.MustMakeKey("pulumi", "disable-default-providers"): config.NewObjectValue(`["simple-invoke"]`),
				},
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					parent := RequireSingleNamedResource(l, res.Snap.Resources, "parent")
					child := RequireSingleNamedResource(l, res.Snap.Resources, "child")
					require.Equal(l, parent.Provider, child.Provider,
						"expected parent and child to have the same provider reference")

					stack := RequireSingleResource(l, res.Snap.Resources, resource.RootStackType)
					outputs := resource.FromResourcePropertyMap(stack.Outputs)
					AssertPropertyMapMember(l, outputs, "parentResult", property.New("parent world"))
					AssertPropertyMapMember(l, outputs, "childResult", property.New("child world"))
				},
			},
		},
	}
}
