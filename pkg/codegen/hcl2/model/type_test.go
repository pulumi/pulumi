// Copyright 2016-2020, Pulumi Corporation.
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

package model

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"
)

func testTraverse(t *testing.T, receiver Traversable, traverser hcl.Traverser, expected Traversable, expectDiags bool) {
	actual, diags := receiver.Traverse(traverser)
	assert.Equal(t, expected, actual)
	if expectDiags {
		assert.Greater(t, len(diags), 0)
	} else {
		assert.Equal(t, 0, len(diags))
	}
}

func TestAnyType(t *testing.T) {
	// Test that AnyType is assignable to and from itself.
	assert.True(t, AnyType.AssignableFrom(AnyType))

	// Test that AnyType is assignable from any type.
	assert.True(t, AnyType.AssignableFrom(BoolType))
	assert.True(t, AnyType.AssignableFrom(IntType))
	assert.True(t, AnyType.AssignableFrom(NumberType))
	assert.True(t, AnyType.AssignableFrom(StringType))

	assert.True(t, AnyType.AssignableFrom(NewOptionalType(BoolType)))
	assert.True(t, AnyType.AssignableFrom(NewOutputType(BoolType)))
	assert.True(t, AnyType.AssignableFrom(NewPromiseType(BoolType)))
	assert.True(t, AnyType.AssignableFrom(NewMapType(BoolType)))
	assert.True(t, AnyType.AssignableFrom(NewArrayType(BoolType)))
	assert.True(t, AnyType.AssignableFrom(NewUnionType(BoolType, IntType)))
	assert.True(t, AnyType.AssignableFrom(NewObjectType(map[string]Type{
		"bool": BoolType,
		"int":  IntType,
	})))

	// Test that AnyType is assignable to any type.
	assert.True(t, BoolType.AssignableFrom(AnyType))
	assert.True(t, IntType.AssignableFrom(AnyType))
	assert.True(t, NumberType.AssignableFrom(AnyType))
	assert.True(t, StringType.AssignableFrom(AnyType))

	assert.True(t, NewOptionalType(BoolType).AssignableFrom(AnyType))
	assert.True(t, NewOutputType(BoolType).AssignableFrom(AnyType))
	assert.True(t, NewPromiseType(BoolType).AssignableFrom(AnyType))
	assert.True(t, NewMapType(BoolType).AssignableFrom(AnyType))
	assert.True(t, NewArrayType(BoolType).AssignableFrom(AnyType))
	assert.True(t, NewUnionType(BoolType, IntType).AssignableFrom(AnyType))
	assert.True(t, NewObjectType(map[string]Type{
		"bool": BoolType,
		"int":  IntType,
	}).AssignableFrom(AnyType))

	// Test that traversals on AnyType work properly.
	testTraverse(t, AnyType, hcl.TraverseAttr{Name: "foo"}, AnyType, false)
	testTraverse(t, AnyType, hcl.TraverseIndex{Key: cty.StringVal("foo")}, AnyType, false)
	testTraverse(t, AnyType, hcl.TraverseIndex{Key: cty.NumberIntVal(0)}, AnyType, false)
	testTraverse(t, AnyType, hcl.TraverseIndex{Key: encapsulateType(AnyType)}, AnyType, false)
}

func TestOptionalType(t *testing.T) {
	typ := NewOptionalType(AnyType)

	// Test that creating an optional type with the same element type does not create a new type.
	typ2 := NewOptionalType(typ.ElementType)
	assert.EqualValues(t, typ, typ2)

	// Test that creating an optional type with an element type that is also optional does not create a new type.
	typ2 = NewOptionalType(typ)
	assert.Equal(t, typ, typ2)

	// Test that an optional type is assignable to and from itself.
	assert.True(t, typ.AssignableFrom(typ))

	// Test that an optional type is assignable from none.
	assert.True(t, typ.AssignableFrom(None))

	// Test that an optional type is assignable from its element type.
	assert.True(t, typ.AssignableFrom(typ.ElementType))

	// Test that an optional(T) is assignable from an U, where U is assignable to T.
	assert.True(t, typ.AssignableFrom(BoolType))

	// Test that an optional(T) is assignable from an optional(U), where U is assignable to T.
	assert.True(t, typ.AssignableFrom(NewOptionalType(BoolType)))

	// Test that traversing an optional(T) returns an optional(U), where U is the result of the inner traversal.
	testTraverse(t, typ, hcl.TraverseAttr{Name: "foo"}, typ, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.StringVal("foo")}, typ, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.NumberIntVal(0)}, typ, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: encapsulateType(typ)}, typ, false)
}

