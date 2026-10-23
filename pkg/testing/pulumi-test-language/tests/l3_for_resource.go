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
	LanguageTests["l3-for-resource"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.NestedObjectProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					require.Len(l, res.Snap.Resources, 6)

					RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:nestedobject")

					source := RequireSingleNamedResource(l, res.Snap.Resources, "source")
					assert.Equal(l, property.NewMap(map[string]property.Value{
						"inputs": property.New([]property.Value{
							property.New("a"),
							property.New("b"),
							property.New("c"),
						}),
					}), resource.FromResourcePropertyMap(source.Inputs))

					receiver := RequireSingleNamedResource(l, res.Snap.Resources, "receiver")
					assert.Equal(l, property.NewMap(map[string]property.Value{
						"details": property.New([]property.Value{
							property.New(map[string]property.Value{
								"key":   property.New("a"),
								"value": property.New("computed-a"),
							}),
							property.New(map[string]property.Value{
								"key":   property.New("b"),
								"value": property.New("computed-b"),
							}),
							property.New(map[string]property.Value{
								"key":   property.New("c"),
								"value": property.New("computed-c"),
							}),
						}),
					}), resource.FromResourcePropertyMap(receiver.Inputs))

					fromSimple := RequireSingleNamedResource(l, res.Snap.Resources, "fromSimple")
					assert.Equal(l, property.NewMap(map[string]property.Value{
						"inputs": property.New([]property.Value{
							property.New("computed-a"),
							property.New("computed-b"),
							property.New("computed-c"),
						}),
					}), resource.FromResourcePropertyMap(fromSimple.Inputs))

					mapped := RequireSingleNamedResource(l, res.Snap.Resources, "mapped")
					assert.Equal(l, property.NewMap(map[string]property.Value{
						"tags": property.New(map[string]property.Value{
							"a": property.New("computed-a"),
							"b": property.New("computed-b"),
							"c": property.New("computed-c"),
						}),
					}), resource.FromResourcePropertyMap(mapped.Inputs))
				},
			},
		},
	}
}
