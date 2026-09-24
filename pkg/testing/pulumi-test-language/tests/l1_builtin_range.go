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
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	ints := func(ns ...float64) resource.PropertyValue {
		values := make([]resource.PropertyValue, 0, len(ns))
		for _, n := range ns {
			values = append(values, resource.NewProperty(n))
		}
		return resource.NewProperty(values)
	}

	addTest := func(from, to string, expectedTo, expectedFromTo resource.PropertyValue) TestRun {
		return TestRun{
			Config: config.Map{
				config.MustMakeKey("l1-builtin-range", "from"): config.NewValue(from),
				config.MustMakeKey("l1-builtin-range", "to"):   config.NewValue(to),
			},
			Assert: func(l *L, res AssertArgs) {
				require.NoError(l, res.Err)
				stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
				assert.Equal(l, resource.PropertyMap{
					"rangeTo":       expectedTo,
					"rangeFromTo":   expectedFromTo,
					"literalTo":     ints(0, 1, 2),
					"literalFromTo": ints(2, 3, 4),
					"literalEmpty":  ints(),
				}, stack.Outputs)
			},
		}
	}

	LanguageTests["l1-builtin-range"] = LanguageTest{
		RunsShareSource: true,
		Runs: []TestRun{
			addTest("0", "3", ints(0, 1, 2), ints(0, 1, 2)),
			addTest("2", "5", ints(0, 1, 2, 3, 4), ints(2, 3, 4)),
			addTest("4", "4", ints(0, 1, 2, 3), ints()),
		},
	}
}
