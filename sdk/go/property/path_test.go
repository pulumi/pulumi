// Copyright 2016-2024, Pulumi Corporation.
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

package property_test

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	t.Parallel()

	type pathFailure struct {
		found property.Value
		msg   string
	}

	tests := []struct {
		name string
		path property.Path

		from     property.Value
		expected property.Value

		failure *pathFailure
	}{
		{
			name: "map-key",
			path: property.Path{
				property.NewSegment("k"),
			},
			from: property.New(map[string]property.Value{
				"k": property.New("v"),
			}),
			expected: property.New("v"),
		},
		{
			name: "missing-key",
			path: property.Path{
				property.NewSegment("missing"),
			},
			from: property.New(map[string]property.Value{
				"k": property.New("v"),
			}),
			failure: &pathFailure{
				found: property.New(map[string]property.Value{
					"k": property.New("v"),
				}),
				msg: `missing key "missing" in map`,
			},
		},
		{
			name: "expected-map",
			path: property.Path{
				property.NewSegment("missing"),
			},
			from: property.New([]property.Value{
				property.New("v"),
			}),
			failure: &pathFailure{
				found: property.New([]property.Value{
					property.New("v"),
				}),
				msg: `expected a map, found a array`,
			},
		},
		{
			name: "array-idx",
			path: property.Path{
				property.NewSegment(1),
			},
			from: property.New([]property.Value{
				property.New("0"),
				property.New("1"),
			}),
			expected: property.New("1"),
		},
		{
			name: "expected-array",
			path: property.Path{
				property.NewSegment(0),
			},
			from: property.New("foo"),
			failure: &pathFailure{
				found: property.New("foo"),
				msg:   `expected an array, found a string`,
			},
		},
		{
			name: "array-out-of-bounds",
			path: property.Path{
				property.NewSegment(1),
			},
			from: property.New([]property.Value{
				property.New("0"),
			}),
			failure: &pathFailure{
				found: property.New([]property.Value{
					property.New("0"),
				}),
				msg: "index 1 out of bounds of an array of length 1",
			},
		},
		{
			name: "negative-array-index",
			path: property.Path{
				property.NewSegment(-1),
			},
			from: property.New([]property.Value{
				property.New("0"),
			}),
			failure: &pathFailure{
				found: property.New([]property.Value{
					property.New("0"),
				}),
				msg: "index -1 out of bounds of an array of length 1",
			},
		},
		{
			name:     "empty-path-map",
			path:     property.Path{},
			from:     property.New(map[string]property.Value{"k": property.New(true)}),
			expected: property.New(map[string]property.Value{"k": property.New(true)}),
		},
		{
			name:     "empty-path-array",
			path:     property.Path{},
			from:     property.New([]property.Value{property.New(true)}),
			expected: property.New([]property.Value{property.New(true)}),
		},
		{
			name:     "empty-path-primitive",
			path:     property.Path{},
			from:     property.New(true),
			expected: property.New(true),
		},
		{
			name: "nested-access",
			path: property.Path{
				property.NewSegment("l1"),
				property.NewSegment(0),
				property.NewSegment("n1"),
			},
			from: property.New(map[string]property.Value{
				"l0": property.New("l0-value"),
				"l1": property.New([]property.Value{
					property.New(map[string]property.Value{
						"n1": property.New("found"),
					}),
				}),
			}),
			expected: property.New("found"),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.path.Get(tt.from)
			if tt.failure == nil {
				assert.Equal(t, tt.expected, got)
				assert.Nil(t, err)
			} else {
				assert.Equal(t, tt.failure.found, err.(property.PathApplyFailure).Found())
				assert.Equal(t, tt.failure.msg, err.Error())
			}
		})
	}
}

func TestSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path property.Path
		src  property.Value
		to   property.Value

		expected property.Value
	}{
		{
			name: "inside map",
			path: property.Path{property.NewSegment("k2")},
			src: property.New(map[string]property.Value{
				"k1": property.New("v1"),
			}),
			to: property.New("v2"),
			expected: property.New(map[string]property.Value{
				"k1": property.New("v1"),
				"k2": property.New("v2"),
			}),
		},
		{
			name: "inside array",
			path: property.Path{property.NewSegment(1)},
			src: property.New([]property.Value{
				property.New("o1"),
				property.New("o2"),
			}),
			to: property.New("v2"),
			expected: property.New([]property.Value{
				property.New("o1"),
				property.New("v2"),
			}),
		},
		{
			name:     "empty path",
			path:     property.Path{},
			src:      property.New("v1"),
			to:       property.New("v2"),
			expected: property.New("v2"),
		},
		{
			name: "nested",
			path: property.Path{
				property.NewSegment("l1"),
				property.NewSegment(0),
				property.NewSegment("n1"),
			},
			src: property.New(map[string]property.Value{
				"l0": property.New("l0-value"),
				"l1": property.New([]property.Value{
					property.New(map[string]property.Value{
						"n1": property.New("old-value"),
					}),
				}),
			}),
			to: property.New(property.Null),
			expected: property.New(map[string]property.Value{
				"l0": property.New("l0-value"),
				"l1": property.New([]property.Value{
					property.New(map[string]property.Value{
						"n1": property.New(property.Null),
					}),
				}),
			}),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := tt.path.Set(tt.src, tt.to)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAlter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		f        func(property.Value) property.Value
		v        property.Value
		expected property.Value
		path     property.Path

		expectErr bool
	}{
		{
			name: "mutate-in-map",
			f: func(v property.Value) property.Value {
				b := v.AsBool()
				if !b {
					panic(v)
				}
				return property.WithGoValue(v, "yes")
			},
			v: property.New(map[string]property.Value{
				"k": property.New(true),
			}),
			path: property.Path{property.NewSegment("k")},
			expected: property.New(map[string]property.Value{
				"k": property.New("yes"),
			}),
		},
		{
			name: "invalid-path",
			f: func(v property.Value) property.Value {
				panic("v")
			},
			v: property.New(map[string]property.Value{
				"k": property.New(true),
			}),
			path:      property.Path{property.NewSegment("invalid")},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := tt.path.Alter(tt.v, tt.f)
			if tt.expectErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
