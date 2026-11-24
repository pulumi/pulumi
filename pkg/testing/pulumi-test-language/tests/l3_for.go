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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l3-for"] = LanguageTest{
		Runs: []TestRun{
			{
				Config: config.Map{
					config.MustMakeKey("l3-for", "names"): config.NewObjectValue(`["a","b","c"]`),
					config.MustMakeKey("l3-for", "tags"):  config.NewObjectValue(`{"app":"web","env":"prod"}`),
				},
				Assert: func(l *L, res AssertArgs) {
					require.NoError(l, res.Err)
					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					assert.Equal(l, property.NewMap(map[string]property.Value{
						"greetings": property.New([]property.Value{
							property.New("Hello, a!"),
							property.New("Hello, b!"),
							property.New("Hello, c!"),
						}),
						"numbered": property.New([]property.Value{
							property.New("0-a"),
							property.New("1-b"),
							property.New("2-c"),
						}),
						"tagList": property.New([]property.Value{
							property.New("app=web"),
							property.New("env=prod"),
						}),
						"greetingMap": property.New(map[string]property.Value{
							"a": property.New("Hello, a!"),
							"b": property.New("Hello, b!"),
							"c": property.New("Hello, c!"),
						}),
						"filteredList": property.New([]property.Value{
							property.New("a"),
							property.New("c"),
						}),
						"filteredMap": property.New(map[string]property.Value{
							"a": property.New("Hello, a!"),
							"c": property.New("Hello, c!"),
						}),
					}), resource.FromResourcePropertyMap(stack.Outputs))
				},
			},
		},
	}
}
