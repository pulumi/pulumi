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

package types_test

import (
	"maps"
	"math"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/types"
	rapidtypes "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/types/rapid"
)

// Tests that Compare is a total order that agrees with ==: it is zero exactly for equal types, antisymmetric, and
// transitive.
func TestCompareIsTotalOrder(t *testing.T) {
	t.Parallel()

	gen := rapidtypes.Type()
	rapid.Check(t, func(t *rapid.T) {
		a, b, c := gen.Draw(t, "a"), gen.Draw(t, "b"), gen.Draw(t, "c")
		same := a
		require.True(t, a == same, "%v != itself", a)
		require.Equal(t, 0, types.Compare(a, a), "%v does not compare equal to itself", a)
		require.Equal(t, a == b, types.Compare(a, b) == 0, "Compare(%v, %v) disagrees with ==", a, b)
		require.Equal(t, -types.Compare(b, a), types.Compare(a, b), "Compare(%v, %v) is not antisymmetric", a, b)
		if types.Compare(a, b) <= 0 && types.Compare(b, c) <= 0 {
			require.LessOrEqual(t, types.Compare(a, c), 0, "%v <= %v <= %v but not %v <= %v", a, b, c, a, c)
		}
	})
}

// Tests that Compare defines one order for any set of types: sorting a shuffled copy gives the same sequence.
func TestCompareIsCanonical(t *testing.T) {
	t.Parallel()

	gen := rapid.SliceOfN(rapidtypes.Type(), 1, 8)
	rapid.Check(t, func(t *rapid.T) {
		sorted := slices.SortedFunc(slices.Values(gen.Draw(t, "types")), types.Compare)
		shuffled := rapid.Permutation(slices.Clone(sorted)).Draw(t, "shuffled")
		slices.SortFunc(shuffled, types.Compare)
		require.Equal(t, sorted, shuffled)
	})
}

// Tests that the element and member accessors return what the constructor was given.
func TestConstructorsRoundTrip(t *testing.T) {
	t.Parallel()

	gen := rapidtypes.Type()
	elems := rapid.SliceOfN(gen, 0, 4)
	rapid.Check(t, func(t *rapid.T) {
		a := gen.Draw(t, "a")
		require.Equal(t, a, types.List(a).Element())
		require.Equal(t, a, types.Set(a).Element())
		require.Equal(t, a, types.Map(a).Element())
		require.Equal(t, types.ResolveOutputs(a), types.Output(a).Element())
		require.Equal(t, types.ResolvePromises(a), types.Promise(a).Element())

		ts := elems.Draw(t, "ts")
		tuple := types.Tuple(ts...)
		require.Equal(t, len(ts), tuple.Len())
		require.Equal(t, ts, tuple.TupleValues())

		properties := make(map[string]types.Type, len(ts))
		for i, elem := range ts {
			properties[rapid.StringN(0, 4, 4).Draw(t, "name")+string(rune('a'+i))] = elem
		}
		object := types.Object(properties)
		require.Equal(t, len(properties), object.Len())
		require.Equal(t, properties, maps.Collect(object.ObjectValues()))
	})
}

// Tests that the output and promise constructors keep their invariants: the element of an output holds no
// eventual, the element of a promise holds no promise, and resolution is idempotent.
func TestEventualInvariants(t *testing.T) {
	t.Parallel()

	gen := rapidtypes.Type()
	rapid.Check(t, func(t *rapid.T) {
		a := gen.Draw(t, "a")
		resolved := types.ResolveOutputs(a)
		require.Equal(t, resolved, types.ResolveOutputs(resolved))
		require.Equal(t, resolved, types.ResolvePromises(resolved))
		require.False(t, contains(resolved, types.KindOutput), "%v holds an output", resolved)
		require.False(t, types.ContainsOutputs(resolved), "%v holds an output", resolved)
		require.False(t, contains(resolved, types.KindPromise), "%v holds a promise", resolved)
		require.Equal(t, contains(a, types.KindOutput), types.ContainsOutputs(a), "%v", a)
		require.Equal(t, types.Output(a), types.Output(types.Output(a)))
		require.Equal(t, types.Output(a), types.Output(types.Promise(a)))

		promised := types.ResolvePromises(a)
		require.Equal(t, promised, types.ResolvePromises(promised))
		require.False(t, contains(promised, types.KindPromise), "%v holds a promise", promised)
		require.Equal(t, types.Promise(a), types.Promise(types.Promise(a)))
	})
}

