// Copyright 2025, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-option-alias"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.ComponentProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, args AssertArgs) {
					RequireStackResource(l, args.Err, args.Changes)

					// 1 stack + 2 providers + 6 resources = 9
					require.Len(l, args.Snap.Resources, 9, "expected 9 resources in snapshot")
				},
			},
			{
				Assert: func(l *L, args AssertArgs) {
					snap := args.Snap
					changes := args.Changes

					// Don't expect any creates.
					require.Equal(l, 0, changes[deploy.OpCreate], "expected no create operations")

					// 1 stack + simple provider + 6 resources = 8
					require.Len(l, snap.Resources, 8, "expected 8 resources in snapshot")

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					parent := RequireSingleNamedResource(l, snap.Resources, "parent")
					assert.Equal(l, stack.URN, parent.Parent, "expected stack to be parent of parent resource")

					aliasURN := RequireSingleNamedResource(l, snap.Resources, "aliasURN")
					assert.Equal(l, parent.URN, aliasURN.Parent, "expected parent to be parent of aliasURN resource")

					aliasNoParent := RequireSingleNamedResource(l, snap.Resources, "aliasNoParent")
					assert.Equal(l, parent.URN, aliasNoParent.Parent, "expected parent to be parent of aliasNoParent resource")

					aliasParent := RequireSingleNamedResource(l, snap.Resources, "aliasParent")
					assert.Equal(l, parent.URN, aliasParent.Parent, "expected parent to be parent of aliasParent resource")

					aliasType := RequireSingleNamedResource(l, snap.Resources, "aliasType")
					assert.Equal(l, "simple:index:Resource", string(aliasType.Type),
						"expected aliasType to be of type simple:index:Resource")
				},
			},
		},
	}
}
