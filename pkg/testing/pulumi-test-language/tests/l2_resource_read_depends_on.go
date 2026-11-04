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

func init() {
	LanguageTests["l2-resource-read-depends-on"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.ReadProvider{} },
		},
		Runs: []TestRun{
			{
				// During preview, src has not actually been created yet. Per issue
				// https://github.com/pulumi/pulumi/issues/16395 the engine should
				// skip the provider Read when a dependsOn target is being newly
				// created, returning unknown outputs to the SDK so a real-world
				// Read is not performed against a resource that does not yet exist.
				AssertPreview: func(l *L, res AssertPreviewArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					var stackEvent engine.ResourceOutputsEventPayload
					foundStackEvent := false
					for _, evt := range res.Events {
						if evt.Type != engine.ResourceOutputsEvent {
							continue
						}
						payload := evt.Payload().(engine.ResourceOutputsEventPayload)
						if payload.Metadata.URN.Type() == resource.RootStackType {
							stackEvent = payload
							foundStackEvent = true
						}
					}
					require.True(l, foundStackEvent, "expected stack outputs event")

					outputs := stackEvent.Metadata.New.Outputs
					assert.False(l, outputs["resourceUrn"].IsComputed(),
						"expected resourceUrn to be known: %v", outputs["resourceUrn"])
					// The id passed to a read is an import/lookup string, not necessarily
					// the real resource id — the provider's Read is what resolves it. Since
					// Read is skipped here, resourceId must come back unknown.
					assert.True(l, outputs["resourceId"].IsComputed(),
						"expected resourceId to be unknown: %v", outputs["resourceId"])
					assert.True(l, outputs["lookup"].IsComputed(),
						"expected lookup to be unknown: %v", outputs["lookup"])
					assert.True(l, outputs["value"].IsComputed(),
						"expected value to be unknown: %v", outputs["value"])
				},
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					src := RequireSingleNamedResource(l, res.Snap.Resources, "src")
					readRes := RequireSingleNamedResource(l, res.Snap.Resources, "res")
					assert.True(l, readRes.External, "expected read resource to be external")
					assert.Equal(l, []resource.URN{src.URN}, readRes.Dependencies,
						"expected read to depend on src via options.dependsOn")
				},
			},
		},
	}
}
