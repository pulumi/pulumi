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

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
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
		expected       *property.ObjectDiff
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
			expected: &property.ObjectDiff{
				Adds:    map[string]property.Value{},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{
					"foo": {
						Old: property.New(42.0),
						New: property.New(24.0),
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
			expected: &property.ObjectDiff{
				Adds:    map[string]property.Value{},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{
					"foo": {
						Old: property.New(42.0),
						New: property.New(42.0),
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
			expected: &property.ObjectDiff{
				Adds:    map[string]property.Value{},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{
					"foo": {
						Old: property.New(42.0),
						New: property.New(24.0),
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
			expected: &property.ObjectDiff{
				Adds:    map[string]property.Value{},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{
					"foo": {
						Old: property.New(42.0),
						New: property.New(24.0),
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
			expected: &property.ObjectDiff{
				Adds: map[string]property.Value{
					"foo": property.New(24.0),
				},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{},
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
			expected: &property.ObjectDiff{
				Adds: map[string]property.Value{},
				Deletes: map[string]property.Value{
					"foo": property.New(24.0),
				},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{},
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
			expected: &property.ObjectDiff{
				Adds: map[string]property.Value{},
				Deletes: map[string]property.Value{
					"foo": property.New(42.0),
				},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{},
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
			expected: &property.ObjectDiff{
				Adds:    map[string]property.Value{},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{
					"foo": {
						Array: &property.ArrayDiff{
							Adds:    map[int]property.Value{},
							Deletes: map[int]property.Value{},
							Sames:   map[int]property.Value{},
							Updates: map[int]property.ValueDiff{
								1: {
									Old: property.New("baz"),
									New: property.New("qux"),
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
			expected: &property.ObjectDiff{
				Adds:    map[string]property.Value{},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{
					"foo": {
						Old: resource.FromResourcePropertyValue(resource.NewPropertyValue([]any{
							"bar",
							"baz",
						})),
						New: resource.FromResourcePropertyValue(resource.NewPropertyValue([]any{
							"bar",
							"qux",
						})),
						Array: &property.ArrayDiff{
							Adds:    map[int]property.Value{},
							Deletes: map[int]property.Value{},
							Sames: map[int]property.Value{
								0: resource.FromResourcePropertyValue(resource.NewPropertyValue("bar")),
							},
							Updates: map[int]property.ValueDiff{
								1: {
									Old: property.New("baz"),
									New: property.New("qux"),
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
			expected: &property.ObjectDiff{
				Adds:    map[string]property.Value{},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{
					"foo": {
						Array: &property.ArrayDiff{
							Adds: map[int]property.Value{
								1: property.New("baz"),
							},
							Deletes: map[int]property.Value{},
							Sames:   map[int]property.Value{},
							Updates: map[int]property.ValueDiff{},
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
			expected: &property.ObjectDiff{
				Adds:    map[string]property.Value{},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{
					"foo": {
						Array: &property.ArrayDiff{
							Adds: map[int]property.Value{},
							Deletes: map[int]property.Value{
								1: property.New("baz"),
							},
							Sames:   map[int]property.Value{},
							Updates: map[int]property.ValueDiff{},
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
			expected: &property.ObjectDiff{
				Adds:    map[string]property.Value{},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{
					"foo": {
						Array: &property.ArrayDiff{
							Adds:    map[int]property.Value{},
							Deletes: map[int]property.Value{},
							Sames:   map[int]property.Value{},
							Updates: map[int]property.ValueDiff{
								100: {
									Old: property.Value{},
									New: property.Value{},
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
			expected: &property.ObjectDiff{
				Adds:    map[string]property.Value{},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{
					"foo": {
						Array: &property.ArrayDiff{
							Adds:    map[int]property.Value{},
							Deletes: map[int]property.Value{},
							Sames:   map[int]property.Value{},
							Updates: map[int]property.ValueDiff{
								100: {
									Array: &property.ArrayDiff{
										Adds:    map[int]property.Value{},
										Deletes: map[int]property.Value{},
										Sames:   map[int]property.Value{},
										Updates: map[int]property.ValueDiff{
											200: {
												Old: property.Value{},
												New: property.Value{},
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
			expected: &property.ObjectDiff{
				Adds:    map[string]property.Value{},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{
					"foo": {
						Array: &property.ArrayDiff{
							Adds: map[int]property.Value{},
							Deletes: map[int]property.Value{
								0: property.New(map[string]property.Value{
									"baz": property.New(42.0),
								}),
							},
							Sames:   map[int]property.Value{},
							Updates: map[int]property.ValueDiff{},
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
			expected: &property.ObjectDiff{
				Adds:    map[string]property.Value{},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{
					"foo": {
						Object: &property.ObjectDiff{
							Adds:    map[string]property.Value{},
							Deletes: map[string]property.Value{},
							Sames:   map[string]property.Value{},
							Updates: map[string]property.ValueDiff{
								"qux": {
									Old: property.New("zed"),
									New: property.New("alpha"),
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
			expected: &property.ObjectDiff{
				Adds:    map[string]property.Value{},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{
					"foo": {
						Old: resource.FromResourcePropertyValue(resource.NewPropertyValue(map[string]any{
							"bar": "baz",
							"qux": "zed",
						})),
						New: resource.FromResourcePropertyValue(resource.NewPropertyValue(map[string]any{
							"bar": "baz",
							"qux": "alpha",
						})),
						Object: &property.ObjectDiff{
							Adds:    map[string]property.Value{},
							Deletes: map[string]property.Value{},
							Sames: map[string]property.Value{
								"bar": resource.FromResourcePropertyValue(resource.NewPropertyValue("baz")),
							},
							Updates: map[string]property.ValueDiff{
								"qux": {
									Old: property.New("zed"),
									New: property.New("alpha"),
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
			expected: &property.ObjectDiff{
				Adds:    map[string]property.Value{},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{
					"foo": {
						Object: &property.ObjectDiff{
							Adds: map[string]property.Value{
								"qux": property.New("alpha"),
							},
							Deletes: map[string]property.Value{},
							Sames:   map[string]property.Value{},
							Updates: map[string]property.ValueDiff{},
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
			expected: &property.ObjectDiff{
				Adds:    map[string]property.Value{},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{
					"foo": {
						Object: &property.ObjectDiff{
							Adds: map[string]property.Value{},
							Deletes: map[string]property.Value{
								"qux": property.New("zed"),
							},
							Sames:   map[string]property.Value{},
							Updates: map[string]property.ValueDiff{},
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
			expected: &property.ObjectDiff{
				Adds:    map[string]property.Value{},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{
					"foo": {
						Object: &property.ObjectDiff{
							Adds:    map[string]property.Value{},
							Deletes: map[string]property.Value{},
							Sames:   map[string]property.Value{},
							Updates: map[string]property.ValueDiff{
								"missing": {
									Old: property.Value{},
									New: property.Value{},
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
			expected: &property.ObjectDiff{
				Adds:    map[string]property.Value{},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{
					"foo": {
						Object: &property.ObjectDiff{
							Adds:    map[string]property.Value{},
							Deletes: map[string]property.Value{},
							Sames:   map[string]property.Value{},
							Updates: map[string]property.ValueDiff{
								"nested": {
									Object: &property.ObjectDiff{
										Adds:    map[string]property.Value{},
										Deletes: map[string]property.Value{},
										Sames:   map[string]property.Value{},
										Updates: map[string]property.ValueDiff{
											"missing": {
												Old: property.Value{},
												New: property.Value{},
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
			expected: &property.ObjectDiff{
				Adds:    map[string]property.Value{},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{
					"foo": {
						Array: &property.ArrayDiff{
							Adds: map[int]property.Value{},
							Deletes: map[int]property.Value{
								0: property.New(map[string]property.Value{
									"baz": property.New(42.0),
								}),
							},
							Sames:   map[int]property.Value{},
							Updates: map[int]property.ValueDiff{},
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
			expected: &property.ObjectDiff{
				Adds:    map[string]property.Value{},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{
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
			expected: &property.ObjectDiff{
				Adds:    map[string]property.Value{},
				Deletes: map[string]property.Value{},
				Sames:   map[string]property.Value{},
				Updates: map[string]property.ValueDiff{
					"fizz": {Object: &property.ObjectDiff{
						Adds:    map[string]property.Value{},
						Deletes: map[string]property.Value{},
						Sames:   map[string]property.Value{},
						Updates: map[string]property.ValueDiff{
							"bar": {},
						},
					}},
					"fizzbuzz": {Object: &property.ObjectDiff{
						Adds:    map[string]property.Value{},
						Deletes: map[string]property.Value{},
						Sames:   map[string]property.Value{},
						Updates: map[string]property.ValueDiff{
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
			diff, hiddenProperties := TranslateDetailedDiff(&StepEventMetadata{
				Old: &StepEventStateMetadata{
					Inputs:  resource.FromResourcePropertyMap(oldInputs),
					Outputs: resource.FromResourcePropertyMap(state),
				},
				New: &StepEventStateMetadata{
					Inputs:    resource.FromResourcePropertyMap(inputs),
					HideDiffs: c.hideDiff,
				},
				DetailedDiff: c.detailedDiff,
			}, false)
			assert.Equal(t, c.expected, diff)
			assert.ElementsMatch(t, c.expectedHidden, hiddenProperties)
		})
	}
}

// Regression test for https://github.com/pulumi/pulumi/issues/23067: a detailed diff may name a property that the
// engine has no value for, and rendering the missing value as a null misreports what is being added.
func TestTranslateDetailedDiffAbsentValue(t *testing.T) {
	t.Parallel()

	oldOutputs := property.NewMap(map[string]property.Value{"forceDestroy": property.New(property.Null)})

	diff, _ := TranslateDetailedDiff(&StepEventMetadata{
		Old: &StepEventStateMetadata{
			Inputs:  property.Map{},
			Outputs: oldOutputs,
		},
		New:          &StepEventStateMetadata{Inputs: property.Map{}},
		DetailedDiff: map[string]plugin.PropertyDiff{"forceDestroy": {Kind: plugin.DiffAdd}},
	}, false)

	assert.Equal(t, map[string]property.Value{
		"forceDestroy": property.New(property.Computed),
	}, diff.Adds)
}

// Refresh diffs are computed over inputs, so the new value must be read from the new inputs rather than the new
// outputs.
func TestTranslateDetailedDiffRefreshInputDiff(t *testing.T) {
	t.Parallel()

	diff, _ := TranslateDetailedDiff(&StepEventMetadata{
		Old: &StepEventStateMetadata{Inputs: property.Map{}, Outputs: property.Map{}},
		New: &StepEventStateMetadata{
			Inputs:  property.NewMap(map[string]property.Value{"oof": property.New("rab")}),
			Outputs: property.NewMap(map[string]property.Value{"foo": property.New("bar")}),
		},
		DetailedDiff: map[string]plugin.PropertyDiff{"oof": {Kind: plugin.DiffAdd, InputDiff: true}},
	}, true /* refresh */)

	assert.Equal(t, map[string]property.Value{"oof": property.New("rab")}, diff.Adds)
}
