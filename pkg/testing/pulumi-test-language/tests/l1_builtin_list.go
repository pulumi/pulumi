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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	commonConfig := config.Map{
		config.MustMakeKey("l1-builtin-list", "aList"):   config.NewObjectValue(`["foo","bar","baz"]`),
		config.MustMakeKey("l1-builtin-list", "aString"): config.NewObjectValue(`"foo-bar-baz"`),
	}

	commonOutputs := resource.PropertyMap{
		"elementOutput1": resource.NewProperty("bar"),
		"elementOutput2": resource.NewProperty("baz"),
		"joinOutput":     resource.NewProperty("foo|bar|baz"),
		"lengthOutput":   resource.NewProperty(3.0),
		"splitOutput": resource.NewProperty([]resource.PropertyValue{
			resource.NewProperty("foo"),
			resource.NewProperty("bar"),
			resource.NewProperty("baz"),
		}),
	}

	LanguageTests["l1-builtin-list"] = LanguageTest{
		RunsShareSource: true,
		Runs: []TestRun{
			{
				// Run 1: singleOrNone with a single-element list returns that element.
				Config: func() config.Map {
					cfg := config.Map{}
					for k, v := range commonConfig {
						cfg[k] = v
					}
					cfg[config.MustMakeKey("l1-builtin-list", "singleOrNoneList")] = config.NewObjectValue(`["single"]`)
					return cfg
				}(),
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")

					want := resource.PropertyMap{}
					for k, v := range commonOutputs {
						want[k] = v
					}
					want["singleOrNoneOutput"] = resource.NewProperty([]resource.PropertyValue{
						resource.NewProperty("single"),
					})

					assert.Equal(l, want, stack.Outputs)
				},
			},
			{
				// Run 2: singleOrNone with a multi-element list errors at runtime.
				Config: func() config.Map {
					cfg := config.Map{}
					for k, v := range commonConfig {
						cfg[k] = v
					}
					cfg[config.MustMakeKey("l1-builtin-list", "singleOrNoneList")] = config.NewObjectValue(`["a","b"]`)
					return cfg
				}(),
				Assert: func(l *L, res AssertArgs) {
					require.Error(l, res.Err)
				},
			},
			{
				// Run 3: singleOrNone with an empty list returns null.
				Config: func() config.Map {
					cfg := config.Map{}
					for k, v := range commonConfig {
						cfg[k] = v
					}
					cfg[config.MustMakeKey("l1-builtin-list", "singleOrNoneList")] = config.NewObjectValue(`[]`)
					return cfg
				}(),
				Assert: func(l *L, res AssertArgs) {
					require.NoError(l, res.Err)
					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")

					want := resource.PropertyMap{}
					for k, v := range commonOutputs {
						want[k] = v
					}
					want["singleOrNoneOutput"] = resource.NewProperty([]resource.PropertyValue{
						resource.NewNullProperty(),
					})

					assert.Equal(l, want, stack.Outputs)
				},
			},
		},
	}
}
