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

// Package rapidmodel generates PCL model types for property-based tests.
package rapidmodel

import (
	"github.com/zclconf/go-cty/cty"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
)

// maxDepth bounds the nesting of collections, unions, and eventuals in a generated type.
const maxDepth = 3

// Type generates a model type. Leaves are the opaque types, constants of each opaque type, enums, none, and
// dynamic. Collections, objects, unions, optionals, outputs, and promises nest to a small depth, and an object
// may refer back to itself or to an object that encloses it, which makes the type recursive.
func Type() *rapid.Generator[model.Type] {
	return rapid.Custom(func(t *rapid.T) model.Type {
		return draw(t, 0, nil)
	})
}

func draw(t *rapid.T, depth int, enclosing []*model.ObjectType) model.Type {
	if depth >= maxDepth {
		return leaf(t)
	}
	switch rapid.IntRange(0, 11).Draw(t, "kind") {
	case 0:
		return model.NewListType(draw(t, depth+1, enclosing))
	case 1:
		return model.NewSetType(draw(t, depth+1, enclosing))
	case 2:
		return model.NewMapType(draw(t, depth+1, enclosing))
	case 3:
		return model.NewTupleType(elements(t, depth, enclosing, 0)...)
	case 4:
		return object(t, depth, enclosing)
	case 5:
		return model.NewOptionalType(draw(t, depth+1, enclosing))
	case 6:
		return model.NewUnionType(elements(t, depth, enclosing, 2)...)
	case 7:
		return model.NewOutputType(draw(t, depth+1, enclosing))
	case 8:
		return model.NewPromiseType(draw(t, depth+1, enclosing))
	case 9, 10:
		if len(enclosing) > 0 {
			return backReference(t, enclosing)
		}
		return leaf(t)
	default:
		return leaf(t)
	}
}

// object draws an object type. Its properties are drawn after the object exists, so that a property can refer
// back to the object.
func object(t *rapid.T, depth int, enclosing []*model.ObjectType) model.Type {
	properties := map[string]model.Type{}
	obj := model.NewObjectType(properties)
	enclosing = append(enclosing, obj)
	for i, property := range elements(t, depth, enclosing, 0) {
		properties[string(rune('a'+i))] = property
	}
	return obj
}

// backReference refers to an enclosing object through a wrapper that keeps the recursion finite for values.
func backReference(t *rapid.T, enclosing []*model.ObjectType) model.Type {
	target := rapid.SampledFrom(enclosing).Draw(t, "enclosing")
	switch rapid.IntRange(0, 2).Draw(t, "wrapper") {
	case 0:
		return model.NewOptionalType(target)
	case 1:
		return model.NewListType(target)
	default:
		return model.NewMapType(target)
	}
}

func elements(t *rapid.T, depth int, enclosing []*model.ObjectType, minimum int) []model.Type {
	count := rapid.IntRange(minimum, 3).Draw(t, "count")
	types := make([]model.Type, count)
	for i := range types {
		types[i] = draw(t, depth+1, enclosing)
	}
	return types
}

func leaf(t *rapid.T) model.Type {
	switch rapid.IntRange(0, 10).Draw(t, "leaf") {
	case 0:
		return model.BoolType
	case 1:
		return model.IntType
	case 2:
		return model.NumberType
	case 3:
		return model.StringType
	case 4:
		return model.NoneType
	case 5:
		return model.DynamicType
	case 6:
		return model.NewConstType(model.BoolType, cty.BoolVal(rapid.Bool().Draw(t, "bool")))
	case 7:
		return model.NewConstType(model.IntType, cty.NumberIntVal(rapid.Int64Range(-2, 2).Draw(t, "int")))
	case 8:
		return model.NewConstType(model.NumberType, cty.NumberFloatVal(
			rapid.SampledFrom([]float64{-1.5, 0, 0.5}).Draw(t, "number")))
	case 9:
		return model.NewConstType(model.StringType, cty.StringVal(
			rapid.SampledFrom([]string{"", "x", "y"}).Draw(t, "string")))
	default:
		if rapid.Bool().Draw(t, "stringEnum") {
			return model.NewEnumType("test:index:Color", model.StringType,
				[]cty.Value{cty.StringVal("x"), cty.StringVal("red")})
		}
		return model.NewEnumType("test:index:Level", model.IntType,
			[]cty.Value{cty.NumberIntVal(1), cty.NumberIntVal(3)})
	}
}
