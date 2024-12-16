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
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-target-up-with-new-dependency"] = LanguageTest{
		Providers: []plugin.Provider{&providers.SimpleProvider{}},
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")
					err = snap.VerifyIntegrity()
					require.NoError(l, err, "expected snapshot to be valid")

					sort.Slice(snap.Resources, func(i, j int) bool {
						return snap.Resources[i].URN.Name() < snap.Resources[j].URN.Name()
					})

					target := snap.Resources[2]
					require.Equal(l, "simple:index:Resource", target.Type.String(), "expected simple resource")
					require.Equal(l, "targetOnly", target.URN.Name(), "expected target resource")
					unrelated := snap.Resources[3]
					require.Equal(l, "simple:index:Resource", unrelated.Type.String(), "expected simple resource")
					require.Equal(l, "unrelated", unrelated.URN.Name(), "expected target resource")
					require.Equal(l, 0, len(unrelated.Dependencies), "expected no dependencies")
				},
			},
			{
				UpdateOptions: engine.UpdateOptions{
					Targets: deploy.NewUrnTargets([]string{
						"**targetOnly**",
					}),
				},
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					sort.Slice(snap.Resources, func(i, j int) bool {
						return snap.Resources[i].URN.Name() < snap.Resources[j].URN.Name()
					})

					target := snap.Resources[2]
					require.Equal(l, "simple:index:Resource", target.Type.String(), "expected simple resource")
					require.Equal(l, "targetOnly", target.URN.Name(), "expected target resource")
					unrelated := snap.Resources[3]
					require.Equal(l, "simple:index:Resource", unrelated.Type.String(), "expected simple resource")
					require.Equal(l, "unrelated", unrelated.URN.Name(), "expected target resource")
					require.Equal(l, 0, len(unrelated.Dependencies), "expected still no dependencies")
				},
			},
		},
	}
}
