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
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l1-builtin-stash"] = LanguageTest{
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					RequireStackResource(l, err, changes)

					// Check we have the one simple resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := stack.Outputs

					require.Len(l, outputs, 4, "expected 4 outputs")

					getOutput := func(key string) resource.PropertyValue {
						got, ok := outputs[resource.PropertyKey(key)]
						require.True(l, ok, "expected property %s", key)
						return got
					}

					expectedStash := resource.NewObjectProperty(resource.PropertyMap{
						"key": resource.NewArrayProperty([]resource.PropertyValue{
							resource.NewStringProperty("value"),
							resource.NewStringProperty("s"),
						}),
						"": resource.NewBoolProperty(false),
					})

					got := getOutput("stashInput")
					assert.Equal(l, expectedStash, got, "unexpected value for stashOutput")

					got = getOutput("stashOutput")
					assert.Equal(l, expectedStash, got, "unexpected value for stashOutput")

					expectedPassthrough := resource.NewStringProperty("old")

					got = getOutput("passthroughInput")
					assert.Equal(l, expectedPassthrough, got, "unexpected value for passthroughInput")

					got = getOutput("passthroughOutput")
					assert.Equal(l, expectedPassthrough, got, "unexpected value for passthroughOutput")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:pulumi")
					myStash := RequireSingleNamedResource(l, snap.Resources, "myStash")

					want := resource.PropertyMap{
						"input":  expectedStash,
						"output": expectedStash,
					}
					assert.Equal(l, want, myStash.Outputs, "expected myStash outputs to be %v", want)
					want = resource.PropertyMap{
						"input": expectedStash,
					}
					assert.Equal(l, want, myStash.Inputs, "expected myStash inputs to be %v", want)

					passthroughStash := RequireSingleNamedResource(l, snap.Resources, "passthroughStash")

					want = resource.PropertyMap{
						"input":  expectedPassthrough,
						"output": expectedPassthrough,
					}
					assert.Equal(l, want, passthroughStash.Outputs, "expected passthroughStash outputs to be %v", want)
					want = resource.PropertyMap{
						"input":       expectedPassthrough,
						"passthrough": resource.NewBoolProperty(true),
					}
					assert.Equal(l, want, passthroughStash.Inputs, "expected passthroughStash inputs to be %v", want)
				},
			},
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
					events []engine.Event,
				) {
					require.NoError(l, err, "expected no error during update")

					// Check we have the one simple resource in the snapshot, its provider and the stack.
					require.Len(l, snap.Resources, 4, "expected 4 resources in snapshot")

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := stack.Outputs

					require.Len(l, outputs, 4, "expected 4 outputs")

					getOutput := func(key string) resource.PropertyValue {
						got, ok := outputs[resource.PropertyKey(key)]
						require.True(l, ok, "expected property %s", key)
						return got
					}

					got := getOutput("stashInput")
					expectedInput := resource.NewStringProperty("ignored")
					assert.Equal(l, expectedInput, got, "unexpected value for stashInput")

					got = getOutput("stashOutput")
					expectedStash := resource.NewObjectProperty(resource.PropertyMap{
						"key": resource.NewArrayProperty([]resource.PropertyValue{
							resource.NewStringProperty("value"),
							resource.NewStringProperty("s"),
						}),
						"": resource.NewBoolProperty(false),
					})
					assert.Equal(l, expectedStash, got, "unexpected value for stashOutput")

					expectedPassthrough := resource.NewStringProperty("new")
					got = getOutput("passthroughInput")
					assert.Equal(l, expectedPassthrough, got, "unexpected value for passthroughInput")

					got = getOutput("passthroughOutput")
					assert.Equal(l, expectedPassthrough, got, "unexpected value for passthroughOutput")

					RequireSingleResource(l, snap.Resources, "pulumi:providers:pulumi")
					myStash := RequireSingleNamedResource(l, snap.Resources, "myStash")

					want := resource.PropertyMap{
						"input":  expectedInput,
						"output": expectedStash,
					}
					assert.Equal(l, want, myStash.Outputs, "expected myStash outputs to be %v", want)
					want = resource.PropertyMap{
						"input": expectedInput,
					}
					assert.Equal(l, want, myStash.Inputs, "expected myStash inputs to be %v", want)

					passthroughStash := RequireSingleNamedResource(l, snap.Resources, "passthroughStash")

					want = resource.PropertyMap{
						"input":  expectedPassthrough,
						"output": expectedPassthrough,
					}
					assert.Equal(l, want, passthroughStash.Outputs, "expected passthroughStash outputs to be %v", want)
					want = resource.PropertyMap{
						"input":       expectedPassthrough,
						"passthrough": resource.NewBoolProperty(true),
					}
					assert.Equal(l, want, passthroughStash.Inputs, "expected passthroughStash inputs to be %v", want)
				},
			},
		},
	}
}
