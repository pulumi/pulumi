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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/types"
)

// rec builds one recursive type from a body over its own placeholder.
func rec(body func(types.Type) types.Type) types.Type {
	return types.Recursive(func(self types.Placeholder) []types.Type {
		return []types.Type{body(self())}
	})[0]
}

func obj(properties map[string]types.Type) types.Type { return types.Object(properties) }

// TestScalarTable checks §4.4 row by row.
func TestScalarTable(t *testing.T) {
	t.Parallel()

	scalars := []types.Type{types.Bool, types.Int, types.Number, types.String, types.ID}
	table := map[types.Type][]convert.Kind{
		types.Bool:   {convert.Safe, convert.Unsafe, convert.Unsafe, convert.Unsafe, convert.Unsafe},
		types.Int:    {convert.Unsafe, convert.Safe, convert.Unsafe, convert.Unsafe, convert.Unsafe},
		types.Number: {convert.Unsafe, convert.Safe, convert.Safe, convert.Unsafe, convert.Unsafe},
		types.String: {convert.Safe, convert.Safe, convert.Safe, convert.Safe, convert.Safe},
		types.ID:     {convert.Safe, convert.Safe, convert.Safe, convert.Safe, convert.Safe},
	}
	for dst, row := range table {
		for i, src := range scalars {
			assert.Equal(t, row[i], convert.To(dst, src), "%v <- %v", dst, src)
		}
	}
}

