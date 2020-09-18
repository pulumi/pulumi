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

func TestDynamicType(t *testing.T) {
	// Test that DynamicType is assignable to and from itself.
	assert.True(t, DynamicType.AssignableFrom(DynamicType))

	// Test that DynamicType is assignable from any type.
	assert.True(t, DynamicType.AssignableFrom(BoolType))
	assert.True(t, DynamicType.AssignableFrom(IntType))
	assert.True(t, DynamicType.AssignableFrom(NumberType))
	assert.True(t, DynamicType.AssignableFrom(StringType))

	assert.True(t, DynamicType.AssignableFrom(NewOptionalType(BoolType)))
	assert.True(t, DynamicType.AssignableFrom(NewOutputType(BoolType)))
	assert.True(t, DynamicType.AssignableFrom(NewPromiseType(BoolType)))
	assert.True(t, DynamicType.AssignableFrom(NewMapType(BoolType)))
	assert.True(t, DynamicType.AssignableFrom(NewListType(BoolType)))
	assert.True(t, DynamicType.AssignableFrom(NewUnionType(BoolType, IntType)))
	assert.True(t, DynamicType.AssignableFrom(NewObjectType(map[string]Type{
		"bool": BoolType,
		"int":  IntType,
	})))

	// Test that DynamicType is assignable to certain types and not assignable to others.
	assert.True(t, NewOptionalType(DynamicType).AssignableFrom(DynamicType))
	assert.True(t, NewOutputType(DynamicType).AssignableFrom(DynamicType))
	assert.True(t, NewPromiseType(DynamicType).AssignableFrom(DynamicType))
	assert.True(t, NewUnionType(BoolType, DynamicType).AssignableFrom(DynamicType))

	assert.False(t, BoolType.AssignableFrom(DynamicType))
	assert.False(t, IntType.AssignableFrom(DynamicType))
	assert.False(t, NumberType.AssignableFrom(DynamicType))
	assert.False(t, StringType.AssignableFrom(DynamicType))

	assert.False(t, NewOptionalType(BoolType).AssignableFrom(DynamicType))
	assert.False(t, NewOutputType(BoolType).AssignableFrom(DynamicType))
	assert.False(t, NewPromiseType(BoolType).AssignableFrom(DynamicType))
	assert.False(t, NewMapType(BoolType).AssignableFrom(DynamicType))
	assert.False(t, NewListType(BoolType).AssignableFrom(DynamicType))
	assert.False(t, NewUnionType(BoolType, IntType).AssignableFrom(DynamicType))
	assert.False(t, NewObjectType(map[string]Type{
		"bool": BoolType,
		"int":  IntType,
	}).AssignableFrom(DynamicType))

	// Test that DynamicType is convertible from any type.
	assert.True(t, DynamicType.ConversionFrom(BoolType).Exists())
	assert.True(t, DynamicType.ConversionFrom(IntType).Exists())
	assert.True(t, DynamicType.ConversionFrom(NumberType).Exists())
	assert.True(t, DynamicType.ConversionFrom(StringType).Exists())

	assert.True(t, DynamicType.ConversionFrom(NewOptionalType(BoolType)).Exists())
	assert.True(t, DynamicType.ConversionFrom(NewOutputType(BoolType)).Exists())
	assert.True(t, DynamicType.ConversionFrom(NewPromiseType(BoolType)).Exists())
	assert.True(t, DynamicType.ConversionFrom(NewMapType(BoolType)).Exists())
	assert.True(t, DynamicType.ConversionFrom(NewListType(BoolType)).Exists())
	assert.True(t, DynamicType.ConversionFrom(NewUnionType(BoolType, IntType)).Exists())
	assert.True(t, DynamicType.ConversionFrom(NewObjectType(map[string]Type{
		"bool": BoolType,
		"int":  IntType,
	})).Exists())

	// Test that DynamicType is convertible to any type.
	assert.True(t, BoolType.ConversionFrom(DynamicType).Exists())
	assert.True(t, IntType.ConversionFrom(DynamicType).Exists())
	assert.True(t, NumberType.ConversionFrom(DynamicType).Exists())
	assert.True(t, StringType.ConversionFrom(DynamicType).Exists())

	assert.True(t, NewOptionalType(BoolType).ConversionFrom(DynamicType).Exists())
	assert.True(t, NewOutputType(BoolType).ConversionFrom(DynamicType).Exists())
	assert.True(t, NewPromiseType(BoolType).ConversionFrom(DynamicType).Exists())
	assert.True(t, NewMapType(BoolType).ConversionFrom(DynamicType).Exists())
	assert.True(t, NewListType(BoolType).ConversionFrom(DynamicType).Exists())
	assert.True(t, NewUnionType(BoolType, IntType).ConversionFrom(DynamicType).Exists())
	assert.True(t, NewObjectType(map[string]Type{
		"bool": BoolType,
		"int":  IntType,
	}).ConversionFrom(DynamicType).Exists())

	// Test that traversals on DynamicType always succeed.
	testTraverse(t, DynamicType, hcl.TraverseAttr{Name: "foo"}, DynamicType, false)
	testTraverse(t, DynamicType, hcl.TraverseIndex{Key: cty.StringVal("foo")}, DynamicType, false)
	testTraverse(t, DynamicType, hcl.TraverseIndex{Key: cty.NumberIntVal(0)}, DynamicType, false)
	testTraverse(t, DynamicType, hcl.TraverseIndex{Key: encapsulateType(DynamicType)}, DynamicType, false)
}

