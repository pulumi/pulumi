// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi-fabric/pkg/compiler/ast"
	"github.com/pulumi/pulumi-fabric/pkg/compiler/symbols"
	"github.com/pulumi/pulumi-fabric/pkg/pack"
	"github.com/pulumi/pulumi-fabric/pkg/tokens"
)

var objectArray = symbols.NewArrayType(Object)
var objectMap = symbols.NewMapType(Object, Object)

func newTestClass(name tokens.Name, extends symbols.Type, implements symbols.Types) *symbols.Class {
	pkg := symbols.NewPackageSym(&pack.Package{Name: "test"})
	mod := symbols.NewModuleSym(&ast.Module{
		DefinitionNode: ast.DefinitionNode{
			Name: &ast.Identifier{Ident: "test"},
		},
	}, pkg)
	return symbols.NewClassSym(&ast.Class{
		ModuleMemberNode: ast.ModuleMemberNode{
			DefinitionNode: ast.DefinitionNode{
				Name: &ast.Identifier{Ident: name},
			},
		},
	}, mod, extends, implements)
}

func assertCanConvert(t *testing.T, from symbols.Type, to symbols.Type) {
	assert.True(t, CanConvert(from, to), fmt.Sprintf("convert(%v,%v)", from, to))
}

func assertCannotConvert(t *testing.T, from symbols.Type, to symbols.Type) {
	assert.False(t, CanConvert(from, to), fmt.Sprintf("convert(%v,%v)", from, to))
}

// TestIdentityConversions tests converting types to themselves.
func TestIdentityConversions(t *testing.T) {
	t.Parallel()

	for _, prim := range Primitives {
		assertCanConvert(t, prim, prim)
	}

	assertCanConvert(t, objectArray, objectArray)
	assertCanConvert(t, objectMap, objectMap)

	class := newTestClass("class", nil, nil)
	assertCanConvert(t, class, class)
}

// TestObjectConversions tests converting a bunch of different types to "any".
func TestObjectConversions(t *testing.T) {
	t.Parallel()

	for _, prim := range Primitives {
		assertCanConvert(t, prim, Object)
	}

	assertCanConvert(t, objectArray, Object)
	assertCanConvert(t, objectMap, Object)

	class := newTestClass("class", nil, nil)
	assertCanConvert(t, class, Object)
}

// TestNullConversions tests converting to and from "null".
func TestNullConversions(t *testing.T) {
	t.Parallel()

	for _, prim := range Primitives {
		assertCanConvert(t, prim, Null)
		assertCanConvert(t, Null, prim)
	}

	assertCanConvert(t, objectArray, Null)
	assertCanConvert(t, Null, objectArray)
	assertCanConvert(t, objectMap, Null)
	assertCanConvert(t, Null, objectMap)

	class := newTestClass("class", nil, nil)
	assertCanConvert(t, class, Null)
	assertCanConvert(t, Null, class)
}

// TestClassConversions tests converting classes to their base types.
func TestClassConversions(t *testing.T) {
	t.Parallel()

	base := newTestClass("base", nil, nil)

	// A simple extends case.
	{
		derived := newTestClass("derived", base, nil)
		assertCanConvert(t, derived, Object)
		assertCanConvert(t, derived, base)
	}

	// An implements case.
	{
		derived := newTestClass("derived", nil, symbols.Types{base})
		assertCanConvert(t, derived, Object)
		assertCanConvert(t, derived, base)
	}

	// A case where the base is different, but an implement exists.
	{
		base2 := newTestClass("base2", nil, nil)
		base3 := newTestClass("base3", nil, nil)
		base4 := newTestClass("base4", nil, nil)
		derived := newTestClass("derived", base2, symbols.Types{base3, base, base4})
		assertCanConvert(t, derived, Object)
		assertCanConvert(t, derived, base)
	}

	// Negative test; cannot convert to primitives or incorrect bases.
	{
		base2 := newTestClass("base2", nil, nil)
		derived := newTestClass("derived", base2, nil)
		for _, prim := range Primitives {
			if prim != Object && prim != Null && prim != Dynamic {
				assertCannotConvert(t, derived, prim)
			}
		}
		assertCannotConvert(t, derived, base)
	}
}

// TestPointerConversions tests pointers converting to their element types.
func TestPointerConversions(t *testing.T) {
	t.Parallel()

	for _, prim := range Primitives {
		ptr := symbols.NewPointerType(prim)
		for i := 0; i < 3; i++ { // test that multiple levels of pointers convert.
			assertCanConvert(t, ptr, prim)
			ptr = symbols.NewPointerType(ptr)
		}
	}

	assertCanConvert(t, symbols.NewPointerType(objectArray), objectArray)
	assertCanConvert(t, symbols.NewPointerType(objectMap), objectMap)

	class := newTestClass("class", nil, nil)
	pclass := symbols.NewPointerType(class)
	for i := 0; i < 3; i++ {
		assertCanConvert(t, pclass, class)
		pclass = symbols.NewPointerType(pclass)
	}
}

