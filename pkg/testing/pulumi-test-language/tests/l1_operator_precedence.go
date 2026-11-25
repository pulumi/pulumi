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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
)

// The PCL binder drops parentheses, so codegen must write back the parentheses that change the order of operations.
func init() {
	LanguageTests["l1-operator-precedence"] = LanguageTest{
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")

					assert.Equal(l, property.NewMap(map[string]property.Value{
						"sumTimes":             property.New(9.0),
						"minusMinus":           property.New(9.0),
						"notAnd":               property.New(true),
						"negateSum":            property.New(-3.0),
						"conditionalIndex":     property.New(1.0),
						"conditionalCondition": property.New("no"),
					}), stack.Outputs)
				},
			},
		},
	}
}
