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
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l3-component-simple"] = LanguageTest{
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					RequireStackResource(l, err, changes)

					// Check we have the one simple resource in the snapshot, its provider, its parent component and the
					// stack.
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					component := RequireSingleResource(l, snap.Resources, "components:index:MyComponent")

					// TODO(https://github.com/pulumi/pulumi/issues/10533): Languages are inconsistent in whether they
					// send inputs for components.
					// want := resource.NewPropertyMapFromMap(map[string]any{
					// 	"input": true,
					// })
					// assert.Equal(l, want, component.Inputs, "expected component inputs to be %v", want)
					want := resource.NewPropertyMapFromMap(map[string]any{
						"output": true,
					})
					assert.Equal(l, want, component.Outputs, "expected component outputs to be %v", want)

					RequireSingleResource(l, snap.Resources, "pulumi:providers:simple")

					simple := RequireSingleResource(l, snap.Resources, "simple:index:Resource")
					assert.Equal(l, component.URN, simple.Parent, "expected simple resource to have component as parent")

					want = resource.NewPropertyMapFromMap(map[string]any{
						"value": true,
					})
					assert.Equal(l, want, simple.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, simple.Inputs, simple.Outputs, "expected inputs and outputs to match")
				},
			},
		},
	}
}
