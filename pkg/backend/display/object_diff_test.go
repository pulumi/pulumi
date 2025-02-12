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
	"bytes"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
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
		{repr: ""},
		{repr: "foo"},
		{repr: "1.0"},
		{repr: "true"},
		{repr: "no"},
		{repr: "-"},
		{repr: "foo: bar"},
		{repr: "[] bar"},
		{repr: "{} bar"},
		{repr: "[] \n not yaml"},
		{repr: "---\n'hello'\n...\n---\ngoodbye\n...\n"},

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
			repr:     `  {"with": "whitespace"}  `,
			kind:     "json",
			expected: map[string]interface{}{"with": "whitespace"},
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

func Test_PrintObject(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		object   resource.PropertyMap
		expected string
	}{
		{
			"numbers",
			resource.NewPropertyMapFromMap(map[string]interface{}{
				"int":         1,
				"float":       2.3,
				"large_int":   1234567,
				"large_float": 1234567.1234567,
			}),
			`<{%reset%}>float      : <{%reset%}><{%reset%}>2.3<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>int        : <{%reset%}><{%reset%}>1<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>large_float: <{%reset%}><{%reset%}>1.2345671234567e+06<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>large_int  : <{%reset%}><{%reset%}>1234567<{%reset%}><{%reset%}>
<{%reset%}>`,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			PrintObject(&buf, c.object, false, 0, deploy.OpSame, false, false, false, false)
			assert.Equal(t, c.expected, buf.String())
		})
	}
}

func Test_PrintObjectNoShowSecret(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		object   resource.PropertyMap
		expected string
	}{
		{
			"numbers",
			resource.NewPropertyMapFromMap(map[string]interface{}{
				"int":         1,
				"float":       2.3,
				"large_int":   1234567,
				"large_float": 1234567.1234567,
				"secret":      resource.NewSecretProperty(&resource.Secret{Element: resource.NewStringProperty("secrets")}),
				"nested_secret": resource.NewPropertyMapFromMap(map[string]interface{}{
					"super_secret": resource.NewSecretProperty(&resource.Secret{Element: resource.NewStringProperty("super_secret")}),
				}),
			}),
			`<{%reset%}>float        : <{%reset%}><{%reset%}>2.3<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>int          : <{%reset%}><{%reset%}>1<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>large_float  : <{%reset%}><{%reset%}>1.2345671234567e+06<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>large_int    : <{%reset%}><{%reset%}>1234567<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>nested_secret: <{%reset%}><{%reset%}>{
<{%reset%}><{%reset%}>    super_secret: <{%reset%}><{%reset%}>[secret]<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>}<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>secret       : <{%reset%}><{%reset%}>[secret]<{%reset%}><{%reset%}>
<{%reset%}>`,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			PrintObject(&buf, c.object, false, 0, deploy.OpSame, false, false, false, false)
			assert.Equal(t, c.expected, buf.String())
		})
	}
}

func Test_PrintObjectShowSecret(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		object   resource.PropertyMap
		expected string
	}{
		{
			"numbers",
			resource.NewPropertyMapFromMap(map[string]interface{}{
				"int":         1,
				"float":       2.3,
				"large_int":   1234567,
				"large_float": 1234567.1234567,
				"secret":      resource.NewSecretProperty(&resource.Secret{Element: resource.NewStringProperty("my_secret")}),
				"nested_secret": resource.NewPropertyMapFromMap(map[string]interface{}{
					"super_secret": resource.NewSecretProperty(&resource.Secret{Element: resource.NewStringProperty("my_super_secret")}),
				}),
			}),
			`<{%reset%}>float        : <{%reset%}><{%reset%}>2.3<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>int          : <{%reset%}><{%reset%}>1<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>large_float  : <{%reset%}><{%reset%}>1.2345671234567e+06<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>large_int    : <{%reset%}><{%reset%}>1234567<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>nested_secret: <{%reset%}><{%reset%}>{
<{%reset%}><{%reset%}>    super_secret: <{%reset%}><{%reset%}>"my_super_secret"<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>}<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>secret       : <{%reset%}><{%reset%}>"my_secret"<{%reset%}><{%reset%}>
<{%reset%}><{%reset%}>
<{%reset%}>`,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			PrintObject(&buf, c.object, false, 0, deploy.OpSame, false, false, false, true)
			assert.Equal(t, c.expected, buf.String())
		})
	}
}
