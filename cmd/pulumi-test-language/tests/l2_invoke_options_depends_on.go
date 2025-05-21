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
	LanguageTests["l2-invoke-options-depends-on"] = LanguageTest{
		Providers: []plugin.Provider{&providers.SimpleInvokeProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					RequireStackResource(l, err, changes)
					require.Len(l, snap.Resources, 5, "expected 5 resources")
					// TODO https://github.com/pulumi/pulumi/issues/17816
					// TODO: the root stack must be the first resource to be registered
					// such that snap.Resources[0].Type == resource.RootStackType
					// however with the python SDK, that is not the case, instead the default
					// provider gets registered first. This is indicating that something might be wrong
					// with the how python SDK registers resources
					var stack *resource.State
					for _, r := range snap.Resources {
						if r.Type == resource.RootStackType {
							stack = r
							break
						}
					}

					require.NotNil(l, stack, "expected a stack resource")
					outputs := stack.Outputs
					AssertPropertyMapMember(l, outputs, "hello", resource.NewStringProperty("hello world"))

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
					dependencies, ok := second.PropertyDependencies["text"]
					require.True(l, ok, "expected dependency on property 'text'")
					require.Len(l, dependencies, 1, "expected one dependency")
					require.Equal(l, first.URN, dependencies[0], "expected second to depend on first")
					require.Equal(l, first.URN, second.Dependencies[0], "expected second to depend on first")
				},
			},
		},
	}
}
