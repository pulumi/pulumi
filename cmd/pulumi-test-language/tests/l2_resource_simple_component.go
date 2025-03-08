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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-simple-component"] = LanguageTest{
		Providers: []plugin.Provider{&providers.SimpleComponentProvider{}, &providers.SimpleProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					// Check we have the simpleComponent resource, it's provider, the inner simple resource,
					// it's provider and the stack.
					require.Len(l, snap.Resources, 5, "expected 5 resources in snapshot")

					simpleComponentProvider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:simple-component",
						simpleComponentProvider.Type.String(), "expected simpleComponent provider")

					simpleComponent := snap.Resources[2]
					assert.Equal(l, "simple-component:index:Resource",
						simpleComponent.Type.String(), "expected simpleComponent resource")

					// N.B. We may want to change this as part of
					// https://github.com/pulumi/pulumi/issues/10533
					assert.Empty(l, simpleComponent.Inputs, "expected inputs to be empty")
					want := resource.NewPropertyMapFromMap(map[string]any{"value": true})
					assert.Equal(l, want, simpleComponent.Outputs, "expected outputs to be {value: true}")

					simpleProvider := snap.Resources[3]
					assert.Equal(l, "pulumi:providers:simple", simpleProvider.Type.String(), "expected simple provider")

					simple := snap.Resources[4]
					assert.Equal(l, "simple:index:Resource", simple.Type.String(), "expected simple resource")
					assert.Equal(l, want, simple.Inputs, "expected inputs to be {value: true}")
					assert.Equal(l, simple.Inputs, simple.Outputs, "expected inputs and outputs to match")

					// Should be parent/child relationship between simpleComponent and simple.
					assert.Equal(l, simpleComponent.URN, simple.Parent, "expected simple to be child of simpleComponent")
				},
			},
		},
	}
}
