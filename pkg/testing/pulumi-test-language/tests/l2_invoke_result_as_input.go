// Copyright 2024, Pulumi Corporation.
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

// A program passes the whole result of a function as an input: to another function, and to a resource. The
// resource must receive the values under their wire names.
func init() {
	LanguageTests["l2-invoke-result-as-input"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.InvokeResultProvider{} },
		},
		Runs: []TestRun{{
			Assert: func(l *L, res AssertArgs) {
				RequireStackResource(l, res.Err, res.Changes)
				RequireSingleResource(l, res.Snap.Resources, "pulumi:providers:invokeresult")

				settings := func(displayName string, maxItems float64) property.Value {
					return property.New(map[string]property.Value{
						"displayName": property.New(displayName),
						"maxItems":    property.New(maxItems),
					})
				}
				first, second := settings("first", 1), settings("second", 2)

				holder := RequireSingleNamedResource(l, res.Snap.Resources, "holder")
				assert.Equal(l, property.NewMap(map[string]property.Value{
					"settings": first,
					"all":      property.New([]property.Value{first, second}),
				}), resource.FromResourcePropertyMap(holder.Inputs))
			},
		}},
	}
}
