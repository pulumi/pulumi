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
	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-option-parent"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					// Stack, provider, parent resource, and 2 child resources
					require.Len(l, res.Snap.Resources, 5, "expected 5 resources in snapshot")

					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")

					parent := RequireSingleNamedResource(l, res.Snap.Resources, "parent")
					assert.Equal(l, stack.URN, parent.Parent, "expected stack to be parent of parent resource")

					withParent := RequireSingleNamedResource(l, res.Snap.Resources, "withParent")
					assert.Equal(l, parent.URN, withParent.Parent, "expected parent to be parent of withParent resource")

					noParent := RequireSingleNamedResource(l, res.Snap.Resources, "noParent")
					assert.Equal(l, stack.URN, noParent.Parent, "expected stack to be parent of noParent resource")
				},
			},
		},
	}
}
