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
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func genKeySegment() *rapid.Generator[PathSegment] { return rapid.Map(rapid.String(), NewSegment) }

func genIndexSegment() *rapid.Generator[PathSegment] {
	return rapid.Map(rapid.Int().Filter(func(i int) bool { return i >= 0 }), NewSegment)
}

func genGlobSegments() *rapid.Generator[[]GlobSegment] {
	return rapid.SliceOf(rapid.OneOf(
		rapid.Map(genKeySegment(), func(k PathSegment) GlobSegment { return k }),
		rapid.Map(genIndexSegment(), func(k PathSegment) GlobSegment { return k }),
		rapid.Just[GlobSegment](Splat),
	))
}

func genGlob() *rapid.Generator[Glob] {
	return rapid.Map(
		genGlobSegments(),
		func(segs []GlobSegment) Glob { return GlobFromSegments(segs...) },
	)
}

func genPath() *rapid.Generator[Path] {
	return rapid.Map(
		rapid.SliceOf(rapid.OneOf(
			genKeySegment(),
			genIndexSegment(),
		)),
		func(segs []PathSegment) Path { return PathFromSegments(segs...) },
	)
}

type tup[A, B any] struct {
	a A
	b B
}

func genTextGlobPath() *rapid.Generator[tup[string, Glob]] {
	type ret = tup[string, GlobSegment]
	return rapid.Map(rapid.SliceOfN(rapid.OneOf(
		rapid.Just(ret{"*", Splat}),   // Raw Splat
		rapid.Just(ret{"[*]", Splat}), // Index Splat
		// Number
		rapid.Map(rapid.Uint64().Filter(func(i uint64) bool { return i < math.MaxInt64 }), func(i uint64) ret {
			return ret{"[" + strconv.FormatInt(int64(i), 10) + "]", IndexSegment{i}} //nolint:gosec // checked above
		}),
		// Unquoted property path
		rapid.Map(rapid.StringMatching("[a-zA-Z_][a-zA-Z0-9_]*"), func(s string) ret {
			return ret{s, KeySegment{s}}
		}),
		// Quoted property path
		rapid.Map(rapid.String(), func(s string) ret { return ret{"[" + strconv.Quote(s) + "]", KeySegment{s}} }),
	), 1, 10), func(segments []ret) tup[string, Glob] {
		var s strings.Builder
		var g Glob
		for i, v := range segments {
			g.pathRepr = g.appendGlobSegment(v.b)
			if v.a[0] == '[' {
				s.WriteString(v.a)
			} else {
				if i > 0 {
					s.WriteRune('.')
				}
				s.WriteString(v.a)
			}
		}

		return tup[string, Glob]{s.String(), g}
	})
}

func rapidTest(t *testing.T, name string, f func(t *rapid.T)) {
	t.Helper()
	t.Run(name, func(t *testing.T) { t.Helper(); t.Parallel(); rapid.Check(t, f) })
}

