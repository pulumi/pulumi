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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
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
					m    map[string]property.Value
				}{
					{"example1", map[string]property.Value{
						"discriminantKind": property.New("variant1"),
						"payload":          property.New("p1"),
						"extra":            property.New("e1"),
					}},
					{"example2", map[string]property.Value{
						"discriminantKind": property.New("variant2"),
						"payload":          property.New("p2"),
						"extra":            property.New("e2"),
					}},
					{"example3", map[string]property.Value{
						"discriminantKind": property.New("variant3"),
						"payload":          property.New("p3"),
						"count":            property.New(3.0),
					}},
					{"example4", map[string]property.Value{
						"discriminantKind": property.New("variant4"),
						"payload":          property.New("p4"),
						"enabled":          property.New(true),
					}},
					{"example5", map[string]property.Value{
						"discriminantKind": property.New("variant5"),
						"payload":          property.New("p5"),
						"label":            property.New("l5"),
					}},
					{"example6", map[string]property.Value{
						"discriminantKind": property.New("variant6"),
						"payload":          property.New("p6"),
						"code":             property.New(6.0),
					}},
					{"example7", map[string]property.Value{
						"discriminantKind": property.New("variant7"),
						"payload":          property.New("p7"),
						"message":          property.New("m7"),
					}},
					{"example8", map[string]property.Value{
						"discriminantKind": property.New("variant8"),
						"payload":          property.New("p8"),
						"size":             property.New(8.0),
					}},
					{"example9", map[string]property.Value{
						"discriminantKind": property.New("variant9"),
						"payload":          property.New("p9"),
						"flag":             property.New(false),
					}},
					{"example10", map[string]property.Value{
						"discriminantKind": property.New("variant10"),
						"payload":          property.New("p10"),
						"note":             property.New("n10"),
					}},
				}

				for _, e := range expected {
					r := RequireSingleNamedResource(l, snapshot.Resources, e.name)
					require.Equal(l, property.NewMap(map[string]property.Value{
						"unionOf": property.New(e.m),
					}), resource.FromResourcePropertyMap(r.Outputs), "resource %s", e.name)
				}

				// Sub-union assignment: subset1's output (a 3-variant union) is
				// passed through to example11's input (the full 10-variant union).
				subsetPayload := map[string]property.Value{
					"discriminantKind": property.New("variant3"),
					"payload":          property.New("sp"),
					"count":            property.New(33.0),
				}
				subset1 := RequireSingleNamedResource(l, snapshot.Resources, "subset1")
				require.Equal(l, property.NewMap(map[string]property.Value{
					"unionOf": property.New(subsetPayload),
				}), resource.FromResourcePropertyMap(subset1.Outputs))

				example11 := RequireSingleNamedResource(l, snapshot.Resources, "example11")
				require.Equal(l, property.NewMap(map[string]property.Value{
					"unionOf": property.New(subsetPayload),
				}), resource.FromResourcePropertyMap(example11.Outputs))
			},
		}},
	}
}
