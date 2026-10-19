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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// l2-resource-read-id-rewrite covers two aspects of ReadResourceResponse.id:
//
//  1. Provider-resolved id: `res` is read with a literal id and the provider's Read
//     returns a rewritten id. The SDK should surface the rewritten id via `.id`.
//  2. Unknown input id: `unknownRes` is read with `src.id`, which is unknown during
//     preview (src is being newly created). The SDK should surface `.id` as unknown.
func init() {
	LanguageTests["l2-resource-read-id-rewrite"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.ReadIdRewriteProvider{} },
		},
		Runs: []TestRun{
			{
				AssertPreview: func(l *L, res AssertPreviewArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					var stackEvent engine.ResourceOutputsEventPayload
					found := false
					for _, evt := range res.Events {
						if evt.Type != engine.ResourceOutputsEvent {
							continue
						}
						payload := evt.Payload().(engine.ResourceOutputsEventPayload)
						if payload.Metadata.URN.Type() == resource.RootStackType {
							stackEvent = payload
							found = true
						}
					}
					require.True(l, found, "expected stack outputs event")
					outputs := stackEvent.Metadata.New.Outputs

					// Known literal id → provider's Read ran and returned a rewritten id.
					assert.False(l, outputs["resourceId"].IsComputed(),
						"expected resourceId to be known: %v", outputs["resourceId"])
					assert.Equal(l, "rewritten-existing-id", outputs["resourceId"].StringValue())
					assert.False(l, outputs["lookup"].IsComputed())
					assert.Equal(l, "existing-key", outputs["lookup"].StringValue())

					// Unknown input id → the engine returns an empty id and the SDK
					// should surface it as unknown rather than echoing the request value.
					assert.True(l, outputs["unknownResourceId"].IsComputed(),
						"expected unknownResourceId to be unknown: %v", outputs["unknownResourceId"])
					assert.True(l, outputs["unknownLookup"].IsComputed(),
						"expected unknownLookup to be unknown: %v", outputs["unknownLookup"])
				},
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					readRes := RequireSingleNamedResource(l, res.Snap.Resources, "res")
					assert.True(l, readRes.External)
					assert.Equal(l, resource.ID("rewritten-existing-id"), readRes.ID,
						"expected provider-rewritten id in snapshot")

					// During update src is created with id "id"; the rewrite provider
					// resolves "rewritten-id" and that should flow into unknownRes.
					unknownRes := RequireSingleNamedResource(l, res.Snap.Resources, "unknownRes")
					assert.True(l, unknownRes.External)
					assert.Equal(l, resource.ID("rewritten-id"), unknownRes.ID,
						"expected provider-rewritten id surfaced after src is created")

					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					AssertPropertyMapMember(l, stack.Outputs, "resourceId",
						resource.NewProperty("rewritten-existing-id"))
					AssertPropertyMapMember(l, stack.Outputs, "lookup",
						resource.NewProperty("existing-key"))
					AssertPropertyMapMember(l, stack.Outputs, "unknownResourceId",
						resource.NewProperty("rewritten-id"))
					AssertPropertyMapMember(l, stack.Outputs, "unknownLookup",
						resource.NewProperty("another-key"))
				},
			},
		},
	}
}