func TestOptionalType(t *testing.T) {
	typ := NewOptionalType(DynamicType)

	// Test that creating an optional type with the same element type does not create a new type.
	typ2 := NewOptionalType(DynamicType)
	assert.EqualValues(t, typ, typ2)

	// Test that creating an optional type with an element type that is also optional does not create a new type.
	typ2 = NewOptionalType(typ)
	assert.Equal(t, typ, typ2)

	// Test that an optional type is assignable to and from itself.
	assert.True(t, typ.AssignableFrom(typ))

	// Test that an optional type is assignable from none.
	assert.True(t, typ.AssignableFrom(NoneType))

	// Test that an optional type is assignable from its element type.
	assert.True(t, NewOptionalType(StringType).AssignableFrom(StringType))

	// Test that an optional(T) is assignable from an U, where U is assignable to T.
	assert.True(t, typ.AssignableFrom(BoolType))

	// Test that an optional(T) is assignable from an optional(U), where U is assignable to T.
	assert.True(t, typ.AssignableFrom(NewOptionalType(BoolType)))

	// Test that traversing an optional(T) returns an optional(U), where U is the result of the inner traversal.
	typ = NewOptionalType(NewMapType(StringType))
	testTraverse(t, typ, hcl.TraverseAttr{Name: "foo"}, NewOptionalType(StringType), false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.StringVal("foo")}, NewOptionalType(StringType), false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.NumberIntVal(0)}, NewOptionalType(StringType), false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: encapsulateType(IntType)}, NewOptionalType(StringType), false)
}

