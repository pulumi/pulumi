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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["provider-depends-on-component"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.ConformanceComponentProvider{} },
		},
		LanguageProviders: []string{"conformance-component"},
		Runs: []TestRun{
			{
				Assert: func(l *L, args AssertArgs) {
					err := args.Err
					snap := args.Snap
					changes := args.Changes

					RequireStackResource(l, err, changes)

					// Stack, 2 providers, noDependsOn, noDependsOn-child, withDependsOn,
					// withDependsOn-child, simpleResource
					require.Len(l, snap.Resources, 9, "expected 9 resources in snapshot")

					noDependsOn := RequireSingleNamedResource(l, snap.Resources, "noDependsOn")
					assert.Empty(l, noDependsOn.Dependencies, "expected noDependsOn component to have no dependencies")

					withDependsOn := RequireSingleNamedResource(l, snap.Resources, "withDependsOn")
					assert.Equal(l, []resource.URN{noDependsOn.URN}, withDependsOn.Dependencies,
						"expected withDependsOn component to depend on noDependsOn")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:conformance-component")
					RequireSingleResource(l, snap.Resources, "pulumi:providers:simple")
					RequireSingleNamedResource(l, snap.Resources, "noDependsOn-child")
					RequireSingleNamedResource(l, snap.Resources, "withDependsOn-child")
					RequireSingleNamedResource(l, snap.Resources, "simpleResource")
				},
			},
		},
	}
}