// Tests the union normal form: a union has at least two members, sorted and distinct, none of them a union; and the
// constructor is commutative, associative, and idempotent. None is a member like any other, since union(T, none)
// is optional(T).
func TestUnionNormalForm(t *testing.T) {
	t.Parallel()

	gen := rapidtypes.Type()
	elems := rapid.SliceOfN(gen, 1, 4)
	rapid.Check(t, func(t *rapid.T) {
		a, b := gen.Draw(t, "a"), gen.Draw(t, "b")
		require.Equal(t, types.None, types.Union())
		require.Equal(t, a, types.Union(a))
		require.Equal(t, a, types.Union(a, a))
		require.Equal(t, types.Union(a, b), types.Union(b, a))

		ts, us := elems.Draw(t, "ts"), elems.Draw(t, "us")
		union := types.Union(ts...)
		require.Equal(t, union, types.Union(rapid.Permutation(slices.Clone(ts)).Draw(t, "shuffled")...))
		require.Equal(t, types.Union(slices.Concat(ts, us)...), types.Union(union, types.Union(us...)))
		if union.Kind() != types.KindUnion {
			return
		}
		members := union.UnionValues()
		require.Equal(t, len(members), union.Len())
		require.GreaterOrEqual(t, len(members), 2)
		for i, member := range members {
			require.NotEqual(t, types.KindUnion, member.Kind(), "%v holds a union", union)
			if i > 0 {
				require.Negative(t, types.Compare(members[i-1], member), "%v is not sorted", union)
			}
		}
	})
}

// Tests that a constant remembers its value and base.
func TestConstRoundTrip(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		switch rapid.IntRange(0, 3).Draw(t, "base") {
		case 0:
			v := rapid.Bool().Draw(t, "v")
			require.Equal(t, types.Bool, types.Const(v).ConstBase())
			require.Equal(t, v, types.Const(v).ConstBoolValue())
		case 1:
			v := rapid.Int64().Draw(t, "v")
			require.Equal(t, types.Int, types.Const(v).ConstBase())
			require.Equal(t, v, types.Const(v).ConstIntValue())
		case 2:
			v := rapid.Float64().Draw(t, "v")
			require.Equal(t, types.Number, types.Const(v).ConstBase())
			require.Equal(t, math.Float64bits(v), math.Float64bits(types.Const(v).ConstNumberValue()))
		default:
			v := rapid.String().Draw(t, "v")
			require.Equal(t, types.String, types.Const(v).ConstBase())
			require.Equal(t, v, types.Const(v).ConstStringValue())
		}
	})
}

// Tests that an enum holds its token, its base, and its values sorted and distinct, in any input order.
func TestEnumNormalForm(t *testing.T) {
	t.Parallel()

	values := rapid.SliceOfN(rapid.Int64Range(-3, 3), 1, 6)
	rapid.Check(t, func(t *rapid.T) {
		vs := values.Draw(t, "values")
		enum := types.Enum("test:index:Level", vs...)
		require.Equal(t, "test:index:Level", enum.EnumToken())
		require.Equal(t, types.Int, enum.EnumBase())
		require.Equal(t, enum, types.Enum("test:index:Level", rapid.Permutation(slices.Clone(vs)).Draw(t, "shuffled")...))

		want := make([]types.Type, 0, len(vs))
		for _, v := range slices.Sorted(slices.Values(vs)) {
			want = append(want, types.Const(v))
		}
		require.Equal(t, slices.Compact(want), enum.EnumValues())
		require.Equal(t, len(enum.EnumValues()), enum.Len())
	})
}