func TestOutputType(t *testing.T) {
	typ := NewOutputType(DynamicType)

	// Test that creating an output type with the same element type does not create a new type.
	typ2 := NewOutputType(typ.ElementType)
	assert.EqualValues(t, typ, typ2)

	// Test that creating an output type with an element type that is also an output does not create a new type.
	typ2 = NewOutputType(typ)
	assert.Equal(t, typ, typ2)

	// Test that an output type is assignable to and from itself.
	assert.True(t, typ.AssignableFrom(typ))

	// Test that an output type is assignable from its element type.
	assert.True(t, NewOutputType(StringType).AssignableFrom(StringType))

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
	typ = NewOutputType(NewMapType(StringType))
	testTraverse(t, typ, hcl.TraverseAttr{Name: "foo"}, NewOutputType(StringType), false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.StringVal("foo")}, NewOutputType(StringType), false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.NumberIntVal(0)}, NewOutputType(StringType), false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: encapsulateType(IntType)}, NewOutputType(StringType), false)

	// Test that ResolveOutputs correctly handles nested outputs.
	assert.Equal(t, NewOptionalType(BoolType), ResolveOutputs(NewOptionalType(NewOutputType(BoolType))))
	assert.Equal(t, BoolType, ResolveOutputs(NewOutputType(BoolType)))
	assert.Equal(t, BoolType, ResolveOutputs(NewPromiseType(BoolType)))
	assert.Equal(t, NewMapType(BoolType), ResolveOutputs(NewMapType(NewOutputType(BoolType))))
	assert.Equal(t, NewListType(BoolType), ResolveOutputs(NewListType(NewOutputType(BoolType))))
	assert.Equal(t, NewUnionType(BoolType, IntType), ResolveOutputs(NewUnionType(NewOutputType(BoolType),
		NewOutputType(IntType))))
	assert.Equal(t, NewObjectType(map[string]Type{
		"bool": BoolType,
		"int":  IntType,
	}), ResolveOutputs(NewObjectType(map[string]Type{
		"bool": NewOutputType(BoolType),
		"int":  NewOutputType(IntType),
	})))

	// Test that NewOutputType correctly handles nested outputs.
	assert.Equal(t, NewOutputType(NewOptionalType(BoolType)), NewOutputType(NewOptionalType(NewOutputType(BoolType))))
	assert.Equal(t, NewOutputType(BoolType), NewOutputType(NewOutputType(BoolType)))
	assert.Equal(t, NewOutputType(BoolType), NewOutputType(NewPromiseType(BoolType)))
	assert.Equal(t, NewOutputType(NewMapType(BoolType)), NewOutputType(NewMapType(NewOutputType(BoolType))))
	assert.Equal(t, NewOutputType(NewListType(BoolType)), NewOutputType(NewListType(NewOutputType(BoolType))))
	assert.Equal(t, NewOutputType(NewUnionType(BoolType, IntType)),
		NewOutputType(NewUnionType(NewOutputType(BoolType), NewOutputType(IntType))))
	assert.Equal(t, NewOutputType(NewObjectType(map[string]Type{
		"bool": BoolType,
		"int":  IntType,
	})), NewOutputType(NewObjectType(map[string]Type{
		"bool": NewOutputType(BoolType),
		"int":  NewOutputType(IntType),
	})))
}

