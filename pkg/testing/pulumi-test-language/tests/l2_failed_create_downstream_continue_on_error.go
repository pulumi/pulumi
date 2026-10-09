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
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func init() {
	LanguageTests["l2-failed-create-downstream-continue-on-error"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.FailOnCreateProvider{} },
		},
		Runs: []TestRun{
			{
				UpdateOptions: engine.UpdateOptions{
					ContinueOnError: true,
				},
				// When a resource create fails, the SDK should surface its outputs as unknown
				// and keep executing the rest of the program rather than halting. The program
				// here reads `failing.value` into a stack output; if the SDK halts on failure
				// the stack output is never registered, if it propagates failure-as-unknown
				// then the stack output is registered as an unknown/computed value.
				AssertPreview: func(l *L, res AssertPreviewArgs) {
					require.True(l, result.IsBail(res.Err), "expected a bail result on preview")
					requireFailedCreateDiagnostic(l, res.Events, "Preview failed: failed create")

					var stackOutputs resource.PropertyMap
					for _, evt := range res.Events {
						if evt.Type != engine.ResourceOutputsEvent {
							continue
						}
						payload := evt.Payload().(engine.ResourceOutputsEventPayload)
						if payload.Metadata.URN.Type() == resource.RootStackType && payload.Metadata.New != nil {
							stackOutputs = payload.Metadata.New.Outputs
						}
					}
					require.NotNil(l, stackOutputs, "expected a stack ResourceOutputsEvent")
					fv, ok := stackOutputs["failingValue"]
					require.True(l, ok,
						"expected stack output 'failingValue' to be registered — SDK must not halt on failed dependency, got %v",
						stackOutputs)
					assert.True(l, fv.IsComputed(),
						"expected stack output 'failingValue' to be unknown, got %v", fv)
				},
				Assert: func(l *L, res AssertArgs) {
					require.True(l, result.IsBail(res.Err), "expected a bail result")
				},
			},
		},
	}
}
