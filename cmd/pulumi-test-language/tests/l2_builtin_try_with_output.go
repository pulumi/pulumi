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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-builtin-try-with-output"] = LanguageTest{
		Providers: []plugin.Provider{&providers.ComponentProvider{}, &providers.SimpleInvokeProvider{}},
		Runs: []TestRun{
			{
				Config: config.Map{
					config.MustMakeKey("l2-builtin-try-with-output", "object"): config.NewObjectValue("{}"),
				},
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					require.NotEmpty(l, snap.Resources, "expected at least 1 resource")
					stack := snap.Resources[0]
					require.Equal(l, resource.RootStackType, stack.Type, "expected a stack resource")

					outputs := stack.Outputs

					_ = RequireSingleNamedResource(l, snap.Resources, "component1")

					assert.Len(l, outputs, 2, "expected 2 outputs")
					AssertPropertyMapMember(
						l,
						outputs,
						"tryWithOutput",
						resource.NewPropertyValue(resource.NewPropertyMapFromMap(map[string]interface{}{
							"id":    "id-foo-bar-baz",
							"urn":   "urn:pulumi:test::l2-builtin-try-with-output::component:index:ComponentCustomRefOutput$component:index:Custom::component1-child",
							"value": "foo-bar-baz",
						})))
					AssertPropertyMapMember(
						l,
						outputs,
						"tryScalar",
						resource.NewStringProperty("str"),
					)
				},
			},
		},
	}
}
