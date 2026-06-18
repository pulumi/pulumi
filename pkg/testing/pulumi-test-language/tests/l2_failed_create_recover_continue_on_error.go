// Copyright 2026, Pulumi Corporation.
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
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-failed-create-recover-continue-on-error"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.FailOnCreateProvider{} },
		},
		Runs: []TestRun{
			{
				UpdateOptions: engine.UpdateOptions{
					ContinueOnError: true,
				},
				AssertPreview: func(l *L, res AssertPreviewArgs) {
					require.True(l, result.IsBail(res.Err), "expected a bail result on preview")
					requireFailedCreateDiagnostic(l, res.Events, "Preview failed: failed create")
				},
				Assert: func(l *L, res AssertArgs) {
					require.True(l, result.IsBail(res.Err), "expected a bail result")
					requireFailedCreateDiagnostic(l, res.Events, "failed create")

					require.Len(l, res.Changes, 1, "expected 1 StepOp")
					require.Equal(l, 3, res.Changes[deploy.OpCreate], "expected 3 Creates")
					require.NotNil(l, res.Snap, "expected snapshot to be non-nil")
					require.Len(l, res.Snap.Resources, 5, "expected 5 resources in snapshot")
					require.NoError(l, res.Snap.VerifyIntegrity(), "expected snapshot to be valid")

					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					recovered := stack.Outputs["recovered"]
					require.True(l, recovered.IsString(), "expected recovered stack output to be a string")
					require.True(l, strings.HasPrefix(recovered.StringValue(), "recovered: "), "expected recovered prefix")
					require.NotEqual(l, "recovered: ", recovered.StringValue(), "expected recovered error message")

					recoveredValue := RequireSingleNamedResource(l, res.Snap.Resources, "recovered_value")
					AssertPropertyMapMember(l, recoveredValue.Outputs, "value",
						resource.NewProperty(true))
				},
			},
		},
	}
}

func requireFailedCreateDiagnostic(l *L, events []engine.Event, message string) {
	for _, evt := range events {
		if d, ok := evt.Payload().(engine.DiagEventPayload); ok {
			if d.Severity == "error" && d.URN.Name() == "failing" {
				require.Contains(l, d.Message, message)
				return
			}
		}
	}
	require.Fail(l, "expected to find error diagnostic for failing resource")
}
