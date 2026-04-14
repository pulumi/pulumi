// Copyright 2019, Pulumi Corporation.
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

package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

func TestTranslateDetailedDiff(t *testing.T) {
	t.Parallel()

	var (
		A = plugin.PropertyDiff{Kind: plugin.DiffAdd}
		D = plugin.PropertyDiff{Kind: plugin.DiffDelete}
		U = plugin.PropertyDiff{Kind: plugin.DiffUpdate}
	)

	cases := []struct {
		state          map[string]any
		oldInputs      map[string]any
		inputs         map[string]any
		detailedDiff   map[string]plugin.PropertyDiff
		expected       *resource.ObjectDiff
		hideDiff       []resource.PropertyPath
		expectedHidden []resource.PropertyPath
	}{
		{
			state: map[string]any{
				"foo": 42,
			},
			inputs: map[string]any{
				"foo": 24,
			},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo": U,
			},
			expected: &resource.ObjectDiff{
				Adds:    resource.PropertyMap{},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{
					"foo": {
						Old: resource.NewProperty(42.0),
						New: resource.NewProperty(24.0),
					},
				},
			},
		},
		{
			state: map[string]any{
				"foo": 42,
			},
			inputs: map[string]any{
				"foo": 42,
			},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo": U,
			},
			expected: &resource.ObjectDiff{
				Adds:    resource.PropertyMap{},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{
					"foo": {
						Old: resource.NewProperty(42.0),
						New: resource.NewProperty(42.0),
					},
				},
			},
		},
		{
			state: map[string]any{
				"foo": 42,
				"bar": "hello",
			},
			inputs: map[string]any{
				"foo": 24,
				"bar": "hello",
			},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo": U,
			},
			expected: &resource.ObjectDiff{
				Adds:    resource.PropertyMap{},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{
					"foo": {
						Old: resource.NewProperty(42.0),
						New: resource.NewProperty(24.0),
					},
				},
			},
		},
		{
			state: map[string]any{
				"foo": 42,
				"bar": "hello",
			},
			inputs: map[string]any{
				"foo": 24,
				"bar": "world",
			},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo": U,
			},
			expected: &resource.ObjectDiff{
				Adds:    resource.PropertyMap{},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{
					"foo": {
						Old: resource.NewProperty(42.0),
						New: resource.NewProperty(24.0),
					},
				},
			},
		},
		{
			state: map[string]any{},
			inputs: map[string]any{
				"foo": 24,
			},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo": A,
			},
			expected: &resource.ObjectDiff{
				Adds: resource.PropertyMap{
					"foo": resource.NewProperty(24.0),
				},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{},
			},
		},
		{
			state: map[string]any{
				"foo": 24,
			},
			inputs: map[string]any{},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo": D,
			},
			expected: &resource.ObjectDiff{
				Adds: resource.PropertyMap{},
				Deletes: resource.PropertyMap{
					"foo": resource.NewProperty(24.0),
				},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{},
			},
		},
		{
			state: map[string]any{
				"foo": 24,
			},
			oldInputs: map[string]any{
				"foo": 42,
			},
			inputs: map[string]any{},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo": {
					Kind:      plugin.DiffDelete,
					InputDiff: true,
				},
			},
			expected: &resource.ObjectDiff{
				Adds: resource.PropertyMap{},
				Deletes: resource.PropertyMap{
					"foo": resource.NewProperty(42.0),
				},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{},
			},
		},
		{
			state: map[string]any{
				"foo": []any{
					"bar",
					"baz",
				},
			},
			inputs: map[string]any{
				"foo": []any{
					"bar",
					"qux",
				},
			},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo[1]": U,
			},
			expected: &resource.ObjectDiff{
				Adds:    resource.PropertyMap{},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{
					"foo": {
						Array: &resource.ArrayDiff{
							Adds:    map[int]resource.PropertyValue{},
							Deletes: map[int]resource.PropertyValue{},
							Sames:   map[int]resource.PropertyValue{},
							Updates: map[int]resource.ValueDiff{
								1: {
									Old: resource.NewProperty("baz"),
									New: resource.NewProperty("qux"),
								},
							},
						},
					},
				},
			},
		},
		{
			state: map[string]any{
				"foo": []any{
					"bar",
					"baz",
				},
			},
			inputs: map[string]any{
				"foo": []any{
					"bar",
					"qux",
				},
			},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo": U,
			},
			expected: &resource.ObjectDiff{
				Adds:    resource.PropertyMap{},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{
					"foo": {
						Old: resource.NewPropertyValue([]any{
							"bar",
							"baz",
						}),
						New: resource.NewPropertyValue([]any{
							"bar",
							"qux",
						}),
						Array: &resource.ArrayDiff{
							Adds:    map[int]resource.PropertyValue{},
							Deletes: map[int]resource.PropertyValue{},
							Sames: map[int]resource.PropertyValue{
								0: resource.NewPropertyValue("bar"),
							},
							Updates: map[int]resource.ValueDiff{
								1: {
									Old: resource.NewProperty("baz"),
									New: resource.NewProperty("qux"),
								},
							},
						},
					},
				},
			},
		},

		{
			state: map[string]any{
				"foo": []any{
					"bar",
				},
			},
			inputs: map[string]any{
				"foo": []any{
					"bar",
					"baz",
				},
			},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo[1]": A,
			},
			expected: &resource.ObjectDiff{
				Adds:    resource.PropertyMap{},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{
					"foo": {
						Array: &resource.ArrayDiff{
							Adds: map[int]resource.PropertyValue{
								1: resource.NewProperty("baz"),
							},
							Deletes: map[int]resource.PropertyValue{},
							Sames:   map[int]resource.PropertyValue{},
							Updates: map[int]resource.ValueDiff{},
						},
					},
				},
			},
		},
		{
			state: map[string]any{
				"foo": []any{
					"bar",
					"baz",
				},
			},
			inputs: map[string]any{
				"foo": []any{
					"bar",
				},
			},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo[1]": D,
			},
			expected: &resource.ObjectDiff{
				Adds:    resource.PropertyMap{},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{
					"foo": {
						Array: &resource.ArrayDiff{
							Adds: map[int]resource.PropertyValue{},
							Deletes: map[int]resource.PropertyValue{
								1: resource.NewProperty("baz"),
							},
							Sames:   map[int]resource.PropertyValue{},
							Updates: map[int]resource.ValueDiff{},
						},
					},
				},
			},
		},
		{
			state: map[string]any{
				"foo": []any{
					"bar",
					"baz",
				},
			},
			inputs: map[string]any{
				"foo": []any{
					"bar",
					"qux",
				},
			},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo[100]": U,
			},
			expected: &resource.ObjectDiff{
				Adds:    resource.PropertyMap{},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{
					"foo": {
						Array: &resource.ArrayDiff{
							Adds:    map[int]resource.PropertyValue{},
							Deletes: map[int]resource.PropertyValue{},
							Sames:   map[int]resource.PropertyValue{},
							Updates: map[int]resource.ValueDiff{
								100: {
									Old: resource.PropertyValue{},
									New: resource.PropertyValue{},
								},
							},
						},
					},
				},
			},
		},
		{
			state: map[string]any{
				"foo": []any{
					"bar",
					"baz",
				},
			},
			inputs: map[string]any{
				"foo": []any{
					"bar",
					"qux",
				},
			},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo[100][200]": U,
			},
			expected: &resource.ObjectDiff{
				Adds:    resource.PropertyMap{},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{
					"foo": {
						Array: &resource.ArrayDiff{
							Adds:    map[int]resource.PropertyValue{},
							Deletes: map[int]resource.PropertyValue{},
							Sames:   map[int]resource.PropertyValue{},
							Updates: map[int]resource.ValueDiff{
								100: {
									Array: &resource.ArrayDiff{
										Adds:    map[int]resource.PropertyValue{},
										Deletes: map[int]resource.PropertyValue{},
										Sames:   map[int]resource.PropertyValue{},
										Updates: map[int]resource.ValueDiff{
											200: {
												Old: resource.PropertyValue{},
												New: resource.PropertyValue{},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			state: map[string]any{
				"foo": []any{
					map[string]any{
						"baz": 42,
					},
				},
			},
			inputs: map[string]any{
				"foo": []any{},
			},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo[0].baz": D,
			},
			expected: &resource.ObjectDiff{
				Adds:    resource.PropertyMap{},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{
					"foo": {
						Array: &resource.ArrayDiff{
							Adds: map[int]resource.PropertyValue{},
							Deletes: map[int]resource.PropertyValue{
								0: resource.NewProperty(resource.PropertyMap{
									"baz": resource.NewProperty(42.0),
								}),
							},
							Sames:   map[int]resource.PropertyValue{},
							Updates: map[int]resource.ValueDiff{},
						},
					},
				},
			},
		},
		{
			state: map[string]any{
				"foo": map[string]any{
					"bar": "baz",
					"qux": "zed",
				},
			},
			inputs: map[string]any{
				"foo": map[string]any{
					"bar": "baz",
					"qux": "alpha",
				},
			},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo.qux": U,
			},
			expected: &resource.ObjectDiff{
				Adds:    resource.PropertyMap{},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{
					"foo": {
						Object: &resource.ObjectDiff{
							Adds:    resource.PropertyMap{},
							Deletes: resource.PropertyMap{},
							Sames:   resource.PropertyMap{},
							Updates: map[resource.PropertyKey]resource.ValueDiff{
								"qux": {
									Old: resource.NewProperty("zed"),
									New: resource.NewProperty("alpha"),
								},
							},
						},
					},
				},
			},
		},
		{
			state: map[string]any{
				"foo": map[string]any{
					"bar": "baz",
					"qux": "zed",
				},
			},
			inputs: map[string]any{
				"foo": map[string]any{
					"bar": "baz",
					"qux": "alpha",
				},
			},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo": U,
			},
			expected: &resource.ObjectDiff{
				Adds:    resource.PropertyMap{},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{
					"foo": {
						Old: resource.NewPropertyValue(map[string]any{
							"bar": "baz",
							"qux": "zed",
						}),
						New: resource.NewPropertyValue(map[string]any{
							"bar": "baz",
							"qux": "alpha",
						}),
						Object: &resource.ObjectDiff{
							Adds:    resource.PropertyMap{},
							Deletes: resource.PropertyMap{},
							Sames: resource.PropertyMap{
								"bar": resource.NewPropertyValue("baz"),
							},
							Updates: map[resource.PropertyKey]resource.ValueDiff{
								"qux": {
									Old: resource.NewProperty("zed"),
									New: resource.NewProperty("alpha"),
								},
							},
						},
					},
				},
			},
		},
		{
			state: map[string]any{
				"foo": map[string]any{
					"bar": "baz",
				},
			},
			inputs: map[string]any{
				"foo": map[string]any{
					"bar": "baz",
					"qux": "alpha",
				},
			},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo.qux": A,
			},
			expected: &resource.ObjectDiff{
				Adds:    resource.PropertyMap{},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{
					"foo": {
						Object: &resource.ObjectDiff{
							Adds: resource.PropertyMap{
								"qux": resource.NewProperty("alpha"),
							},
							Deletes: resource.PropertyMap{},
							Sames:   resource.PropertyMap{},
							Updates: map[resource.PropertyKey]resource.ValueDiff{},
						},
					},
				},
			},
		},
		{
			state: map[string]any{
				"foo": map[string]any{
					"bar": "baz",
					"qux": "zed",
				},
			},
			inputs: map[string]any{
				"foo": map[string]any{
					"bar": "baz",
				},
			},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo.qux": D,
			},
			expected: &resource.ObjectDiff{
				Adds:    resource.PropertyMap{},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{
					"foo": {
						Object: &resource.ObjectDiff{
							Adds: resource.PropertyMap{},
							Deletes: resource.PropertyMap{
								"qux": resource.NewProperty("zed"),
							},
							Sames:   resource.PropertyMap{},
							Updates: map[resource.PropertyKey]resource.ValueDiff{},
						},
					},
				},
			},
		},
		{
			state: map[string]any{
				"foo": map[string]any{
					"bar": "baz",
					"qux": "zed",
				},
			},
			inputs: map[string]any{
				"foo": map[string]any{
					"bar": "baz",
					"qux": "alpha",
				},
			},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo.missing": U,
			},
			expected: &resource.ObjectDiff{
				Adds:    resource.PropertyMap{},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{
					"foo": {
						Object: &resource.ObjectDiff{
							Adds:    resource.PropertyMap{},
							Deletes: resource.PropertyMap{},
							Sames:   resource.PropertyMap{},
							Updates: map[resource.PropertyKey]resource.ValueDiff{
								"missing": {
									Old: resource.PropertyValue{},
									New: resource.PropertyValue{},
								},
							},
						},
					},
				},
			},
		},
		{
			state: map[string]any{
				"foo": map[string]any{
					"bar": "baz",
					"qux": "zed",
				},
			},
			inputs: map[string]any{
				"foo": map[string]any{
					"bar": "baz",
					"qux": "alpha",
				},
			},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo.nested.missing": U,
			},
			expected: &resource.ObjectDiff{
				Adds:    resource.PropertyMap{},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{
					"foo": {
						Object: &resource.ObjectDiff{
							Adds:    resource.PropertyMap{},
							Deletes: resource.PropertyMap{},
							Sames:   resource.PropertyMap{},
							Updates: map[resource.PropertyKey]resource.ValueDiff{
								"nested": {
									Object: &resource.ObjectDiff{
										Adds:    resource.PropertyMap{},
										Deletes: resource.PropertyMap{},
										Sames:   resource.PropertyMap{},
										Updates: map[resource.PropertyKey]resource.ValueDiff{
											"missing": {
												Old: resource.PropertyValue{},
												New: resource.PropertyValue{},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			state: map[string]any{
				"foo": []any{
					map[string]any{
						"baz": 42,
					},
				},
			},
			inputs: map[string]any{
				"foo": []any{},
			},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo[0].baz": D,
			},
			expected: &resource.ObjectDiff{
				Adds:    resource.PropertyMap{},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{
					"foo": {
						Array: &resource.ArrayDiff{
							Adds: map[int]resource.PropertyValue{},
							Deletes: map[int]resource.PropertyValue{
								0: resource.NewProperty(resource.PropertyMap{
									"baz": resource.NewProperty(42.0),
								}),
							},
							Sames:   map[int]resource.PropertyValue{},
							Updates: map[int]resource.ValueDiff{},
						},
					},
				},
			},
		},
		{
			state:  map[string]any{},
			inputs: map[string]any{},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo[something.wonky]probably/miscalculated.by.provider": U,
			},
			expected: &resource.ObjectDiff{
				Adds:    resource.PropertyMap{},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{
					"foo[something.wonky]probably/miscalculated.by.provider": {},
				},
			},
		},
		{
			state:  map[string]any{},
			inputs: map[string]any{},
			detailedDiff: map[string]plugin.PropertyDiff{
				"foo":            U, // Should be hidden by "foo"
				"foo.bar":        U, // Should be hidden by "foo"
				"fizz.bar":       U,
				"fizzbuzz.bar":   U, // Should be hidden by "fizzbuzz.bar"
				"fizzbuzz.other": U,
			},
			hideDiff: []resource.PropertyPath{
				{"foo"},
				{"fizzbuzz", "bar"},
				{"not", "updated"},
			},
			expected: &resource.ObjectDiff{
				Adds:    resource.PropertyMap{},
				Deletes: resource.PropertyMap{},
				Sames:   resource.PropertyMap{},
				Updates: map[resource.PropertyKey]resource.ValueDiff{
					"fizz": {Object: &resource.ObjectDiff{
						Adds:    resource.PropertyMap{},
						Deletes: resource.PropertyMap{},
						Sames:   resource.PropertyMap{},
						Updates: map[resource.PropertyKey]resource.ValueDiff{
							"bar": {},
						},
					}},
					"fizzbuzz": {Object: &resource.ObjectDiff{
						Adds:    resource.PropertyMap{},
						Deletes: resource.PropertyMap{},
						Sames:   resource.PropertyMap{},
						Updates: map[resource.PropertyKey]resource.ValueDiff{
							"other": {},
						},
					}},
				},
			},
			expectedHidden: []resource.PropertyPath{
				{"foo"},
				{"fizzbuzz", "bar"},
			},
		},
	}

	for _, c := range cases {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			oldInputs := resource.NewPropertyMapFromMap(c.oldInputs)
			state := resource.NewPropertyMapFromMap(c.state)
			inputs := resource.NewPropertyMapFromMap(c.inputs)
			diff, hiddenProperties, _ := TranslateDetailedDiff(&StepEventMetadata{
				Old:          &StepEventStateMetadata{Inputs: oldInputs, Outputs: state},
				New:          &StepEventStateMetadata{Inputs: inputs, HideDiffs: c.hideDiff},
				DetailedDiff: c.detailedDiff,
			}, false)
			assert.Equal(t, c.expected, diff)
			assert.ElementsMatch(t, c.expectedHidden, hiddenProperties)
		})
	}
}

func TestTranslateDetailedDiffReplacePaths(t *testing.T) {
	t.Parallel()

	state := resource.NewPropertyMapFromMap(map[string]any{
		"region": "us-east-1",
		"size":   "t2.micro",
		"tags":   map[string]any{"env": "prod"},
	})
	inputs := resource.NewPropertyMapFromMap(map[string]any{
		"region": "eu-west-1",
		"size":   "t2.large",
		"tags":   map[string]any{"env": "staging"},
	})

	t.Run("no replace diffs returns empty replacePaths", func(t *testing.T) {
		t.Parallel()
		_, _, replacePaths := TranslateDetailedDiff(&StepEventMetadata{
			Old: &StepEventStateMetadata{Outputs: state},
			New: &StepEventStateMetadata{Inputs: inputs},
			DetailedDiff: map[string]plugin.PropertyDiff{
				"size": {Kind: plugin.DiffUpdate},
			},
		}, false)
		assert.Empty(t, replacePaths)
	})

	t.Run("UPDATE_REPLACE returns that path", func(t *testing.T) {
		t.Parallel()
		_, _, replacePaths := TranslateDetailedDiff(&StepEventMetadata{
			Old: &StepEventStateMetadata{Outputs: state},
			New: &StepEventStateMetadata{Inputs: inputs},
			DetailedDiff: map[string]plugin.PropertyDiff{
				"region": {Kind: plugin.DiffUpdateReplace},
			},
		}, false)
		assert.Equal(t, []resource.PropertyPath{{"region"}}, replacePaths)
	})

	t.Run("ADD_REPLACE returns that path", func(t *testing.T) {
		t.Parallel()
		_, _, replacePaths := TranslateDetailedDiff(&StepEventMetadata{
			Old: &StepEventStateMetadata{Outputs: resource.NewPropertyMapFromMap(map[string]any{})},
			New: &StepEventStateMetadata{Inputs: inputs},
			DetailedDiff: map[string]plugin.PropertyDiff{
				"region": {Kind: plugin.DiffAddReplace},
			},
		}, false)
		assert.Equal(t, []resource.PropertyPath{{"region"}}, replacePaths)
	})

	t.Run("DELETE_REPLACE returns that path", func(t *testing.T) {
		t.Parallel()
		_, _, replacePaths := TranslateDetailedDiff(&StepEventMetadata{
			Old: &StepEventStateMetadata{Outputs: state},
			New: &StepEventStateMetadata{Inputs: resource.NewPropertyMapFromMap(map[string]any{})},
			DetailedDiff: map[string]plugin.PropertyDiff{
				"region": {Kind: plugin.DiffDeleteReplace},
			},
		}, false)
		assert.Equal(t, []resource.PropertyPath{{"region"}}, replacePaths)
	})

	t.Run("mixed UPDATE and UPDATE_REPLACE returns only REPLACE paths", func(t *testing.T) {
		t.Parallel()
		_, _, replacePaths := TranslateDetailedDiff(&StepEventMetadata{
			Old: &StepEventStateMetadata{Outputs: state},
			New: &StepEventStateMetadata{Inputs: inputs},
			DetailedDiff: map[string]plugin.PropertyDiff{
				"size":   {Kind: plugin.DiffUpdate},
				"region": {Kind: plugin.DiffUpdateReplace},
			},
		}, false)
		assert.Equal(t, []resource.PropertyPath{{"region"}}, replacePaths)
	})

	t.Run("nested path UPDATE_REPLACE returns nested path", func(t *testing.T) {
		t.Parallel()
		nestedState := resource.NewPropertyMapFromMap(map[string]any{
			"tags": map[string]any{"env": "prod"},
		})
		nestedInputs := resource.NewPropertyMapFromMap(map[string]any{
			"tags": map[string]any{"env": "staging"},
		})
		_, _, replacePaths := TranslateDetailedDiff(&StepEventMetadata{
			Old: &StepEventStateMetadata{Outputs: nestedState},
			New: &StepEventStateMetadata{Inputs: nestedInputs},
			DetailedDiff: map[string]plugin.PropertyDiff{
				"tags.env": {Kind: plugin.DiffUpdateReplace},
			},
		}, false)
		assert.Equal(t, []resource.PropertyPath{{"tags", "env"}}, replacePaths)
	})

	t.Run("duplicate replace paths are deduplicated and sorted", func(t *testing.T) {
		t.Parallel()
		_, _, replacePaths := TranslateDetailedDiff(&StepEventMetadata{
			Old: &StepEventStateMetadata{Outputs: state},
			New: &StepEventStateMetadata{Inputs: inputs},
			DetailedDiff: map[string]plugin.PropertyDiff{
				"region": {Kind: plugin.DiffUpdateReplace},
				"size":   {Kind: plugin.DiffUpdateReplace},
			},
		}, false)
		require.Len(t, replacePaths, 2)
		// Paths should be sorted
		assert.Equal(t, "region", replacePaths[0].String())
		assert.Equal(t, "size", replacePaths[1].String())
	})
}
