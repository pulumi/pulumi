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
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/stretchr/testify/require"
)

func init() {
	// Outputs of a create skipped by a targeted update must resolve as unknown, not null.
	// https://github.com/pulumi/pulumi-dotnet/issues/1057
	LanguageTests["l2-target-up-skipped-create-output"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.NestedObjectProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					require.Len(l, res.Snap.Resources, 5, "expected 5 resources in snapshot")
					require.NoError(l, res.Snap.VerifyIntegrity(), "expected snapshot to be valid")

					target := RequireSingleNamedResource(l, res.Snap.Resources, "target")
					require.Equal(l, "simple:index:Resource", target.Type.String(), "expected simple resource")
					other := RequireSingleNamedResource(l, res.Snap.Resources, "other")
					require.Equal(l, "nestedobject:index:Container", other.Type.String(), "expected nestedobject resource")
				},
			},
			{
				UpdateOptions: engine.UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{
						"**simple:index:Resource::target",
					}),
				},
				Assert: func(l *L, res AssertArgs) {
					require.NoError(l, res.Err, "expected no error from the targeted update")
					snap := res.Snap
					require.NoError(l, snap.VerifyIntegrity(), "expected snapshot to be valid")

					target := RequireSingleNamedResource(l, snap.Resources, "target")
					require.Equal(l, "simple:index:Resource", target.Type.String(), "expected simple resource")

					// The skipped resource must not have been created.
					for _, r := range snap.Resources {
						require.NotEqual(l, "skipped", r.URN.Name(), "expected skipped resource to not be in the snapshot")
					}

					// The stack output derived from the skipped resource's output must not have
					// resolved to a known value.
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					if out, has := stack.Outputs["skippedOutput"]; has {
						unknown := out.IsComputed() ||
							(out.IsString() && out.StringValue() == plugin.UnknownStringValue)
						require.True(l, unknown,
							"expected skippedOutput stack output to be unknown, got %v", out)
					}
				},
			},
		},
	}
}
