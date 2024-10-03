// Copyright 2019-2024, Pulumi Corporation.
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

package resource

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/deepcopy"
	"github.com/stretchr/testify/assert"
)

func TestPropertyPath(t *testing.T) {
	t.Parallel()

	makeValue := func() PropertyValue {
		return NewProperty(NewPropertyMapFromMap(map[string]interface{}{
			"root": map[string]interface{}{
				"nested": map[string]interface{}{
					"array": []interface{}{
						map[string]interface{}{
							"double": []interface{}{
								nil,
								true,
							},
						},
					},
				},
				"double": map[string]interface{}{
					"nest": true,
				},
				"array": []interface{}{
					map[string]interface{}{
						"nested": true,
					},
					true,
				},
				"array2": []interface{}{
					[]interface{}{
						nil,
						map[string]interface{}{
							"nested": true,
						},
					},
				},
				`key with "escaped" quotes`: true,
				"key with a .":              true,
			},
			`root key with "escaped" quotes`: map[string]interface{}{
				"nested": true,
			},
			"root key with a .": []interface{}{
				nil,
				true,
			},
		}))
	}

	cases := []struct {
		path     string
		parsed   PropertyPath
		expected string
	}{
		{
			"root",
			PropertyPath{"root"},
			"root",
		},
		{
			"root.nested",
			PropertyPath{"root", "nested"},
			"root.nested",
		},
		{
			`root["nested"]`,
			PropertyPath{"root", "nested"},
			`root.nested`,
		},
		{
			"root.double.nest",
			PropertyPath{"root", "double", "nest"},
			"root.double.nest",
		},
		{
			`root["double"].nest`,
			PropertyPath{"root", "double", "nest"},
			`root.double.nest`,
		},
		{
			`root["double"]["nest"]`,
			PropertyPath{"root", "double", "nest"},
			`root.double.nest`,
		},
		{
			"root.array[0]",
			PropertyPath{"root", "array", 0},
			"root.array[0]",
		},
		{
			"root.array[1]",
			PropertyPath{"root", "array", 1},
			"root.array[1]",
		},
		{
			"root.array[0].nested",
			PropertyPath{"root", "array", 0, "nested"},
			"root.array[0].nested",
		},
		{
			"root.array2[0][1].nested",
			PropertyPath{"root", "array2", 0, 1, "nested"},
			"root.array2[0][1].nested",
		},
		{
			"root.nested.array[0].double[1]",
			PropertyPath{"root", "nested", "array", 0, "double", 1},
			"root.nested.array[0].double[1]",
		},
		{
			`root["key with \"escaped\" quotes"]`,
			PropertyPath{"root", `key with "escaped" quotes`},
			`root["key with \"escaped\" quotes"]`,
		},
		{
			`root["key with a ."]`,
			PropertyPath{"root", "key with a ."},
			`root["key with a ."]`,
		},
		{
			`["root key with \"escaped\" quotes"].nested`,
			PropertyPath{`root key with "escaped" quotes`, "nested"},
			`["root key with \"escaped\" quotes"].nested`,
		},
		{
			`["root key with a ."][1]`,
			PropertyPath{"root key with a .", 1},
			`["root key with a ."][1]`,
		},
		// The following two cases are regressions for https://github.com/pulumi/pulumi/issues/14439. Ideally
		// these would be a syntax error, but it seems providers have been emitting paths of this style and so
		// we need to keep supporting them.
		{
			`root.array.[1]`,
			PropertyPath{"root", "array", 1},
			`root.array[1]`,
		},
		{
			`root.["key with a ."]`,
			PropertyPath{"root", "key with a ."},
			`root["key with a ."]`,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.path, func(t *testing.T) {
			t.Parallel()

			parsed, err := ParsePropertyPath(c.path)
			assert.NoError(t, err)
			assert.Equal(t, c.parsed, parsed)
			assert.Equal(t, c.expected, parsed.String())

			value := makeValue()

			v, ok := parsed.Get(value)
			assert.True(t, ok, "Failed to get %v from %v", parsed, value)
			assert.False(t, v.IsNull())

			ok = parsed.Delete(value)
			assert.True(t, ok, "Failed to delete %v from %v", parsed, value)

			ok = parsed.Set(value, v)
			assert.True(t, ok, "Failed to set %v in %v", v, parsed)

			u, ok := parsed.Get(value)
			assert.True(t, ok, "Failed to get %v from %v", parsed, value)
			assert.Equal(t, v, u)

			vv := PropertyValue{}
			vv, ok = parsed.Add(vv, v)
			assert.True(t, ok, "Failed to add %v at %v", v, parsed)

			u, ok = parsed.Get(vv)
			assert.True(t, ok, "Failed to get %v from %v", parsed, vv)
			assert.Equal(t, v, u)
		})
	}

	simpleCases := []struct {
		path     string
		expected PropertyPath
	}{
		{
			`root["*"].field[1]`,
			PropertyPath{"root", "*", "field", 1},
		},
		{
			`root[*].field[2]`,
			PropertyPath{"root", "*", "field", 2},
		},
		{
			`root[3].*[3]`,
			PropertyPath{"root", 3, "*", 3},
		},
		{
			`*.bar`,
			PropertyPath{"*", "bar"},
		},
	}

	t.Run("Simple", func(t *testing.T) {
		t.Parallel()

		for _, c := range simpleCases {
			c := c
			t.Run(c.path, func(t *testing.T) {
				t.Parallel()

				parsed, err := ParsePropertyPath(c.path)
				assert.NoError(t, err)
				assert.Equal(t, c.expected, parsed)
			})
		}
	})

	negativeCases := []string{
		// Syntax errors
		"root[",
		`root["nested]`,
		`root."double".nest`,
		"root.array[abc]",
		"root.",

		// Missing values
		"root[1]",
		"root.nested.array[100]",
		"root.nested.array.bar",
		"foo",
	}

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, c := range negativeCases {
		c := c
		t.Run(c, func(t *testing.T) {
			t.Parallel()

			parsed, err := ParsePropertyPath(c)
			if err == nil {
				value := makeValue()

				v, ok := parsed.Get(value)
				assert.False(t, ok)
				assert.True(t, v.IsNull())
			}
		})
	}

	negativeCasesStrict := []string{
		// Syntax erros
		`root.array.[1]`,
		`root.["key with a ."]`,
	}
	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, c := range negativeCasesStrict {
		c := c
		t.Run(c, func(t *testing.T) {
			t.Parallel()

			_, err := ParsePropertyPathStrict(c)
			assert.NotNil(t, err)
		})
	}
}

