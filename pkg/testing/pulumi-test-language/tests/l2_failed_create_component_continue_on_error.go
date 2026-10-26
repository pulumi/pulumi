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
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-failed-create-component-continue-on-error"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.FailingComponentProvider{} },
			func() plugin.Provider { return &providers.SimpleProvider{} },
		},
		Runs: []TestRun{
			{
				UpdateOptions: engine.UpdateOptions{
					ContinueOnError: true,
				},
				// The remote component's child fails to create, so the component's output that comes from the
				// child must be unknown in the program: resources that depend on it are skipped, and the stack
				// output derived from it is unknown rather than an empty value.
				Assert: func(l *L, res AssertArgs) {
					require.True(l, result.IsBail(res.Err), "expected a bail result")
					require.NotNil(l, res.Snap, "expected snapshot to be non-nil")

					RequireSingleNamedResource(l, res.Snap.Resources, "independent")
					for _, r := range res.Snap.Resources {
						require.NotEqual(l, "dependent", r.URN.Name(), "expected dependent to be skipped")
					}

					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					require.Contains(l, stack.Outputs, resource.PropertyKey("componentValue"))
					assert.True(l, stack.Outputs["componentValue"].IsComputed(),
						"expected stack output 'componentValue' to be unknown, got %v", stack.Outputs["componentValue"])
				},
			},
		},
	}
}
