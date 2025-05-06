// Copyright 2024-2025, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-invoke-simple"] = LanguageTest{
		Providers: []plugin.Provider{&providers.SimpleInvokeProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 2, "expected 2 resource")

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
					AssertPropertyMapMember(l, outputs, "goodbye", resource.NewStringProperty("goodbye world"))
				},
			},
		},
	}
}
