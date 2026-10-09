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

package types

import (
	"maps"
	"math"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a, b Type
	}{
		{"scalar", String, String},
		{"const", Const("a"), Const("a")},
		{"list", List(Map(Bool)), List(Map(Bool))},
		{"object key order", Object(map[string]Type{"a": String, "b": Int}), Object(map[string]Type{"b": Int, "a": String})},
		{"union order", Union(String, Int), Union(Int, String)},
		{"union flatten", Union(Union(String, Int), Bool), Union(String, Union(Int, Bool))},
		{"union dedup", Union(String, String, Int), Union(Int, String)},
		{"union of one", Union(String), String},
		{"union of none", Union(), None},
		{"enum value order", Enum("tok", "a", "b"), Enum("tok", "b", "a", "b")},
		{"output of output", Output(Output(String)), Output(String)},
		{"output resolves promises", Output(List(Promise(String))), Output(List(String))},
		{
			"output resolves in objects",
			Output(Object(map[string]Type{"a": Output(Int)})),
			Output(Object(map[string]Type{"a": Int})),
		},
		{"output resolves in unions", Output(Union(Output(Int), String)), Output(Union(Int, String))},
		{"promise of promise", Promise(Promise(String)), Promise(String)},
		{"promise keeps outputs", Promise(Output(String)), Promise(Output(String))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.True(t, tt.a == tt.b, "%v != %v", tt.a, tt.b)
			assert.Equal(t, 0, Compare(tt.a, tt.b))
		})
	}
}

// distinct lists types that are pairwise unequal.
var distinct = []Type{
	None, Bool, Int, Number, String, ID, Dynamic,
	Const(true), Const(false),
	Const(int64(-1)), Const(int64(1)),
	Const(0.0), Const(math.Copysign(0, -1)), Const(1.5), Const(math.NaN()),
	Const(""), Const("a"),
	Enum("a", "x"), Enum("b", "x"), Enum("b", "x", "y"), Enum("b", int64(1)),
	List(String), List(Int), Set(String), Map(String),
	Tuple(), Tuple(String), Tuple(String, Int), Tuple(Int, String),
	Object(map[string]Type{}),
	Object(map[string]Type{"a": String}),
	Object(map[string]Type{"b": String}),
	Object(map[string]Type{"a": Int}),
	Object(map[string]Type{"a": String, "b": String}),
	Union(String, Int), Union(String, Bool), Union(String, Int, Bool),
	Output(String), Output(Int), Promise(String), Promise(Output(String)),
}

func TestCompareIsATotalOrder(t *testing.T) {
	t.Parallel()

	for i, a := range distinct {
		for j, b := range distinct {
			c := Compare(a, b)
			assert.Equal(t, i == j, a == b, "%v == %v", a, b)
			assert.Equal(t, i == j, c == 0, "Compare(%v, %v) = %d", a, b, c)
			assert.Equal(t, -c, Compare(b, a), "Compare(%v, %v) is not antisymmetric", a, b)
		}
	}

	sorted := slices.Clone(distinct)
	slices.SortFunc(sorted, Compare)
	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			assert.Negative(t, Compare(sorted[i], sorted[j]), "%v < %v", sorted[i], sorted[j])
		}
	}
}

func TestString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		t    Type
		want string
	}{
		{None, "none"},
		{Const("a"), `const("a")`},
		{Const(int64(3)), "const(3)"},
		{Const(1.5), "const(1.5)"},
		{Const(true), "const(true)"},
		{Enum("pkg:Color", "red", "blue"), `enum(pkg:Color, string, {"blue", "red"})`},
		{List(Output(String)), "list(output(string))"},
		{Tuple(String, Int), "tuple(string, int)"},
		{Union(None, String), "union(none, string)"},
		{Object(map[string]Type{"b": Int, "a": String}), "object({a = string, b = int})"},
		{Promise(Set(Map(Bool))), "promise(set(map(bool)))"},
		{Opaque("Asset"), "opaque(Asset)"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.t.String())
		})
	}
}