func TestPropertyPathContains(t *testing.T) {
	t.Parallel()

	cases := []struct {
		p1       PropertyPath
		p2       PropertyPath
		expected bool
	}{
		{
			PropertyPath{"root", "nested"},
			PropertyPath{"root"},
			false,
		},
		{
			PropertyPath{"root"},
			PropertyPath{"root", "nested"},
			true,
		},
		{
			PropertyPath{"root", 1},
			PropertyPath{"root"},
			false,
		},
		{
			PropertyPath{"root"},
			PropertyPath{"root", 1},
			true,
		},
		{
			PropertyPath{"root", "double", "nest1"},
			PropertyPath{"root", "double", "nest2"},
			false,
		},
		{
			PropertyPath{"root", "nest1", "double"},
			PropertyPath{"root", "nest2", "double"},
			false,
		},
		{
			PropertyPath{"root", "nest", "double"},
			PropertyPath{"root", "nest", "double"},
			true,
		},
		{
			PropertyPath{"root", 1, "double"},
			PropertyPath{"root", 1, "double"},
			true,
		},
		{
			PropertyPath{},
			PropertyPath{},
			true,
		},
		{
			PropertyPath{"root"},
			PropertyPath{},
			false,
		},
		{
			PropertyPath{},
			PropertyPath{"root"},
			true,
		},
		{
			PropertyPath{"foo", "bar", 1},
			PropertyPath{"foo", "bar", 1, "baz"},
			true,
		},
		{
			PropertyPath{"foo", "*", "baz"},
			PropertyPath{"foo", "bar", "baz", "bam"},
			true,
		},
		{
			PropertyPath{"*", "bar", "baz"},
			PropertyPath{"foo", "bar", "baz", "bam"},
			true,
		},
		{
			PropertyPath{"foo", "*", "baz"},
			PropertyPath{"foo", 1, "baz", "bam"},
			true,
		},
		{
			PropertyPath{"foo", 1, "*", "bam"},
			PropertyPath{"foo", 1, "baz", "bam"},
			true,
		},
		{
			PropertyPath{"*"},
			PropertyPath{"a", "b"},
			true,
		},
		{
			PropertyPath{"*"},
			PropertyPath{"a", 1},
			true,
		},
		{
			PropertyPath{"*"},
			PropertyPath{"a", 1, "b"},
			true,
		},
	}

	for _, tcase := range cases {
		res := tcase.p1.Contains(tcase.p2)
		assert.Equal(t, tcase.expected, res)
	}
}

