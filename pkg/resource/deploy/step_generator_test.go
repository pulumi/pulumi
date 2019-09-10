package deploy

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/stretchr/testify/assert"
)

func TestIgnoreChanges(t *testing.T) {
	cases := []struct {
		name          string
		oldInputs     map[string]interface{}
		newInputs     map[string]interface{}
		expected      map[string]interface{}
		ignoreChanges []string
		expectFailure bool
	}{
		{
			name: "Present in old and new sets",
			oldInputs: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
				},
			},
			newInputs: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "bar",
				},
				"c": 42,
			},
			expected: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
				},
				"c": 42,
			},
			ignoreChanges: []string{"a.b"},
		},
		{
			name: "Missing in new sets",
			oldInputs: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
				},
			},
			newInputs: map[string]interface{}{
				"a": map[string]interface{}{},
				"c": 42,
			},
			expected: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
				},
				"c": 42,
			},
			ignoreChanges: []string{"a.b"},
		},
		{
			name:      "Missing in old deletes",
			oldInputs: map[string]interface{}{},
			newInputs: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
				},
				"c": 42,
			},
			expected: map[string]interface{}{
				"a": map[string]interface{}{},
				"c": 42,
			},
			ignoreChanges: []string{"a.b"},
		},
		{
			name:      "Missing keys in old and new are OK",
			oldInputs: map[string]interface{}{},
			newInputs: map[string]interface{}{},
			ignoreChanges: []string{
				"a",
				"a.b",
				"a.c[0]",
			},
		},
		{
			name: "Missing parent keys in only new fail",
			oldInputs: map[string]interface{}{
				"a": map[string]interface{}{
					"b": "foo",
				},
			},
			newInputs:     map[string]interface{}{},
			ignoreChanges: []string{"a.b"},
			expectFailure: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			olds, news := resource.NewPropertyMapFromMap(c.oldInputs), resource.NewPropertyMapFromMap(c.newInputs)

			expected := olds
			if c.expected != nil {
				expected = resource.NewPropertyMapFromMap(c.expected)
			}

			processed, res := processIgnoreChanges(news, olds, c.ignoreChanges)
			if c.expectFailure {
				assert.NotNil(t, res)
			} else {
				assert.Nil(t, res)
				assert.Equal(t, expected, processed)
			}
		})
	}
}

