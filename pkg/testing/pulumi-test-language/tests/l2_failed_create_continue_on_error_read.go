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

func init() {
	LanguageTests["l2-failed-create-continue-on-error-read"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ReadProvider{} },
			func() plugin.Provider { return &providers.FailOnCreateProvider{} },
		},
		Runs: []TestRun{
			{
				UpdateOptions: engine.UpdateOptions{
					ContinueOnError: true,
				},
				AssertPreview: func(l *L, res AssertPreviewArgs) {
					require.True(l, result.IsBail(res.Err), "expected a bail result on preview")
				},
				Assert: func(l *L, res AssertArgs) {
					require.True(l, result.IsBail(res.Err), "expected a bail result")

					// The failing create should surface as an error diagnostic.
					foundErr := false
					for _, evt := range res.Events {
						if d, ok := evt.Payload().(engine.DiagEventPayload); ok {
							if d.Severity == "error" && d.URN.Name() == "failing" {
								foundErr = true
								break
							}
						}
					}
					require.True(l, foundErr, "expected error diagnostic for failing resource")

					// The read step should have been skipped due to its dependency on the failing
					// resource, so it should not appear in the snapshot - this is the observable
					// consequence of the read returning Unknown outputs.
					require.NotNil(l, res.Snap, "expected snapshot to be non-nil")
					for _, r := range res.Snap.Resources {
						assert.NotEqual(l, "read:index:Resource", string(r.Type),
							"expected skipped read to be absent from snapshot, found %s", r.URN)
					}

					// The stack output derived from the skipped read's value must not have
					// resolved to a known value, and so shouldn't have been saved in the snapshot.
					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					require.NotContains(l, stack.Outputs, resource.PropertyKey("readValue"))
					require.NotContains(l, stack.Outputs, resource.PropertyKey("readPropDepValue"))
				},
			},
		},
	}
}
