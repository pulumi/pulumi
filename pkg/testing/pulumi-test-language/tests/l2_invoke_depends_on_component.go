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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func init() {
	// An output-form invoke that depends on a remote component must not resolve during a preview that still has
	// to create the component's children (pulumi/pulumi#18299). The caller cannot see those children: it declares
	// the dependency on the request and the engine expands and gates it.
	LanguageTests["l2-invoke-depends-on-component"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.ComponentProvider{} },
		},
		Runs: []TestRun{
			{
				AssertPreview: func(l *L, res AssertPreviewArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					var stackEvent engine.ResourceOutputsEventPayload
					foundStackEvent := false
					for _, evt := range res.Events {
						if evt.Type == engine.ResourceOutputsEvent {
							payload := evt.Payload().(engine.ResourceOutputsEventPayload)
							if payload.Metadata.URN.Type() == resource.RootStackType {
								stackEvent = payload
								foundStackEvent = true
							}
						}
					}
					require.True(l, foundStackEvent, "expected stack outputs event")
					assert.True(l, stackEvent.Metadata.New.Outputs["echoed"].IsComputed(),
						"the invoke depends on the component, whose child is pending creation:"+
							" its result must be unknown, got %v", stackEvent.Metadata.New.Outputs)
				},
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					stack := RequireSingleResource(l, res.Snap.Resources, resource.RootStackType)
					AssertPropertyMapMember(l, stack.Outputs, "echoed", resource.NewProperty("reachable"))
				},
			},
		},
	}
}
