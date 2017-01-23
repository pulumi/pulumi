// Copyright 2016 Marapongo, Inc. All rights reserved.

package tokens

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestTypeToken(nm string) Type {
	pkg := NewPackageToken("test/package")
	mod := NewModuleToken(pkg, "test/module")
	return NewTypeToken(mod, TypeName(nm))
}

func TestArrayTypes(t *testing.T) {
	// Test simple primitives.
	for _, prim := range []string{"any", "bool", "string", "number"} {
		ptr := NewArrayTypeToken(Type(prim))
		assert.True(t, ptr.Array(), "Expected array type token to be an array")
		parsed := ParseArrayType(ptr)
		assert.Equal(t, prim, string(parsed.Elem))
	}

	// Test more complex array type elements.
	class := newTestTypeToken("ArrayTest")
	ptr := NewArrayTypeToken(class)
	assert.True(t, ptr.Array(), "Expected array type token to be an array")
	parsed := ParseArrayType(ptr)
	assert.Equal(t, string(class), string(parsed.Elem))
}

func TestPointerTypes(t *testing.T) {
	// Test simple primitives.
	for _, prim := range []string{"any", "bool", "string", "number"} {
		ptr := NewPointerTypeToken(Type(prim))
		assert.True(t, ptr.Pointer(), "Expected pointer type token to be a pointer")
		parsed := ParsePointerType(ptr)
		assert.Equal(t, prim, string(parsed.Elem))
	}

	// Test more complex pointer type elements.
	class := newTestTypeToken("PointerTest")
	ptr := NewPointerTypeToken(class)
	assert.True(t, ptr.Pointer(), "Expected pointer type token to be a pointer")
	parsed := ParsePointerType(ptr)
	assert.Equal(t, string(class), string(parsed.Elem))
}

func TestMapTypes(t *testing.T) {
	// Test simple primitives.
	for _, key := range []string{"string", "bool", "number"} {
		for _, elem := range []string{"any", "bool", "string", "number"} {
			ptr := NewMapTypeToken(Type(key), Type(elem))
			assert.True(t, ptr.Map(), "Expected map type token to be a map")
			parsed := ParseMapType(ptr)
			assert.Equal(t, key, string(parsed.Key))
			assert.Equal(t, elem, string(parsed.Elem))
		}
	}

	// Test more complex map type elements.
	for _, key := range []string{"string", "bool", "number"} {
		class := newTestTypeToken("MapTest")
		ptr := NewMapTypeToken(Type(key), class)
		assert.True(t, ptr.Map(), "Expected map type token to be a map")
		parsed := ParseMapType(ptr)
		assert.Equal(t, key, string(parsed.Key))
		assert.Equal(t, string(class), string(parsed.Elem))
	}
}
