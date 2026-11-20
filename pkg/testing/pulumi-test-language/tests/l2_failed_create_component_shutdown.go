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
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-failed-create-component-shutdown"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.FailingComponentProvider{WaitForCancellation: true} },
		},
		AssertRuntimeErrors: func(l *L, messages []string, stderr string) {
			for _, message := range messages {
				assertNoMonitorShutdownError(l, message)
			}
			assertNoMonitorShutdownError(l, stderr)
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					require.True(l, result.IsBail(res.Err), "expected the update to fail")
					var childErrors []string
					for _, d := range updateDiagnostics(res.Events) {
						if d.URN != "" && d.URN.Name() == "component-child" &&
							d.URN.Type() == "failingcomponent:index:Custom" && d.Severity == diag.Error {
							childErrors = append(childErrors, d.Message)
						}
						assertNoMonitorShutdownError(l, d.Message)
					}
					require.Len(l, childErrors, 1, "expected one error for the failed child")
					require.Contains(l, childErrors[0], "failed create")
				},
			},
		},
	}
}

func updateDiagnostics(events []engine.Event) []engine.DiagEventPayload {
	var diagnostics []engine.DiagEventPayload
	for _, event := range events {
		d, ok := event.Payload().(engine.DiagEventPayload)
		if ok && !d.Ephemeral && (d.Severity == diag.Error || d.Severity == diag.Infoerr) {
			diagnostics = append(diagnostics, d)
		}
	}
	return diagnostics
}

func assertNoMonitorShutdownError(l *L, message string) {
	l.Helper()
	for _, noise := range []string{"resource monitor shut down", "resource monitor has terminated", "exited prematurely"} {
		assert.NotContains(l, strings.ToLower(message), noise, "secondary error: %s", message)
	}
}