// Tests that a type is the constructor of its kind applied to its children, which for a recursive type means that
// its unfolding rebuilds the type itself.
func TestRebuildFromHead(t *testing.T) {
	t.Parallel()

	gen := rapidtypes.Type()
	rapid.Check(t, func(t *rapid.T) {
		a := gen.Draw(t, "a")
		require.Equal(t, a, rebuild(a))
		require.NotEmpty(t, a.String())
		require.NotEmpty(t, a.GoString())
	})
}

// Tests the fixed-point laws of Recursive: a group that unrolls a body twice is the group that unrolls it once, a
// body applied to its own fixed point is that fixed point, and a body that ignores its placeholder is itself.
func TestRecursiveFixedPoints(t *testing.T) {
	t.Parallel()

	bodies := []func(types.Type) types.Type{
		types.List,
		types.Set,
		types.Map,
		func(s types.Type) types.Type { return types.Tuple(s, types.String) },
		func(s types.Type) types.Type { return types.Object(map[string]types.Type{"a": s, "b": types.Int}) },
		func(s types.Type) types.Type { return types.List(types.Union(s, types.None)) },
		func(s types.Type) types.Type { return types.List(types.Union(types.List(s), types.Set(s))) },
		func(s types.Type) types.Type { return types.Output(types.List(s)) },
		func(s types.Type) types.Type { return types.Map(types.Promise(s)) },
	}
	gen := rapidtypes.Type()
	rapid.Check(t, func(t *rapid.T) {
		f := rapid.SampledFrom(bodies).Draw(t, "body")
		fixed := rec(f)
		require.Equal(t, fixed, rec(func(s types.Type) types.Type { return f(f(s)) }))
		require.Equal(t, fixed, f(fixed))
		require.Equal(t, fixed, f(f(f(fixed))))

		a := gen.Draw(t, "a")
		require.Equal(t, a, rec(func(types.Type) types.Type { return a }))
		require.Equal(t, f(a), rec(func(types.Type) types.Type { return f(a) }))
	})
}

// rec builds one recursive type from a body over its own placeholder.
func rec(body func(types.Type) types.Type) types.Type {
	return types.Recursive(func(self types.Placeholder) []types.Type {
		return []types.Type{body(self())}
	})[0]
}

// rebuild applies the constructor of t's kind to t's children.
func rebuild(t types.Type) types.Type {
	switch t.Kind() {
	case types.KindList:
		return types.List(t.Element())
	case types.KindSet:
		return types.Set(t.Element())
	case types.KindMap:
		return types.Map(t.Element())
	case types.KindOutput:
		return types.Output(t.Element())
	case types.KindPromise:
		return types.Promise(t.Element())
	case types.KindTuple:
		return types.Tuple(t.TupleValues()...)
	case types.KindUnion:
		return types.Union(t.UnionValues()...)
	case types.KindObject:
		return types.Object(maps.Collect(t.ObjectValues()))
	case types.KindNone, types.KindBool, types.KindInt, types.KindNumber, types.KindString, types.KindID,
		types.KindDynamic, types.KindConst, types.KindEnum, types.KindOpaque:
	}
	return t
}

// contains reports whether a type of the given kind occurs anywhere in t.
func contains(t types.Type, kind types.Kind) bool {
	return containsFrom(t, kind, map[types.Type]bool{})
}

func containsFrom(t types.Type, kind types.Kind, seen map[types.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	if t.Kind() == kind {
		return true
	}
	in := func(e types.Type) bool { return containsFrom(e, kind, seen) }
	switch t.Kind() {
	case types.KindList, types.KindSet, types.KindMap, types.KindOutput, types.KindPromise:
		return in(t.Element())
	case types.KindTuple:
		return slices.ContainsFunc(t.TupleValues(), in)
	case types.KindUnion:
		return slices.ContainsFunc(t.UnionValues(), in)
	case types.KindObject:
		for _, v := range t.ObjectValues() {
			if in(v) {
				return true
			}
		}
	case types.KindNone, types.KindBool, types.KindInt, types.KindNumber, types.KindString, types.KindID,
		types.KindDynamic, types.KindConst, types.KindEnum, types.KindOpaque:
	}
	return false
}
