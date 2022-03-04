// Copyright 2016-2021, Pulumi Corporation.
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

package plugin

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/stretchr/testify/assert"
)

func TestNewDetailedDiff(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		diff     *resource.ObjectDiff
		expected map[string]PropertyDiff
	}{
		{
			name: "updates",
			diff: resource.NewPropertyMapFromMap(map[string]interface{}{
				"a": 1,
				"b": map[string]interface{}{
					"c": 2,
					"d": 3,
				},
			}).Diff(resource.NewPropertyMapFromMap(map[string]interface{}{
				"a": -1,
				"b": map[string]interface{}{
					"c": -2,
					"d": 3,
				},
			})),
			expected: map[string]PropertyDiff{
				"a": {
					Kind: DiffUpdate,
				},
				"b.c": {
					Kind: DiffUpdate,
				},
			},
		},
		{
			name: "adds and deletes",
			diff: resource.NewPropertyMapFromMap(map[string]interface{}{
				"b": map[string]interface{}{
					"c": 2,
					"d": 3,
				},
			}).Diff(resource.NewPropertyMapFromMap(map[string]interface{}{
				"a": 1,
				"b": map[string]interface{}{
					"d": 3,
				},
			})),
			expected: map[string]PropertyDiff{
				"a": {
					Kind: DiffAdd,
				},
				"b.c": {
					Kind: DiffDelete,
				},
			},
		},
		{
			name: "arrays",
			diff: resource.NewPropertyMapFromMap(map[string]interface{}{
				"a": []interface{}{
					map[string]interface{}{
						"a": 1,
						"b": []interface{}{
							2,
							3,
						},
					},
				},
			}).Diff(resource.NewPropertyMapFromMap(
				map[string]interface{}{
					"a": []interface{}{
						map[string]interface{}{
							"a": -1,
							"b": []interface{}{
								2,
							},
						},
						4,
					},
				})),
			expected: map[string]PropertyDiff{
				"a[0].a": {
					Kind: DiffUpdate,
				},
				"a[0].b[1]": {
					Kind: DiffDelete,
				},
				"a[1]": {
					Kind: DiffAdd,
				},
			},
		},
		{
			name:     "nil diff",
			diff:     nil,
			expected: map[string]PropertyDiff{},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			actual := NewDetailedDiffFromObjectDiff(c.diff)
			assert.Equal(t, c.expected, actual)
		})
	}
}
