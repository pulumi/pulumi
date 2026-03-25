// Copyright 2026, Pulumi Corporation.
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

package property

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func genPathSegment() *rapid.Generator[PathSegment] {
	return rapid.OneOf(
		rapid.Map(rapid.String(), func(s string) PathSegment {
			return NewSegment(s)
		}),
		rapid.Map(rapid.Int().Filter(func(i int) bool { return i >= 0 }), func(i int) PathSegment {
			return NewSegment(i)
		}),
	)
}

func genGlob() *rapid.Generator[Glob] { return rapid.Map(genPath(), Path.AsGlob) }

func genPath() *rapid.Generator[Path] {
	return rapid.Map(
		rapid.SliceOf(genPathSegment()),
		func(segs []PathSegment) Path { return Path(segs) },
	)
}

func rapidTest(t *testing.T, name string, f func(t *rapid.T)) {
	t.Helper()
	t.Run(name, func(t *testing.T) { t.Helper(); t.Parallel(); rapid.Check(t, f) })
}

func TestGlobEncoding(t *testing.T) {
	t.Parallel()

	rapidTest(t, "canonical values roundtrip", func(t *rapid.T) {
		path1 := genGlob().Filter(func(p Glob) bool { return len(p) > 0 }).Draw(t, "path")
		text1, err := path1.MarshalText()
		require.NoError(t, err)

		var path2 Glob
		err = path2.UnmarshalText(text1)
		require.NoError(t, err)

		require.Equal(t, path1, path2)
		text2, err := path2.MarshalText()
		require.NoError(t, err)

		require.Equal(t, text1, text2, "stable to-text mapping")
	})

	t.Run("unmarshal", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			text     string
			expected Glob
		}{
			{"x.*", Glob{KeySegment{"x"}, Splat}},
			{"*", Glob{Splat}},
			{`["x"]`, Glob{KeySegment{"x"}}},
		}

		for _, tt := range tests {
			t.Run(tt.text, func(t *testing.T) {
				t.Parallel()

				var g Glob
				err := g.UnmarshalText([]byte(tt.text))
				require.NoError(t, err)
				assert.Equal(t, tt.expected, g)
			})
		}
	})

	t.Run("errors", func(t *testing.T) {
		t.Parallel()

		tests := []struct{ text, expectedError string }{
			{"", "cannot unmarshal an empty property path"},
			{".", "expected character"},
			{"[", "unclosed '['"},
			{"[1", "unclosed number [1"},
			{`["x`, `unclosed string ["x`},
			{`["x"`, `unclosed index ["x"`},
		}
		for _, tt := range tests {
			t.Run(tt.text, func(t *testing.T) {
				t.Parallel()

				var g Glob
				err := g.UnmarshalText([]byte(tt.text))
				require.EqualError(t, err, tt.expectedError)
			})
		}
	})

	rapidTest(t, "does not panic", func(t *rapid.T) {
		s := rapid.String().Draw(t, "input")
		var g Glob
		_ = g.UnmarshalText([]byte(s))
	})
}

func TestHasPrefix(t *testing.T) {
	t.Parallel()

	rapidTest(t, "prefixes are always have prefix", func(t *rapid.T) {
		path := genPath().Draw(t, "path")
		prefixLen := rapid.IntRange(0, len(path)).Draw(t, "prefixLen")

		assert.True(t, (path[:prefixLen]).AsGlob().HasPrefix(path),
			"%#v should be a prefix of %#v", path[:prefixLen], path)
	})

	rapidTest(t, "mutations are never prefixes", func(t *rapid.T) {
		path := genPath().Filter(func(p Path) bool { return len(p) > 0 }).Draw(t, "path")
		glob := make(Glob, len(path))
		for i, v := range path {
			glob[i] = v
		}

		prefixLen := rapid.IntRange(0, len(path)-1).Draw(t, "mutation index")
		switch s := glob[prefixLen].(type) {
		case IndexSegment:
			glob[prefixLen] = NewSegment(s.int + 1)
		case KeySegment:
			glob[prefixLen] = NewSegment(s.string + "!")
		default:
			require.Fail(t, "unexpected type %T", s)
		}
		assert.False(t, glob.HasPrefix(path), "%#v should not be a prefix of %#v", path[:prefixLen], path)
	})
}

func TestGlobGet(t *testing.T) {
	t.Parallel()

	nested := New(map[string]Value{
		"a": New(map[string]Value{
			"x": New("ax"),
			"y": New("ay"),
		}),
		"b": New(map[string]Value{
			"x": New("bx"),
			"y": New("by"),
		}),
	})

	arrayValue := New([]Value{
		New("zero"),
		New("one"),
		New("two"),
	})

	tests := []struct {
		name     string
		glob     Glob
		from     Value
		expected []Value
		errMsg   string
	}{
		{
			name: "single-key",
			glob: Glob{NewSegment("a")},
			from: nested,
			expected: []Value{
				New(map[string]Value{
					"x": New("ax"),
					"y": New("ay"),
				}),
			},
		},
		{
			name: "single-index",
			glob: Glob{NewSegment(1)},
			from: arrayValue,
			expected: []Value{
				New("one"),
			},
		},
		{
			name: "nested-key-key",
			glob: Glob{NewSegment("a"), NewSegment("x")},
			from: nested,
			expected: []Value{
				New("ax"),
			},
		},
		{
			name: "splat-on-map",
			glob: Glob{Splat},
			from: nested,
			expected: []Value{
				New(map[string]Value{
					"x": New("ax"),
					"y": New("ay"),
				}),
				New(map[string]Value{
					"x": New("bx"),
					"y": New("by"),
				}),
			},
		},
		{
			name: "splat-on-array",
			glob: Glob{Splat},
			from: arrayValue,
			expected: []Value{
				New("zero"),
				New("one"),
				New("two"),
			},
		},
		{
			name: "splat-then-key",
			glob: Glob{Splat, NewSegment("x")},
			from: nested,
			expected: []Value{
				New("ax"),
				New("bx"),
			},
		},
		{
			name:   "splat-on-primitive",
			glob:   Glob{Splat},
			from:   New("hello"),
			errMsg: "expected a map or array, found string",
		},
		{
			name:     "empty-glob",
			glob:     Glob{},
			from:     nested,
			expected: []Value{nested},
		},
		{
			name:   "missing-key-returns-error",
			glob:   Glob{NewSegment("missing")},
			from:   nested,
			errMsg: `missing key "missing" in map`,
		},
		{
			name: "path-as-glob",
			glob: Path{
				NewSegment("a"),
				NewSegment("y"),
			}.AsGlob(),
			from: nested,
			expected: []Value{
				New("ay"),
			},
		},
		{
			name: "splat-on-empty-map",
			glob: Glob{Splat},
			from: New(map[string]Value{}),
		},
		{
			name: "splat-on-empty-array",
			glob: Glob{Splat},
			from: New([]Value{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.glob.Get(tt.from)
			if tt.errMsg != "" {
				require.Error(t, err)
				assert.Equal(t, tt.errMsg, err.Error())
				return
			}
			require.NoError(t, err)

			if tt.expected == nil {
				assert.Nil(t, got)
				return
			}

			assert.Equal(t, tt.expected, got)
		})
	}
}
