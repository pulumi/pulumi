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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l2-enum"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.EnumProvider{} },
		},
		Runs: []TestRun{{
			Assert: func(l *L, res AssertArgs) {
				err, snapshot, changes := res.Err, res.Snap, res.Changes
				RequireStackResource(l, err, changes)

				require.Len(l, snapshot.Resources, 6, "4 resources & provider & stack")

				sink1 := RequireSingleNamedResource(l, snapshot.Resources, "sink1")
				sink2 := RequireSingleNamedResource(l, snapshot.Resources, "sink2")
				sink3 := RequireSingleNamedResource(l, snapshot.Resources, "sink3")
				sink4 := RequireSingleNamedResource(l, snapshot.Resources, "sink4")

				expect := property.NewMap(map[string]property.Value{
					"intEnum":    property.New(1.0),
					"stringEnum": property.New("two"),
				})

				assert.Equal(l, expect, resource.FromResourcePropertyMap(sink1.Outputs))
				assert.Equal(l, expect, resource.FromResourcePropertyMap(sink2.Outputs))
				assert.Equal(l, expect, resource.FromResourcePropertyMap(sink3.Outputs))

				assert.Equal(l, property.NewMap(map[string]property.Value{
					"numberEnum": property.New(0.1),
					"wordyEnum":  property.New("It's got apostrophes"),
					"arrayOfEnum": property.New([]property.Value{
						property.New("one"),
						property.New("two"),
					}),
					"mapOfEnum": property.New(map[string]property.Value{
						"small": property.New(1.0),
						"large": property.New(2.0),
					}),
					"holder": property.New(map[string]property.Value{
						"size":  property.New(2.0),
						"color": property.New("one"),
					}),
					"unionEnum": property.New("A Value With Spaces."),
				}), resource.FromResourcePropertyMap(sink4.Outputs))
			},
		}},
	}
}