func TestOutputType(t *testing.T) {
	typ := NewOutputType(AnyType)

	// Test that creating an output type with the same element type does not create a new type.
	typ2 := NewOutputType(typ.ElementType)
	assert.EqualValues(t, typ, typ2)

	// Test that creating an output type with an element type that is also an output does not create a new type.
	typ2 = NewOutputType(typ)
	assert.Equal(t, typ, typ2)

	// Test that an output type is assignable to and from itself.
	assert.True(t, typ.AssignableFrom(typ))

	// Test that an output type is assignable from its element type.
	assert.True(t, typ.AssignableFrom(typ.ElementType))

	// Test that output(T) is assignable from U, where U is assignable to T.
	assert.True(t, typ.AssignableFrom(BoolType))

	// Test that output(T) is assignable from output(U), where U is assignable to T.
	assert.True(t, typ.AssignableFrom(NewOutputType(BoolType)))

	// Test that output(T) is assignable from promise(U), where U is assignable to T.
	assert.True(t, typ.AssignableFrom(NewPromiseType(BoolType)))

	// Test that output(T) is _not_ assignable from U, where U is not assignable to T.
	assert.False(t, NewOutputType(BoolType).AssignableFrom(IntType))

	// Test that output(T) is _not_ assignable from output(U), where U is not assignable to T.
	assert.False(t, NewOutputType(BoolType).AssignableFrom(NewOutputType(IntType)))

	// Test that output(T) is _not_ assignable from promise(U), where U is not assignable to T.
	assert.False(t, NewOutputType(BoolType).AssignableFrom(NewPromiseType(IntType)))

	// Test that traversing an output(T) returns an output(U), where U is the result of the inner traversal.
	testTraverse(t, typ, hcl.TraverseAttr{Name: "foo"}, typ, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.StringVal("foo")}, typ, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.NumberIntVal(0)}, typ, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: encapsulateType(typ)}, typ, false)
}

func TestPromiseType(t *testing.T) {
	typ := NewPromiseType(AnyType)

	// Test that creating an promise type with the same element type does not create a new type.
	typ2 := NewPromiseType(typ.ElementType)
	assert.EqualValues(t, typ, typ2)

	// Test that creating an promise type with an element type that is also a promise does not create a new type.
	typ2 = NewPromiseType(typ)
	assert.Equal(t, typ, typ2)

	// Test that a promise type is assignable to and from itself.
	assert.True(t, typ.AssignableFrom(typ))

	// Test that a promise type is assignable from its element type.
	assert.True(t, typ.AssignableFrom(typ.ElementType))

	// Test that promise(T) is assignable from U, where U is assignable to T.
	assert.True(t, typ.AssignableFrom(BoolType))

	// Test that promise(T) is assignable from promise(U), where U is assignable to T.
	assert.True(t, typ.AssignableFrom(NewPromiseType(BoolType)))

	// Test that promise(T) is _not_ assignable from U, where U is not assignable to T.
	assert.False(t, NewPromiseType(BoolType).AssignableFrom(IntType))

	// Test that promise(T) is _not_ assignable from promise(U), where U is not assignable to T.
	assert.False(t, NewPromiseType(BoolType).AssignableFrom(NewPromiseType(IntType)))

	// Test that traversing an promise(T) returns an promise(U), where U is the result of the inner traversal.
	testTraverse(t, typ, hcl.TraverseAttr{Name: "foo"}, typ, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.StringVal("foo")}, typ, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.NumberIntVal(0)}, typ, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: encapsulateType(typ)}, typ, false)
}

