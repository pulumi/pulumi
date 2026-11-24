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
	const escapeKey = "Some ${common} \"characters\" 'that' need escaping: " +
		"\\ (backslash), \t (tab), \u001b (escape), \u0007 (bell), \u0000 (null), \U000e0021 (tag space)"

	// Keys that have historically generated invalid code in languages whose code generators use
	// format or glob strings, e.g. https://github.com/pulumi/pulumi/issues/8940.
	const formatKey = "Format and glob specifiers: %percent ...ellipsis {open }close *asterisk " +
		"?question ,comma &&and ||or !not =>arrow ==equal :colon /slash"

	LanguageTests["l2-map-keys-adversarial"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.PrimitiveProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)

					require.Len(l, snap.Resources, 3, "expected 3 resources in snapshot")

					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")
					RequireSingleResource(l, snap.Resources, "pulumi:providers:primitive")
					primResource := RequireSingleResource(l, snap.Resources, "primitive:index:Resource")

					adversarialMap := map[string]property.Value{
						"__type":     property.New(true),
						"__internal": property.New(false),
						"__provider": property.New(true),
						"__version":  property.New(false),
						"":           property.New(true),
						escapeKey:    property.New(false),
						formatKey:    property.New(true),
					}

					want := property.NewMap(map[string]property.Value{
						"boolean": property.New(false),
						"float":   property.New(2.17),
						"integer": property.New(-12.0),
						"string":  property.New("adversarial"),
						"numberArray": property.New([]property.Value{
							property.New(0.0), property.New(1.0),
						}),
						"booleanMap": property.New(adversarialMap),
					})

					primInputs := resource.FromResourcePropertyMap(primResource.Inputs)
					assert.Equal(l, want, primInputs, "resource inputs should preserve adversarial map keys")
					assert.Equal(l, primInputs,
						resource.FromResourcePropertyMap(primResource.Outputs),
						"expected inputs and outputs to match")

					stackOutputs := resource.FromResourcePropertyMap(stack.Outputs)
					AssertPropertyMapMember(l, stackOutputs,
						"resourceBooleanMap", property.New(adversarialMap))
					AssertPropertyMapMember(l, stackOutputs,
						"invokeBooleanMap", property.New(adversarialMap))
				},
			},
		},
	}
}
