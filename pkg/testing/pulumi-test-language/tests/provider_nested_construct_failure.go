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
	"context"
	"errors"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/stretchr/testify/require"
)

const nestedConstructFailure = "the inner component failed on purpose"

type constructFailureProvider struct {
	providers.FailingComponentProvider
}

func (p *constructFailureProvider) Construct(
	ctx context.Context, req plugin.ConstructRequest,
) (plugin.ConstructResponse, error) {
	if req.Info.DryRun {
		return p.FailingComponentProvider.Construct(ctx, req)
	}
	return plugin.ConstructResponse{}, errors.New(nestedConstructFailure)
}

func init() {
	LanguageTests["provider-nested-construct-failure"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.NestedFailureProvider{} },
			func() plugin.Provider { return &constructFailureProvider{} },
		},
		LanguageProviders: []string{"nested-failure"},
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
					var innerErrors []string
					for _, d := range updateDiagnostics(res.Events) {
						if d.Severity == diag.Error &&
							strings.Contains(d.Message, "failingcomponent:index:Component resource 'outer-inner'") {
							innerErrors = append(innerErrors, d.Message)
						}
						assertNoMonitorShutdownError(l, d.Message)
					}
					require.Len(l, innerErrors, 1, "expected one error for the inner component")
					require.Contains(l, innerErrors[0], nestedConstructFailure)
				},
			},
		},
	}
}
