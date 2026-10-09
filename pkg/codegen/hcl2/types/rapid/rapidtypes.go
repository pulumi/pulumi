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

// Package rapidtypes generates PCL types for property-based tests.
package rapidtypes

import (
	"math"

	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/types"
)

// maxDepth bounds the nesting of collections, unions, eventuals, and recursive groups in a generated type.
const maxDepth = 4

// Type generates a type. Leaves are the scalars, constants of each base, and enums. Collections, tuples, objects,
// unions, optionals, outputs, promises, and recursive groups nest to a small depth. Inside a group, a placeholder
// may appear as a leaf once it is under a list, set, map, tuple, or object, and a body may be another placeholder
// of the group as long as the aliases form no cycle.
func Type() *rapid.Generator[types.Type] {
	return rapid.Custom(func(t *rapid.T) types.Type {
		return draw(t, 0, nil, false)
	})
}

// Leaf generates a type with no children: a scalar, a constant, or an enum.
func Leaf() *rapid.Generator[types.Type] {
	return rapid.Custom(func(t *rapid.T) types.Type {
		return leaf(t, nil, false)
	})
}

// Const generates a constant type.
func Const() *rapid.Generator[types.Type] {
	return rapid.Custom(constant)
}

// Enum generates an enum type.
func Enum() *rapid.Generator[types.Type] {
	return rapid.Custom(enum)
}

// draw generates a type. vars holds the placeholders of the innermost group, and guarded says whether a placeholder
// may appear here: unions and eventuals do not guard a placeholder, since a type that reaches itself through them
// alone has no unfolding.
func draw(t *rapid.T, depth int, vars []types.Type, guarded bool) types.Type {
	if depth >= maxDepth {
		return leaf(t, vars, guarded)
	}
	switch rapid.IntRange(0, 10).Draw(t, "kind") {
	case 0:
		return types.List(draw(t, depth+1, vars, true))
	case 1:
		return types.Set(draw(t, depth+1, vars, true))
	case 2:
		return types.Map(draw(t, depth+1, vars, true))
	case 3:
		return types.Tuple(elements(t, depth, vars, true, 0)...)
	case 4:
		elems := elements(t, depth, vars, true, 0)
		properties := make(map[string]types.Type, len(elems))
		for i, property := range elems {
			properties[string(rune('a'+i))] = property
		}
		return types.Object(properties)
	case 5:
		return types.Union(draw(t, depth+1, vars, guarded), types.None)
	case 6:
		return types.Union(elements(t, depth, vars, guarded, 2)...)
	case 7:
		return types.Output(draw(t, depth+1, vars, guarded))
	case 8:
		return types.Promise(draw(t, depth+1, vars, guarded))
	case 9:
		return recursive(t, depth)
	default:
		return leaf(t, vars, guarded)
	}
}

// recursive draws a group of one to three mutually recursive types and returns one of them. A body may alias a
// later placeholder of the group, so the aliases form no cycle.
func recursive(t *rapid.T, depth int) types.Type {
	n := rapid.IntRange(1, 3).Draw(t, "slots")
	group := types.Recursive(func(self types.Placeholder) []types.Type {
		vars := make([]types.Type, n)
		for i := range vars {
			vars[i] = self()
		}
		bodies := make([]types.Type, n)
		for i := range bodies {
			if i < n-1 && rapid.IntRange(0, 4).Draw(t, "alias") == 0 {
				bodies[i] = vars[rapid.IntRange(i+1, n-1).Draw(t, "target")]
			} else {
				bodies[i] = draw(t, depth+1, vars, false)
			}
		}
		return bodies
	})
	return group[rapid.IntRange(0, n-1).Draw(t, "member")]
}

func elements(t *rapid.T, depth int, vars []types.Type, guarded bool, minimum int) []types.Type {
	count := rapid.IntRange(minimum, 3).Draw(t, "count")
	elems := make([]types.Type, count)
	for i := range elems {
		elems[i] = draw(t, depth+1, vars, guarded)
	}
	return elems
}

func leaf(t *rapid.T, vars []types.Type, guarded bool) types.Type {
	if guarded && len(vars) > 0 && rapid.IntRange(0, 2).Draw(t, "placeholder") == 0 {
		return rapid.SampledFrom(vars).Draw(t, "var")
	}
	switch rapid.IntRange(0, 9).Draw(t, "leaf") {
	case 0:
		return types.None
	case 1:
		return types.Bool
	case 2:
		return types.Int
	case 3:
		return types.Number
	case 4:
		return types.String
	case 5:
		return types.ID
	case 6:
		return types.Dynamic
	case 7:
		return constant(t)
	case 8:
		return enum(t)
	default:
		return opaque(t)
	}
}

var numbers = []float64{-1.5, math.Copysign(0, -1), 0, 0.5, 1, math.Inf(1), math.NaN()}

var strings = []string{"", "x", "y"}

func constant(t *rapid.T) types.Type {
	switch rapid.IntRange(0, 3).Draw(t, "base") {
	case 0:
		return types.Const(rapid.Bool().Draw(t, "bool"))
	case 1:
		return types.Const(rapid.Int64Range(-2, 2).Draw(t, "int"))
	case 2:
		return types.Const(rapid.SampledFrom(numbers).Draw(t, "number"))
	default:
		return types.Const(rapid.SampledFrom(strings).Draw(t, "string"))
	}
}

var opaques = []string{"Asset", "pkg:index:Token"}

func opaque(t *rapid.T) types.Type {
	return types.Opaque(rapid.SampledFrom(opaques).Draw(t, "opaque"))
}

// enum draws one of two fixed enums, so that every enum with a given token has the same values.
func enum(t *rapid.T) types.Type {
	if rapid.Bool().Draw(t, "stringEnum") {
		return types.Enum("test:index:Color", "x", "red")
	}
	return types.Enum("test:index:Level", int64(1), int64(3))
}
