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
)

// l2-map-keys-invoke-call verifies that a map<string> argument survives a round-trip
// through both invokes and remote method calls, including keys with characters that
// SDKs sometimes treat specially (e.g. "__"-prefixed keys, which some SDKs have been
// known to mistake for reserved metadata sigils).
func init() {
	expected := property.New(map[string]property.Value{
		"my key":     property.New("one"),
		"my.key":     property.New("two"),
		"my-key":     property.New("three"),
		"my_key":     property.New("four"),
		"MY_KEY":     property.New("five"),
		"myKey":      property.New("six"),
		"__type":     property.New("seven"),
		"__internal": property.New("eight"),
	})

	LanguageTests["l2-map-keys-invoke-call"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.SimpleInvokeProvider{} },
			func() plugin.Provider { return &providers.ComponentProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					assert.Equal(l, property.NewMap(map[string]property.Value{
						"invokeResult": expected,
						"callResult":   expected,
					}), resource.FromResourcePropertyMap(stack.Outputs))
				},
			},
		},
	}
}