// TestArrayConversions tests converting between structurally identical array types.
func TestArrayConversions(t *testing.T) {
	t.Parallel()

	// Simple primitive cases:
	for _, prim := range Primitives {
		arr1 := symbols.NewArrayType(prim)
		assertCanConvert(t, arr1, Object)
		arr2 := symbols.NewArrayType(prim)
		assertCanConvert(t, arr1, arr2)
	}

	// Check that classes work for identity, but not conversions (arrays are not covariant):
	base := newTestClass("base", nil, nil)
	derived := newTestClass("derived", base, nil)
	arr1 := symbols.NewArrayType(base)
	arr2 := symbols.NewArrayType(base)
	assertCanConvert(t, arr1, arr2)
	arr3 := symbols.NewArrayType(derived)
	assertCannotConvert(t, arr2, arr3)
	assertCannotConvert(t, arr3, arr2)

	// And also ensure that covariant conversions for primitive "any" isn't allowed either:
	arr4 := objectArray
	assertCannotConvert(t, arr3, arr4)
	assertCannotConvert(t, arr4, arr3)
}

// TestMapConversions tests converting between structurally identical map types.
func TestMapConversions(t *testing.T) {
	t.Parallel()

	// Map types with the same key and element types can convert.
	for _, prim := range Primitives {
		map1 := symbols.NewMapType(String, prim)
		assertCanConvert(t, map1, Object)
		map2 := symbols.NewMapType(String, prim)
		assertCanConvert(t, map1, map2)
	}

	// Check that classes work for identity, but not conversions (maps are not covariant):
	base := newTestClass("base", nil, nil)
	derived := newTestClass("derived", base, nil)
	map1 := symbols.NewMapType(String, base)
	map2 := symbols.NewMapType(String, base)
	assertCanConvert(t, map1, map2)
	map3 := symbols.NewMapType(String, derived)
	assertCannotConvert(t, map2, map3)
	assertCannotConvert(t, map3, map2)

	// And also ensure that covariant conversions for primitive "any" isn't allowed either:
	map4 := symbols.NewMapType(String, Object)
	assertCannotConvert(t, map3, map4)
	assertCannotConvert(t, map4, map3)
}

// TestFuncConversions tests converting between structurally identical or safely variant function types.
func TestFuncConversions(t *testing.T) {
	t.Parallel()

	// Empty functions convert to each other.
	{
		func1 := symbols.NewFunctionType(nil, nil)
		assertCanConvert(t, func1, Object)
		func2 := symbols.NewFunctionType(nil, nil)
		assertCanConvert(t, func1, func2)
		assertCanConvert(t, func2, func1)
	}

	// Simple equivalent functions convert to each other.
	for _, param1 := range Primitives {
		for _, param2 := range Primitives {
			func1 := symbols.NewFunctionType([]symbols.Type{param1, param2}, nil)
			assertCanConvert(t, func1, Object)
			func2 := symbols.NewFunctionType([]symbols.Type{param1, param2}, nil)
			assertCanConvert(t, func1, func2)
			assertCanConvert(t, func2, func1)

			for _, ret := range Primitives {
				func3 := symbols.NewFunctionType([]symbols.Type{param1, param2}, ret)
				assertCanConvert(t, func3, Object)
				func4 := symbols.NewFunctionType([]symbols.Type{param1, param2}, ret)
				assertCanConvert(t, func3, func4)
				assertCanConvert(t, func4, func3)
			}
		}
	}

	base := newTestClass("base", nil, nil)
	derived := newTestClass("derived", base, nil)

	// Parameter types are contravariant (source may be weaker).
	{
		// Simple primitive case.
		strong1 := symbols.NewFunctionType([]symbols.Type{String, Number}, String)
		weak1 := symbols.NewFunctionType([]symbols.Type{Object, Object}, String)
		assertCanConvert(t, weak1, strong1)
		assertCannotConvert(t, strong1, weak1)

		// More complex subtyping case.
		strong2 := symbols.NewFunctionType([]symbols.Type{derived, derived}, String)
		weak2 := symbols.NewFunctionType([]symbols.Type{base, base}, String)
		assertCanConvert(t, weak2, strong2)
		assertCannotConvert(t, strong2, weak2)
	}

	// Return types are covariant (source may be strengthened).
	{
		// Simple primitive case.
		strong1 := symbols.NewFunctionType([]symbols.Type{String, Number}, String)
		weak1 := symbols.NewFunctionType([]symbols.Type{String, Number}, Object)
		assertCanConvert(t, strong1, weak1)
		assertCannotConvert(t, weak1, strong1)

		// More complex subtyping case.
		strong2 := symbols.NewFunctionType([]symbols.Type{String, Number}, derived)
		weak2 := symbols.NewFunctionType([]symbols.Type{String, Number}, base)
		assertCanConvert(t, strong2, weak2)
		assertCannotConvert(t, weak2, strong2)
	}

	// Both can happen at once.
	{
		from := symbols.NewFunctionType([]symbols.Type{Object, base, Object}, derived)
		to := symbols.NewFunctionType([]symbols.Type{String, derived, Number}, base)
		assertCanConvert(t, from, to)
		assertCannotConvert(t, to, from)
	}
}