func TestMapType(t *testing.T) {
	typ := NewMapType(AnyType)

	// Test that creating an map type with the same element type does not create a new type.
	typ2 := NewMapType(typ.ElementType)
	assert.EqualValues(t, typ, typ2)

	// Test that a map type is assignable to and from itself.
	assert.True(t, typ.AssignableFrom(typ))

	// Test that map(T) is _not_ assignable from U, where U is not map(T).
	assert.False(t, typ.AssignableFrom(BoolType))

	// Test that map(T) is assignable from map(U), where U is assignable to T.
	assert.True(t, typ.AssignableFrom(NewMapType(BoolType)))

	// Test that map(T) is _not_ assignable from map(U), where U is not assignable to T.
	assert.False(t, NewMapType(BoolType).AssignableFrom(NewMapType(IntType)))

	// Test that traversing a map(T) with a string returns T.
	testTraverse(t, typ, hcl.TraverseAttr{Name: "foo"}, typ.ElementType, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.StringVal("foo")}, typ.ElementType, false)

	// Test that traversing a map(T) with a number or other type returns AnyType and an error.
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.NumberIntVal(0)}, AnyType, true)
	testTraverse(t, typ, hcl.TraverseIndex{Key: encapsulateType(typ)}, AnyType, true)
}

func TestArrayType(t *testing.T) {
	typ := NewArrayType(AnyType)

	// Test that creating an array type with the same element type does not create a new type.
	typ2 := NewArrayType(typ.ElementType)
	assert.EqualValues(t, typ, typ2)

	// Test that an array type is assignable to and from itself.
	assert.True(t, typ.AssignableFrom(typ))

	// Test that array(T) is _not_ assignable from U, where U is not array(T).
	assert.False(t, typ.AssignableFrom(BoolType))

	// Test that array(T) is assignable from array(U), where U is assignable to T.
	assert.True(t, typ.AssignableFrom(NewArrayType(BoolType)))

	// Test that array(T) is _not_ assignable from array(U), where U is not assignable to T.
	assert.False(t, NewArrayType(BoolType).AssignableFrom(NewArrayType(IntType)))

	// Test that traversing a array(T) with a string returns T.
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.NumberIntVal(0)}, typ.ElementType, false)

	// Test that traversing a array(T) with a string or other type returns AnyType and an error.
	testTraverse(t, typ, hcl.TraverseAttr{Name: "foo"}, AnyType, true)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.StringVal("foo")}, AnyType, true)
	testTraverse(t, typ, hcl.TraverseIndex{Key: encapsulateType(typ)}, AnyType, true)
}

func TestUnionType(t *testing.T) {
	typ := NewUnionType(BoolType, IntType, NumberType, StringType).(*UnionType)

	// Test that creating a union with the same element types does not create a new type.
	typ2 := NewUnionType(BoolType, IntType, NumberType, StringType).(*UnionType)
	assert.EqualValues(t, typ, typ2)

	// Test that creating a union with duplicated element types unifies all of the duplicated types.
	assert.Equal(t, BoolType, NewUnionType(BoolType, BoolType))
	assert.Equal(t, typ, NewUnionType(BoolType, IntType, IntType, NumberType, StringType))

	// Test that a union type is assignable to and from itself.
	assert.True(t, typ.AssignableFrom(typ))

	// Test that a union type is assignable from each of its element types.
	for _, et := range typ.ElementTypes {
		assert.True(t, typ.AssignableFrom(et))
	}

	// Test that union(T_0, ..., T_N) is assignable from union(U_0, ..., U_M) if union(T_0, ..., T_N) is assignable
	// from all of U_0 through U_M.
	assert.True(t, typ.AssignableFrom(NewUnionType(BoolType, IntType)))
	assert.True(t, NewUnionType(AnyType, StringType).AssignableFrom(typ))

	// Test that union(T_0, ..., T_N) is _not_ assignable from union(U_0, ..., U_M) if union(T_0, ..., T_N) is not
	// assignable from any of U_0 through U_M.
	assert.False(t, typ.AssignableFrom(NewUnionType(BoolType, NewOptionalType(NumberType))))

	// Test that traversing a union type always fails.
	testTraverse(t, typ, hcl.TraverseAttr{Name: "foo"}, AnyType, true)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.StringVal("foo")}, AnyType, true)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.NumberIntVal(0)}, AnyType, true)
	testTraverse(t, typ, hcl.TraverseIndex{Key: encapsulateType(typ)}, AnyType, true)
}

