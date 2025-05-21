// Copyright 2024, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-invoke-dependencies"] = LanguageTest{
		Providers: []plugin.Provider{
			&providers.SimpleInvokeProvider{},
			&providers.SimpleProvider{},
		},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					RequireStackResource(l, err, changes)
					var first *resource.State
					var second *resource.State
					for _, r := range snap.Resources {
						if r.URN.Name() == "first" {
							first = r
						}
						if r.URN.Name() == "second" {
							second = r
						}
					}

					require.NotNil(l, first, "expected first resource")
					require.NotNil(l, second, "expected second resource")
					require.Empty(l, first.Dependencies, "expected no dependencies")
					require.Len(l, second.Dependencies, 1, "expected one dependency")
					dependencies, ok := second.PropertyDependencies["value"]
					require.True(l, ok, "expected dependency on property 'value'")
					require.Len(l, dependencies, 1, "expected one dependency")
					require.Equal(l, first.URN, dependencies[0], "expected second to depend on first")
					require.Equal(l, first.URN, second.Dependencies[0], "expected second to depend on first")
				},
			},
		},
	}
}
