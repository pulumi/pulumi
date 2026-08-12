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

					assert.Equal(l, resource.PropertyMap{
						"empty": resource.NewProperty(resource.PropertyMap{}),
						"strings": resource.NewProperty(resource.PropertyMap{
							"greeting": resource.NewProperty("Hello, world!"),
							"farewell": resource.NewProperty("Goodbye, world!"),
						}),
						"numbers": resource.NewProperty(resource.PropertyMap{
							"1": resource.NewProperty(1.0),
							"2": resource.NewProperty(2.0),
						}),
						"keys": resource.NewProperty(resource.PropertyMap{
							"my.key": resource.NewProperty(1.0),
							"my-key": resource.NewProperty(2.0),
							"my_key": resource.NewProperty(3.0),
							"MY_KEY": resource.NewProperty(4.0),
							"mykey":  resource.NewProperty(5.0),
							"MYKEY":  resource.NewProperty(6.0),
						}),
						"adversarialStrings": resource.NewProperty(resource.PropertyMap{
							"__type":       resource.NewProperty("dunder type"),
							"__internal":   resource.NewProperty("dunder internal"),
							"__provider":   resource.NewProperty("dunder provider"),
							"__version":    resource.NewProperty("dunder version"),
							"":             resource.NewProperty("empty key"),
							"empty value":  resource.NewProperty(""),
							"dunder value": resource.NewProperty("__dunder"),
							escapeString:   resource.NewProperty(escapeString),
						}),
					}, stack.Outputs)
				},
			},
		},
	}
}
