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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
)

func init() {
	const escapeString = "Some ${common} \"characters\" 'that' need escaping: " +
		"\\ (backslash), \t (tab), \u001b (escape), \u0007 (bell), \u0000 (null), \U000e0021 (tag space)"
	LanguageTests["l1-output-map"] = LanguageTest{
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					err := res.Err
					snap := res.Snap
					changes := res.Changes

					RequireStackResource(l, err, changes)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					assert.Equal(l, property.NewMap(map[string]property.Value{
						"empty": property.New(property.Map{}),
						"strings": property.New(map[string]property.Value{
							"greeting": property.New("Hello, world!"),
							"farewell": property.New("Goodbye, world!"),
						}),
						"numbers": property.New(map[string]property.Value{
							"1": property.New(1.0),
							"2": property.New(2.0),
						}),
						"keys": property.New(map[string]property.Value{
							"my.key": property.New(1.0),
							"my-key": property.New(2.0),
							"my_key": property.New(3.0),
							"MY_KEY": property.New(4.0),
							"mykey":  property.New(5.0),
							"MYKEY":  property.New(6.0),
						}),
						"adversarialStrings": property.New(map[string]property.Value{
							"__type":       property.New("dunder type"),
							"__internal":   property.New("dunder internal"),
							"__provider":   property.New("dunder provider"),
							"__version":    property.New("dunder version"),
							"":             property.New("empty key"),
							"empty value":  property.New(""),
							"dunder value": property.New("__dunder"),
							escapeString:   property.New(escapeString),
						}),
					}), resource.FromResourcePropertyMap(stack.Outputs))
				},
			},
		},
	}
}
