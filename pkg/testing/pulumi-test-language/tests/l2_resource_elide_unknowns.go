// Copyright 2025, Pulumi Corporation.
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
	LanguageTests["l2-resource-elide-unknowns"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleProvider{} },
			func() plugin.Provider { return &providers.OutputProvider{} },
		},
		Runs: []TestRun{
			{
				AssertPreview: func(l *L, res AssertPreviewArgs) {
					err := res.Err
					changes := res.Changes
					RequireStackResource(l, err, changes)

					// Should have tried to create the simple resources and the stack outputs.
					simpleComputedInputs := 0
					var stackEvent engine.ResourceOutputsEventPayload
					foundStackEvent := false
					for _, evt := range res.Events {
						if evt.Type == engine.ResourceOutputsEvent {
							payload := evt.Payload().(engine.ResourceOutputsEventPayload)
							if payload.Metadata.URN.Type() == "simple:index:Resource" &&
								payload.Metadata.New.Inputs["value"].IsComputed() {
								simpleComputedInputs++
							} else if payload.Metadata.URN.Type() == resource.RootStackType {
								stackEvent = payload
								foundStackEvent = true
							}
						}
					}

					assert.Equal(l, 4, simpleComputedInputs, "expected all simple resource inputs to be unknown")
					require.True(l, foundStackEvent, "expected stack outputs event")
					assert.True(l, stackEvent.Metadata.New.Outputs["out"].IsComputed(),
						"expected stack output to be unknown: %v", stackEvent.Metadata.New.Outputs)
					assert.True(l, stackEvent.Metadata.New.Outputs["outArray"].IsComputed(),
						"expected stack output to be unknown: %v", stackEvent.Metadata.New.Outputs)
					assert.True(l, stackEvent.Metadata.New.Outputs["outMap"].IsComputed(),
						"expected stack output to be unknown: %v", stackEvent.Metadata.New.Outputs)
					assert.True(l, stackEvent.Metadata.New.Outputs["outObject"].IsComputed(),
						"expected stack output to be unknown: %v", stackEvent.Metadata.New.Outputs)
				},
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					simples := RequireNResources(l, snap.Resources, "simple:index:Resource", 4)
					want := resource.PropertyMap{
						"value": resource.NewProperty(true),
					}
					for _, simple := range simples {
						assert.Equal(l, want, simple.Inputs, "expected resource inputs to match %v", want)
					}

					complex := RequireSingleNamedResource(l, snap.Resources, "complex")
					assert.Equal(l, resource.NewProperty([]resource.PropertyValue{
						resource.NewProperty("hello"),
					}), complex.Outputs["outputArray"])
					assert.Equal(l, resource.NewProperty(resource.PropertyMap{
						"x": resource.NewProperty("hello"),
					}), complex.Outputs["outputMap"])
					assert.Equal(l, resource.NewProperty(resource.PropertyMap{
						"output": resource.NewProperty("hello"),
					}), complex.Outputs["outputObject"])

					stk := RequireSingleResource(l, snap.Resources, resource.RootStackType)
					want = resource.PropertyMap{
						"out":       resource.NewProperty("hello"),
						"outArray":  resource.NewProperty("hello"),
						"outMap":    resource.NewProperty("hello"),
						"outObject": resource.NewProperty("hello"),
					}
					assert.Equal(l, want, stk.Outputs, "expected stack outputs to match %v", want)
				},
			},
		},
	}
}
