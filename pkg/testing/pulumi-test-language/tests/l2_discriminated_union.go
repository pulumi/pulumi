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

				example1 := RequireSingleNamedResource(l, snapshot.Resources, "example1")
				require.Equal(l, resource.PropertyMap{
					"unionOf": resource.NewProperty(resource.PropertyMap{
						"discriminantKind": resource.NewProperty("variant1"),
						"field1":           resource.NewProperty("v1 union"),
					}),
					"arrayOfUnionOf": resource.NewProperty([]resource.PropertyValue{
						resource.NewProperty(resource.PropertyMap{
							"discriminantKind": resource.NewProperty("variant1"),
							"field1":           resource.NewProperty("v1 array(union)"),
						}),
					}),
				}, example1.Outputs)

				example2 := RequireSingleNamedResource(l, snapshot.Resources, "example2")
				require.Equal(l, resource.PropertyMap{
					"unionOf": resource.NewProperty(resource.PropertyMap{
						"discriminantKind": resource.NewProperty("variant2"),
						"field2":           resource.NewProperty("v2 union"),
					}),
					"arrayOfUnionOf": resource.NewProperty([]resource.PropertyValue{
						resource.NewProperty(resource.PropertyMap{
							"discriminantKind": resource.NewProperty("variant2"),
							"field2":           resource.NewProperty("v2 array(union)"),
						}),
						resource.NewProperty(resource.PropertyMap{
							"discriminantKind": resource.NewProperty("variant1"),
							"field1":           resource.NewProperty("v1 array(union)"),
						}),
					}),
				}, example2.Outputs)
			},
		}},
	}
}
