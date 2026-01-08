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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l1-builtin-stash"] = LanguageTest{
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					projectDirectory, err, snap, changes, events, sdks := res.ProjectDirectory, res.Err, res.Snap, res.Changes, res.Events, res.SDKs
					_, _, _, _, _, _ = projectDirectory, err, snap, changes, events, sdks
					RequireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := stack.Outputs

					require.Len(l, outputs, 2, "expected 2 outputs")

					getOutput := func(key string) resource.PropertyValue {
						got, ok := outputs[resource.PropertyKey(key)]
						require.True(l, ok, "expected property %s", key)
						return got
					}

					expectedStash := resource.NewProperty(resource.PropertyMap{
						"key": resource.NewProperty([]resource.PropertyValue{
							resource.NewProperty("value"),
							resource.NewProperty("s"),
						}),
						"": resource.NewProperty(false),
					})

					got := getOutput("stashInput")
					assert.Equal(l, expectedStash, got, "unexpected value for stashOutput")

					got = getOutput("stashOutput")
					assert.Equal(l, expectedStash, got, "unexpected value for stashOutput")

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
				},
			},
			{
				Assert: func(l *L, res AssertArgs) {
					projectDirectory, err, snap, changes, events, sdks := res.ProjectDirectory, res.Err, res.Snap, res.Changes, res.Events, res.SDKs
					_, _, _, _, _, _ = projectDirectory, err, snap, changes, events, sdks
					require.NoError(l, err, "expected no error during update")

					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := stack.Outputs

					require.Len(l, outputs, 2, "expected 2 outputs")

					getOutput := func(key string) resource.PropertyValue {
						got, ok := outputs[resource.PropertyKey(key)]
						require.True(l, ok, "expected property %s", key)
						return got
					}

					got := getOutput("stashInput")
					expectedInput := resource.NewProperty("ignored")
					assert.Equal(l, expectedInput, got, "unexpected value for stashInput")

					got = getOutput("stashOutput")
					expectedStash := resource.NewProperty(resource.PropertyMap{
						"key": resource.NewProperty([]resource.PropertyValue{
							resource.NewProperty("value"),
							resource.NewProperty("s"),
						}),
						"": resource.NewProperty(false),
					})
					assert.Equal(l, expectedStash, got, "unexpected value for stashOutput")

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
				},
			},
		},
	}
}
