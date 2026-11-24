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
	"os"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-hook-on-error"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.FlakyCreateProvider{} },
		},
		Runs: []TestRun{
			{
				Config: config.Map{
					config.MustMakeKey("l2-resource-hook-on-error", "hookTestFile"): config.NewValue("hook-test.txt"),
				},
				Assert: func(l *L, res AssertArgs) {
					// The resource's first create fails retryably, the error hook requests a
					// retry, and the second create succeeds, so the update as a whole succeeds.
					RequireStackResource(l, res.Err, res.Changes)

					testFile := filepath.Join(res.ProjectDirectory, "hook-test.txt")
					_, err := os.Stat(testFile)
					require.NoError(l, err,
						"expected error hook to have created file at %s", testFile)

					// The engine took the retry path because the hook returned true.
					found := false
					for _, evt := range res.Events {
						if d, ok := evt.Payload().(engine.DiagEventPayload); ok {
							if d.Severity == "warning" && d.URN.Name() == "res" &&
								strings.Contains(d.Message, "retrying create due to on-error hook request") {
								found = true
								break
							}
						}
					}
					require.True(l, found,
						"expected a warning diagnostic for the on-error hook retry")
				},
			},
		},
	}
}
