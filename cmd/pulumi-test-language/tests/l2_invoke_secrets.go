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
	LanguageTests["l2-invoke-secrets"] = LanguageTest{
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
					var stack *resource.State
					for _, r := range snap.Resources {
						if r.Type == resource.RootStackType {
							stack = r
							break
						}
					}

					require.NotNil(l, stack, "expected a stack resource")

					outputs := stack.Outputs
					AssertPropertyMapMember(l, outputs, "nonSecret",
						resource.NewStringProperty("hello world"))
					AssertPropertyMapMember(l, outputs, "firstSecret",
						resource.MakeSecret(resource.NewStringProperty("hello world")))
					AssertPropertyMapMember(l, outputs, "secondSecret",
						resource.MakeSecret(resource.NewStringProperty("goodbye world")))
				},
			},
		},
	}
}
