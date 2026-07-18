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
	testRun := func(config config.Map, expected string) TestRun {
		return TestRun{
			Config: config,
			Assert: func(l *L, res AssertArgs) {
				require.NoError(l, res.Err)

				stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
				AssertPropertyMapMember(l, resource.ToResourcePropertyMap(stack.Outputs), "result", resource.NewProperty(expected))
			},
		}
	}
	// Exercises a config variable whose default is derived from the result of an invoke. Codegen
	// must promote the config variable to an output because its default carries a promise type.
	LanguageTests["l2-config-default-from-invoke"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleInvokeProvider{} },
		},
		RunsShareSource: true,
		Runs: []TestRun{
			testRun(nil, "hello world"),
			testRun(config.Map{
				config.MustMakeKey("l2-config-default-from-invoke", "defaultFromInvoke"): config.NewValue("goodbye world"),
			}, "goodbye world"),
		},
	}
}