func TestGoString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		t    Type
		want string
	}{
		{None, "types.None"},
		{ID, "types.ID"},
		{Const("a"), `types.Const("a")`},
		{Const(int64(-3)), "types.Const(int64(-3))"},
		{Const(1.0), "types.Const(float64(1))"},
		{Const(math.Copysign(0, -1)), "types.Const(math.Copysign(0, -1))"},
		{Const(math.Inf(-1)), "types.Const(math.Inf(-1))"},
		{Const(true), "types.Const(true)"},
		{Enum("pkg:Color", "red", "blue"), `types.Enum("pkg:Color", "blue", "red")`},
		{Enum("pkg:Count", int64(1)), `types.Enum("pkg:Count", int64(1))`},
		{List(Output(Number)), "types.List(types.Output(types.Number))"},
		{Tuple(), "types.Tuple()"},
		{Tuple(String, Int), "types.Tuple(types.String, types.Int)"},
		{Union(None, String), "types.Union(types.None, types.String)"},
		{Object(map[string]Type{}), "types.Object(map[string]types.Type{})"},
		{
			Object(map[string]Type{"b": Int, "a": String}),
			`types.Object(map[string]types.Type{"a": types.String, "b": types.Int})`,
		},
		{Promise(Set(Map(Bool))), "types.Promise(types.Set(types.Map(types.Bool)))"},
		{Opaque("pkg:index:Token"), `types.Opaque("pkg:index:Token")`},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.t.GoString())
		})
	}

	assert.Equal(t, "types.KindList", KindList.GoString())
	assert.Equal(t, "types.KindID", KindID.GoString())
	assert.Equal(t, "types.KindNone", KindNone.GoString())
}

func TestAccessors(t *testing.T) {
	t.Parallel()

	assert.Equal(t, KindList, List(String).Kind())
	assert.Equal(t, String, List(String).Element())
	assert.Equal(t, String, Output(Promise(String)).Element())

	assert.Equal(t, 0, Tuple().Len())
	assert.Equal(t, 2, Tuple(String, Int).Len())
	assert.Equal(t, []Type{String, Int}, Tuple(String, Int).TupleValues())
	assert.Equal(t, []Type{Int, String}, Union(String, Int).UnionValues())

	object := Object(map[string]Type{"b": Int, "a": String})
	assert.Equal(t, 2, object.Len())
	assert.Equal(t, map[string]Type{"a": String, "b": Int}, maps.Collect(object.ObjectValues()))

	enum := Enum("pkg:Count", int64(2), int64(1))
	assert.Equal(t, "pkg:Count", enum.EnumToken())
	assert.Equal(t, Int, enum.EnumBase())
	assert.Equal(t, []Type{Const(int64(1)), Const(int64(2))}, enum.EnumValues())

	assert.Equal(t, Bool, Const(true).ConstBase())
	assert.True(t, Const(true).ConstBoolValue())
	assert.Equal(t, int64(-7), Const(int64(-7)).ConstIntValue())
	assert.Equal(t, 2.5, Const(2.5).ConstNumberValue())
	assert.Equal(t, "s", Const("s").ConstStringValue())

	assert.Panics(t, func() { String.Element() })
	assert.Panics(t, func() { Const("s").ConstIntValue() })
	assert.Equal(t, KindNone, Type{}.Kind())
	assert.Equal(t, None, Type{})
	assert.Panics(t, func() { Type{}.Element() })
}

func TestOpaque(t *testing.T) {
	t.Parallel()

	asset := Opaque("Asset")
	assert.Equal(t, KindOpaque, asset.Kind())
	assert.Equal(t, "Asset", asset.OpaqueName())
	assert.Equal(t, asset, Opaque("Asset"))
	assert.NotEqual(t, asset, Opaque("Archive"))
	assert.NotEqual(t, asset, Enum("Asset", "Asset"))
	assert.Negative(t, Compare(Opaque("Archive"), asset))
	assert.Positive(t, Compare(asset, Promise(asset)))
	assert.Equal(t, []Type{String, asset}, Union(asset, String).UnionValues())
	assert.Panics(t, func() { Opaque("") })
	assert.Panics(t, func() { String.OpaqueName() })
}