func TestStables(t *testing.T) {
	oldState := map[string]interface{}{
		"foo": "oof",
		"bar": map[string]interface{}{
			"baz": "zab",
			"qux": map[string]interface{}{
				"wah": "haw",
			},
			"zed": []interface{}{
				42,
				map[string]interface{}{
					"foo": "bar",
					"baz": "qux",
				},
			},
		},
		"baz": []interface{}{
			"alpha",
			[]interface{}{"beta"},
			map[string]interface{}{
				"gamma": "delta",
				"eta":   "theta",
			},
			"iota",
		},
	}

	computed := resource.Computed{Element: resource.NewStringProperty("")}

	cases := []struct {
		name     string
		expected map[string]interface{}
		stables  []string
	}{
		{
			name:     "All top-level properties",
			expected: oldState,
			stables:  []string{"foo", "bar", "baz"},
		},
		{
			name:     "No top-level properties",
			expected: map[string]interface{}{},
			stables:  []string{},
		},
		{
			name: "Top-level primitive",
			expected: map[string]interface{}{
				"foo": "oof",
			},
			stables: []string{"foo"},
		},
		{
			name: "Top-level map",
			expected: map[string]interface{}{
				"bar": map[string]interface{}{
					"baz": "zab",
					"qux": map[string]interface{}{
						"wah": "haw",
					},
					"zed": []interface{}{
						42,
						map[string]interface{}{
							"foo": "bar",
							"baz": "qux",
						},
					},
				},
			},
			stables: []string{"bar"},
		},
		{
			name: "Top-level array",
			expected: map[string]interface{}{
				"baz": []interface{}{
					"alpha",
					[]interface{}{"beta"},
					map[string]interface{}{
						"gamma": "delta",
						"eta":   "theta",
					},
					"iota",
				},
			},
			stables: []string{"baz"},
		},
		{
			name: "primitive in map",
			expected: map[string]interface{}{
				"bar": map[string]interface{}{
					"baz": "zab",
					"qux": computed,
					"zed": computed,
				},
			},
			stables: []string{"bar.baz"},
		},
		{
			name: "map in map",
			expected: map[string]interface{}{
				"bar": map[string]interface{}{
					"baz": computed,
					"qux": map[string]interface{}{
						"wah": "haw",
					},
					"zed": computed,
				},
			},
			stables: []string{"bar.qux"},
		},
		{
			name: "primitive in map in map",
			expected: map[string]interface{}{
				"bar": map[string]interface{}{
					"baz": computed,
					"qux": map[string]interface{}{
						"wah": "haw",
					},
					"zed": computed,
				},
			},
			stables: []string{"bar.qux.wah"},
		},
		{
			name: "array in map",
			expected: map[string]interface{}{
				"bar": map[string]interface{}{
					"baz": computed,
					"qux": computed,
					"zed": []interface{}{
						42,
						map[string]interface{}{
							"foo": "bar",
							"baz": "qux",
						},
					},
				},
			},
			stables: []string{"bar.zed"},
		},
		{
			name: "primitive in array in map",
			expected: map[string]interface{}{
				"bar": map[string]interface{}{
					"baz": computed,
					"qux": computed,
					"zed": []interface{}{
						42,
						computed,
					},
				},
			},
			stables: []string{"bar.zed[0]"},
		},
		{
			name: "map in array in map",
			expected: map[string]interface{}{
				"bar": map[string]interface{}{
					"baz": computed,
					"qux": computed,
					"zed": []interface{}{
						computed,
						map[string]interface{}{
							"foo": "bar",
							"baz": "qux",
						},
					},
				},
			},
			stables: []string{"bar.zed[1]"},
		},
		{
			name: "multuple elements in map",
			expected: map[string]interface{}{
				"bar": map[string]interface{}{
					"baz": "zab",
					"qux": computed,
					"zed": []interface{}{
						42,
						map[string]interface{}{
							"foo": "bar",
							"baz": "qux",
						},
					},
				},
			},
			stables: []string{"bar.baz", "bar.zed"},
		},
		{
			name: "first element in array",
			expected: map[string]interface{}{
				"baz": []interface{}{
					"alpha",
					computed,
					computed,
					computed,
				},
			},
			stables: []string{"baz[0]"},
		},
		{
			name: "last element in array",
			expected: map[string]interface{}{
				"baz": []interface{}{
					computed,
					computed,
					computed,
					"iota",
				},
			},
			stables: []string{"baz[3]"},
		},
		{
			name: "first and last elements in array",
			expected: map[string]interface{}{
				"baz": []interface{}{
					"alpha",
					computed,
					computed,
					"iota",
				},
			},
			stables: []string{"baz[0]", "baz[3]"},
		},
		{
			name: "array in array",
			expected: map[string]interface{}{
				"baz": []interface{}{
					computed,
					[]interface{}{"beta"},
					computed,
					computed,
				},
			},
			stables: []string{"baz[1]"},
		},
		{
			name: "primitive in array in array",
			expected: map[string]interface{}{
				"baz": []interface{}{
					computed,
					[]interface{}{"beta"},
					computed,
					computed,
				},
			},
			stables: []string{"baz[1][0]"},
		},
		{
			name: "map in array",
			expected: map[string]interface{}{
				"baz": []interface{}{
					computed,
					computed,
					map[string]interface{}{
						"gamma": "delta",
						"eta":   "theta",
					},
					computed,
				},
			},
			stables: []string{"baz[2]"},
		},
		{
			name: "primitive in map in array",
			expected: map[string]interface{}{
				"baz": []interface{}{
					computed,
					computed,
					map[string]interface{}{
						"gamma": "delta",
						"eta":   computed,
					},
					computed,
				},
			},
			stables: []string{"baz[2].gamma"},
		},
		{
			name: "mix of nested properties",
			expected: map[string]interface{}{
				"foo": "oof",
				"bar": map[string]interface{}{
					"baz": "zab",
					"qux": computed,
					"zed": []interface{}{
						computed,
						map[string]interface{}{
							"foo": "bar",
							"baz": computed,
						},
					},
				},
				"baz": []interface{}{
					"alpha",
					computed,
					map[string]interface{}{
						"gamma": computed,
						"eta":   "theta",
					},
					computed,
				},
			},
			stables: []string{
				"foo",
				"bar.baz",
				"bar.zed[1].foo",
				"baz[0]",
				"baz[2].eta",
			},
		},
		{
			name:     "missing top-level property",
			expected: map[string]interface{}{},
			stables:  []string{"oof"},
		},
		{
			name: "missing nested property",
			expected: map[string]interface{}{
				"bar": map[string]interface{}{
					"baz": computed,
					"qux": computed,
					"zed": computed,
				},
			},
			stables: []string{"bar.zab"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			old := resource.NewPropertyMapFromMap(oldState)
			expected := resource.NewPropertyMapFromMap(c.expected)
			processed := processStables(old, c.stables)
			assert.Equal(t, expected, processed)
		})
	}
}
