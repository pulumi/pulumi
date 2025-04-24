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
	LanguageTests["l2-explicit-parameterized-provider"] = LanguageTest{
		Providers: []plugin.Provider{&providers.ParameterizedProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					// Check we have the one resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					require.Equal(l,
						resource.NewStringProperty("Goodbye World"),
						stack.Outputs["parameterValue"],
						"parameter value and provider config should be correct")

					provider := RequireSingleResource(l, snap.Resources, "pulumi:providers:goodbye")
					assert.Equal(l, "prov", provider.URN.Name(), "expected explicit provider resource")

					simple := RequireSingleResource(l, snap.Resources, "goodbye:index:Goodbye")
					assert.Equal(l, string(provider.URN)+"::"+string(provider.ID), simple.Provider)
				},
			},
		},
	}
}
