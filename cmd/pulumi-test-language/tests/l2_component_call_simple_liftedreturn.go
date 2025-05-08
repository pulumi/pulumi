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
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-component-call-simple-liftedreturn"] = LanguageTest{
		Providers: []plugin.Provider{&providers.ComponentProviderReturnScalar{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					// We expect the following resources:
					//
					// 0. The stack
					//
					// 1. The builtin Pulumi provider (used for hydrating resource references)
					// 2. The default component provider
					//
					// 3. The component1 resource
					// 4. The child of the component1 resource
					require.Len(l, snap.Resources, 5, "expected 5 resources in snapshot")

					builtinProvider := RequireSingleNamedResource(l, snap.Resources, "default")
					require.Equal(
						l, "pulumi:providers:pulumi", builtinProvider.Type.String(),
						"expected builtin provider",
					)

					defaultProvider := RequireSingleNamedResource(l, snap.Resources, "default_18_0_0")
					require.Equal(
						l, "pulumi:providers:componentreturnscalar", defaultProvider.Type.String(),
						"expected default component provider",
					)

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					component1 := RequireSingleNamedResource(l, snap.Resources, "component1")

					// component1 should satisfy the following properties:
					//
					// * Its value output should be "bar".
					require.Equal(
						l, "bar", component1.Outputs["value"].StringValue(),
						"expected component1 to have correct value output",
					)

					// The stack should have the following outputs:
					//
					// * from_identity, whose value should be the value output of component1
					// * from_prefixed, whose value should be the value output of component1, prefixed with "foo-".
					outputs := stack.Outputs
					require.Len(l, outputs, 2, "expected 2 outputs")
					AssertPropertyMapMember(l, outputs, "from_identity", resource.NewStringProperty("bar"))
					AssertPropertyMapMember(l, outputs, "from_prefixed", resource.NewStringProperty("foo-bar"))
				},
			},
		},
	}
}
