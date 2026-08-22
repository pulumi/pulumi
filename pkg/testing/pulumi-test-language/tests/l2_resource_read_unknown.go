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
	LanguageTests["l2-resource-read-unknown"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ReadProvider{} },
		},
		Runs: []TestRun{
			{
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
					assert.True(l, outputs["resourceId"].IsComputed(),
						"expected resourceId to be unknown: %v", outputs["resourceId"])
					assert.True(l, outputs["lookup"].IsComputed(),
						"expected lookup to be unknown: %v", outputs["lookup"])
					assert.True(l, outputs["value"].IsComputed(),
						"expected value to be unknown: %v", outputs["value"])
				},
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					// stack + provider + src (Create) + res (Read)
					require.Len(l, res.Snap.Resources, 4,
						"expected stack + provider + src + read resource")

					reads := 0
					for _, r := range res.Snap.Resources {
						if r.Type != "read:index:Resource" || !r.External {
							continue
						}
						reads++
						assert.Equal(l, resource.ID("created"), r.ID)
						assert.Equal(l, "existing-key", r.Inputs["lookup"].StringValue())
						assert.Equal(l, "existing-key", r.Outputs["lookup"].StringValue())
						assert.Equal(l, true, r.Outputs["value"].BoolValue())
					}
					assert.Equal(l, 1, reads, "expected exactly one external read resource")
				},
			},
		},
	}
}
