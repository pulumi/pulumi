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

package convert_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/types"
	rapidtypes "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/types/rapid"
)

// Tests the laws of §4.3 that hold for every type: reflexivity, dynamic as the top, and a dynamic source.
func TestLaws(t *testing.T) {
	t.Parallel()

	gen := rapidtypes.Type()
	leaf := rapidtypes.Leaf()
	rapid.Check(t, func(t *rapid.T) {
		a := gen.Draw(t, "a")
		require.Equal(t, convert.Safe, convert.To(a, a), "%v <- %v", a, a)
		require.Equal(t, convert.Safe, convert.To(types.Dynamic, a), "dynamic <- %v", a)

		if l := leaf.Draw(t, "leaf"); l != types.Dynamic {
			require.Equal(t, convert.Unsafe, convert.To(l, types.Dynamic), "%v <- dynamic", l)
		}
	})
}

// Tests that Safe conversion is transitive on types without objects. §4.3 does not claim it with objects, since
// C-Object drops extra source keys while C-Map reads every source key.
func TestSafeIsTransitiveWithoutObjects(t *testing.T) {
	t.Parallel()

	gen := rapidtypes.Type().Filter(func(a types.Type) bool { return !contains(a, types.KindObject) })
	rapid.Check(t, func(t *rapid.T) {
		a, b, c := gen.Draw(t, "a"), gen.Draw(t, "b"), gen.Draw(t, "c")
		if convert.To(a, b) == convert.Safe && convert.To(b, c) == convert.Safe {
			require.Equal(t, convert.Safe, convert.To(a, c), "%v <- %v <- %v", a, b, c)
		}
	})
}

// Tests the union rules: a union destination takes the best member, and a union source is decided member by
// member. The destination law needs a source that is not a union, since C-USrc precedes C-UDst.
func TestUnions(t *testing.T) {
	t.Parallel()

	gen := rapidtypes.Type()
	notUnion := gen.Filter(func(a types.Type) bool { return a.Kind() != types.KindUnion })
	rapid.Check(t, func(t *rapid.T) {
		a, b := gen.Draw(t, "a"), gen.Draw(t, "b")
		require.Equal(t, convert.Safe, convert.To(types.Union(a, b), a))

		c := notUnion.Draw(t, "c")
		require.Equal(t, max(convert.To(a, c), convert.To(b, c)), convert.To(types.Union(a, b), c),
			"union(%v, %v) <- %v", a, b, c)
		require.Equal(t, members(convert.To(c, a), convert.To(c, b)), convert.To(c, types.Union(a, b)),
			"%v <- union(%v, %v)", c, a, b)
	})
}

// Tests the eventual rules: an output destination resolves both sides, a promise destination rejects an output
// source and resolves promises on both sides otherwise, and a type converts safely to the output of itself.
func TestEventuals(t *testing.T) {
	t.Parallel()

	gen := rapidtypes.Type()
	notUnion := gen.Filter(func(a types.Type) bool { return a.Kind() != types.KindUnion })
	rapid.Check(t, func(t *rapid.T) {
		a, b := gen.Draw(t, "a"), notUnion.Draw(t, "b")
		require.Equal(t, convert.Safe, convert.To(types.Output(a), a), "output(%v) <- %v", a, a)
		require.Equal(t, convert.To(types.ResolveOutputs(a), types.ResolveOutputs(b)), convert.To(types.Output(a), b),
			"output(%v) <- %v", a, b)

		want := convert.To(types.ResolvePromises(a), types.ResolvePromises(b))
		if b.Kind() == types.KindOutput {
			want = convert.No
		}
		require.Equal(t, want, convert.To(types.Promise(a), b), "promise(%v) <- %v", a, b)
		if !hasOutputMember(a) {
			require.Equal(t, convert.Safe, convert.To(types.Promise(a), a), "promise(%v) <- %v", a, a)
		}
	})
}

// hasOutputMember reports whether a is an output or a union with an output member, which C-Prom rejects as a
// source.
func hasOutputMember(a types.Type) bool {
	if a.Kind() == types.KindUnion {
		return slices.ContainsFunc(a.UnionValues(), hasOutputMember)
	}
	return a.Kind() == types.KindOutput
}

