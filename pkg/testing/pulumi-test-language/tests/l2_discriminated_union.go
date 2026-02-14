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
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-discriminated-union"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.DiscriminatedUnionProvider{} },
		},
		Runs: []TestRun{{
			Assert: func(l *L, res AssertArgs) {
				err, snapshot, changes := res.Err, res.Snap, res.Changes
				RequireStackResource(l, err, changes)

				catExample := RequireSingleNamedResource(l, snapshot.Resources, "catExample")
				require.Equal(l, resource.PropertyMap{
					"pet": resource.NewProperty(resource.PropertyMap{
						"petType": resource.NewProperty("cat"),
						"meow":    resource.NewProperty("meow"),
					}),
					"pets": resource.NewProperty([]resource.PropertyValue{
						resource.NewProperty(resource.PropertyMap{
							"petType": resource.NewProperty("cat"),
							"meow":    resource.NewProperty("purr"),
						}),
					}),
				}, catExample.Outputs)

				dogExample := RequireSingleNamedResource(l, snapshot.Resources, "dogExample")
				require.Equal(l, resource.PropertyMap{
					"pet": resource.NewProperty(resource.PropertyMap{
						"petType": resource.NewProperty("dog"),
						"bark":    resource.NewProperty("woof"),
					}),
					"pets": resource.NewProperty([]resource.PropertyValue{
						resource.NewProperty(resource.PropertyMap{
							"petType": resource.NewProperty("dog"),
							"bark":    resource.NewProperty("bark"),
						}),
						resource.NewProperty(resource.PropertyMap{
							"petType": resource.NewProperty("cat"),
							"meow":    resource.NewProperty("hiss"),
						}),
					}),
				}, dogExample.Outputs)
			},
		}},
	}
}
