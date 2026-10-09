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
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/types"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/unify"
)

func obj(properties map[string]types.Type) types.Type { return types.Object(properties) }

func optional(t types.Type) types.Type { return types.Union(t, types.None) }

// rec builds one recursive type from a body over its own placeholder.
func rec(body func(types.Type) types.Type) types.Type {
	return types.Recursive(func(self types.Placeholder) []types.Type {
		return []types.Type{body(self())}
	})[0]
}

// TestExamples checks the table of §5.4.
func TestExamples(t *testing.T) {
	t.Parallel()

	e := types.Enum("E", int64(1), int64(2))
	f := types.Enum("F", "a")
	asset, archive := types.Opaque("Asset"), types.Opaque("Archive")
	tests := []struct {
		inputs []types.Type
		want   types.Type
	}{
		{[]types.Type{types.Bool, types.Number}, types.Union(types.Bool, types.Number)},
		{[]types.Type{asset, archive}, types.Union(asset, archive)},
		{[]types.Type{asset, types.String}, types.Union(asset, types.String)},
		{[]types.Type{asset, types.Const("Asset")}, types.Union(asset, types.Const("Asset"))},
		{[]types.Type{asset, types.Output(asset)}, types.Output(asset)},
		{[]types.Type{asset, types.Dynamic}, types.Dynamic},
		{[]types.Type{types.String, types.Number}, types.String},
		{[]types.Type{types.Int, types.Const(int64(1))}, types.Int},
		{
			[]types.Type{types.Const(int64(1)), types.Const(int64(2))},
			types.Union(types.Const(int64(1)), types.Const(int64(2))),
		},
		{[]types.Type{types.Int, types.Const(0.01)}, types.Union(types.Int, types.Const(0.01))},
		{
			[]types.Type{types.Map(types.Const(0.01)), types.Map(types.Int)},
			types.Map(types.Union(types.Int, types.Const(0.01))),
		},
		{
			[]types.Type{types.Tuple(types.Number), types.Tuple(types.Bool)},
			types.Tuple(types.Union(types.Bool, types.Number)),
		},
		{
			[]types.Type{types.Tuple(types.Int), types.Tuple(types.Int, types.Bool)},
			types.List(types.Union(types.Bool, types.Int)),
		},
		{[]types.Type{types.List(types.String), types.Set(types.Bool)}, types.List(types.String)},
		{[]types.Type{types.Set(types.Bool), types.Tuple(types.Int)}, types.List(types.Union(types.Bool, types.Int))},
		{[]types.Type{types.Set(types.Bool), types.Set(types.Int)}, types.Set(types.Union(types.Bool, types.Int))},
		{
			[]types.Type{obj(map[string]types.Type{"a": types.Bool}), types.Map(types.Int)},
			types.Map(types.Union(types.Bool, types.Int)),
		},
		{
			[]types.Type{obj(map[string]types.Type{"a": types.Int}), obj(map[string]types.Type{"b": types.Bool})},
			obj(map[string]types.Type{"a": optional(types.Int), "b": optional(types.Bool)}),
		},
		{
			[]types.Type{obj(map[string]types.Type{}), obj(map[string]types.Type{"a": types.Int})},
			obj(map[string]types.Type{"a": optional(types.Int)}),
		},
		{[]types.Type{types.Output(types.Bool), types.Bool}, types.Output(types.Bool)},
		{[]types.Type{types.Output(types.Int), types.String}, types.Output(types.String)},
		{[]types.Type{types.Promise(types.Bool), types.Output(types.String)}, types.Output(types.String)},
		// §5.4 lists output(map(union(const(false), const(true)))) for this row, but U-Scalar puts bool above the
		// two constants, and the model implementation agrees with the rules.
		{
			[]types.Type{
				types.Map(types.Output(types.Bool)),
				types.Output(types.Map(types.Union(types.Const(false), types.Const(true)))),
			},
			types.Output(types.Map(types.Bool)),
		},
		{
			[]types.Type{optional(types.Map(types.Const(int64(0)))), types.Map(types.Const(int64(1)))},
			types.Union(types.None, types.Map(types.Union(types.Const(int64(0)), types.Const(int64(1))))),
		},
		{[]types.Type{types.Bool, types.Dynamic}, types.Dynamic},
		{[]types.Type{optional(types.Int), types.Dynamic}, optional(types.Dynamic)},
		{[]types.Type{types.None, types.Bool}, optional(types.Bool)},
		{[]types.Type{e, types.Int}, types.Int},
		{[]types.Type{e, types.Const(int64(1))}, e},
		{[]types.Type{e, f}, types.Union(e, f)},
		{[]types.Type{types.Union(types.Int, types.String), types.Int}, types.String},
		{[]types.Type{types.String, types.ID}, types.String},
		{[]types.Type{types.List(types.Int), types.Map(types.Int)}, types.Union(types.List(types.Int), types.Map(types.Int))},
		{[]types.Type{types.Set(types.String), types.Tuple(types.Int), types.List(types.Bool)}, types.List(types.String)},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, unify.Types(tt.inputs...), "unify%v", tt.inputs)
	}
}