// TestRules checks the worked examples of §4.2, §4.3, and §12.1.
func TestRules(t *testing.T) {
	t.Parallel()

	color := types.Enum("pkg:Color", "red", "blue")
	level := types.Enum("pkg:Level", int64(1), int64(2))
	selfMap := rec(func(a types.Type) types.Type { return obj(map[string]types.Type{"k": types.Map(a)}) })
	selfList := rec(types.List)
	selfOrInt := rec(func(s types.Type) types.Type { return types.List(types.Union(s, types.Int)) })

	tests := []struct {
		dst, src types.Type
		want     convert.Kind
	}{
		// C-Eq, C-Dyn, C-DynSrc
		{types.None, types.None, convert.Safe},
		{types.Dynamic, types.None, convert.Safe},
		{types.Dynamic, types.List(types.Dynamic), convert.Safe},
		{types.String, types.Dynamic, convert.Unsafe},
		{types.None, types.Dynamic, convert.Unsafe},
		{types.None, types.String, convert.No},
		{types.String, types.None, convert.No},

		// Unions
		{types.Union(types.Int, types.String), types.Union(types.Int, types.Bool), convert.Safe},
		{types.Union(types.Bool, types.Dynamic), types.Dynamic, convert.Safe},
		{types.Union(types.Int, types.Bool), types.String, convert.Unsafe},
		{types.Union(types.Int, types.Bool), types.List(types.Int), convert.No},
		{types.Int, types.Union(types.Int, types.String), convert.Unsafe},
		{types.Int, types.Union(types.List(types.Int), types.None), convert.No},
		{types.Union(types.Int, types.None), types.None, convert.Safe},

		// Eventuals
		{types.Output(types.Dynamic), types.Dynamic, convert.Safe},
		{types.Output(types.Int), types.Dynamic, convert.Unsafe},
		{types.Output(types.Map(types.Bool)), types.Map(types.Output(types.Bool)), convert.Safe},
		{types.Promise(types.List(types.String)), types.List(types.Output(types.Bool)), convert.No},
		{types.Promise(types.List(types.Output(types.String))), types.List(types.Output(types.Bool)), convert.Safe},
		{types.Promise(types.Int), types.Output(types.Int), convert.No},
		{types.Promise(types.Int), types.Promise(types.Int), convert.Safe},
		{types.Output(types.Int), types.Promise(types.Int), convert.Safe},
		{types.Int, types.Output(types.Int), convert.No},
		{types.Int, types.Promise(types.Int), convert.No},

		// Constants and enums
		{types.Const("a"), types.Const("a"), convert.Safe},
		{types.Const("a"), types.Const("b"), convert.No},
		{types.Const(int64(1)), types.Const(1.0), convert.No},
		{types.Const("a"), types.String, convert.Unsafe},
		{types.Const("a"), types.Int, convert.Unsafe},
		{types.Const("a"), types.List(types.String), convert.No},
		{types.String, types.Const("a"), convert.Safe},
		{types.Int, types.Const("a"), convert.Unsafe},
		{types.Number, types.Const(int64(1)), convert.Safe},
		{color, types.Const("red"), convert.Safe},
		{color, types.Const("green"), convert.No},
		{color, types.Const(int64(1)), convert.No},
		{color, types.String, convert.Unsafe},
		{color, types.Int, convert.Unsafe},
		{color, types.List(types.String), convert.No},
		{color, level, convert.Unsafe},
		{types.String, color, convert.Safe},
		{types.Int, level, convert.Safe},
		{types.Number, level, convert.Safe},
		{types.Bool, level, convert.Unsafe},
		{types.Const("red"), color, convert.Unsafe},
		{types.Const("green"), color, convert.Unsafe},

		// Collections
		{types.List(types.Int), types.List(types.Int), convert.Safe},
		{types.List(types.Number), types.List(types.Int), convert.Safe},
		{types.List(types.Int), types.Set(types.Int), convert.Safe},
		{types.Set(types.Int), types.List(types.Int), convert.Unsafe},
		{types.List(types.Int), types.Tuple(types.Int, types.Int), convert.Safe},
		{types.List(types.Int), types.Tuple(), convert.Safe},
		{types.List(types.Int), types.Tuple(types.Int, types.String), convert.Unsafe},
		{types.List(types.Int), types.Tuple(types.Int, types.List(types.Int)), convert.No},
		{types.Set(types.Int), types.Tuple(types.Int), convert.Unsafe},
		{types.Tuple(types.Int, types.Int), types.Tuple(types.Int), convert.No},
		{types.Tuple(types.Int, types.Number), types.Tuple(types.Int, types.Int), convert.Safe},
		{types.Tuple(types.Int), types.List(types.Int), convert.Unsafe},
		{types.Tuple(), types.List(types.Int), convert.Unsafe},
		{types.Map(types.Int), obj(map[string]types.Type{"a": types.Int}), convert.Safe},
		{types.Map(types.Int), obj(map[string]types.Type{}), convert.Safe},
		{types.Map(types.Int), obj(map[string]types.Type{"a": types.Int, "b": types.List(types.Int)}), convert.No},
		{obj(map[string]types.Type{"a": types.Int}), types.Map(types.Int), convert.Unsafe},
		{
			obj(map[string]types.Type{"a": types.Int}),
			obj(map[string]types.Type{"a": types.Int, "b": types.Bool}),
			convert.Safe,
		},
		{obj(map[string]types.Type{"a": types.Int}), obj(map[string]types.Type{}), convert.No},
		{obj(map[string]types.Type{"a": types.Union(types.Int, types.None)}), obj(map[string]types.Type{}), convert.Safe},
		{obj(map[string]types.Type{"a": types.Int}), obj(map[string]types.Type{"a": types.String}), convert.Unsafe},
		{types.List(types.Int), types.Map(types.Int), convert.No},

		// Recursive types (§12.1)
		{selfMap, types.Map(selfMap), convert.Unsafe},
		{types.Map(selfMap), selfMap, convert.Unsafe},
		{selfList, selfOrInt, convert.Unsafe},
		{selfOrInt, selfList, convert.Safe},
		{types.List(selfList), selfList, convert.Safe},
		{rec(types.Set), selfList, convert.Unsafe},
		{selfList, rec(types.Set), convert.Safe},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, convert.To(tt.dst, tt.src), "%v <- %v", tt.dst, tt.src)
	}
}

func TestKindString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "No", convert.No.String())
	assert.Equal(t, "Unsafe", convert.Unsafe.String())
	assert.Equal(t, "Safe", convert.Safe.String())
	assert.Panics(t, func() { _ = convert.Kind(7).String() })
}
