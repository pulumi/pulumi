package resource

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/deepcopy"
	"github.com/stretchr/testify/assert"
)

func TestPropertyPath(t *testing.T) {
	t.Parallel()

	makeValue := func() PropertyValue {
		return NewObjectProperty(NewPropertyMapFromMap(map[string]interface{}{
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
			assert.True(t, ok)
			assert.False(t, v.IsNull())

			ok = parsed.Delete(value)
			assert.True(t, ok)

			ok = parsed.Set(value, v)
			assert.True(t, ok)

			u, ok := parsed.Get(value)
			assert.True(t, ok)
			assert.Equal(t, v, u)

			vv := PropertyValue{}
			vv, ok = parsed.Add(vv, v)
			assert.True(t, ok)

			u, ok = parsed.Get(vv)
			assert.True(t, ok)
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
		"root.[1]",

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
	_, ok := path.Add(NewArrayProperty([]PropertyValue{}), NewNumberProperty(42))
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
			PropertyMap{"root": NewNumberProperty(1)},
			PropertyMap{"root": NewNumberProperty(2)},
			&PropertyMap{"root": NewNumberProperty(2)},
		},
		{
			"Missing key, not an object",
			PropertyPath{"root", "missing"},
			PropertyMap{"root": NewNumberProperty(1)},
			PropertyMap{"root": NewNumberProperty(2)},
			&PropertyMap{"root": NewNumberProperty(2)},
		},
		{
			"Missing index, changed from an object",
			PropertyPath{"root", "nested"},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(1)})},
			PropertyMap{"root": NewNumberProperty(2)},
			nil,
		},
		{
			"Missing index, changed to an object",
			PropertyPath{"root", "nested"},
			PropertyMap{"root": NewNumberProperty(1)},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(2)})},
			&PropertyMap{"root": NewObjectProperty(PropertyMap{})},
		},
		{
			"Missing key along path, not in object",
			PropertyPath{"missing", "path"},
			PropertyMap{"root": NewNumberProperty(1)},
			PropertyMap{"root": NewNumberProperty(2)},
			&PropertyMap{"root": NewNumberProperty(2)},
		},
		{
			"Missing key along path, not an object",
			PropertyPath{"root", "missing", "path"},
			PropertyMap{"root": NewNumberProperty(1)},
			PropertyMap{"root": NewNumberProperty(2)},
			&PropertyMap{"root": NewNumberProperty(2)},
		},
		{
			"Missing index, not in array",
			PropertyPath{"array", 1},
			PropertyMap{"array": NewArrayProperty([]PropertyValue{NewNumberProperty(1)})},
			PropertyMap{"array": NewArrayProperty([]PropertyValue{NewNumberProperty(1)})},
			&PropertyMap{"array": NewArrayProperty([]PropertyValue{NewNumberProperty(1)})},
		},
		{
			"Missing index, not an array",
			PropertyPath{"root", 0},
			PropertyMap{"root": NewNumberProperty(1)},
			PropertyMap{"root": NewNumberProperty(2)},
			&PropertyMap{"root": NewNumberProperty(2)},
		},
		{
			"Missing index, changed from an array",
			PropertyPath{"root", 0},
			PropertyMap{"root": NewArrayProperty([]PropertyValue{NewNumberProperty(1)})},
			PropertyMap{"root": NewNumberProperty(2)},
			nil,
		},
		{
			"Missing index, changed to an array",
			PropertyPath{"root", 0},
			PropertyMap{"root": NewNumberProperty(1)},
			PropertyMap{"root": NewArrayProperty([]PropertyValue{NewNumberProperty(2)})},
			nil,
		},
		{
			"Invalid index, not in array",
			PropertyPath{"array", -1},
			PropertyMap{"array": NewArrayProperty([]PropertyValue{NewNumberProperty(1)})},
			PropertyMap{"array": NewArrayProperty([]PropertyValue{NewNumberProperty(1)})},
			nil,
		},
		{
			"Invalid index, not an array",
			PropertyPath{"root", -1},
			PropertyMap{"root": NewNumberProperty(1)},
			PropertyMap{"root": NewNumberProperty(2)},
			nil,
		},
		{
			"Index out of bound in old",
			PropertyPath{"root", 1},
			PropertyMap{"root": NewArrayProperty([]PropertyValue{NewNumberProperty(2)})},
			PropertyMap{"root": NewArrayProperty([]PropertyValue{NewNumberProperty(3), NewNumberProperty(4)})},
			nil,
		},
		{
			"Index out of bound in new",
			PropertyPath{"root", 1},
			PropertyMap{"root": NewArrayProperty([]PropertyValue{NewNumberProperty(1), NewNumberProperty(2)})},
			PropertyMap{"root": NewArrayProperty([]PropertyValue{NewNumberProperty(3)})},
			nil,
		},
		{
			"Missing index along path",
			PropertyPath{"root", 0, "other"},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(1)})},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(2)})},
			&PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(2)})},
		},
		{
			"Index out of bound in old along path",
			PropertyPath{"root", 1, "nested"},
			PropertyMap{"root": NewArrayProperty([]PropertyValue{
				NewObjectProperty(PropertyMap{"nested": NewNumberProperty(1)}),
			})},
			PropertyMap{"root": NewArrayProperty([]PropertyValue{
				NewObjectProperty(PropertyMap{"nested": NewNumberProperty(3)}),
				NewObjectProperty(PropertyMap{"nested": NewNumberProperty(4)}),
			})},
			nil,
		},
		{
			"Index out of bound in new along path",
			PropertyPath{"root", 1, "nested"},
			PropertyMap{"root": NewArrayProperty([]PropertyValue{
				NewObjectProperty(PropertyMap{"nested": NewNumberProperty(1)}),
				NewObjectProperty(PropertyMap{"nested": NewNumberProperty(2)}),
			})},
			PropertyMap{"root": NewArrayProperty([]PropertyValue{
				NewObjectProperty(PropertyMap{"nested": NewNumberProperty(3)}),
			})},
			nil,
		},
		{
			"Single path element",
			PropertyPath{"root"},
			PropertyMap{"root": NewNumberProperty(1)},
			PropertyMap{"root": NewNumberProperty(2)},
			&PropertyMap{"root": NewNumberProperty(1)},
		},
		{
			"Nested path element, changed",
			PropertyPath{"root", "nested"},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(1)})},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(2)})},
			&PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(1)})},
		},
		{
			"Nested path element, added",
			PropertyPath{"root", "nested"},
			PropertyMap{"root": NewObjectProperty(PropertyMap{})},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(2)})},
			&PropertyMap{"root": NewObjectProperty(PropertyMap{})},
		},
		{
			"Nested path element, removed",
			PropertyPath{"root", "nested"},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(1)})},
			PropertyMap{"root": NewObjectProperty(PropertyMap{})},
			&PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(1)})},
		},
		{
			"Nested path element, fully removed",
			PropertyPath{"root", "nested"},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(1)})},
			PropertyMap{},
			nil,
		},
		{
			"Nested path element, nested added",
			PropertyPath{"root", "nested"},
			PropertyMap{},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(2)})},
			&PropertyMap{"root": NewObjectProperty(PropertyMap{})},
		},
		{
			"Nested path element, nested removed",
			PropertyPath{"root", "nested"},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(1)})},
			PropertyMap{},
			nil,
		},
		{
			"Array index",
			PropertyPath{"array", 0},
			PropertyMap{"array": NewArrayProperty([]PropertyValue{NewNumberProperty(1)})},
			PropertyMap{"array": NewArrayProperty([]PropertyValue{NewNumberProperty(2)})},
			&PropertyMap{"array": NewArrayProperty([]PropertyValue{NewNumberProperty(1)})},
		},
		{
			"Array index, along path",
			PropertyPath{"array", 0, "item"},
			PropertyMap{"array": NewArrayProperty([]PropertyValue{
				NewObjectProperty(PropertyMap{"item": NewNumberProperty(1)}),
			})},
			PropertyMap{"array": NewArrayProperty([]PropertyValue{
				NewObjectProperty(PropertyMap{"item": NewNumberProperty(2)}),
			})},
			&PropertyMap{"array": NewArrayProperty([]PropertyValue{
				NewObjectProperty(PropertyMap{"item": NewNumberProperty(1)}),
			})},
		},
		{
			"Array index, added",
			PropertyPath{"array", 0},
			PropertyMap{"array": NewArrayProperty([]PropertyValue{})},
			PropertyMap{"array": NewArrayProperty([]PropertyValue{NewNumberProperty(2)})},
			nil,
		},
		{
			"Array index, removed",
			PropertyPath{"array", 0},
			PropertyMap{"array": NewArrayProperty([]PropertyValue{NewNumberProperty(1)})},
			PropertyMap{"array": NewArrayProperty([]PropertyValue{})},
			nil,
		},
		{
			"Single wildcard at root",
			PropertyPath{"*"},
			PropertyMap{"root": NewNumberProperty(1)},
			PropertyMap{"root": NewNumberProperty(2)},
			&PropertyMap{"root": NewNumberProperty(1)},
		},
		{
			"Wildcard followed by path element",
			PropertyPath{"*", "nested"},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(1)})},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(2)})},
			&PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(1)})},
		},
		{
			"Wildcard in array followed by path element",
			PropertyPath{"root", "*", "nested"},
			PropertyMap{"root": NewArrayProperty([]PropertyValue{
				NewObjectProperty(PropertyMap{"nested": NewNumberProperty(1)}),
				NewObjectProperty(PropertyMap{"nested": NewNumberProperty(2)}),
			})},
			PropertyMap{"root": NewArrayProperty([]PropertyValue{
				NewObjectProperty(PropertyMap{"nested": NewNumberProperty(3)}),
				NewObjectProperty(PropertyMap{"nested": NewNumberProperty(4)}),
			})},
			&PropertyMap{"root": NewArrayProperty([]PropertyValue{
				NewObjectProperty(PropertyMap{"nested": NewNumberProperty(1)}),
				NewObjectProperty(PropertyMap{"nested": NewNumberProperty(2)}),
			})},
		},
		{
			"Nested wildcard",
			PropertyPath{"root", "*"},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(1)})},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(2)})},
			&PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(1)})},
		},
		{
			"Nested wildcard in array",
			PropertyPath{"root", "*"},
			PropertyMap{"root": NewArrayProperty([]PropertyValue{
				NewNumberProperty(1),
				NewNumberProperty(2),
			})},
			PropertyMap{"root": NewArrayProperty([]PropertyValue{
				NewNumberProperty(3),
				NewNumberProperty(4),
			})},
			&PropertyMap{"root": NewArrayProperty([]PropertyValue{
				NewNumberProperty(1),
				NewNumberProperty(2),
			})},
		},
		{
			"Nested wildcard, added",
			PropertyPath{"root", "*"},
			PropertyMap{"root": NewObjectProperty(PropertyMap{})},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(2)})},
			&PropertyMap{"root": NewObjectProperty(PropertyMap{})},
		},
		{
			"Nested wildcard, removed",
			PropertyPath{"root", "*"},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(2)})},
			PropertyMap{"root": NewObjectProperty(PropertyMap{})},
			&PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(2)})},
		},
		{
			"Nested wildcard, change of type (array)",
			PropertyPath{"root", "*"},
			PropertyMap{"root": NewArrayProperty([]PropertyValue{NewNumberProperty(1)})},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(2)})},
			&PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(2)})},
		},
		{
			"Nested wildcard, change of type (object)",
			PropertyPath{"root", "*"},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(2)})},
			PropertyMap{"root": NewArrayProperty([]PropertyValue{NewNumberProperty(1)})},
			&PropertyMap{"root": NewArrayProperty([]PropertyValue{NewNumberProperty(1)})},
		},
		{
			"Nested wildcard, change of type (number)",
			PropertyPath{"root", "*"},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(2)})},
			PropertyMap{"root": NewNumberProperty(1)},
			nil,
		},
		{
			"Untraversable in old wildcard followed by path element",
			PropertyPath{"root", "*", "nested"},
			PropertyMap{"root": NewNumberProperty(1)},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(2)})},
			nil,
		},
		{
			"Untraversable in new (not an object) wildcard followed by path element",
			PropertyPath{"root", "*", "nested"},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(1)})},
			PropertyMap{"root": NewNumberProperty(2)},
			nil,
		},
		{
			"Untraversable in new (missing) wildcard followed by path element",
			PropertyPath{"root", "*", "nested"},
			PropertyMap{"root": NewObjectProperty(PropertyMap{"nested": NewNumberProperty(1)})},
			PropertyMap{"root": NewObjectProperty(PropertyMap{})},
			nil,
		},
		{
			"Nested object wildcard reset fails",
			PropertyPath{"root", "*", 0},
			PropertyMap{"root": NewObjectProperty(PropertyMap{
				"passes": NewNumberProperty(1),
				"fails":  NewArrayProperty([]PropertyValue{NewNumberProperty(1)}),
			})},
			PropertyMap{"root": NewObjectProperty(PropertyMap{
				"passes": NewNumberProperty(2),
				"fails":  NewArrayProperty([]PropertyValue{}),
			})},
			nil,
		},
		{
			"Nested array wildcard reset fails",
			PropertyPath{"root", "*", 0},
			PropertyMap{"root": NewArrayProperty([]PropertyValue{
				NewNumberProperty(1),
				NewArrayProperty([]PropertyValue{NewNumberProperty(1)}),
			})},
			PropertyMap{"root": NewArrayProperty([]PropertyValue{
				NewNumberProperty(2),
				NewArrayProperty([]PropertyValue{}),
			})},
			nil,
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