// Tests that the collection rules reduce to the element conversion.
func TestCollections(t *testing.T) {
	t.Parallel()

	gen := rapidtypes.Type()
	rapid.Check(t, func(t *rapid.T) {
		a, b := gen.Draw(t, "a"), gen.Draw(t, "b")
		ab := convert.To(a, b)
		capped := min(convert.Unsafe, ab)
		require.Equal(t, ab, convert.To(types.List(a), types.List(b)), "list(%v) <- list(%v)", a, b)
		require.Equal(t, ab, convert.To(types.List(a), types.Set(b)), "list(%v) <- set(%v)", a, b)
		require.Equal(t, ab, convert.To(types.List(a), types.Tuple(b, b)), "list(%v) <- tuple(%v, %v)", a, b, b)
		require.Equal(t, convert.Safe, convert.To(types.List(a), types.Tuple()), "list(%v) <- tuple()", a)
		require.Equal(t, ab, convert.To(types.Set(a), types.Set(b)), "set(%v) <- set(%v)", a, b)
		require.Equal(t, capped, convert.To(types.Set(a), types.List(b)), "set(%v) <- list(%v)", a, b)
		require.Equal(t, capped, convert.To(types.Set(a), types.Tuple(b)), "set(%v) <- tuple(%v)", a, b)
		require.Equal(t, ab, convert.To(types.Map(a), types.Map(b)), "map(%v) <- map(%v)", a, b)
		require.Equal(t, ab, convert.To(types.Map(a), obj(map[string]types.Type{"x": b})), "map(%v) <- {x: %v}", a, b)
		require.Equal(t, ab, convert.To(types.Tuple(a), types.Tuple(b)), "tuple(%v) <- tuple(%v)", a, b)
		require.Equal(t, convert.No, convert.To(types.Tuple(a, a), types.Tuple(b)), "tuple(%v, %v) <- tuple(%v)", a, a, b)
		require.Equal(t, capped, convert.To(types.Tuple(a), types.List(b)), "tuple(%v) <- list(%v)", a, b)
		require.Equal(t, ab, convert.To(obj(map[string]types.Type{"x": a}), obj(map[string]types.Type{"x": b})),
			"{x: %v} <- {x: %v}", a, b)
		require.Equal(t, convert.To(a, types.None), convert.To(obj(map[string]types.Type{"x": a}), obj(nil)),
			"{x: %v} <- {}", a)
		require.Equal(t, capped, convert.To(obj(map[string]types.Type{"x": a}), types.Map(b)), "{x: %v} <- map(%v)", a, b)
	})
}

// Tests that a constant or enum source widens to its base when the destination has no rule of its own for it.
func TestWidening(t *testing.T) {
	t.Parallel()

	widening := rapidtypes.Type().Filter(func(a types.Type) bool {
		return !slices.Contains([]types.Kind{
			types.KindConst, types.KindEnum, types.KindDynamic, types.KindUnion, types.KindOutput, types.KindPromise,
		}, a.Kind())
	})
	leaf := rapidtypes.Leaf().Filter(func(a types.Type) bool {
		return a.Kind() == types.KindConst || a.Kind() == types.KindEnum
	})
	rapid.Check(t, func(t *rapid.T) {
		dst, src := widening.Draw(t, "dst"), leaf.Draw(t, "src")
		base := src.ConstBase
		if src.Kind() == types.KindEnum {
			base = src.EnumBase
		}
		require.Equal(t, convert.To(dst, base()), convert.To(dst, src), "%v <- %v", dst, src)
	})
}

// members is Safe when both kinds are Safe, No when both are No, and Unsafe otherwise.
func members(a, b convert.Kind) convert.Kind {
	switch {
	case a == convert.Safe && b == convert.Safe:
		return convert.Safe
	case a == convert.No && b == convert.No:
		return convert.No
	}
	return convert.Unsafe
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
		types.KindDynamic, types.KindConst, types.KindEnum:
	}
	return false
}