func TestPromiseType(t *testing.T) {
	typ := NewPromiseType(DynamicType)

	// Test that creating an promise type with the same element type does not create a new type.
	typ2 := NewPromiseType(typ.ElementType)
	assert.EqualValues(t, typ, typ2)

	// Test that creating an promise type with an element type that is also a promise does not create a new type.
	typ2 = NewPromiseType(typ)
	assert.Equal(t, typ, typ2)

	// Test that a promise type is assignable to and from itself.
	assert.True(t, typ.AssignableFrom(typ))

	// Test that a promise type is assignable from its element type.
	assert.True(t, NewPromiseType(StringType).AssignableFrom(StringType))

	// Test that promise(T) is assignable from U, where U is assignable to T.
	assert.True(t, typ.AssignableFrom(BoolType))

	// Test that promise(T) is assignable from promise(U), where U is assignable to T.
	assert.True(t, typ.AssignableFrom(NewPromiseType(BoolType)))

	// Test that promise(T) is _not_ assignable from U, where U is not assignable to T.
	assert.False(t, NewPromiseType(BoolType).AssignableFrom(IntType))

	// Test that promise(T) is _not_ assignable from promise(U), where U is not assignable to T.
	assert.False(t, NewPromiseType(BoolType).AssignableFrom(NewPromiseType(IntType)))

	// Test that traversing an promise(T) returns an promise(U), where U is the result of the inner traversal.
	typ = NewPromiseType(NewMapType(StringType))
	testTraverse(t, typ, hcl.TraverseAttr{Name: "foo"}, NewPromiseType(StringType), false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.StringVal("foo")}, NewPromiseType(StringType), false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.NumberIntVal(0)}, NewPromiseType(StringType), false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: encapsulateType(IntType)}, NewPromiseType(StringType), false)

	// Test that ResolvePromises correctly handles nested promises.
	assert.Equal(t, NewOptionalType(BoolType), ResolvePromises(NewOptionalType(NewPromiseType(BoolType))))
	assert.Equal(t, BoolType, ResolvePromises(NewPromiseType(BoolType)))
	assert.Equal(t, BoolType, ResolvePromises(NewPromiseType(BoolType)))
	assert.Equal(t, NewMapType(BoolType), ResolvePromises(NewMapType(NewPromiseType(BoolType))))
	assert.Equal(t, NewListType(BoolType), ResolvePromises(NewListType(NewPromiseType(BoolType))))
	assert.Equal(t, NewUnionType(BoolType, IntType),
		ResolvePromises(NewUnionType(NewPromiseType(BoolType), NewPromiseType(IntType))))
	assert.Equal(t, NewObjectType(map[string]Type{
		"bool": BoolType,
		"int":  IntType,
	}), ResolvePromises(NewObjectType(map[string]Type{
		"bool": NewPromiseType(BoolType),
		"int":  NewPromiseType(IntType),
	})))

	// Test that NewPromiseType correctly handles nested promises.
	assert.Equal(t, NewPromiseType(NewOptionalType(BoolType)), NewPromiseType(NewOptionalType(NewPromiseType(BoolType))))
	assert.Equal(t, NewPromiseType(BoolType), NewPromiseType(NewPromiseType(BoolType)))
	assert.Equal(t, NewPromiseType(BoolType), NewPromiseType(NewPromiseType(BoolType)))
	assert.Equal(t, NewPromiseType(NewMapType(BoolType)), NewPromiseType(NewMapType(NewPromiseType(BoolType))))
	assert.Equal(t, NewPromiseType(NewListType(BoolType)), NewPromiseType(NewListType(NewPromiseType(BoolType))))
	assert.Equal(t, NewPromiseType(NewUnionType(BoolType, IntType)),
		NewPromiseType(NewUnionType(NewPromiseType(BoolType), NewPromiseType(IntType))))
	assert.Equal(t, NewPromiseType(NewObjectType(map[string]Type{
		"bool": BoolType,
		"int":  IntType,
	})), NewPromiseType(NewObjectType(map[string]Type{
		"bool": NewPromiseType(BoolType),
		"int":  NewPromiseType(IntType),
	})))
}

func TestMapType(t *testing.T) {
	typ := NewMapType(DynamicType)

	// Test that creating an map type with the same element type does not create a new type.
	typ2 := NewMapType(typ.ElementType)
	assert.EqualValues(t, typ, typ2)

	// Test that a map type is assignable to and from itself.
	assert.True(t, typ.AssignableFrom(typ))

	// Test that map(T) is _not_ assignable from U, where U is not map(T).
	assert.False(t, typ.AssignableFrom(BoolType))

	// Test that map(T) is assignable from map(U), where U is assignable to T.
	assert.True(t, typ.AssignableFrom(NewMapType(BoolType)))

	// Test that map(T) is convertible from object(K_0=U_0, .., K_N=U_N) where unify(U_0, ..., U_N) is assignable to T.
	assert.True(t, typ.ConversionFrom(NewObjectType(map[string]Type{
		"foo": IntType,
		"bar": NumberType,
		"baz": StringType,
	})).Exists())

	// Test that map(T) is _not_ assignable from map(U), where U is not assignable to T.
	assert.False(t, NewMapType(BoolType).AssignableFrom(NewMapType(IntType)))

	// Test that traversing a map(T) with a type that is convertible to string returns T.
	testTraverse(t, typ, hcl.TraverseAttr{Name: "foo"}, typ.ElementType, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.StringVal("foo")}, typ.ElementType, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.NumberIntVal(0)}, typ.ElementType, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: encapsulateType(IntType)}, typ.ElementType, false)

	// Test that traversing a map(T) with a type that is not convertible to string returns DynamicType and an error.
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.ListVal([]cty.Value{cty.NumberIntVal(0)})}, typ.ElementType, true)
	testTraverse(t, typ, hcl.TraverseIndex{Key: encapsulateType(typ)}, typ.ElementType, true)
}

