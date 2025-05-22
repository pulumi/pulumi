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
	LanguageTests["l2-namespaced-provider"] = LanguageTest{
		Providers: []plugin.Provider{&providers.ComponentProvider{}, &providers.NamespacedProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					RequireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 7, "expected 5 resources in snapshot")

					provider := RequireSingleResource(l, snap.Resources, "pulumi:providers:namespaced")
					require.Equal(l, "pulumi:providers:namespaced", provider.Type.String(), "expected namespaced provider")
					componentProv := RequireSingleResource(l, snap.Resources, "pulumi:providers:component")
					require.Equal(l, "pulumi:providers:component", componentProv.Type.String(), "expected component provider")

					namespaced := RequireSingleResource(l, snap.Resources, "namespaced:index:Resource")
					require.Equal(l, "namespaced:index:Resource", namespaced.Type.String(), "expected namespaced resource")

					//nolint:lll // Breaking the URN up makes it harder to read
					want := resource.NewPropertyMapFromMap(map[string]any{
						"value": true,
						"resourceRef": resource.NewResourceReferenceProperty(resource.ResourceReference{
							URN: "urn:pulumi:test::l2-namespaced-provider::component:index:ComponentCustomRefOutput$component:index:Custom::componentRes-child",
							ID:  resource.NewStringProperty("id-foo-bar-baz"),
						}),
					})
					require.Equal(l, want, namespaced.Inputs)
					require.Equal(l, namespaced.Inputs, namespaced.Outputs, "expected inputs and outputs to match")

					component := RequireSingleResource(l, snap.Resources, "component:index:Custom")
					require.Equal(l, "component:index:Custom", component.Type.String(), "expected component resource")
				},
			},
		},
	}
}
