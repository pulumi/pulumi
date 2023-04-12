// Copyright 2016-2023, Pulumi Corporation.
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

package display

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_decodeValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		repr     string
		kind     string
		expected interface{}
	}{
		// Negative cases
		{repr: "foo"},
		{repr: "1.0"},
		{repr: "true"},
		{repr: "no"},
		{repr: "-"},
		{repr: "foo: bar"},

		// Positive cases
		{
			repr:     "[]",
			kind:     "json",
			expected: []interface{}{},
		},
		{
			repr:     "[\"foo\", \"bar\"]",
			kind:     "json",
			expected: []interface{}{"foo", "bar"},
		},
		{
			repr:     "{}",
			kind:     "json",
			expected: map[string]interface{}{},
		},
		{
			repr:     `{"foo": "bar"}`,
			kind:     "json",
			expected: map[string]interface{}{"foo": "bar"},
		},
		{
			repr:     "- foo\n- bar",
			kind:     "yaml",
			expected: []interface{}{"foo", "bar"},
		},
		{
			repr:     "foo: bar\nbaz: qux\n",
			kind:     "yaml",
			expected: map[string]interface{}{"foo": "bar", "baz": "qux"},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.repr, func(t *testing.T) {
			t.Parallel()

			var printer propertyPrinter
			actual, kind, ok := printer.decodeValue(c.repr)

			if c.kind == "" {
				assert.Equal(t, resource.PropertyValue{}, actual)
				assert.Equal(t, "", kind)
				assert.False(t, ok)
			} else {
				require.True(t, ok)
				require.Equal(t, c.kind, kind)
				assert.True(t, resource.NewPropertyValue(c.expected).DeepEquals(actual))
			}
		})
	}
}
