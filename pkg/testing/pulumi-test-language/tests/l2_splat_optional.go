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
	LanguageTests["l2-splat-optional"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SplatOptionalProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					require.NoError(l, res.Err)

					sink := RequireSingleNamedResource(l, res.Snap.Resources, "sink")
					assert.Equal(l, property.NewMap(map[string]property.Value{
						"inputs": property.New([]property.Value{
							property.New("computed-a"),
							property.New("computed-b"),
						}),
					}), resource.FromResourcePropertyMap(sink.Inputs))

					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					outputs := resource.FromResourcePropertyMap(stack.Outputs)
					AssertPropertyMapMember(l, outputs, "values", property.New([]property.Value{
						property.New("computed-a"),
						property.New("computed-b"),
					}))
				},
			},
		},
	}
}
