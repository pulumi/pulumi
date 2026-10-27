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
)

func init() {
	// Shared instance so the parameter persists across the harness's per-load
	// factory calls; see l2_extension_parameterized_resource.go.
	shared := &providers.ExtensionParameterizedProvider{}
	LanguageTests["l2-extension-invoke-provider-inheritance"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return shared },
		},
		Runs: []TestRun{
			{
				// An invoke that falls back to a default provider fails the update, so a passing update
				// shows that the invoke used prov.
				Config: config.Map{
					config.MustMakeKey("pulumi", "disable-default-providers"): config.NewObjectValue(`["extbase"]`),
				},
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					AssertPropertyMapMember(l, resource.FromResourcePropertyMap(stack.Outputs), "greeting",
						property.New("Hello, World"))
				},
			},
		},
	}
}