func TestListType(t *testing.T) {
	typ := NewListType(DynamicType)

	// Test that creating an list type with the same element type does not create a new type.
	typ2 := NewListType(typ.ElementType)
	assert.EqualValues(t, typ, typ2)

	// Test that an list type is assignable to and from itself.
	assert.True(t, typ.AssignableFrom(typ))

	// Test that list(T) is _not_ assignable from U, where U is not list(T).
	assert.False(t, typ.AssignableFrom(BoolType))

	// Test that list(T) is assignable from list(U), where U is assignable to T.
	assert.True(t, typ.AssignableFrom(NewListType(BoolType)))

	// Test that list(T) is _not_ assignable from list(U), where U is not assignable to T.
	assert.False(t, NewListType(BoolType).AssignableFrom(NewListType(IntType)))

	// Test that traversing a list(T) with a type that is convertible to number returns T.
	testTraverse(t, typ, hcl.TraverseAttr{Name: "0"}, typ.ElementType, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.StringVal("0")}, typ.ElementType, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.NumberIntVal(0)}, typ.ElementType, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: encapsulateType(IntType)}, typ.ElementType, false)

	// Test that traversing a list(T) with a type that is not convertible to number returns DynamicType and an error.
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.ListVal([]cty.Value{cty.NumberIntVal(0)})}, typ.ElementType, true)
	testTraverse(t, typ, hcl.TraverseIndex{Key: encapsulateType(typ)}, typ.ElementType, true)
}

