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
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/rivo/uniseg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	addTest := func(input string) TestRun {
		parts := strings.Split(input, "-")
		splitResult := make([]resource.PropertyValue, len(parts))
		for i, p := range parts {
			splitResult[i] = resource.NewProperty(p)
		}

		return TestRun{
			Config: config.Map{
				config.MustMakeKey("l1-builtin-string", "aString"): config.NewValue(input),
			},
			Assert: func(l *L, res AssertArgs) {
				require.NoError(l, res.Err)
				stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")
				assert.Equal(l, resource.PropertyMap{
					// GraphemeClusterCount returns the number of Unicode grapheme clusters in s, matching the behavior
					// of PCL's length() on strings.
					"lengthResult": resource.NewProperty(
						float64(uniseg.GraphemeClusterCount(input))),
					"splitResult":       resource.NewProperty(splitResult),
					"joinResult":        resource.NewProperty(strings.Join(parts, "|")),
					"interpolateResult": resource.NewProperty("prefix-" + input),
				}, stack.Outputs)
			},
		}
	}

	LanguageTests["l1-builtin-string"] = LanguageTest{
		RunsShareSource: true,
		Runs: []TestRun{
			// ASCII
			addTest("foo-bar-baz"),
			addTest("hello"),
			addTest("a-b-c-d"),
			// Multi-byte Latin: each rune is a single grapheme cluster.
			addTest("café-thé"),
			// Emoji with variation selectors: grapheme clusters < runes < bytes.
			// "👾🕹️" = 2 grapheme clusters (🕹️ has a variation selector).
			addTest("👾-🕹️"),
			// ZWJ family sequence = 1 grapheme cluster, split against ASCII parts.
			addTest("👨‍👩‍👧‍👦-family-👶"),
		},
	}
}
