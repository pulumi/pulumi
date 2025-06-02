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
	LanguageTests["l2-resource-primitives"] = LanguageTest{
		Providers: []plugin.Provider{&providers.PrimitiveProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					RequireStackResource(l, err, changes)

					// Check we have the one simple resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:primitive")
					prim := RequireSingleResource(l, snap.Resources, "primitive:index:Resource")

					want := resource.NewPropertyMapFromMap(map[string]any{
						"boolean":     true,
						"float":       3.14,
						"integer":     42,
						"string":      "hello",
						"numberArray": []interface{}{-1.0, 0.0, 1.0},
						"booleanMap":  map[string]interface{}{"t": true, "f": false},
					})
					assert.Equal(l, want, prim.Inputs, "expected inputs to be %v", want)
					assert.Equal(l, prim.Inputs, prim.Outputs, "expected inputs and outputs to match")
				},
			},
		},
	}
}
