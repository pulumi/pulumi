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
	"sort"

	"github.com/pulumi/pulumi/cmd/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-destroy"] = LanguageTest{
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					// check that both expected resources are in the snapshot
					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:simple", provider.Type.String(), "expected simple provider")

					// Make sure we can assert the resource names in a consistent order
					sort.Slice(snap.Resources[2:4], func(i, j int) bool {
						i = i + 2
						j = j + 2
						return snap.Resources[i].URN.Name() < snap.Resources[j].URN.Name()
					})

					simple := snap.Resources[2]
					assert.Equal(l, "simple:index:Resource", simple.Type.String(), "expected simple resource")
					assert.Equal(l, "aresource", simple.URN.Name(), "expected aresource resource")
					simple2 := snap.Resources[3]
					assert.Equal(l, "simple:index:Resource", simple2.Type.String(), "expected simple resource")
					assert.Equal(l, "other", simple2.URN.Name(), "expected other resource")
				},
			},
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					assert.Equal(l, 1, changes[deploy.OpDelete], "expected a delete operation")
					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					// No need to sort here, since we have only resources that depend on each other in a chain.
					provider := snap.Resources[1]
					assert.Equal(l, "pulumi:providers:simple", provider.Type.String(), "expected simple provider")
					// check that only the expected resource is left in the snapshot
					simple := snap.Resources[2]
					assert.Equal(l, "simple:index:Resource", simple.Type.String(), "expected simple resource")
					assert.Equal(l, "aresource", simple.URN.Name(), "expected aresource resource")
				},
			},
		},
	}
}