func TestAddResizePropertyPath(t *testing.T) {
	t.Parallel()

	// Regression test for https://github.com/pulumi/pulumi/issues/5871:
	// Ensure that adding a new element beyond the size of an array will resize it.
	path, err := ParsePropertyPath("[1]")
	assert.NoError(t, err)
	_, ok := path.Add(NewProperty([]PropertyValue{}), NewProperty(42.0))
	assert.True(t, ok)
}

func TestReset(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		path     PropertyPath
		old      PropertyMap
		new      PropertyMap
		expected *PropertyMap
	}{
		{
			"Missing key, not in object",
			PropertyPath{"missing"},
			PropertyMap{"root": NewProperty(1.0)},
			PropertyMap{"root": NewProperty(2.0)},
			&PropertyMap{"root": NewProperty(2.0)},
		},
		{
			"Missing key, not an object",
			PropertyPath{"root", "missing"},
			PropertyMap{"root": NewProperty(1.0)},
			PropertyMap{"root": NewProperty(2.0)},
			&PropertyMap{"root": NewProperty(2.0)},
		},
		{
			"Missing index, changed from an object",
			PropertyPath{"root", "nested"},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(1.0)})},
			PropertyMap{"root": NewProperty(2.0)},
			nil,
		},
		{
			"Missing index, changed to an object",
			PropertyPath{"root", "nested"},
			PropertyMap{"root": NewProperty(1.0)},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(2.0)})},
			&PropertyMap{"root": NewProperty(PropertyMap{})},
		},
		{
			"Missing key along path, not in object",
			PropertyPath{"missing", "path"},
			PropertyMap{"root": NewProperty(1.0)},
			PropertyMap{"root": NewProperty(2.0)},
			&PropertyMap{"root": NewProperty(2.0)},
		},
		{
			"Missing key along path, not an object",
			PropertyPath{"root", "missing", "path"},
			PropertyMap{"root": NewProperty(1.0)},
			PropertyMap{"root": NewProperty(2.0)},
			&PropertyMap{"root": NewProperty(2.0)},
		},
		{
			"Missing index, not in array",
			PropertyPath{"array", 1},
			PropertyMap{"array": NewProperty([]PropertyValue{NewProperty(1.0)})},
			PropertyMap{"array": NewProperty([]PropertyValue{NewProperty(1.0)})},
			&PropertyMap{"array": NewProperty([]PropertyValue{NewProperty(1.0)})},
		},
		{
			"Missing index, not an array",
			PropertyPath{"root", 0},
			PropertyMap{"root": NewProperty(1.0)},
			PropertyMap{"root": NewProperty(2.0)},
			&PropertyMap{"root": NewProperty(2.0)},
		},
		{
			"Missing index, changed from an array",
			PropertyPath{"root", 0},
			PropertyMap{"root": NewProperty([]PropertyValue{NewProperty(1.0)})},
			PropertyMap{"root": NewProperty(2.0)},
			nil,
		},
		{
			"Missing index, changed to an array",
			PropertyPath{"root", 0},
			PropertyMap{"root": NewProperty(1.0)},
			PropertyMap{"root": NewProperty([]PropertyValue{NewProperty(2.0)})},
			nil,
		},
		{
			"Invalid index, not in array",
			PropertyPath{"array", -1},
			PropertyMap{"array": NewProperty([]PropertyValue{NewProperty(1.0)})},
			PropertyMap{"array": NewProperty([]PropertyValue{NewProperty(1.0)})},
			nil,
		},
		{
			"Invalid index, not an array",
			PropertyPath{"root", -1},
			PropertyMap{"root": NewProperty(1.0)},
			PropertyMap{"root": NewProperty(2.0)},
			nil,
		},
		{
			"Index out of bound in old",
			PropertyPath{"root", 1},
			PropertyMap{"root": NewProperty([]PropertyValue{NewProperty(2.0)})},
			PropertyMap{"root": NewProperty([]PropertyValue{NewProperty(3.0), NewProperty(4.0)})},
			nil,
		},
		{
			"Index out of bound in new",
			PropertyPath{"root", 1},
			PropertyMap{"root": NewProperty([]PropertyValue{NewProperty(1.0), NewProperty(2.0)})},
			PropertyMap{"root": NewProperty([]PropertyValue{NewProperty(3.0)})},
			nil,
		},
		{
			"Missing index along path",
			PropertyPath{"root", 0, "other"},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(1.0)})},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(2.0)})},
			&PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(2.0)})},
		},
		{
			"Index out of bound in old along path",
			PropertyPath{"root", 1, "nested"},
			PropertyMap{"root": NewProperty([]PropertyValue{
				NewProperty(PropertyMap{"nested": NewProperty(1.0)}),
			})},
			PropertyMap{"root": NewProperty([]PropertyValue{
				NewProperty(PropertyMap{"nested": NewProperty(3.0)}),
				NewProperty(PropertyMap{"nested": NewProperty(4.0)}),
			})},
			nil,
		},
		{
			"Index out of bound in new along path",
			PropertyPath{"root", 1, "nested"},
			PropertyMap{"root": NewProperty([]PropertyValue{
				NewProperty(PropertyMap{"nested": NewProperty(1.0)}),
				NewProperty(PropertyMap{"nested": NewProperty(2.0)}),
			})},
			PropertyMap{"root": NewProperty([]PropertyValue{
				NewProperty(PropertyMap{"nested": NewProperty(3.0)}),
			})},
			nil,
		},
		{
			"Single path element",
			PropertyPath{"root"},
			PropertyMap{"root": NewProperty(1.0)},
			PropertyMap{"root": NewProperty(2.0)},
			&PropertyMap{"root": NewProperty(1.0)},
		},
		{
			"Nested path element, changed",
			PropertyPath{"root", "nested"},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(1.0)})},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(2.0)})},
			&PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(1.0)})},
		},
		{
			"Nested path element, added",
			PropertyPath{"root", "nested"},
			PropertyMap{"root": NewProperty(PropertyMap{})},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(2.0)})},
			&PropertyMap{"root": NewProperty(PropertyMap{})},
		},
		{
			"Nested path element, removed",
			PropertyPath{"root", "nested"},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(1.0)})},
			PropertyMap{"root": NewProperty(PropertyMap{})},
			&PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(1.0)})},
		},
		{
			"Nested path element, fully removed",
			PropertyPath{"root", "nested"},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(1.0)})},
			PropertyMap{},
			nil,
		},
		{
			"Nested path element, nested added",
			PropertyPath{"root", "nested"},
			PropertyMap{},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(2.0)})},
			&PropertyMap{"root": NewProperty(PropertyMap{})},
		},
		{
			"Nested path element, nested removed",
			PropertyPath{"root", "nested"},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(1.0)})},
			PropertyMap{},
			nil,
		},
		{
			"Array index",
			PropertyPath{"array", 0},
			PropertyMap{"array": NewProperty([]PropertyValue{NewProperty(1.0)})},
			PropertyMap{"array": NewProperty([]PropertyValue{NewProperty(2.0)})},
			&PropertyMap{"array": NewProperty([]PropertyValue{NewProperty(1.0)})},
		},
		{
			"Array index, along path",
			PropertyPath{"array", 0, "item"},
			PropertyMap{"array": NewProperty([]PropertyValue{
				NewProperty(PropertyMap{"item": NewProperty(1.0)}),
			})},
			PropertyMap{"array": NewProperty([]PropertyValue{
				NewProperty(PropertyMap{"item": NewProperty(2.0)}),
			})},
			&PropertyMap{"array": NewProperty([]PropertyValue{
				NewProperty(PropertyMap{"item": NewProperty(1.0)}),
			})},
		},
		{
			"Array index, added",
			PropertyPath{"array", 0},
			PropertyMap{"array": NewProperty([]PropertyValue{})},
			PropertyMap{"array": NewProperty([]PropertyValue{NewProperty(2.0)})},
			nil,
		},
		{
			"Array index, removed",
			PropertyPath{"array", 0},
			PropertyMap{"array": NewProperty([]PropertyValue{NewProperty(1.0)})},
			PropertyMap{"array": NewProperty([]PropertyValue{})},
			nil,
		},
		{
			"Single wildcard at root",
			PropertyPath{"*"},
			PropertyMap{"root": NewProperty(1.0)},
			PropertyMap{"root": NewProperty(2.0)},
			&PropertyMap{"root": NewProperty(1.0)},
		},
		{
			"Wildcard followed by path element",
			PropertyPath{"*", "nested"},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(1.0)})},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(2.0)})},
			&PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(1.0)})},
		},
		{
			"Wildcard in array followed by path element",
			PropertyPath{"root", "*", "nested"},
			PropertyMap{"root": NewProperty([]PropertyValue{
				NewProperty(PropertyMap{"nested": NewProperty(1.0)}),
				NewProperty(PropertyMap{"nested": NewProperty(2.0)}),
			})},
			PropertyMap{"root": NewProperty([]PropertyValue{
				NewProperty(PropertyMap{"nested": NewProperty(3.0)}),
				NewProperty(PropertyMap{"nested": NewProperty(4.0)}),
			})},
			&PropertyMap{"root": NewProperty([]PropertyValue{
				NewProperty(PropertyMap{"nested": NewProperty(1.0)}),
				NewProperty(PropertyMap{"nested": NewProperty(2.0)}),
			})},
		},
		{
			"Nested wildcard",
			PropertyPath{"root", "*"},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(1.0)})},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(2.0)})},
			&PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(1.0)})},
		},
		{
			"Nested wildcard in array",
			PropertyPath{"root", "*"},
			PropertyMap{"root": NewProperty([]PropertyValue{
				NewProperty(1.0),
				NewProperty(2.0),
			})},
			PropertyMap{"root": NewProperty([]PropertyValue{
				NewProperty(3.0),
				NewProperty(4.0),
			})},
			&PropertyMap{"root": NewProperty([]PropertyValue{
				NewProperty(1.0),
				NewProperty(2.0),
			})},
		},
		{
			"Nested wildcard, added",
			PropertyPath{"root", "*"},
			PropertyMap{"root": NewProperty(PropertyMap{})},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(2.0)})},
			&PropertyMap{"root": NewProperty(PropertyMap{})},
		},
		{
			"Nested wildcard, removed",
			PropertyPath{"root", "*"},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(2.0)})},
			PropertyMap{"root": NewProperty(PropertyMap{})},
			&PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(2.0)})},
		},
		{
			"Nested wildcard, change of type (array)",
			PropertyPath{"root", "*"},
			PropertyMap{"root": NewProperty([]PropertyValue{NewProperty(1.0)})},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(2.0)})},
			&PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(2.0)})},
		},
		{
			"Nested wildcard, change of type (object)",
			PropertyPath{"root", "*"},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(2.0)})},
			PropertyMap{"root": NewProperty([]PropertyValue{NewProperty(1.0)})},
			&PropertyMap{"root": NewProperty([]PropertyValue{NewProperty(1.0)})},
		},
		{
			"Nested wildcard, change of type (number)",
			PropertyPath{"root", "*"},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(2.0)})},
			PropertyMap{"root": NewProperty(1.0)},
			nil,
		},
		{
			"Untraversable in old wildcard followed by path element",
			PropertyPath{"root", "*", "nested"},
			PropertyMap{"root": NewProperty(1.0)},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(2.0)})},
			nil,
		},
		{
			"Untraversable in new (not an object) wildcard followed by path element",
			PropertyPath{"root", "*", "nested"},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(1.0)})},
			PropertyMap{"root": NewProperty(2.0)},
			nil,
		},
		{
			"Untraversable in new (missing) wildcard followed by path element",
			PropertyPath{"root", "*", "nested"},
			PropertyMap{"root": NewProperty(PropertyMap{"nested": NewProperty(1.0)})},
			PropertyMap{"root": NewProperty(PropertyMap{})},
			nil,
		},
		{
			"Nested object wildcard index reset fails",
			PropertyPath{"root", "*", 0},
			PropertyMap{"root": NewProperty(PropertyMap{
				"passes": NewProperty(1.0),
				"fails":  NewProperty([]PropertyValue{NewProperty(1.0)}),
			})},
			PropertyMap{"root": NewProperty(PropertyMap{
				"passes": NewProperty(2.0),
				"fails":  NewProperty([]PropertyValue{}),
			})},
			nil,
		},
		{
			"Nested array wildcard, new array is shorter fails",
			PropertyPath{"root", "array", "*"},
			PropertyMap{"root": NewProperty(PropertyMap{
				"array": NewProperty([]PropertyValue{
					NewProperty(1.0),
				}),
			})},
			PropertyMap{"root": NewProperty(PropertyMap{
				"array": NewProperty([]PropertyValue{}),
			})},
			nil,
		},
		{
			"Nested array wildcard index reset fails",
			PropertyPath{"root", "*", 0},
			PropertyMap{"root": NewProperty([]PropertyValue{
				NewProperty(1.0),
				NewProperty([]PropertyValue{NewProperty(1.0)}),
			})},
			PropertyMap{"root": NewProperty([]PropertyValue{
				NewProperty(2.0),
				NewProperty([]PropertyValue{}),
			})},
			nil,
		},
		{
			"Nested array wildcard index, old array is longer fails",
			PropertyPath{"root", "*", 0},
			PropertyMap{"root": NewProperty([]PropertyValue{
				NewProperty(1.0),
				NewProperty(2.0),
			})},
			PropertyMap{"root": MakeSecret(NewProperty([]PropertyValue{
				NewProperty(3.0),
			}))},
			nil,
		},
		{
			"Nested array wildcard index, new array is longer fails",
			PropertyPath{"root", "*", 0},
			PropertyMap{"root": NewProperty([]PropertyValue{
				NewProperty(1.0),
			})},
			PropertyMap{"root": MakeSecret(NewProperty([]PropertyValue{
				NewProperty(3.0),
				NewProperty(4.0),
			}))},
			nil,
		},
		{
			"Nested path, secret old",
			PropertyPath{"root", "secret"},
			PropertyMap{"root": MakeSecret(NewProperty(PropertyMap{"secret": NewProperty(1.0)}))},
			PropertyMap{"root": NewProperty(PropertyMap{"secret": NewProperty(2.0)})},
			&PropertyMap{"root": NewProperty(PropertyMap{"secret": MakeSecret(NewProperty(1.0))})},
		},
		{
			"Nested path, secret new",
			PropertyPath{"root", "secret"},
			PropertyMap{"root": NewProperty(PropertyMap{"secret": NewProperty(1.0)})},
			PropertyMap{"root": MakeSecret(NewProperty(PropertyMap{"secret": NewProperty(2.0)}))},
			&PropertyMap{"root": MakeSecret(NewProperty(PropertyMap{"secret": NewProperty(1.0)}))},
		},
		{
			"Nested path, secret both",
			PropertyPath{"root", "secret"},
			PropertyMap{"root": MakeSecret(NewProperty(PropertyMap{"secret": NewProperty(1.0)}))},
			PropertyMap{"root": MakeSecret(NewProperty(PropertyMap{"secret": NewProperty(2.0)}))},
			&PropertyMap{"root": MakeSecret(NewProperty(PropertyMap{"secret": NewProperty(1.0)}))},
		},
		{
			"Nested array, secret old",
			PropertyPath{"root", 0},
			PropertyMap{"root": MakeSecret(NewProperty([]PropertyValue{
				NewProperty(1.0),
				NewProperty(2.0),
			}))},
			PropertyMap{"root": NewProperty([]PropertyValue{
				NewProperty(3.0),
				NewProperty(4.0),
			})},
			&PropertyMap{"root": NewProperty([]PropertyValue{
				MakeSecret(NewProperty(1.0)),
				NewProperty(4.0),
			})},
		},
		{
			"Nested array, secret new",
			PropertyPath{"root", 0},
			PropertyMap{"root": NewProperty([]PropertyValue{
				NewProperty(1.0),
				NewProperty(2.0),
			})},
			PropertyMap{"root": MakeSecret(NewProperty([]PropertyValue{
				NewProperty(3.0),
				NewProperty(4.0),
			}))},
			&PropertyMap{"root": MakeSecret(NewProperty([]PropertyValue{
				NewProperty(1.0),
				NewProperty(4.0),
			}))},
		},
		{
			"Nested array, secret both",
			PropertyPath{"root", 0},
			PropertyMap{"root": MakeSecret(NewProperty([]PropertyValue{
				NewProperty(1.0),
				NewProperty(2.0),
			}))},
			PropertyMap{"root": MakeSecret(NewProperty([]PropertyValue{
				NewProperty(3.0),
				NewProperty(4.0),
			}))},
			&PropertyMap{"root": MakeSecret(NewProperty([]PropertyValue{
				NewProperty(1.0),
				NewProperty(4.0),
			}))},
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			newCopy := deepcopy.Copy(tt.new).(PropertyMap)

			res := tt.path.Reset(tt.old, tt.new)
			if tt.expected == nil {
				assert.False(t, res)
				assert.Equal(t, newCopy, tt.new)
			} else {
				assert.True(t, res)
				assert.Equal(t, *tt.expected, tt.new)
			}
		})
	}
}
