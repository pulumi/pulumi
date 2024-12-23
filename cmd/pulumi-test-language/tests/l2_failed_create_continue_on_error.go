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
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-failed-create-continue-on-error"] = LanguageTest{
		Providers: []plugin.Provider{&providers.SimpleProvider{}, &providers.FailOnCreateProvider{}},
		Runs: []TestRun{
			{
				UpdateOptions: engine.UpdateOptions{
					ContinueOnError: true,
				},
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					require.True(l, result.IsBail(err), "expected a bail result")
					require.Equal(l, 1, len(changes), "expected 1 StepOp")
					require.Equal(l, 2, changes[deploy.OpCreate], "expected 2 Creates")
					require.NotNil(l, snap, "expected snapshot to be non-nil")
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot") // 1 stack, 2 providers, 1 resource
					require.NoError(l, snap.VerifyIntegrity(), "expected snapshot to be valid")

					sort.Slice(snap.Resources, func(i, j int) bool {
						return snap.Resources[i].URN.Name() < snap.Resources[j].URN.Name()
					})

					require.Equal(l, "independent", snap.Resources[2].URN.Name(), "expected independent resource")
				},
			},
		},
	}
}