func TestGlobEncoding(t *testing.T) {
	t.Parallel()

	rapidTest(t, "canonical values roundtrip", func(t *rapid.T) {
		path1 := genGlob().Filter(func(p Glob) bool { return p.len() > 0 }).Draw(t, "path")
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

	rapidTest(t, "unmarshal", func(t *rapid.T) {
		pair := genTextGlobPath().Draw(t, "text")
		var g Glob
		err := g.UnmarshalText([]byte(pair.a))
		require.NoError(t, err)
		assert.Equal(t, pair.b, g)

		text2, err := g.MarshalText()
		require.NoError(t, err)

		var g2 Glob
		err = g2.UnmarshalText(text2)
		require.NoError(t, err)
		assert.Equal(t, g, g2, "assert that we can round-trip the Glob")
	})

	t.Run("unmarshal", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			text     string
			expected Glob
		}{
			{"x.*", GlobFromSegments(KeySegment{"x"}, Splat)},
			{"*", GlobFromSegments(Splat)},
			{`["x"]`, GlobFromSegments(KeySegment{"x"})},
			{"[9223372036854775807]", GlobFromSegments(IndexSegment{9223372036854775807})},
			{"[0]", GlobFromSegments(IndexSegment{0})},
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
			{"[-1]", "indexes cannot be negative"},
			{"[9223372036854775808]", "indexes cannot exceed 9223372036854775807"},
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

func TestMatches(t *testing.T) {
	t.Parallel()

	rapidTest(t, "prefixes are always have prefix", func(t *rapid.T) {
		path := genPath().Draw(t, "path")
		prefixLen := rapid.IntRange(0, path.len()).Draw(t, "prefixLen")

		glob := GlobFromSegments(slices.Collect(path.segments)[:prefixLen]...)

		assert.True(t, glob.Matches(path), "%#v should be a prefix of %#v", glob, path)
	})

	rapidTest(t, "mutations are never prefixes", func(t *rapid.T) {
		path := genPath().Filter(func(p Path) bool { return p.len() > 0 }).Draw(t, "path")
		glob := make([]GlobSegment, path.len())
		for i, v := range path.enumerate {
			glob[i] = v
		}

		prefixLen := rapid.IntRange(0, path.len()-1).Draw(t, "mutation index")
		switch s := glob[prefixLen].(type) {
		case IndexSegment:
			if s.Index() == math.MaxInt64 {
				glob[prefixLen] = NewSegment(s.Index() - 1)
			} else {
				glob[prefixLen] = NewSegment(s.Index() + 1)
			}
		case KeySegment:
			glob[prefixLen] = NewSegment(s.string + "!")
		default:
			require.Fail(t, "unexpected type %T", s)
		}
		assert.False(t, GlobFromSegments(glob...).Matches(path), "%#v should not be a prefix of %#v",
			GlobFromSegments(glob...), path)
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
		expected map[Path]Value
		errMsg   string
	}{
		{
			name: "single-key",
			glob: GlobFromSegments(NewSegment("a")),
			from: nested,
			expected: map[Path]Value{
				PathFromSegments(NewSegment("a")): New(map[string]Value{
					"x": New("ax"),
					"y": New("ay"),
				}),
			},
		},
		{
			name: "single-index",
			glob: GlobFromSegments(NewSegment(1)),
			from: arrayValue,
			expected: map[Path]Value{
				PathFromSegments(NewSegment(1)): New("one"),
			},
		},
		{
			name: "nested-key-key",
			glob: GlobFromSegments(NewSegment("a"), NewSegment("x")),
			from: nested,
			expected: map[Path]Value{
				PathFromSegments(NewSegment("a"), NewSegment("x")): New("ax"),
			},
		},
		{
			name: "splat-on-map",
			glob: GlobFromSegments(Splat),
			from: nested,
			expected: map[Path]Value{
				PathFromSegments(NewSegment("a")): New(map[string]Value{
					"x": New("ax"),
					"y": New("ay"),
				}),
				PathFromSegments(NewSegment("b")): New(map[string]Value{
					"x": New("bx"),
					"y": New("by"),
				}),
			},
		},
		{
			name: "splat-on-array",
			glob: GlobFromSegments(Splat),
			from: arrayValue,
			expected: map[Path]Value{
				PathFromSegments(NewSegment(0)): New("zero"),
				PathFromSegments(NewSegment(1)): New("one"),
				PathFromSegments(NewSegment(2)): New("two"),
			},
		},
		{
			name: "splat-then-key",
			glob: GlobFromSegments(Splat, NewSegment("x")),
			from: nested,
			expected: map[Path]Value{
				PathFromSegments(NewSegment("a"), NewSegment("x")): New("ax"),
				PathFromSegments(NewSegment("b"), NewSegment("x")): New("bx"),
			},
		},
		{
			name:   "splat-on-primitive",
			glob:   GlobFromSegments(Splat),
			from:   New("hello"),
			errMsg: "expected a map or array, found string",
		},
		{
			name: "empty-glob",
			glob: Glob{},
			from: nested,
			expected: map[Path]Value{
				PathFromSegments(): nested,
			},
		},
		{
			name:   "missing-key-returns-error",
			glob:   GlobFromSegments(NewSegment("missing")),
			from:   nested,
			errMsg: `missing key "missing" in map`,
		},
		{
			name: "splat-on-empty-map",
			glob: GlobFromSegments(Splat),
			from: New(map[string]Value{}),
		},
		{
			name: "splat-on-empty-array",
			glob: GlobFromSegments(Splat),
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
