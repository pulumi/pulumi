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

// l3-map-keys is the local-component analogue of l2-map-keys-adversarial: it verifies
// that the same set of adversarial map keys (including keys starting with "__", the
// empty string, and strings full of characters that codegen might mishandle) survive
// being passed through a local component boundary.
func init() {
	const escapeKey = "Some ${common} \"characters\" 'that' need escaping: " +
		"\\ (backslash), \t (tab), \u001b (escape), \u0007 (bell), \u0000 (null), \U000e0021 (tag space)"
	const formatKey = "Format and glob specifiers: %percent ...ellipsis {open }close *asterisk " +
		"?question ,comma &&and ||or !not =>arrow ==equal :colon /slash"

	adversarialMap := map[string]property.Value{
		"my key":     property.New(false),
		"my.key":     property.New(true),
		"my-key":     property.New(false),
		"my_key":     property.New(true),
		"MY_KEY":     property.New(false),
		"myKey":      property.New(true),
		"__type":     property.New(true),
		"__internal": property.New(false),
		"__provider": property.New(true),
		"__version":  property.New(false),
		"":           property.New(true),
		escapeKey:    property.New(false),
		formatKey:    property.New(true),
	}
	adversarialProp := property.New(adversarialMap)

	LanguageTests["l3-map-keys"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.PrimitiveProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)

					// Stack, primitive provider, local component, and its child resource.
					require.Len(l, res.Snap.Resources, 4, "expected 4 resources")

					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					AssertPropertyMapMember(l, resource.FromResourcePropertyMap(stack.Outputs),
						"resourceBooleanMap", adversarialProp)

					child := RequireSingleNamedResource(l, res.Snap.Resources, "cmp-res")
					assert.Equal(l, "primitive:index:Resource", child.Type.String())
					want := property.NewMap(map[string]property.Value{
						"boolean":     property.New(false),
						"float":       property.New(2.17),
						"integer":     property.New(-12.0),
						"string":      property.New("adversarial"),
						"numberArray": property.New([]property.Value{property.New(0.0), property.New(1.0)}),
						"booleanMap":  adversarialProp,
					})
					assert.Equal(l, want, resource.FromResourcePropertyMap(child.Inputs))
					assert.Equal(l, child.Inputs, child.Outputs)
				},
			},
		},
	}
}
