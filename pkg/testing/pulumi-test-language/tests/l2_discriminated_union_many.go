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
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/require"
)

func init() {
	// Exercises a discriminated union with 10 variants, and with two variants
	// (variant1 and variant2) that share identical property shapes — only the
	// discriminator distinguishes them.
	LanguageTests["l2-discriminated-union-many"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.DiscriminatedUnionManyProvider{} },
		},
		Runs: []TestRun{{
			Assert: func(l *L, res AssertArgs) {
				err, snapshot, changes := res.Err, res.Snap, res.Changes
				RequireStackResource(l, err, changes)

				expected := []struct {
					name string
					m    resource.PropertyMap
				}{
					{"example1", resource.PropertyMap{
						"discriminantKind": resource.NewProperty("variant1"),
						"payload":          resource.NewProperty("p1"),
						"extra":            resource.NewProperty("e1"),
					}},
					{"example2", resource.PropertyMap{
						"discriminantKind": resource.NewProperty("variant2"),
						"payload":          resource.NewProperty("p2"),
						"extra":            resource.NewProperty("e2"),
					}},
					{"example3", resource.PropertyMap{
						"discriminantKind": resource.NewProperty("variant3"),
						"payload":          resource.NewProperty("p3"),
						"count":            resource.NewProperty(3.0),
					}},
					{"example4", resource.PropertyMap{
						"discriminantKind": resource.NewProperty("variant4"),
						"payload":          resource.NewProperty("p4"),
						"enabled":          resource.NewProperty(true),
					}},
					{"example5", resource.PropertyMap{
						"discriminantKind": resource.NewProperty("variant5"),
						"payload":          resource.NewProperty("p5"),
						"label":            resource.NewProperty("l5"),
					}},
					{"example6", resource.PropertyMap{
						"discriminantKind": resource.NewProperty("variant6"),
						"payload":          resource.NewProperty("p6"),
						"code":             resource.NewProperty(6.0),
					}},
					{"example7", resource.PropertyMap{
						"discriminantKind": resource.NewProperty("variant7"),
						"payload":          resource.NewProperty("p7"),
						"message":          resource.NewProperty("m7"),
					}},
					{"example8", resource.PropertyMap{
						"discriminantKind": resource.NewProperty("variant8"),
						"payload":          resource.NewProperty("p8"),
						"size":             resource.NewProperty(8.0),
					}},
					{"example9", resource.PropertyMap{
						"discriminantKind": resource.NewProperty("variant9"),
						"payload":          resource.NewProperty("p9"),
						"flag":             resource.NewProperty(false),
					}},
					{"example10", resource.PropertyMap{
						"discriminantKind": resource.NewProperty("variant10"),
						"payload":          resource.NewProperty("p10"),
						"note":             resource.NewProperty("n10"),
					}},
				}

				for _, e := range expected {
					r := RequireSingleNamedResource(l, snapshot.Resources, e.name)
					require.Equal(l, resource.PropertyMap{
						"unionOf": resource.NewProperty(e.m),
					}, r.Outputs, "resource %s", e.name)
				}

				// Sub-union assignment: subset1's output (a 3-variant union) is
				// passed through to example11's input (the full 10-variant union).
				subsetPayload := resource.PropertyMap{
					"discriminantKind": resource.NewProperty("variant3"),
					"payload":          resource.NewProperty("sp"),
					"count":            resource.NewProperty(33.0),
				}
				subset1 := RequireSingleNamedResource(l, snapshot.Resources, "subset1")
				require.Equal(l, resource.PropertyMap{
					"unionOf": resource.NewProperty(subsetPayload),
				}, subset1.Outputs)

				example11 := RequireSingleNamedResource(l, snapshot.Resources, "example11")
				require.Equal(l, resource.PropertyMap{
					"unionOf": resource.NewProperty(subsetPayload),
				}, example11.Outputs)
			},
		}},
	}
}