func TestObjectType(t *testing.T) {
	typ := NewObjectType(map[string]Type{
		"foo": BoolType,
		"bar": IntType,
		"baz": NumberType,
		"qux": NewOptionalType(BoolType),
	})

	// Test that creating a union with the same element types does not create a new type.
	typ2 := NewObjectType(map[string]Type{
		"foo": BoolType,
		"bar": IntType,
		"baz": NumberType,
		"qux": NewOptionalType(BoolType),
	})
	assert.EqualValues(t, typ, typ2)

	// Test that an object type is assignable to and from itself.
	assert.True(t, typ.AssignableFrom(typ))

	// Test that object(K_0=T_0, ..., K_N=T_N) is assignable from object(K_0=U_0, ..., K_N=U_N) if for each key K_i
	// T_i is assignable from U_i.
	assert.True(t, typ.AssignableFrom(NewObjectType(map[string]Type{
		"foo": BoolType,
		"bar": IntType,
		"baz": AnyType,
		"qux": BoolType,
	})))
	// Test that object(K_0=T_0, ..., K_N=T_N) is assignable from object(K_0=U_0, ..., K_M=U_M) if M < N and for each
	// key K_i where 0 <= i <= M, T_i is assignable from U_i and for each K_j where M < j <= N, T_j is optional.
	assert.True(t, typ.AssignableFrom(NewObjectType(map[string]Type{
		"foo": BoolType,
		"bar": IntType,
		"baz": NumberType,
	})))

	// Test that object(K_0=T_0, ..., K_N=T_N) is _not_ assignable from object(L_0=U_0, ..., L_M=U_M) if there exists
	// some key K_i
	// a matching key K_i exists and T_i is assignable from U_i.
	assert.False(t, typ.AssignableFrom(NewObjectType(map[string]Type{
		"foo": BoolType,
		"bar": IntType,
		"baz": NumberType,
		"qux": StringType,
	})))
	assert.False(t, typ.AssignableFrom(NewObjectType(map[string]Type{
		"foo": BoolType,
		"bar": IntType,
		"baz": StringType,
	})))

	// Test that traversing an object type with a property name K_i returns T_i.
	testTraverse(t, typ, hcl.TraverseAttr{Name: "foo"}, BoolType, false)
	testTraverse(t, typ, hcl.TraverseAttr{Name: "bar"}, IntType, false)
	testTraverse(t, typ, hcl.TraverseAttr{Name: "baz"}, NumberType, false)
	testTraverse(t, typ, hcl.TraverseAttr{Name: "qux"}, NewOptionalType(BoolType), false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.StringVal("foo")}, BoolType, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.StringVal("bar")}, IntType, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.StringVal("baz")}, NumberType, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.StringVal("qux")}, NewOptionalType(BoolType), false)

	// Test that traversing an object type with any other type fails.
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.NumberIntVal(0)}, AnyType, true)
	testTraverse(t, typ, hcl.TraverseIndex{Key: encapsulateType(typ)}, AnyType, true)
}

func TestInputType(t *testing.T) {
	assert.Equal(t, AnyType, InputType(AnyType))

	assert.Equal(t, NewUnionType(BoolType, NewOutputType(BoolType)), InputType(BoolType))

	assert.Equal(t, NewUnionType(
		NewOptionalType(NewUnionType(BoolType, NewOutputType(BoolType))),
		NewOutputType(NewOptionalType(BoolType))), InputType(NewOptionalType(BoolType)))

	assert.Equal(t, NewUnionType(
		NewPromiseType(NewUnionType(BoolType, NewOutputType(BoolType))),
		NewOutputType(BoolType)), InputType(NewPromiseType(BoolType)))

	assert.Equal(t, NewUnionType(
		NewMapType(NewUnionType(BoolType, NewOutputType(BoolType))),
		NewOutputType(NewMapType(BoolType))), InputType(NewMapType(BoolType)))

	assert.Equal(t, NewUnionType(
		NewArrayType(NewUnionType(BoolType, NewOutputType(BoolType))),
		NewOutputType(NewArrayType(BoolType))), InputType(NewArrayType(BoolType)))

	assert.Equal(t, NewUnionType(
		NewUnionType(BoolType, IntType, NewOutputType(BoolType), NewOutputType(IntType)),
		NewOutputType(NewUnionType(BoolType, IntType))),
		InputType(NewUnionType(BoolType, IntType)))

	assert.Equal(t, NewUnionType(
		NewObjectType(map[string]Type{"foo": NewUnionType(BoolType, NewOutputType(BoolType))}),
		NewOutputType(NewObjectType(map[string]Type{"foo": BoolType}))),
		InputType(NewObjectType(map[string]Type{"foo": BoolType})))
}