func TestSetType(t *testing.T) {
	typ := NewSetType(DynamicType)

	// Test that creating an set type with the same element type does not create a new type.
	typ2 := NewSetType(typ.ElementType)
	assert.EqualValues(t, typ, typ2)

	// Test that an set type is assignable to and from itself.
	assert.True(t, typ.AssignableFrom(typ))

	// Test that set(T) is _not_ assignable from U, where U is not set(T).
	assert.False(t, typ.AssignableFrom(BoolType))

	// Test that set(T) is assignable from set(U), where U is assignable to T.
	assert.True(t, typ.AssignableFrom(NewSetType(BoolType)))

	// Test that set(T) is _not_ assignable from set(U), where U is not assignable to T.
	assert.False(t, NewSetType(BoolType).AssignableFrom(NewSetType(IntType)))

	// Test that traversing a set(T) fails.
	testTraverse(t, typ, hcl.TraverseAttr{Name: "0"}, DynamicType, true)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.StringVal("0")}, DynamicType, true)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.NumberIntVal(0)}, DynamicType, true)
	testTraverse(t, typ, hcl.TraverseIndex{Key: encapsulateType(IntType)}, DynamicType, true)
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
	assert.True(t, NewUnionType(NoneType, StringType).ConversionFrom(typ).Exists())

	// Test that union(T_0, ..., T_N) is _not_ assignable from union(U_0, ..., U_M) if union(T_0, ..., T_N) is not
	// assignable from any of U_0 through U_M.
	assert.False(t, typ.AssignableFrom(NewUnionType(BoolType, NewOptionalType(NumberType))))

	// Test that traversing a union type fails if the element type cannot be traversed.
	testTraverse(t, typ, hcl.TraverseAttr{Name: "foo"}, DynamicType, true)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.StringVal("foo")}, DynamicType, true)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.NumberIntVal(0)}, DynamicType, true)
	testTraverse(t, typ, hcl.TraverseIndex{Key: encapsulateType(typ)}, DynamicType, true)

	// Test that traversing a union type succeeds if some element type can be traversed.
	typ = NewUnionType(typ, NewObjectType(map[string]Type{"foo": StringType}), NewListType(StringType)).(*UnionType)
	testTraverse(t, typ, hcl.TraverseAttr{Name: "foo"}, StringType, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.StringVal("foo")}, StringType, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.NumberIntVal(0)}, StringType, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: encapsulateType(StringType)}, StringType, false)

	// Test that traversing a union type produces a union if more than one element can be traversed.
	typ = NewUnionType(NewMapType(IntType), NewObjectType(map[string]Type{"foo": StringType})).(*UnionType)
	testTraverse(t, typ, hcl.TraverseAttr{Name: "foo"}, NewUnionType(StringType, IntType), false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.StringVal("foo")}, NewUnionType(StringType, IntType), false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.NumberIntVal(0)}, IntType, false)
	testTraverse(t, typ, hcl.TraverseIndex{Key: encapsulateType(IntType)}, NewUnionType(StringType, IntType), false)
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
	assert.True(t, typ.ConversionFrom(NewObjectType(map[string]Type{
		"foo": BoolType,
		"bar": IntType,
		"baz": IntType,
		"qux": BoolType,
	})).Exists())
	// Test that object(K_0=T_0, ..., K_N=T_N) is assignable from object(K_0=U_0, ..., K_M=U_M) if M < N and for each
	// key K_i where 0 <= i <= M, T_i is assignable from U_i and for each K_j where M < j <= N, T_j is optional.
	assert.True(t, typ.ConversionFrom(NewObjectType(map[string]Type{
		"foo": BoolType,
		"bar": IntType,
		"baz": NumberType,
	})).Exists())

	// Test that object(K_0=T_0, ..., K_N=T_N) is _unsafely_ convertible from object(L_0=U_0, ..., L_M=U_M) if there exists
	// some key K_i a matching key K_i exists and T_i is unsafely convertible from U_i.
	assert.Equal(t, UnsafeConversion, typ.ConversionFrom(NewObjectType(map[string]Type{
		"foo": BoolType,
		"bar": IntType,
		"baz": NumberType,
		"qux": StringType,
	})))
	assert.Equal(t, UnsafeConversion, typ.ConversionFrom(NewObjectType(map[string]Type{
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

	// Test that traversing an object type with a dynamic value produces the union of the object's property types..
	testTraverse(t, typ, hcl.TraverseIndex{Key: encapsulateType(StringType)}, NewUnionType(BoolType, IntType,
		NumberType, NewOptionalType(BoolType)), false)

	// Test that traversing an object type with any other type fails.
	testTraverse(t, typ, hcl.TraverseIndex{Key: cty.NumberIntVal(0)}, DynamicType, true)
	testTraverse(t, typ, hcl.TraverseIndex{Key: encapsulateType(typ)}, DynamicType, true)
}

func TestOpaqueType(t *testing.T) {
	foo, err := NewOpaqueType("foo")
	assert.NotNil(t, foo)
	assert.NoError(t, err)

	foo2, ok := GetOpaqueType("foo")
	assert.EqualValues(t, foo, foo2)
	assert.True(t, ok)

	foo3, err := NewOpaqueType("foo")
	assert.Nil(t, foo3)
	assert.Error(t, err)

	bar, ok := GetOpaqueType("bar")
	assert.Nil(t, bar)
	assert.False(t, ok)

	bar, err = NewOpaqueType("bar")
	assert.NotNil(t, bar)
	assert.NoError(t, err)

	assert.NotEqual(t, foo, bar)
}

func TestInputType(t *testing.T) {
	// Test that InputType(DynamicType) just returns DynamicType.
	assert.Equal(t, DynamicType, InputType(DynamicType))

	// Test that InputType(T) correctly recurses through constructed types. The result of InputType(T) should be
	// union(innerInputType(T), output(innerInputType(T))), where innerInputType(T) recurses thorough constructed
	// types.

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
		NewListType(NewUnionType(BoolType, NewOutputType(BoolType))),
		NewOutputType(NewListType(BoolType))), InputType(NewListType(BoolType)))

	assert.Equal(t, NewUnionType(
		NewUnionType(BoolType, IntType, NewOutputType(BoolType), NewOutputType(IntType)),
		NewOutputType(NewUnionType(BoolType, IntType))),
		InputType(NewUnionType(BoolType, IntType)))

	assert.Equal(t, NewUnionType(
		NewObjectType(map[string]Type{"foo": NewUnionType(BoolType, NewOutputType(BoolType))}),
		NewOutputType(NewObjectType(map[string]Type{"foo": BoolType}))),
		InputType(NewObjectType(map[string]Type{"foo": BoolType})))

	assert.True(t, InputType(BoolType).ConversionFrom(BoolType).Exists())
	assert.True(t, InputType(NumberType).ConversionFrom(NumberType).Exists())
}

func assertUnified(t *testing.T, expectedSafe, expectedUnsafe Type, types ...Type) {
	actualSafe, actualUnsafe := UnifyTypes(types...)
	assert.Equal(t, expectedSafe, actualSafe)
	assert.Equal(t, expectedUnsafe, actualUnsafe)

	// Reverse the types and ensure we get the same results.
	for i, j := 0, len(types)-1; i < j; i, j = i+1, j-1 {
		types[i], types[j] = types[j], types[i]
	}
	actualSafe2, actualUnsafe2 := UnifyTypes(types...)
	assert.Equal(t, actualSafe, actualSafe2)
	assert.Equal(t, actualUnsafe, actualUnsafe2)
}

func TestUnifyType(t *testing.T) {
	// Number, int, and bool unify with string by preferring string.
	assertUnified(t, StringType, StringType, NumberType, StringType)
	assertUnified(t, StringType, StringType, IntType, StringType)
	assertUnified(t, StringType, StringType, BoolType, StringType)

	// Number and int unify by preferring number.
	assertUnified(t, NumberType, NumberType, IntType, NumberType)

	// Number or int and bool unify by preferring number or int.
	assertUnified(t, NewUnionType(NumberType, BoolType), NumberType, BoolType, NumberType)
	assertUnified(t, NewUnionType(IntType, BoolType), IntType, BoolType, IntType)

	// Two collection types of the same kind unify according to the unification of their element types.
	assertUnified(t, NewMapType(StringType), NewMapType(StringType), NewMapType(BoolType), NewMapType(StringType))
	assertUnified(t, NewListType(StringType), NewListType(StringType), NewListType(BoolType), NewListType(StringType))
	assertUnified(t, NewSetType(StringType), NewSetType(StringType), NewSetType(BoolType), NewSetType(StringType))

	// List and set types unify by preferring the list type.
	assertUnified(t, NewListType(StringType), NewListType(StringType), NewListType(StringType), NewSetType(BoolType))
	assertUnified(t, NewListType(StringType), NewListType(StringType), NewListType(BoolType), NewSetType(StringType))

	assert.True(t, StringType.ConversionFrom(NewOptionalType(NewUnionType(NewMapType(StringType), BoolType))).Exists())

	// Map and object types unify by preferring the map type.
	m0, m1 := NewObjectType(map[string]Type{"foo": StringType}), NewObjectType(map[string]Type{"foo": BoolType})
	assertUnified(t, NewMapType(StringType), NewMapType(StringType), m0, NewMapType(BoolType))
	assertUnified(t, NewMapType(StringType), NewMapType(StringType), m1, NewMapType(StringType))

	// List or set and tuple types unify by preferring the list or set type.
	t0, t1 := NewTupleType(NumberType, BoolType), NewTupleType(StringType, NumberType)
	assertUnified(t, NewListType(StringType), NewListType(StringType), t0, NewListType(StringType))
	assertUnified(t, NewListType(StringType), NewListType(StringType), t1, NewListType(BoolType))
	assertUnified(t, NewUnionType(t0, NewSetType(StringType)), NewSetType(StringType), t0, NewSetType(StringType))
	assertUnified(t, NewUnionType(t1, NewSetType(BoolType)), NewSetType(StringType), t1, NewSetType(BoolType))

	// The dynamic type unifies with any other type by selecting the other type.
	assertUnified(t, NewUnionType(BoolType, DynamicType), BoolType, BoolType, DynamicType)

	// Object types unify by constructing a new object type whose attributes are the unification of the two input types.
	m2 := NewObjectType(map[string]Type{"bar": StringType})
	m3 := NewObjectType(map[string]Type{"foo": NewOptionalType(StringType), "bar": NewOptionalType(StringType)})
	m4 := NewObjectType(map[string]Type{"foo": NewMapType(StringType), "bar": NewListType(StringType)})
	m5 := NewObjectType(map[string]Type{
		"foo": NewOptionalType(NewUnionType(NewMapType(StringType), StringType, NoneType)),
		"bar": NewOptionalType(NewUnionType(NewListType(StringType), StringType, NoneType)),
	})
	assertUnified(t, m0, m0, m0, m1)
	assertUnified(t, m3, m3, m0, m2)
	assertUnified(t, m5, m5, m4, m2, m0, m1)
	assertUnified(t, m5, m5, m4, m0, m2, m1)

	// Tuple types unify by constructing a new tuple type whose element types are the unification of the corresponding
	// element types.
	t2 := NewTupleType(StringType, NumberType)
	t3 := NewTupleType(StringType, IntType)
	t4 := NewTupleType(NumberType, BoolType, StringType)
	t5 := NewTupleType(NumberType, BoolType, NewOptionalType(StringType))
	assertUnified(t, NewUnionType(t0, t1), t2, t0, t1)
	assertUnified(t, t2, t2, t3, t1)
	assertUnified(t, t5, t5, t4, t0)

	//
	//	assertUnified(t, NewUnionType(BoolType, IntType), IntType, BoolType, IntType)
	//	assertUnified(t, NewOptionalType(NumberType), NewOptionalType(NumberType), IntType, NewOptionalType(NumberType))
	//	assertUnified(t, NewOptionalType(BoolType), NewOptionalType(BoolType), BoolType, NewOptionalType(BoolType))
	//	assertUnified(t, NewOutputType(BoolType), NewOutputType(BoolType), BoolType, NewOutputType(BoolType))
	//	assertUnified(t, NewPromiseType(BoolType), NewPromiseType(BoolType), BoolType, NewPromiseType(BoolType))
	//	assertUnified(t, AnyType, AnyType, BoolType, IntType, AnyType)
	//
	//	assertUnified(t, BoolType, BoolType, DynamicType, BoolType)

	//	t0 := Type(NewObjectType(map[string]Type{"foo": IntType}))
	//	t1 := Type(NewObjectType(map[string]Type{"foo": IntType, "bar": NewOptionalType(NumberType)}))
	//
	//	assert.Equal(t, NewMapType(AnyType), unifyTypes(NewMapType(AnyType), t0))
	//	assert.Equal(t, t1, unifyTypes(t0, t1))
	//	assert.Equal(t, t1, unifyTypes(t1, t0))
	//
	//	t0 = NewOutputType(NumberType)
	//	t1 = NewOutputType(NewUnionType(NumberType, IntType))
	//	assert.Equal(t, t0, unifyTypes(t0, t1))
	//	assert.Equal(t, t0, unifyTypes(t1, t0))
}

func TestRecursiveObjectType(t *testing.T) {
	props := map[string]Type{
		"data": NewOutputType(IntType),
	}
	linkedListType := NewOptionalType(NewObjectType(props))
	props["next"] = linkedListType

	propsOther := map[string]Type{
		"data": NewOutputType(IntType),
	}
	linkedListTypeOther := NewOptionalType(NewObjectType(propsOther))
	propsOther["next"] = linkedListTypeOther

	// Equals
	assert.True(t, linkedListType.Equals(linkedListTypeOther))

	// Contains eventuals
	hasOutputs, hasPromises := ContainsEventuals(linkedListType)
	assert.True(t, hasOutputs)
	assert.False(t, hasPromises)

	// Resolving eventuals
	resolvedLinkedListType := ResolveOutputs(linkedListType)
	data := resolvedLinkedListType.(*UnionType).ElementTypes[1].(*ObjectType).Properties["data"]
	assert.True(t, data.Equals(IntType))
	hasOutputs, _ = ContainsEventuals(resolvedLinkedListType)
	assert.False(t, hasOutputs)

	// InputType conversion
	inputLinkedListType := InputType(resolvedLinkedListType)
	hasOutputs, _ = ContainsEventuals(inputLinkedListType)
	assert.True(t, hasOutputs)
}
