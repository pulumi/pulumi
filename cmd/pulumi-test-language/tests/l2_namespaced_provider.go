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
	LanguageTests["l2-namespaced-provider"] = LanguageTest{
		Providers: []plugin.Provider{&providers.NamespacedProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)

					// Check we have the one namespaced resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					provider := snap.Resources[1]
					require.Equal(l, "pulumi:providers:namespaced", provider.Type.String(), "expected namespaced provider")

					namespaced := snap.Resources[2]
					require.Equal(l, "namespaced:index:Resource", namespaced.Type.String(), "expected namespaced resource")

					want := resource.NewPropertyMapFromMap(map[string]interface{}{"value": true})
					require.Equal(l, want, namespaced.Inputs, "expected inputs to be {value: true}")
					require.Equal(l, namespaced.Inputs, namespaced.Outputs, "expected inputs and outputs to match")
				},
			},
		},
	}
}
