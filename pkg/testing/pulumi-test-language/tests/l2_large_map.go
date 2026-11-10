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

const largeMapDepth = 300

func init() {
	LanguageTests["l2-large-map"] = LanguageTest{
		Providers: []func() plugin.Provider{
			func() plugin.Provider { return &providers.LargeProvider{} },
		},
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					require.Len(l, res.Snap.Resources, 3, "expected 3 resources in snapshot")

					largeMap := makeLargeMap()
					large := RequireSingleResource(l, res.Snap.Resources, "large:index:Map")
					largeInputs := resource.FromResourcePropertyMap(large.Inputs)
					require.Equal(l, property.New("leaf"), largeInputs.Get("value"))
					require.Equal(l, property.New(float64(largeMapDepth)), largeInputs.Get("depth"))
					require.Equal(l, largeMap, resource.FromResourcePropertyMap(large.Outputs).Get("value"))

					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
					require.Equal(l, largeMap,
						resource.FromResourcePropertyMap(stack.Outputs).Get("output"),
						"expected large map stack output")
				},
			},
		},
	}
}

func makeLargeMap() property.Value {
	value := property.New(map[string]property.Value{
		"value": property.New("leaf"),
	})
	for range largeMapDepth {
		value = property.New(map[string]property.Value{
			"next": value,
		})
	}
	return value
}
