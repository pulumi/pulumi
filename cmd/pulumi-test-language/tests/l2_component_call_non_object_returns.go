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
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-component-call-non-object-returns"] = LanguageTest{
		Providers: []plugin.Provider{&providers.CallReturnsProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges, events []engine.Event,
				) {
					RequireStackResource(l, err, changes)

					// We expect the following resources:
					//
					// 0. The stack
					//
					// 1. The default component provider
					//
					// 2. The component1 resource
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					defaultProvider := RequireSingleNamedResource(l, snap.Resources, "default_18_0_0")
					require.Equal(
						l, "pulumi:providers:callreturnsprovider", defaultProvider.Type.String(),
						"expected default component provider",
					)

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					// The stack should have the following outputs:
					//
					// * from_identity, whose value should be the value of the input
					outputs := stack.Outputs
					require.Len(l, outputs, 1, "expected 1 output")
					AssertPropertyMapMember(l, outputs, "from_identity", resource.NewStringProperty("bar"))
				},
			},
		},
	}
}
