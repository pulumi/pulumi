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
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-hook-after-failure"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					require.True(l, result.IsBail(res.Err), "expected a bail result when after hook fails")

					found := false
					for _, evt := range res.Events {
						if d, ok := evt.Payload().(engine.DiagEventPayload); ok {
							if d.Severity == "error" && d.URN.Name() == "res" &&
								strings.Contains(d.Message, "failingHook") {
								found = true
								break
							}
						}
					}
					require.True(l, found, "expected an error diagnostic for the failing afterCreate hook")
				},
			},
		},
	}
}
