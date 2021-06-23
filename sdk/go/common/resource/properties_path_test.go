package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPropertyPath(t *testing.T) {
	value := NewObjectProperty(NewPropertyMapFromMap(map[string]interface{}{
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

	cases := []struct {
		path   string
		parsed PropertyPath
	}{
		{
			"root",
			PropertyPath{"root"},
		},
		{
			"root.nested",
			PropertyPath{"root", "nested"},
		},
		{
			`root["nested"]`,
			PropertyPath{"root", "nested"},
		},
		{
			"root.double.nest",
			PropertyPath{"root", "double", "nest"},
		},
		{
			`root["double"].nest`,
			PropertyPath{"root", "double", "nest"},
		},
		{
			`root["double"]["nest"]`,
			PropertyPath{"root", "double", "nest"},
		},
		{
			"root.array[0]",
			PropertyPath{"root", "array", 0},
		},
		{
			"root.array[1]",
			PropertyPath{"root", "array", 1},
		},
		{
			"root.array[0].nested",
			PropertyPath{"root", "array", 0, "nested"},
		},
		{
			"root.array2[0][1].nested",
			PropertyPath{"root", "array2", 0, 1, "nested"},
		},
		{
			"root.nested.array[0].double[1]",
			PropertyPath{"root", "nested", "array", 0, "double", 1},
		},
		{
			`root["key with \"escaped\" quotes"]`,
			PropertyPath{"root", `key with "escaped" quotes`},
		},
		{
			`root["key with a ."]`,
			PropertyPath{"root", "key with a ."},
		},
		{
			`["root key with \"escaped\" quotes"].nested`,
			PropertyPath{`root key with "escaped" quotes`, "nested"},
		},
		{
			`["root key with a ."][1]`,
			PropertyPath{"root key with a .", 1},
		},
	}

	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			parsed, err := ParsePropertyPath(c.path)
			assert.NoError(t, err)
			assert.Equal(t, c.parsed, parsed)

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

	for _, c := range negativeCases {
		t.Run(c, func(t *testing.T) {
			parsed, err := ParsePropertyPath(c)
			if err == nil {
				v, ok := parsed.Get(value)
				assert.False(t, ok)
				assert.True(t, v.IsNull())
			}
		})
	}
}

func TestPropertyPathContains(t *testing.T) {
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
	// Regression test for https://github.com/pulumi/pulumi/issues/5871:
	// Ensure that adding a new element beyond the size of an array will resize it.
	path, err := ParsePropertyPath("[1]")
	assert.Nil(t, err)
	_, ok := path.Add(NewArrayProperty([]PropertyValue{}), NewNumberProperty(42))
	assert.True(t, ok)
}
