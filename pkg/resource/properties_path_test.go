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
