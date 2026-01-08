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
	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-proxy-index"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.RefRefProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					projectDirectory, err, snap, changes, events, sdks := res.ProjectDirectory, res.Err, res.Snap, res.Changes, res.Events, res.SDKs
					_, _, _, _, _, _ = projectDirectory, err, snap, changes, events, sdks
					RequireStackResource(l, err, changes)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := stack.Outputs

					require.Len(l, outputs, 4, "expected 4 outputs")
					AssertPropertyMapMember(l, outputs, "bool", resource.NewProperty(true))
					AssertPropertyMapMember(l, outputs, "array", resource.NewProperty(true))
					AssertPropertyMapMember(l, outputs, "map", resource.NewProperty("100"))
					AssertPropertyMapMember(l, outputs, "nested", resource.NewProperty("french hens"))
				},
			},
		},
	}
}
