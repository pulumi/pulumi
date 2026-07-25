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
	"github.com/stretchr/testify/assert"
)

func init() {
	// l2-component-extends-methods calls the inherited getStatus method on a Derived instance. getStatus is declared
	// only on Base and not overridden, so it dispatches on the base-owned function token (inherit:index:Base/getStatus)
	// even though the receiver's URN carries the derived type. The provider's Call handler accepts only that token and
	// echoes it back, so a successful run with the expected status output proves the base token was used.
	LanguageTests["l2-component-extends-methods"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.InheritanceProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					snap := res.Snap

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					status, ok := stack.Outputs["status"]
					assert.True(l, ok, "expected a status stack output")
					assert.Equal(l, "inherit:index:Base/getStatus", status.StringValue(),
						"the inherited method must dispatch on the base-owned function token")
				},
			},
		},
	}
}
