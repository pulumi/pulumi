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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-camel-names"] = LanguageTest{
		Providers: []plugin.Provider{&providers.CamelNamesProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					RequireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:camelNames")
					first := RequireSingleNamedResource(l, snap.Resources, "firstResource")
					second := RequireSingleNamedResource(l, snap.Resources, "secondResource")

					wantInputs := resource.NewPropertyMapFromMap(map[string]any{
						"theInput": resource.NewBoolProperty(true),
					})
					assert.Equal(l, wantInputs, first.Inputs, "expected inputs to be %v", wantInputs)
					assert.Equal(l, wantInputs, second.Inputs, "expected inputs to be %v", wantInputs)

					wantOutputs := resource.NewPropertyMapFromMap(map[string]any{
						"theOutput": resource.NewBoolProperty(true),
					})
					assert.Equal(l, wantOutputs, first.Outputs, "expected outputs to be %v", wantOutputs)
					assert.Equal(l, wantOutputs, second.Outputs, "expected outputs to be %v", wantOutputs)
				},
			},
		},
	}
}