// TestNullaryAndUnary checks U-Empty and U-Eq: a union input is returned as is, not merged.
func TestNullaryAndUnary(t *testing.T) {
	t.Parallel()

	assert.Equal(t, types.None, unify.Types())
	u := types.Union(types.Int, types.String)
	assert.Equal(t, u, unify.Types(u))
	assert.Equal(t, u, unify.Types(u, u, u))
	assert.Equal(t, types.String, unify.Types(u, types.Int))
}

// TestNotAssociative checks the witness of §6: the n-ary result differs from a binary fold.
func TestNotAssociative(t *testing.T) {
	t.Parallel()

	a, b := obj(map[string]types.Type{"a": types.Int}), obj(map[string]types.Type{"b": types.Bool})
	m := types.Map(types.Const(int64(0)))
	assert.Equal(t, types.Map(types.Union(types.Bool, types.Int)), unify.Types(a, b, m))
	assert.Equal(t, types.Map(types.Union(types.Bool, types.Int, types.None)), unify.Types(unify.Types(a, b), m))
}

// TestOutputDoesNotDistribute checks the S8 witness of §6 for outputs over inputs with eventuals.
func TestOutputDoesNotDistribute(t *testing.T) {
	t.Parallel()

	a, b := types.List(types.Output(types.None)), types.List(types.Dynamic)
	assert.Equal(t, types.Output(types.List(types.Dynamic)), types.Output(unify.Types(a, b)))
	assert.Equal(t, types.Output(types.List(optional(types.Dynamic))), unify.Types(types.Output(a), types.Output(b)))
}

// TestRecursive checks the examples of §12.2.
func TestRecursive(t *testing.T) {
	t.Parallel()

	// With a = {k: map(a)}, unify(map(a), a) is map(union(a, map(a))).
	a := rec(func(a types.Type) types.Type { return obj(map[string]types.Type{"k": types.Map(a)}) })
	assert.Equal(t, types.Map(types.Union(a, types.Map(a))), unify.Types(types.Map(a), a))
	assert.Equal(t, types.Map(types.Union(a, types.Map(a))), unify.Types(a, types.Map(a)))

	// With a = {a: optional(a), b: map(a)} and b = {a: bool, b: map(b)}, the merge r has r.b = map(r).
	a = rec(func(a types.Type) types.Type {
		return obj(map[string]types.Type{"a": optional(a), "b": types.Map(a)})
	})
	b := rec(func(b types.Type) types.Type {
		return obj(map[string]types.Type{"a": types.Bool, "b": types.Map(b)})
	})
	r := unify.Types(a, b)
	want := rec(func(r types.Type) types.Type {
		return obj(map[string]types.Type{"a": types.Union(types.None, types.Bool, a), "b": types.Map(r)})
	})
	require.Equal(t, want, r)
	assert.Equal(t, map[string]types.Type{"a": types.Union(types.None, types.Bool, a), "b": types.Map(r)},
		maps.Collect(r.ObjectValues()))
	assert.Equal(t, types.Map(r), unify.Types(types.Map(a), types.Map(b)))

	// Recursive lists merge element-wise through the cycle.
	ints := rec(func(s types.Type) types.Type { return types.List(types.Union(s, types.Int)) })
	strings := rec(func(s types.Type) types.Type { return types.List(types.Union(s, types.String)) })
	assert.Equal(t, rec(func(s types.Type) types.Type { return types.List(types.Union(s, types.String)) }),
		unify.Types(ints, strings))
	assert.Equal(t, ints, unify.Types(ints, rec(types.List)))
}
