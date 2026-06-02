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
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-exists"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					// Snapshot should have: stack + default provider + the created resource = 3
					// The exists call does NOT register a resource.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					// Verify the resource was created
					_ = RequireSingleNamedResource(l, snap.Resources, "res")

					// Verify the output was set to true (resource exists)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					require.NotNil(l, stack.Outputs, "expected stack outputs")
					existsResult, ok := stack.Outputs["existsResult"]
					require.True(l, ok, "expected existsResult output")
					require.Equal(l, resource.NewProperty(true), existsResult, "expected existsResult to be true")
				},
			},
		},
	}
}
