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
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-option-replace-on-changes"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ReplaceOnChangesProvider{} },
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.ComponentProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					snap := res.Snap

					RequireStackResource(l, res.Err, res.Changes)

					// Stack, 3 providers, 8 custom resources, 1 remote component + child, and 1 simple resource.
					require.Len(l, snap.Resources, 15, "expected 15 resources in snapshot")

					RequireSingleNamedResource(l, snap.Resources, "schemaReplace")
					RequireSingleNamedResource(l, snap.Resources, "optionReplace")
					RequireSingleNamedResource(l, snap.Resources, "bothReplaceValue")
					RequireSingleNamedResource(l, snap.Resources, "bothReplaceProp")
					RequireSingleNamedResource(l, snap.Resources, "regularUpdate")
					RequireSingleNamedResource(l, snap.Resources, "noChange")
					RequireSingleNamedResource(l, snap.Resources, "wrongPropChange")
					RequireSingleNamedResource(l, snap.Resources, "multiplePropReplace")
					RequireSingleNamedResource(l, snap.Resources, "remoteWithReplace")
					RequireSingleNamedResource(l, snap.Resources, "simpleResource")
				},
			},
			{
				AssertPreview: func(_ *L, _ AssertPreviewArgs) {},
				Assert: func(l *L, res AssertArgs) {
					require.GreaterOrEqual(l, len(res.Snap.Resources), 15, "expected at least 15 resources in snapshot")
					require.NoError(l, res.Err, "expected no error")
					require.GreaterOrEqual(l, res.Changes[deploy.OpReplace], 4, "expected several replacements")
					require.GreaterOrEqual(l, res.Changes[deploy.OpUpdate], 1, "expected at least one update")
					RequireSingleNamedResource(l, res.Snap.Resources, "remoteWithReplace")
					RequireSingleNamedResource(l, res.Snap.Resources, "simpleResource")
				},
			},
		},
	}
}
