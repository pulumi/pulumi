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
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-conditional"] = LanguageTest{
		Providers: []plugin.Provider{&providers.SimpleProvider{UnknownOutputs: true}},
		Runs: []TestRun{
			{
				AssertPreview: func(l *L, _ string, err error, plan *deploy.Plan, changes display.ResourceChanges) {
					// Check resA, resB and resC are all in the plan.
				},
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					// Check we have the one simple resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:simple")
					resA := RequireSingleNamedResource(l, snap.Resources, "resA")
					assert.Equal(l, tokens.Type("simple:index:Resource"), resA.Type, "expected resource to be simple:index:Resource")

					want := resource.NewPropertyMapFromMap(map[string]any{"value": true})
					assert.Equal(l, want, resA.Inputs, "expected inputs to be {value: true}")
					assert.Equal(l, resA.Inputs, resA.Outputs, "expected inputs and outputs to match")

					resB := RequireSingleNamedResource(l, snap.Resources, "resB")
					assert.Equal(l, tokens.Type("simple:index:Resource"), resB.Type, "expected resource to be simple:index:Resource")

					want = resource.NewPropertyMapFromMap(map[string]any{"value": false})
					assert.Equal(l, want, resB.Inputs, "expected inputs to be {value: false}")
					assert.Equal(l, resB.Inputs, resB.Outputs, "expected inputs and outputs to match")

					// There should be no resC
				},
			},
		},
	}
}
