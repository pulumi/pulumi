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

package unify_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/types"
	rapidtypes "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/types/rapid"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/unify"
)

// Tests S1 of §6: every input converts safely to the result.
func TestS1ResultConvertsFromInputs(t *testing.T) {
	t.Parallel()

	gen := rapid.SliceOfN(rapidtypes.Type(), 1, 4)
	rapid.Check(t, func(t *rapid.T) {
		inputs := gen.Draw(t, "inputs")
		result := unify.Types(inputs...)
		for _, in := range inputs {
			require.Equal(t, convert.Safe, convert.To(result, in), "unify%v = %v <- %v", inputs, result, in)
		}
	})
}

// Tests S4 and S5 of §6: every permutation of the inputs gives one result, and a repeated input changes nothing.
func TestS4S5SymmetricAndIdempotent(t *testing.T) {
	t.Parallel()

	gen := rapid.SliceOfN(rapidtypes.Type(), 1, 4)
	rapid.Check(t, func(t *rapid.T) {
		inputs := gen.Draw(t, "inputs")
		result := unify.Types(inputs...)
		shuffled := rapid.Permutation(slices.Clone(inputs)).Draw(t, "shuffled")
		require.Equal(t, result, unify.Types(shuffled...), "unify%v", inputs)
		require.Equal(t, result, unify.Types(append(slices.Clone(inputs), inputs[0])...), "unify%v", inputs)
		require.Equal(t, inputs[0], unify.Types(inputs[0], inputs[0]), "unify(%v, %v)", inputs[0], inputs[0])
	})
}

// Tests the unnamed law of §6 that the result absorbs itself: unify(a, b, a ⊔ b) = a ⊔ b.
//
// The law does not hold as the rules stand, and the model implementation fails it the same way. With
// a = list({a: output(union(bool, string))}) and b = list(output(list(list(none)))), a ⊔ b keeps the property
// union(bool, string) by U-Eq, since the object is the only member of its class, while unify(a, b, a ⊔ b) merges
// that property with itself and reaches string. The test is skipped until the specification settles the law.
func TestResultAbsorbsItself(t *testing.T) {
	t.Parallel()
	t.Skip("§6 law unify(a, b, a ⊔ b) = a ⊔ b fails on unions nested in a single-member class; see the comment")

	gen := rapidtypes.Type()
	rapid.Check(t, func(t *rapid.T) {
		a, b := gen.Draw(t, "a"), gen.Draw(t, "b")
		result := unify.Types(a, b)
		require.Equal(t, result, unify.Types(a, b, result), "unify(%v, %v)", a, b)
	})
}

// Tests S8 of §6: unification distributes over list, set, map, one-element tuple, and one-key object on every
// input, and over output and promise on inputs without eventuals.
func TestS8Distributes(t *testing.T) {
	t.Parallel()

	gen := rapidtypes.Type()
	plain := gen.Filter(func(a types.Type) bool { return !containsEventual(a) })
	rapid.Check(t, func(t *rapid.T) {
		a, b := gen.Draw(t, "a"), gen.Draw(t, "b")
		result := unify.Types(a, b)
		require.Equal(t, types.List(result), unify.Types(types.List(a), types.List(b)), "list(%v ⊔ %v)", a, b)
		require.Equal(t, types.Set(result), unify.Types(types.Set(a), types.Set(b)), "set(%v ⊔ %v)", a, b)
		require.Equal(t, types.Map(result), unify.Types(types.Map(a), types.Map(b)), "map(%v ⊔ %v)", a, b)
		require.Equal(t, types.Tuple(result), unify.Types(types.Tuple(a), types.Tuple(b)), "tuple(%v ⊔ %v)", a, b)
		require.Equal(t, obj(map[string]types.Type{"k": result}),
			unify.Types(obj(map[string]types.Type{"k": a}), obj(map[string]types.Type{"k": b})), "{k: %v ⊔ %v}", a, b)

		c, d := plain.Draw(t, "c"), plain.Draw(t, "d")
		plainResult := unify.Types(c, d)
		require.Equal(t, types.Output(plainResult), unify.Types(types.Output(c), types.Output(d)), "output(%v ⊔ %v)", c, d)
		require.Equal(t, types.Promise(plainResult), unify.Types(types.Promise(c), types.Promise(d)),
			"promise(%v ⊔ %v)", c, d)
	})
}

// Tests S10 of §6: dynamic absorbs every member but none.
func TestS10Dynamic(t *testing.T) {
	t.Parallel()

	gen := rapidtypes.Type()
	rapid.Check(t, func(t *rapid.T) {
		a := gen.Draw(t, "a")
		want := types.Dynamic
		if slices.Contains(members(a), types.None) {
			want = types.Union(types.Dynamic, types.None)
		}
		require.Equal(t, want, unify.Types(a, types.Dynamic), "%v ⊔ dynamic", a)
	})
}

// Tests the scalar absorption law of §6: when b converts safely from a and not back, a ⊔ b = b.
func TestScalarAbsorbs(t *testing.T) {
	t.Parallel()

	scalar := rapidtypes.Leaf().Filter(func(a types.Type) bool {
		return a != types.None && a != types.Dynamic
	})
	rapid.Check(t, func(t *rapid.T) {
		a, b := scalar.Draw(t, "a"), scalar.Draw(t, "b")
		if convert.To(b, a) == convert.Safe && convert.To(a, b) != convert.Safe {
			require.Equal(t, b, unify.Types(a, b), "%v ⊔ %v", a, b)
		}
	})
}

// Tests the merged normal form of §6: no two members of a result union merge further.
func TestMergedNormalForm(t *testing.T) {
	t.Parallel()

	gen := rapidtypes.Type()
	rapid.Check(t, func(t *rapid.T) {
		a, b := gen.Draw(t, "a"), gen.Draw(t, "b")
		result := unify.Types(a, b)
		if result.Kind() != types.KindUnion {
			return
		}
		ms := result.UnionValues()
		for i, x := range ms {
			for _, y := range ms[i+1:] {
				require.Equal(t, types.Union(x, y), unify.Types(x, y), "members %v and %v of %v merge", x, y, result)
			}
		}
	})
}

func members(t types.Type) []types.Type {
	if t.Kind() == types.KindUnion {
		return t.UnionValues()
	}
	return []types.Type{t}
}

// containsEventual reports whether an output or promise occurs at any depth of t.
func containsEventual(t types.Type) bool {
	return containsFrom(t, map[types.Type]bool{})
}

func containsFrom(t types.Type, seen map[types.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	in := func(e types.Type) bool { return containsFrom(e, seen) }
	switch t.Kind() {
	case types.KindOutput, types.KindPromise:
		return true
	case types.KindList, types.KindSet, types.KindMap:
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
