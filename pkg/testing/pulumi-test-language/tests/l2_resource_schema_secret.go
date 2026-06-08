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
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-resource-schema-secret"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.OutputProvider{} },
		},
		Runs: []TestRun{
			{
				AssertPreview: func(l *L, res AssertPreviewArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					var stackOutputs resource.PropertyMap
					for _, evt := range res.Events {
						if evt.Type == engine.ResourceOutputsEvent {
							payload := evt.Payload().(engine.ResourceOutputsEventPayload)
							if payload.Metadata.URN.Type() == resource.RootStackType {
								stackOutputs = payload.Metadata.New.Outputs
							}
						}
					}
					require.NotNil(l, stackOutputs, "expected stack outputs event")

					for _, name := range []resource.PropertyKey{"topLevel", "nested"} {
						value, ok := stackOutputs[name]
						require.True(l, ok, "expected stack output %q", name)
						assert.True(l, value.ContainsSecrets(), "expected stack output %q to be secret: %v", name, value)
						assert.True(l, value.ContainsUnknowns(), "expected stack output %q to be unknown: %v", name, value)
					}
				},
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					topLevel := RequireSingleNamedResource(l, res.Snap.Resources, "topLevel")
					assert.True(l, topLevel.Outputs["secretOutput"].ContainsSecrets(),
						"expected top-level schema-secret output to be secret")

					nested := RequireSingleNamedResource(l, res.Snap.Resources, "nested")
					assert.True(l, nested.Outputs["outputObject"].ContainsSecrets(),
						"expected output object containing a nested schema-secret property to be secret")

					stack := RequireSingleResource(l, res.Snap.Resources, resource.RootStackType)
					assert.True(l, stack.Outputs["topLevel"].ContainsSecrets(),
						"expected top-level stack output to be secret")
					assert.True(l, stack.Outputs["nested"].ContainsSecrets(),
						"expected nested stack output to be secret")
				},
			},
		},
	}
}
