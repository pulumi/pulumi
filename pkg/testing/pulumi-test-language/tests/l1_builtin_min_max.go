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
	addTest := func(a, b, c, d string, expectedMax, expectedMin, expectedIntMax, expectedIntMin float64) TestRun {
		return TestRun{
			Config: config.Map{
				config.MustMakeKey("l1-builtin-min-max", "a"): config.NewValue(a),
				config.MustMakeKey("l1-builtin-min-max", "b"): config.NewValue(b),
				config.MustMakeKey("l1-builtin-min-max", "c"): config.NewValue(c),
				config.MustMakeKey("l1-builtin-min-max", "d"): config.NewValue(d),
			},
			Assert: func(l *L, res AssertArgs) {
				require.NoError(l, res.Err)
				stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
				assert.Equal(l, resource.PropertyMap{
					"maxResult":    resource.NewProperty(expectedMax),
					"minResult":    resource.NewProperty(expectedMin),
					"intMaxResult": resource.NewProperty(expectedIntMax),
					"intMinResult": resource.NewProperty(expectedIntMin),
				}, stack.Outputs)
			},
		}
	}

	LanguageTests["l1-builtin-min-max"] = LanguageTest{
		RunsShareSource: true,
		Runs: []TestRun{
			addTest("1.5", "2.5", "1", "3", 2.5, 1.5, 3, 1),
			addTest("10", "-5", "7", "2", 10, -5, 7, 2),
			addTest("0.5", "0.1", "100", "-50", 0.5, 0.1, 100, -50),
		},
	}
}
