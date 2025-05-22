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
	"github.com/stretchr/testify/assert"
)

func init() {
	LanguageTests["l2-resource-invoke-dynamic-function"] = LanguageTest{
		Providers: []plugin.Provider{&providers.AnyTypeFunctionProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					RequireStackResource(l, err, changes)

					r := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					assert.Equal(l, resource.RootStackType, r.Type, "expected a stack resource")
					assert.Equal(l, 1, len(r.Outputs))

					AssertPropertyMapMember(
						l,
						r.Outputs,
						"dynamic",
						resource.NewObjectProperty(
							resource.NewPropertyMapFromMap(
								map[string]any{"resultProperty": resource.NewStringProperty("resultValue")},
							),
						),
					)
				},
			},
		},
	}
}
