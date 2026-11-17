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
	LanguageTests["l2-discriminated-union"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.DiscriminatedUnionProvider{} },
		},
		Runs: []TestRun{{
			Assert: func(l *L, res AssertArgs) {
				err, snapshot, changes := res.Err, res.Snap, res.Changes
				RequireStackResource(l, err, changes)

				example1 := RequireSingleNamedResource(l, snapshot.Resources, "example1")
				require.Equal(l, property.NewMap(map[string]property.Value{
					"unionOf": property.New(map[string]property.Value{
						"discriminantKind": property.New("variant1"),
						"field1":           property.New("v1 union"),
					}),
					"arrayOfUnionOf": property.New([]property.Value{
						property.New(map[string]property.Value{
							"discriminantKind": property.New("variant1"),
							"field1":           property.New("v1 array(union)"),
						}),
					}),
				}), resource.FromResourcePropertyMap(example1.Outputs))

				example2 := RequireSingleNamedResource(l, snapshot.Resources, "example2")
				require.Equal(l, property.NewMap(map[string]property.Value{
					"unionOf": property.New(map[string]property.Value{
						"discriminantKind": property.New("variant2"),
						"field2":           property.New("v2 union"),
					}),
					"arrayOfUnionOf": property.New([]property.Value{
						property.New(map[string]property.Value{
							"discriminantKind": property.New("variant2"),
							"field2":           property.New("v2 array(union)"),
						}),
						property.New(map[string]property.Value{
							"discriminantKind": property.New("variant1"),
							"field1":           property.New("v1 array(union)"),
						}),
					}),
				}), resource.FromResourcePropertyMap(example2.Outputs))
			},
		}},
	}
}
